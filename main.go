package main

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/UBotPlatform/UBot.Account.Tomon/tomon"
	ubot "github.com/UBotPlatform/UBot.Common.Go"
)

var event *ubot.AccountEventEmitter
var bot *tomon.Bot

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
			builder.WriteEntity(ubot.MsgEntity{
				Type:      "file",
				Args:      []string{attachment.URL},
				NamedArgs: map[string]string{"filename": attachment.Filename},
			})
		} else {
			builder.WriteEntity(ubot.MsgEntity{
				Type: "image",
				Args: []string{attachment.URL},
			})
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
func login(loginInfo tomon.LoginInfo) error {
	var err error
	bot, err = tomon.New(loginInfo)
	if err != nil {
		return err
	}
	bot.Event.OnClose = func(err error) {
		if err != nil {
			panic(fmt.Errorf("the connection is closed unexpectedly: %w", err))
		}
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
func sendChatMessage(msgType ubot.MsgType, source string, target string, message string) error {
	entities := ubot.ParseMsg(message)
	var builder strings.Builder
	for _, entity := range entities {
		switch entity.Type {
		case "text":
			builder.WriteString(entity.FirstArgOrEmpty())
		case "at":
			builder.WriteString(fmt.Sprintf("<@%s>", entity.FirstArgOrEmpty()))
		default:
			builder.WriteString("[不支持的消息]")
		case "image":
			if builder.Len() != 0 {
				_, _ = bot.CreateMessage(source, builder.String())
				builder.Reset()
			}
			var imageReader io.Reader
			var imageExt string
			imageBase64, useBase64 := entity.NamedArgs["base64"]
			if useBase64 {
				imageBinary, err := base64.StdEncoding.DecodeString(imageBase64)
				if err != nil {
					break
				}
				imageExt = guessImageExtByBytes(imageBinary, ".png")
				imageReader = bytes.NewReader(imageBinary)
			} else {
				resp, err := http.Get(entity.FirstArgOrEmpty())
				if err != nil {
					break
				}
				imageExt = guessImageExtByMIMEType(resp.Header.Get("Content-Type"), ".png")
				imageReader = resp.Body
			}
			_, _ = bot.CreateAttachmentMessage(source, []tomon.ReaderWithName{{
				Reader: imageReader,
				Name:   fmt.Sprintf("image-%d%s", time.Now().UnixNano(), imageExt),
			}})
			if imageReadCloser, canClose := imageReader.(io.ReadCloser); canClose {
				imageReadCloser.Close()
			}
		case "file":
			if builder.Len() != 0 {
				_, _ = bot.CreateMessage(source, builder.String())
			}
			builder.Reset()
			fileName := entity.NamedArgOr("filename", fmt.Sprintf("untitled-file-%d", time.Now().UnixNano()))
			url := entity.FirstArgOrEmpty()
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

func getPlatformID() (string, error) {
	return "Tomon", nil
}

func getGroupList() ([]string, error) {
	var r []string
	channels := bot.Channels()
	for _, channel := range channels {
		if channel.Type == 0 {
			r = append(r, channel.ID)
		}
	}
	return r, nil
}

func getMemberList(id string) ([]string, error) {
	var r []string
	channel, err := bot.Channel(id)
	if err != nil {
		return nil, err
	}
	members := bot.Members(channel.GuildID)
	for _, member := range members {
		r = append(r, member.User.ID)
	}
	return r, nil
}

func main() {
	var err error
	var loginInfo tomon.LoginInfo
	switch strings.ToLower(os.Args[3]) {
	case "account":
		loginInfo = &tomon.LoginByPassword{FullName: os.Args[4], Password: os.Args[5]}
	case "token":
		loginInfo = &tomon.LoginByToken{Token: os.Args[4]}
	default:
		loginInfo = &tomon.LoginByToken{Token: os.Args[3]}
	}
	err = login(loginInfo)
	if err != nil {
		fmt.Println("Failed to login to tomon:", err)
		os.Exit(111)
	}
	err = ubot.HostAccount("Tomon Bot", func(e *ubot.AccountEventEmitter) *ubot.Account {
		event = e
		return &ubot.Account{
			GetGroupName:    getGroupName,
			GetUserName:     getUserName,
			SendChatMessage: sendChatMessage,
			RemoveMember:    removeMember,
			ShutupMember:    shutupMember,
			ShutupAllMember: shutupAllMember,
			GetMemberName:   getMemberName,
			GetUserAvatar:   getUserAvatar,
			GetSelfID:       getSelfID,
			GetPlatformID:   getPlatformID,
			GetGroupList:    getGroupList,
			GetMemberList:   getMemberList,
		}
	})
	ubot.AssertNoError(err)
	_ = bot.Close()
}
