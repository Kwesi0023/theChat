package models

import "time"

//  message sent through WebSocket
type WebSocketMessage struct {
	Type      string     `json:"type"` // "message", "join", "leave", "user_list", "reaction", "history", "system", "offline"
	Content   string     `json:"content,omitempty"`
	Username  string     `json:"username,omitempty"`
	UserID    uint       `json:"user_id,omitempty"`
	RoomID    string     `json:"room_id,omitempty"`
	MsgType   string     `json:"msg_type,omitempty"` // for system messages
	Users     []User     `json:"users,omitempty"`
	Messages  []Message  `json:"messages,omitempty"` // for history
	Reactions []Reaction `json:"reactions,omitempty"`
	MessageID uint       `json:"message_id,omitempty"` // for reactions
	Emoji     string     `json:"emoji,omitempty"`      // for reactions "fire", "heart"
	Timestamp time.Time  `json:"timestamp,omitempty"`
}
