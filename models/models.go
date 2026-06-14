package models

import "time"

type Message struct {
	ID        string      `json:"id"`
	RoomID    string      `json:"room_id"`
	SenderID  uint        `json:"sender_id"`
	Content   string      `json:"content"`
	MsgType   string      `json:"msg_type"` // 'message', 'join', 'leave'
	Timestamp time.Time   `json:"timestamp"`
	CreatedAt time.Time   `json:"created_at"`
	Reactions []*Reaction `json:"reactions"`
}

type User struct {
	ID            uint      `json:"id"`
	Username      string    `json:"username"`
	RoomID        string    `json:"room_id,omitempty"`
	Password_hash string    `json:"-"`
	JoinedAt      time.Time `json:"joined_at,omitempty"`
	IsAdmin       bool      `json:"is_admin"`
}
