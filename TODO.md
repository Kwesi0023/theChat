# Real-Time Room-Based Backend - TODO

## Project Setup
- [X] Initialize Go module and project structure
- [ ] Set up main.go with server initialization
- [ ] Configure database (PostgreSQL/SQLite) connection
- [ ] Set up logging and error handling

## Data Models
- [ ] Define Room struct (ID, name, created_at)
- [ ] Define Message struct (ID, room_id, user_id, content, reactions, timestamp)
- [ ] Define User struct (ID, username, room_id, joined_at)
- [ ] Design database schema and create migrations

## Database Layer
- [ ] Implement room repository (create, fetch by ID, list all)
- [ ] Implement message repository (save, fetch by room, delete)
- [ ] Implement user repository (create, fetch, delete)
- [ ] Set up connection pooling and transaction handling

## REST API Endpoints
- [ ] POST /api/rooms - create a room
- [ ] GET /api/rooms/{id}/messages - fetch message history for a room
- [ ] Basic error handling and HTTP status codes
- [ ] Input validation for API requests

## WebSocket Server Foundation
- [ ] Set up WebSocket upgrade handler
- [ ] Implement connection lifecycle (connect, disconnect, reconnect)
- [ ] Design message protocol/frame format (JSON event structure)
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
- [ ] Add code comments for complex logic
- [ ] Add graceful shutdown handling
- [ ] Review event-driven architecture patterns
