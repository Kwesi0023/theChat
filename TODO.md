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
- [ ] Implement message repository (save, fetch by room, delete)
- [ ] Implement user repository (create, fetch, delete)
- [ ] Set up connection pooling and transaction handling

## REST API Endpoints
- [X] POST /api/rooms - create a room
- [X] GET /api/rooms/{id}/messages - fetch message history for a room
- [X] Basic error handling and HTTP status codes
- [X] Input validation for API requests

## WebSocket Server Foundation
- [X] Set up WebSocket upgrade handler
- [ ] Implement connection lifecycle (connect, disconnect, reconnect)
- [X] Design message protocol/frame format (JSON event structure)
- [ ] Create connection manager to track active clients

## WebSocket Events - User Management
- [ ] Implement join event (user joins room)
- [ ] Implement leave event (user leaves room)
- [ ] Broadcast user list to room on join/leave
- [ ] Handle disconnection cleanup (mark user as left)

## WebSocket Events - Messaging
- [ ] Implement send message event
- [ ] Save message to database
- [ ] Broadcast message to all users in room
- [ ] Timestamp and message ID assignment

## WebSocket Events - Reactions
- [ ] Implement add reaction event
- [ ] Update message reactions in database
- [ ] Broadcast reaction update to room
- [ ] Support multiple reactions per message

## In-Memory User Tracking
- [ ] Create room manager (track active rooms and users)
- [ ] Implement user list per room (in-memory store)
- [ ] Update user presence on join/leave
- [ ] Clean up empty rooms

## Testing & Validation
- [ ] Write unit tests for data models
- [ ] Write tests for repository layer
- [ ] Test REST API endpoints
- [ ] Test WebSocket connection lifecycle
- [ ] Test concurrent user interactions
- [ ] Test message persistence and retrieval

## Documentation & Polish
- [ ] Write API documentation
- [ ] Document WebSocket event protocol
- [X] Add code comments for complex logic
- [X] Add graceful shutdown handling
- [ ] Review event-driven architecture patterns
