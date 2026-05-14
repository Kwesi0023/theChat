package models

import "time"

//  chat room
type Room struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	CreatedBy   uint      `json:"created_by"` // ID of room creator	Status      string    `json:"status"` // 'active', 'archived', 'hidden'
	Type        string    `json:"type"`       // 'public', 'private'
	Status      string    `json:"status"`     // 'active', 'archived', 'hidden'
}
