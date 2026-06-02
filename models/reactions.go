package models

import (
	"time"
)

type Reaction struct {
	ID        string    `json:"id"`
	MessageID string    `json:"message_id"`
	UserID    uint      `json:"user_id"`
	Username  string    `json:"username,omitempty"`
	Emoji     string    `json:"emoji"` // The emoji string (e.g., "heart", "fire")
	CreatedAt time.Time `json:"created_at"`
}

type ReactionUpdate struct {
	RoomID   string   `json:"room_id"`
	Reaction Reaction `json:"reaction"`
}
