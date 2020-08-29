package tomon

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const BaseURL = "https://beta.tomon.co/api/v1"
const GatewayURL = "wss://gateway.tomon.co"

func fullURL(endpoint string) string {
	return BaseURL + endpoint
}

type Bot struct {
	token             string
	self              UserInfo
	gateway           *websocket.Conn
	lastPong          time.Time
	heartbeatInterval time.Duration
	closed            bool
	mux               sync.Mutex
	state             struct {
		Guilds          map[string]GuildInfo             //[GuildID]
		Channels        map[string]ChannelInfo           //[ChannelID]
		Members         map[string]map[string]MemberInfo //[GuildID][MemberID]
		ChannelsInGuild map[string]map[string]int
	}
	Event struct {
		OnGuildCreate       func(info *GuildInfo)
		OnGuildDelete       func(info *GuildInfo)
		OnGuildUpdate       func(info *GuildInfo)
		OnChannelCreate     func(info *ChannelInfo)
		OnChannelDelete     func(info *ChannelInfo)
		OnChannelUpdate     func(info *ChannelInfo)
		OnGuildMemberAdd    func(info *MemberInfo)
		OnGuildMemberRemove func(info *MemberInfo)
		OnGuildMemberUpdate func(info *MemberInfo)
		OnMessageCreate     func(info *MessageInfo)
		OnMessageDelete     func(info *MessageInfo)
		OnMessageUpdate     func(info *MessageInfo)
	}
}

func (bot *Bot) RawREST(method string, endpoint string, contentType string, content io.Reader, response interface{}) error {
	var err error
	req, err := http.NewRequest(method, fullURL(endpoint), content)
	if err != nil {
		return err
	}
	req.Header.Add("Content-Type", contentType)
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", bot.token))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 && resp.StatusCode != 204 {
		return fmt.Errorf("failed to send REST request: %s", resp.Status)
	}
	if response != nil {
		err = json.NewDecoder(resp.Body).Decode(&response)
		if err != nil {
			return err
		}
	}
	return nil
}

func (bot *Bot) REST(method string, endpoint string, request interface{}, response interface{}) error {
	var requestBody []byte
	var err error
	if request != nil {
		requestBody, err = json.Marshal(request)
		if err != nil {
			return err
		}
	} else {
		requestBody = make([]byte, 0)
	}
	return bot.RawREST(method, endpoint, "application/json", bytes.NewReader(requestBody), response)
}
func New(payload LoginInfo) (*Bot, error) {
	var err error
	var result loginResult
	buf := payload.Body()
	resp, err := http.Post(fullURL("/auth/login"), "application/json", bytes.NewBuffer(buf))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to login: %s", resp.Status)
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return nil, err
	}
	var bot = &Bot{
		token:    result.Token,
		self:     result.UserInfo,
		lastPong: time.Now(),
	}
	bot.resetState()
	go bot.connectToGateway()
	return bot, nil
}

func (bot *Bot) connectToGateway() {
	if bot.closed {
		return
	}
	for retryIndex := 0; retryIndex < 5; retryIndex++ {
		success := func() bool {
			defer func() {
				if err := recover(); err != nil {
					log.Println("An error occurred:", err)
				}
			}()
			gateway, _, err := websocket.DefaultDialer.Dial(GatewayURL, nil)
			if err != nil {
				return false
			}
			bot.gateway = gateway
			err = bot.gatewayIdentity()
			if err != nil {
				return false
			}
			return bot.receiveNotification()
		}()
		if success {
			retryIndex = 0
		}
		if bot.closed {
			break
		}
		time.Sleep(3 * time.Second)
	}
}

