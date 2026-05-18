# theChat - Real-Time Room-Based Chat Backend

Build a simple real time room based backend where users can join with a username, enter a room, and 
interact live through messages. Keep the REST API minimal, just enough for creating rooms
and fetching message history, and use WebSockets for real time interactions like sending messages,
broadcasting reactions, and tracking when users join or leave. Messages should be stored in a database,
while active users in each room can be managed in memory. The focus is not on complexity but on getting
the fundamentals right, clean API design, sensible data modeling, and a solid understanding of event
driven systems and the WebSocket lifecycle.

##  Features

- **Real-Time Messaging** - Messages broadcast instantly to all users in a room via WebSocket
- **Room Management** - Create and manage chat rooms via REST API
- **User Presence** - Track active users, join/leave notifications
- **Message Persistence** - All messages stored in MySQL
- **Event-Driven Architecture** - Hub pattern with goroutines and channels
- **Clean API Design** - RESTful endpoints, WebSocket protocol
- **Proper Lifecycle** - Graceful WebSocket connection handling
- **Production-Ready Code** - Error handling, connection pooling, proper cleanup


### Test Endpoints

```bash
# Create room
curl -X POST http://localhost:8080/api/rooms \
  -H "Content-Type: application/json" \
  -d '{"name": "General"}'

# Get all rooms
curl http://localhost:8080/api/rooms

# Health check
curl http://localhost:8080/health

# Connect WebSocket (use Postman)
ws://localhost:8080/ws?username=Alice&room_id=<room_id>
```
