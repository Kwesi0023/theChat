package models

import (
	"time"
)

// Reaction represents a user's emotional response to a specific message.
// This struct is used for both Database storage and JSON communication.
type Reaction struct {
	ID        string    `json:"id"`         // Primary Key in DB
	MessageID uint      `json:"message_id"` // Foreign Key to messages table
	UserID    uint      `json:"user_id"`    // The student who reacted
	Emoji     string    `json:"emoji"`      // The emoji string (e.g., "heart", "fire")
	CreatedAt time.Time `json:"created_at"` // When the reaction happened

	// Optional: Include the username so the frontend can show
	// "Kofi and 2 others reacted" without a second API call.
	Username string `json:"username,omitempty"`
}

// ReactionUpdate is a small helper struct used specifically for
// broadcasting a new reaction to everyone in a RoomHub.
type ReactionUpdate struct {
	RoomID   string   `json:"room_id"`
	Reaction Reaction `json:"reaction"`
}
