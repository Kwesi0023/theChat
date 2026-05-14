package models

import "time"

// Room represents a chat room
type Room struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatorID   uint      `json:"creator_id"` // ID of room creator (numeric)
	Status      string    `json:"status"`     // 'active', 'archived', 'hidden'
	Type        string    `json:"type"`       // 'public', 'private'
	CreatedAt   time.Time `json:"created_at"`
}
