# Real-Time Room-Based Backend - TODO

## Project Setup
- [X] Initialize Go module and project structure
- [X] Set up main.go with server initialization
- [X] Configure database (PostgreSQL/SQLite) connection
- [X] Set up logging and error handling

## Data Models
- [X] Define Room struct (ID, name, created_at)
- [X] Define Message struct (ID, room_id, user_id, content, reactions, timestamp)
- [X] Define User struct (ID, username, room_id, joined_at)
- [X] Design database schema

## Database Layer
- [X] Implement room repository (create, fetch by ID, list all)
- [X] Implement message repository (save, fetch by room, delete)
- [ ] Implement user repository (create, fetch, delete)
- [ ] Set up connection pooling and transaction handling

## REST API Endpoints
- [X] POST /api/rooms - create a room
- [X] GET /api/rooms/{id}/messages - fetch message history for a room
- [X] Basic error handling and HTTP status codes
- [X] Input validation for API requests

## WebSocket Server Foundation
- [X] Set up WebSocket upgrade handler
- [X] Implement connection lifecycle (connect, disconnect, reconnect)
- [X] Design message protocol/frame format (JSON event structure)
- [ ] Create connection manager to track active clients

## WebSocket Events - User Management
- [X] Implement join event (user joins room)
- [X] Implement leave event (user leaves room)
- [X] Broadcast user list to room on join/leave
- [X] Handle disconnection cleanup (mark user as left)

## WebSocket Events - Messaging
- [X] Implement send message event
- [X] Save message to database
- [X] Broadcast message to all users in room
- [X] Timestamp and message ID assignment

## WebSocket Events - Reactions
- [X] Implement add reaction event
- [X] Update message reactions in database
- [X] Broadcast reaction update to room
- [X] Support multiple reactions per message

## In-Memory User Tracking
- [ ] Create room manager (track active rooms and users)
- [ ] Implement user list per room (in-memory store)
- [X] Update user presence on join/leave
- [ ] Clean up empty rooms

## Testing & Validation
- [X] Write unit tests for data models
- [X] Write tests for repository layer
- [X] Test REST API endpoints
- [X] Test WebSocket connection lifecycle
- [X] Test concurrent user interactions
- [X] Test message persistence and retrieval

## Documentation & Polish
- [ ] Write API documentation
- [ ] Document WebSocket event protocol
- [X] Add code comments for complex logic
- [X] Add graceful shutdown handling
- [ ] Review event-driven architecture patterns