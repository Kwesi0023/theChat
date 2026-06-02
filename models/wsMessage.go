package models

import "time"

// WebSocketMessage represents a message sent through WebSocket
type WebSocketMessage struct {
	Type      string      `json:"type"` // "message", "join", "leave", "user_list", "reaction", "history", "system", "offline"
	Content   string      `json:"content,omitempty"`
	Username  string      `json:"username,omitempty"`
	UserID    uint        `json:"user_id,omitempty"`
	RoomID    string      `json:"room_id,omitempty"`
	MsgType   string      `json:"msg_type,omitempty"` // for system messages
	Users     []User      `json:"users,omitempty"`
	Messages  []*Message  `json:"messages,omitempty"` // for history
	Reactions []*Reaction `json:"reactions,omitempty"`
	MessageID string      `json:"message_id,omitempty"` // you'll need this to react to the message
	Emoji     string      `json:"emoji,omitempty"`      // 👍 👎 🥳 😊 😂 😱 🤪 😡 😭
	Timestamp time.Time   `json:"timestamp,omitempty"`
}
