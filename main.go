package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/Kwesi0023/theChat/database"
	"github.com/Kwesi0023/theChat/handlers"
	_ "github.com/Kwesi0023/theChat/websocket"
	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
)

const (
	serverPort = ":8080"
)

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: No .env file found, relying on system environment variables")
	}
	// Initialize database
	dsn := os.Getenv("DB_DSN")
	if err := database.InitDB(dsn); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer database.CloseDB()

	// Create tables
	if err := database.CreateTables(); err != nil {
		log.Fatalf("Failed to create tables: %v", err)
	}

	// Initialize handlers (including WebSocket hub)
	handlers.Initialize()

	// Set up router
	router := mux.NewRouter()

	// REST API endpoints
	router.HandleFunc("/api/auth/register", handlers.Register).Methods("POST")
	router.HandleFunc("/api/auth/login", handlers.Login).Methods("POST")
	router.HandleFunc("/api/rooms", handlers.CreateRoom).Methods("POST")
	router.HandleFunc("/api/rooms", handlers.GetAllRooms).Methods("GET")
	router.HandleFunc("/api/rooms/{id}/messages", handlers.GetRoomMessages).Methods("GET")
	router.HandleFunc("/api/rooms/{id}/status", func(w http.ResponseWriter, r *http.Request) {
		handlers.UpdateRoomStatus(handlers.Hub, w, r)
	}).Methods("PATCH")
	router.HandleFunc("/api/rooms/{id}", func(w http.ResponseWriter, r *http.Request) {
		handlers.DeleteRoom(handlers.Hub, w, r)
	}).Methods("DELETE")

	// Health check
	router.HandleFunc("/health", handlers.HealthCheck).Methods("GET")

	// WebSocket endpoint with JWT validation
	router.HandleFunc("/ws", handlers.ServeWs).Methods("GET")

	// Setup graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		log.Printf("Starting server on port %s", serverPort)
		if err := http.ListenAndServe(serverPort, router); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sig := <-sigChan
	fmt.Printf("\nReceived signal: %v\n", sig)
	log.Println("Shutting down server...")
	log.Println("Server has been shut down")
}