func (bot *Bot) receiveNotification() bool {
	success := false
	for {
		var n gatewayNotification
		_, content, err := bot.gateway.ReadMessage()
		if err != nil {
			break
		}
		err = json.Unmarshal(content, &n)
		if err != nil {
			log.Println("invaild notification:", err)
			continue
		}
		switch n.Op {
		case 0: //DISPATCH
			switch n.E {
			case "GUILD_CREATE":
				var data GuildInfo
				err = json.Unmarshal(n.D, &data)
				if err != nil {
					log.Println("invaild GUILD_CREATE notification:", err, string(n.D))
					break
				}
				bot.state.Guilds[data.ID] = data
				if bot.Event.OnGuildCreate != nil {
					bot.Event.OnGuildCreate(&data)
				}
			case "GUILD_UPDATE":
				var data GuildInfo
				err = json.Unmarshal(n.D, &data)
				if err != nil {
					log.Println("invaild GUILD_UPDATE notification:", err, string(n.D))
					break
				}
				bot.state.Guilds[data.ID] = data
				if bot.Event.OnGuildUpdate != nil {
					bot.Event.OnGuildUpdate(&data)
				}
			case "GUILD_DELETE":
				var data GuildInfo
				err = json.Unmarshal(n.D, &data)
				if err != nil {
					log.Println("invaild GUILD_DELETE notification:", err, string(n.D))
					break
				}
				if bot.Event.OnGuildDelete != nil {
					bot.Event.OnGuildDelete(&data)
				}
				delete(bot.state.Guilds, data.ID)
				delete(bot.state.ChannelsInGuild, data.ID)
			case "CHANNEL_CREATE":
				var data ChannelInfo
				err = json.Unmarshal(n.D, &data)
				if err != nil {
					log.Println("invaild CHANNEL_CREATE notification:", err, string(n.D))
					break
				}
				bot.state.Channels[data.ID] = data
				cpg, ok := bot.state.ChannelsInGuild[data.GuildID]
				if !ok {
					cpg = make(map[string]int)
					bot.state.ChannelsInGuild[data.GuildID] = cpg
				}
				cpg[data.ID] = 0
				if bot.Event.OnChannelCreate != nil {
					bot.Event.OnChannelCreate(&data)
				}
			case "CHANNEL_UPDATE":
				var data ChannelInfo
				err = json.Unmarshal(n.D, &data)
				if err != nil {
					log.Println("invaild CHANNEL_UPDATE notification:", err, string(n.D))
					break
				}
				bot.state.Channels[data.ID] = data
				if bot.Event.OnChannelUpdate != nil {
					bot.Event.OnChannelUpdate(&data)
				}
			case "CHANNEL_DELETE":
				var data ChannelInfo
				err = json.Unmarshal(n.D, &data)
				if err != nil {
					log.Println("invaild CHANNEL_DELETE notification:", err, string(n.D))
					break
				}
				if bot.Event.OnChannelDelete != nil {
					bot.Event.OnChannelDelete(&data)
				}
				delete(bot.state.Channels, data.ID)
				cpg, ok := bot.state.ChannelsInGuild[data.GuildID]
				if ok {
					delete(cpg, data.ID)
				}
			case "GUILD_MEMBER_ADD":
				var data MemberInfo
				err = json.Unmarshal(n.D, &data)
				if err != nil {
					log.Println("invaild GUILD_MEMBER_ADD notification:", err, string(n.D))
					break
				}
				bot.Members(data.GuildID)[data.User.ID] = data
				if bot.Event.OnGuildMemberAdd != nil {
					bot.Event.OnGuildMemberAdd(&data)
				}
			case "GUILD_MEMBER_UPDATE":
				var data MemberInfo
				err = json.Unmarshal(n.D, &data)
				if err != nil {
					log.Println("invaild GUILD_MEMBER_UPDATE notification:", err, string(n.D))
					break
				}
				bot.Members(data.GuildID)[data.User.ID] = data
				if bot.Event.OnGuildMemberUpdate != nil {
					bot.Event.OnGuildMemberUpdate(&data)
				}
			case "GUILD_MEMBER_REMOVE":
				var data MemberInfo
				err = json.Unmarshal(n.D, &data)
				if err != nil {
					log.Println("invaild GUILD_MEMBER_REMOVE notification:", err, string(n.D))
					break
				}
				if bot.Event.OnGuildMemberRemove != nil {
					bot.Event.OnGuildMemberRemove(&data)
				}
				delete(bot.Members(data.GuildID), data.User.ID)

			case "MESSAGE_CREATE":
				var data MessageInfo
				err = json.Unmarshal(n.D, &data)
				if err != nil {
					log.Println("invaild MESSAGE_CREATE notification:", err, string(n.D))
					break
				}
				if bot.Event.OnMessageCreate != nil {
					bot.Event.OnMessageCreate(&data)
				}
			case "MESSAGE_UPDATE":
				var data MessageInfo
				err = json.Unmarshal(n.D, &data)
				if err != nil {
					log.Println("invaild MESSAGE_UPDATE notification:", err, string(n.D))
					break
				}
				if bot.Event.OnMessageUpdate != nil {
					bot.Event.OnMessageUpdate(&data)
				}
			case "MESSAGE_DELETE":
				var data MessageInfo
				err = json.Unmarshal(n.D, &data)
				if err != nil {
					log.Println("invaild MESSAGE_DELETE notification:", err, string(n.D))
					break
				}
				if bot.Event.OnMessageDelete != nil {
					bot.Event.OnMessageDelete(&data)
				}
			}
		case 1: //HEARTBEAT
			_ = bot.gatewayPong()
		case 2: //IDENTITY
			var data identityNotification
			err = json.Unmarshal(n.D, &data)
			if err != nil {
				log.Println("invaild identity notification:", err)
				break
			}
			success = true
			bot.resetState()
			for _, dmChannel := range data.DMChannels {
				bot.state.Channels[dmChannel.ID] = dmChannel
			}
			for _, guild := range data.Guilds {
				bot.state.Guilds[guild.ID] = guild.GuildInfo
				for _, channel := range guild.Channels {
					bot.state.Channels[channel.ID] = channel
				}
				memberSubMap := bot.Members(guild.ID)
				for _, member := range guild.Members {
					memberSubMap[member.User.ID] = member
				}
			}
		case 3: //HELLO
			var data helloNotification
			err = json.Unmarshal(n.D, &data)
			if err != nil {
				log.Println("invaild hello notification:", err)
				log.Println("fallback: set heartbeat interval to 10 seconds")
				bot.heartbeatInterval = 10 * time.Second
			} else {
				bot.heartbeatInterval = time.Duration(data.HeartbeatInterval) * time.Millisecond
			}
			go bot.heartbeatLoop()
		case 4: //HEARTBEAT_ACK
			bot.lastPong = time.Now()
		case 5: //VOICE_STATE_UPDATE
		default:
			log.Println("unkown notification, op:", n.Op)
		}
	}
	return success
}
func (bot *Bot) User(userID string) (*UserInfo, error) {
	for _, channel := range bot.state.Channels {
		for _, recipient := range channel.Recipients {
			if recipient.ID == userID {
				return &recipient, nil
			}
		}
		r, ok := bot.Members(channel.ID)[userID]
		if ok {
			return &r.User, nil
		}
	}
	return nil, errors.New("failed to get the user info, please check if it is reachable")
}
func (bot *Bot) Channel(channelID string) (*ChannelInfo, error) {
	sr, ok := bot.state.Channels[channelID]
	if ok {
		return &sr, nil
	}
	var r ChannelInfo
	err := bot.REST("GET", fmt.Sprintf("/channels/%s", channelID), nil, &r)
	if err != nil {
		return nil, err
	}
	return &r, nil
}
func (bot *Bot) Channels() map[string]ChannelInfo {
	return bot.state.Channels
}
func (bot *Bot) ChannelsInGuild(guildID string) (map[string]int, error) {
	sr, ok := bot.state.ChannelsInGuild[guildID]
	if ok {
		return sr, nil
	}
	var rr []ChannelInfo
	err := bot.REST("GET", fmt.Sprintf("/guilds/%s/channels", guildID), nil, &rr)
	if err != nil {
		return nil, err
	}
	r := make(map[string]int)
	for _, channel := range rr {
		r[channel.ID] = 0
	}
	return r, nil
}
func (bot *Bot) Members(guildID string) map[string]MemberInfo {
	bot.mux.Lock()
	defer bot.mux.Unlock()
	r, ok := bot.state.Members[guildID]
	if !ok {
		r = make(map[string]MemberInfo)
		bot.state.Members[guildID] = r
	}
	return r
}
func (bot *Bot) Member(guildID string, userID string) (*MemberInfo, error) {
	ms, ok := bot.state.Members[guildID]
	if ok {
		r, ok := ms[userID]
		if ok {
			return &r, nil
		}
	}
	var r MemberInfo
	err := bot.REST("GET", fmt.Sprintf("/guilds/%s/members/%s", guildID, userID), nil, &r)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (bot *Bot) RemoveMember(guildID string, userID string) error {
	err := bot.REST("DELETE", fmt.Sprintf("/guilds/%s/members/%s", guildID, userID), nil, nil)
	return err
}

func (bot *Bot) CreateMessage(channelID string, content string) (*MessageInfo, error) {
	var payload sendMessagePayload
	var r MessageInfo
	payload.Content = content
	payload.Nonce = fmt.Sprint(time.Now().UnixNano())
	err := bot.REST("POST", fmt.Sprintf("/channels/%s/messages", channelID), payload, &r)
	if err != nil {
		return nil, err
	}
	return &r, nil
}
func (bot *Bot) CreateAttachmentMessage(channelID string, files []ReaderWithName) (*MessageInfo, error) {
	var payload sendMessagePayload
	var r MessageInfo
	payload.Nonce = fmt.Sprint(time.Now().UnixNano())
	payloadBody, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	payloadWriter, err := writer.CreateFormField("payload_json")
	if err != nil {
		return nil, err
	}
	_, err = payloadWriter.Write(payloadBody)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		fileWriter, err := writer.CreateFormFile("files", file.Name)
		if err != nil {
			return nil, err
		}
		_, err = io.Copy(fileWriter, file.Reader)
		if err != nil {
			return nil, err
		}
	}
	err = writer.Close()
	if err != nil {
		return nil, err
	}
	err = bot.RawREST("POST", fmt.Sprintf("/channels/%s/messages", channelID), writer.FormDataContentType(), body, &r)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (bot *Bot) heartbeatLoop() {
	for {
		err := bot.gatewayPing()
		if err != nil {
			_ = bot.gateway.Close()
			break
		}
		time.Sleep(bot.heartbeatInterval / 2)
		if time.Since(bot.lastPong) > bot.heartbeatInterval {
			bot.mux.Lock()
			_ = bot.gateway.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseTryAgainLater, ""))
			bot.mux.Unlock()
			_ = bot.gateway.Close()
			break
		}
	}
}

