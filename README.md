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


### API Endpoints

#### Login 
`POST /api/auth/login`

Request body:
```json
{
  "username": "username"
  "password": "passswordd"
}
```
Response: users logs in directly. if not its registers the user into the database

#### Create a room
`POST /api/rooms`

Request body:
```json
{
  "name": "General",
}
```

Response: the created room object.

#### List all rooms
`GET /api/rooms`

Response: list of active and archived rooms.

#### Delete a room
`DELETE /api/rooms/{id}`

Request body:
```json
{
  "user_id": "1",
  "is_admin": false
}
```

Response: confirmation message.

#### Health check
`GET /health`

Response:
```json
{
  "status": "oooohhhhhhh yyyhhhhhh"
}
```

### Starting the server
```bash
go run main.go
```
### WebSocket connection flow
1. Log in via `/api/auth/login`.
2. Connect using the secure WebSocket endpoint
```bash
3. const ws = new WebSocket("ws://localhost:8080/ws?roomID=___&userID=__&username=___");
4. ws2.onmessage = (e) => { 
const msg = JSON.parse(e.data); 
console.log("User1 recieved", msg);
}
5. ws2.send(JSON.stringify({type: "message", content: "Hi there, from User1"}));
```