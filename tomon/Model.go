package tomon

import (
	"encoding/json"
	"io"
	"time"
)

type LoginInfo interface {
	Body() []byte
}
type LoginByPassword struct {
	FullName string `json:"full_name"`
	Password string `json:"password"`
}

func (info *LoginByPassword) Body() []byte {
	body, err := json.Marshal(info)
	if err != nil {
		panic(err)
	}
	return body
}

type LoginByToken struct {
	Token string `json:"token"`
}

func (info *LoginByToken) Body() []byte {
	body, err := json.Marshal(info)
	if err != nil {
		panic(err)
	}
	return body
}

type loginResult struct {
	Token string `json:"token"`
	SelfInfo
}

type UserInfo struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	Avatar        string `json:"avatar"`
	Name          string `json:"name"`
	AvatarURL     string `json:"avatar_url"`
	Type          int    `json:"type"`
}
type SelfInfo struct {
	UserInfo
	Email         *string `json:"email"`
	EmailVerified bool    `json:"email_verified"`
	Phone         *string `json:"phone"`
	PhoneVerified bool    `json:"phone_verified"`
	Banned        bool    `json:"banned"`
}
type GuildInfo struct {
	ID                 string     `json:"id"`
	Name               string     `json:"name"`
	Icon               string     `json:"icon"`
	IconURL            string     `json:"icon_url"`
	JoinedAt           *time.Time `json:"joined_at"`
	Background         string     `json:"background"`
	BackgroundProps    string     `json:"background_props"`
	BackgroundURL      string     `json:"background_url"`
	OwnerID            string     `json:"owner_id"`
	Position           int        `json:"position"`
	SystemChannelFlags int        `json:"system_channel_flags"`
	SystemChannelID    string     `json:"system_channel_id"`
}
type Overwrite struct {
	ID    string `json:"id"`
	Type  string `json:"type"`
	Allow uint64 `json:"allow,omitempty"`
	Deny  uint64 `json:"deny,omitempty"`
}
type ChannelInfo struct {
	DefaultMessageNotifications int         `json:"default_message_notifications"`
	GuildID                     string      `json:"guild_id,omitempty"`
	ID                          string      `json:"id"`
	LastMessageID               string      `json:"last_message_id,omitempty"`
	Name                        string      `json:"name,omitempty"`
	ParentID                    string      `json:"parent_id,omitempty"`
	PermissionOverwrites        []Overwrite `json:"permission_overwrites,omitempty"`
	Position                    int         `json:"position,omitempty"`
	Recipients                  []UserInfo  `json:"recipients,omitempty"`
	Topic                       string      `json:"topic,omitempty"`
	Type                        int         `json:"type"`
}
type RoleInfo struct {
	Color       int    `json:"color"`
	GuildID     string `json:"guild_id"`
	Hoist       bool   `json:"hoist"`
	ID          string `json:"id"`
	Mentionable bool   `json:"mentionable"`
	Name        string `json:"name"`
	Permissions uint64 `json:"permissions"`
	Position    int    `json:"position"`
}
type MemberInfo struct {
	Deaf     bool       `json:"deaf"`
	GuildID  string     `json:"guild_id"`
	JoinedAt *time.Time `json:"joined_at"`
	Mute     bool       `json:"mute"`
	Nick     *string    `json:"nick,omitempty"`
	Roles    []string   `json:"roles,omitempty"`
	User     UserInfo   `json:"user"`
}

type gatewayIdentityRequest struct {
	Op int `json:"op"`
	D  struct {
		Token string `json:"token"`
	} `json:"d"`
}

type gatewayNotification struct {
	Op int             `json:"op"`
	D  json.RawMessage `json:"d,omitempty"`
	E  string          `json:"e,omitempty"`
}

type helloNotification struct {
	HeartbeatInterval int64  `json:"heartbeat_interval"`
	SessionID         string `json:"session_id"`
}
type identityNotification struct {
	DMChannels []ChannelInfo `json:"dm_channels,omitempty"`
	Guilds     []struct {
		GuildInfo
		Channels []ChannelInfo `json:"channels"`
		Members  []MemberInfo  `json:"members"`
	} `json:"guilds"`
}
type sendMessagePayload struct {
	Content string `json:"content"`
	Nonce   string `json:"nonce"`
}

type MessageInfo struct {
	ID              string           `json:"id"`
	ChannelID       *string          `json:"channel_id,omitempty"`
	Author          *UserInfo        `json:"author"`
	Type            int              `json:"type"`
	Content         *string          `json:"content"`
	Timestamp       string           `json:"timestamp"`
	Nonce           string           `json:"nonce"`
	Attachments     []AttachmentInfo `json:"attachments"`
	Reactions       []ReactionInfo   `json:"reactions"`
	Mentions        []UserInfo       `json:"mentions"`
	Stamps          []StampsInfo     `json:"stamps"`
	Reply           *MessageInfo     `json:"reply,omitempty"`
	Pinned          bool             `json:"pinned"`
	EditedTimestamp *string          `json:"edited_timestamp"`
	Member          *MemberInfo      `json:"member,omitempty"`
}
type AttachmentInfo struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Hash     string `json:"hash"`
	Type     string `json:"type"`
	Size     int    `json:"size"`
	Height   int    `json:"height,omitempty"`
	Width    int    `json:"width,omitempty"`
	URL      string `json:"url"`
}
type ReactionInfo struct {
	Emoji struct {
		ID   *string `json:"id,omitempty"`
		Name *string `json:"name,omitempty"`
	} `json:"emoji"`
	Count int  `json:"count"`
	Me    bool `json:"me"`
}
type StampsInfo struct {
	ID        string     `json:"id"`
	Alias     string     `json:"alias"`
	AuthorID  string     `json:"author_id"`
	PackID    string     `json:"pack_id"`
	Position  int        `json:"position"`
	Hash      string     `json:"hash"`
	Animated  bool       `json:"animated"`
	URL       string     `json:"url"`
	Width     int        `json:"width"`
	Height    int        `json:"height"`
	UpdatedAt *time.Time `json:"updated_at"`
}
type ReaderWithName struct {
	Reader io.Reader
	Name   string
}