func (bot *Bot) gatewayIdentity() error {
	bot.mux.Lock()
	defer bot.mux.Unlock()
	var request gatewayIdentityRequest
	request.Op = 2
	request.D.Token = bot.token
	return bot.gateway.WriteJSON(request)
}

func (bot *Bot) gatewayPing() error {
	bot.mux.Lock()
	defer bot.mux.Unlock()
	return bot.gateway.WriteMessage(websocket.TextMessage, []byte(`{"op":1}`))
}

func (bot *Bot) gatewayPong() error {
	bot.mux.Lock()
	defer bot.mux.Unlock()
	return bot.gateway.WriteMessage(websocket.TextMessage, []byte(`{"op":4}`))
}

func (bot *Bot) Self() *UserInfo {
	return &bot.self
}

func (bot *Bot) Close() error {
	bot.mux.Lock()
	defer bot.mux.Unlock()
	bot.closed = true
	bot.resetState()
	_ = bot.gateway.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
	_ = bot.gateway.Close()
	return nil
}

func (bot *Bot) resetState() {
	bot.state.Guilds = make(map[string]GuildInfo)
	bot.state.Channels = make(map[string]ChannelInfo)
	bot.state.Members = make(map[string]map[string]MemberInfo)
	bot.state.ChannelsInGuild = make(map[string]map[string]int)
}
