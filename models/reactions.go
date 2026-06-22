package models

import (
	"time"
)

type Reaction struct {
	ID        string    `json:"id"`
	MessageID string    `json:"message_id"`
	UserID    uint      `json:"user_id"`
	Username  string    `json:"username,omitempty"`
	Emoji     string    `json:"emoji"`
	Content   string    `json:"content,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}
