package middleware

import (
	"log"
	"net/http"
	"strings"
)

// AuthMiddleware validates that username and room_id are provided as query parameters
func AuthMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract username and room_id from query parameters
		username := strings.TrimSpace(r.URL.Query().Get("username"))
		roomID := strings.TrimSpace(r.URL.Query().Get("room_id"))

		// Validate username and room_id
		if username == "" {
			log.Printf("Missing username parameter")
			http.Error(w, "Missing username parameter", http.StatusBadRequest)
			return
		}

		if roomID == "" {
			log.Printf("Missing room_id parameter")
			http.Error(w, "Missing room id parameter", http.StatusBadRequest)
			return
		}

		if len(username) >= 50 {
			log.Printf("Username too long: %s", username)
			http.Error(w, "Username too long (max 50 characters)", http.StatusBadRequest)
			return
		}

		if len(roomID) > 30 {
			log.Printf("Room ID too long: %s", roomID)
			http.Error(w, "Invalid room ID", http.StatusBadRequest)
			return
		}

		log.Printf("User %s attempting to join room %s", username, roomID)

		// Call the next handler
		next(w, r)
	}
}
