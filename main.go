package main

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/UBotPlatform/UBot.Account.Tomon/tomon"
	ubot "github.com/UBotPlatform/UBot.Common.Go"
)

var event *ubot.AccountEventEmitter
var bot *tomon.Bot
var loginInfo tomon.LoginInfo

func getGroupName(id string) (string, error) {
	info, err := bot.Channel(id)
	if err != nil {
		return "", err
	}
	return info.Name, nil
}
func getUserName(id string) (string, error) {
	info, err := bot.User(id)
	if err != nil {
		return "", err
	}
	return info.Name, nil
}
func toUBotMessage(msg *tomon.MessageInfo) string {
	//dbgBytes, _ := json.MarshalIndent(msg, "", "    ")
	//fmt.Println(string(dbgBytes))

	var builder ubot.MsgBuilder
	for _, attachment := range msg.Attachments {
		if attachment.Height|attachment.Width == 0 {
			builder.WriteEntity(ubot.MsgEntity{Type: "file_online", Data: attachment.Filename + "|" + attachment.URL})
		} else {
			builder.WriteEntity(ubot.MsgEntity{Type: "image_online", Data: attachment.URL})
		}
	}
	if msg.Content != nil {
		builder.WriteString(*msg.Content)
	}

	r := builder.String()
	for _, at := range msg.Mentions {
		r = strings.ReplaceAll(r, fmt.Sprintf("<@%s>", at.ID), fmt.Sprintf("[at:%s]", at.ID))
	}

	//fmt.Println(r)
	return r
}
func login() error {
	var err error
	bot, err = tomon.New(loginInfo)
	if err != nil {
		return err
	}
	bot.Event.OnGuildMemberAdd = func(member *tomon.MemberInfo) {
		channels, err := bot.ChannelsInGuild(member.GuildID)
		if err != nil {
			return
		}
		for channelID := range channels {
			_ = event.OnMemberJoined(channelID, member.User.ID, "")
		}
	}
	bot.Event.OnGuildMemberRemove = func(member *tomon.MemberInfo) {
		channels, err := bot.ChannelsInGuild(member.GuildID)
		if err != nil {
			return
		}
		for channelID := range channels {
			_ = event.OnMemberLeft(channelID, member.User.ID)
		}
	}
	bot.Event.OnMessageCreate = func(msg *tomon.MessageInfo) {
		if msg.Author == nil {
			return
		}
		if msg.Author.ID == bot.Self().ID {
			return
		}
		ubotMsg := toUBotMessage(msg)
		if ubotMsg == "" {
			return
		}
		info := ubot.MsgInfo{
			ID: msg.ID,
		}
		if msg.ChannelID == nil || *msg.ChannelID == "" || *msg.ChannelID == "0" {
			_ = event.OnReceiveChatMessage(ubot.PrivateMsg, "", msg.Author.ID, ubotMsg, info)
		} else {
			_ = event.OnReceiveChatMessage(ubot.GroupMsg, *msg.ChannelID, msg.Author.ID, ubotMsg, info)
		}
	}
	return err
}
func logout() error {
	localBot := bot
	bot = nil
	return localBot.Close()
}
func sendChatMessage(msgType ubot.MsgType, source string, target string, message string) error {
	entities := ubot.ParseMsg(message)
	var builder strings.Builder
	for _, entity := range entities {
		switch entity.Type {
		case "text":
			builder.WriteString(entity.Data)
		case "at":
			builder.WriteString(fmt.Sprintf("<@%s>", entity.Data))
		default:
			builder.WriteString("[不支持的消息]")
		case "image_online":
			if builder.Len() != 0 {
				_, _ = bot.CreateMessage(source, builder.String())
			}
			builder.Reset()
			resp, err := http.Get(entity.Data)
			if err != nil {
				break
			}
			defer resp.Body.Close()
			_, _ = bot.CreateAttachmentMessage(source, []tomon.ReaderWithName{{
				Reader: resp.Body,
				Name:   fmt.Sprintf("image-%d.png", time.Now().UnixNano()),
			}})
		case "file_online":
			if builder.Len() != 0 {
				_, _ = bot.CreateMessage(source, builder.String())
			}
			builder.Reset()
			var fileName string
			var url string
			pSeq := strings.IndexByte(entity.Data, '|')
			if pSeq == -1 {
				fileName = fmt.Sprintf("file-%d", time.Now().UnixNano())
				url = entity.Data
			} else {
				fileName = entity.Data[:pSeq]
				url = entity.Data[pSeq+1:]
			}
			resp, err := http.Get(url)
			if err != nil {
				break
			}
			defer resp.Body.Close()
			_, _ = bot.CreateAttachmentMessage(source, []tomon.ReaderWithName{{
				Reader: resp.Body,
				Name:   fileName,
			}})
		}
	}
	if builder.Len() != 0 {
		_, _ = bot.CreateMessage(source, builder.String())
	}
	return nil
}

func removeMember(source string, target string) error {
	info, err := bot.Channel(source)
	if err != nil {
		return err
	}
	return bot.RemoveMember(info.GuildID, target)
}

func shutupMember(source string, target string, duration int) error {
	return errors.New("not supported")
}
func shutupAllMember(source string, shutupSwitch bool) error {
	return errors.New("not supported")
}

func getMemberName(source string, target string) (string, error) {
	info, err := bot.Member(source, target)
	if err != nil {
		return "", err
	}
	if info.Nick == nil || *info.Nick == "" {
		return info.User.Name, nil
	}
	return *info.Nick, nil
}

func getUserAvatar(id string) (string, error) {
	info, err := bot.User(id)
	if err != nil {
		return "", err
	}
	return info.AvatarURL, nil
}

func getSelfID() (string, error) {
	return bot.Self().ID, nil
}

func main() {
	var err error
	switch strings.ToLower(os.Args[3]) {
	case "account":
		loginInfo = &tomon.LoginByPassword{FullName: os.Args[4], Password: os.Args[5]}
	case "token":
		loginInfo = &tomon.LoginByToken{Token: os.Args[4]}
	}
	go login() //nolint:errcgeck
	err = ubot.HostAccount("Tomon Bot", func(e *ubot.AccountEventEmitter) *ubot.Account {
		event = e
		return &ubot.Account{
			GetGroupName:    getGroupName,
			GetUserName:     getUserName,
			Login:           login,
			Logout:          logout,
			SendChatMessage: sendChatMessage,
			RemoveMember:    removeMember,
			ShutupMember:    shutupMember,
			ShutupAllMember: shutupAllMember,
			GetMemberName:   getMemberName,
			GetUserAvatar:   getUserAvatar,
			GetSelfID:       getSelfID,
		}
	})
	ubot.AssertNoError(err)
	_ = logout()
}
