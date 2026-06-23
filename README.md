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

#### 1. Login/Register
`POST /api/auth/login`

Request body:
```json
{
  "username": "john_doe",
  "password": "secretpassword"
}
```
NB: If this is your first time logging in the system would first register you into the systems database
then allows you to be able to enter the program.

**cURL Example:**
```bash
curl.exe -X POST {{baseURL}}/api/auth/login 
  -H "Content-Type: application/json" 
  -d `"{\"username\":\"john_doe\",\"password\":\"secretpassword\"}"`
```

Response: User object with ID and auth status. If user doesn't exist, auto-registers.

---

#### 2. Create a Room
`POST /api/rooms`

Request body:
```json
{
  "name": "General",
  "creator_id": "00", // when you login successfully your user_id would be visible from the response given to you.
  "room_type": "private" // either private or public
}
```

**cURL Example:**
```bash
curl.exe -X POST {{baseURL}}/api/rooms 
  -H "Content-Type: application/json" 
  -d "{\"name\":\"General\",\"creator_id\":\"00\",\"room_type\":\"private\"}"
```

Response: Created room object with ID, name, description, creator_id, status, type, and created_at.

---

#### 3. List All Rooms
`GET /api/rooms`

**cURL Example:**
```bash
curl.exe -X GET {{baseURL}}/api/rooms
```

Response: List of active and archived rooms (excludes hidden rooms).

---

#### 4. Get Room Messages
`GET /api/rooms/{id}/messages`

**cURL Example:**
```bash
curl.exe -X GET {{baseURL}}/api/rooms/general/messages
```

Response: Room object and last 50 messages from the room.

---

#### 5. Update Room Status (Admin Only)
`PATCH /api/rooms/{id}/status`

Request body:
```json
{
  "status": "archived",
  "user_id": "1",
  "is_admin": true
}
```

**Available statuses:** `active`, `archived`, `hidden`

**cURL Example:**
```bash
curl.exe -X PATCH {{baseURL}}/api/rooms/general/status ^
  -H "Content-Type: application/json" ^
  -d "{\"status\":\"archived\",\"user_id\":\"1\",\"is_admin\":true}"
```

**Security:** Requires `is_admin: true`. If not an admin, returns `403 Forbidden` with error:
```json
{
  "error": "Unauthorized. Admin privileges required."
}
```

**Effect:** 
- Updates database
- Syncs in-memory hub status immediately
- All connected clients are blocked from sending messages or reactions
- Clients receive error: `"This room has been archived. You cannot send messages."`

---

#### 6. Delete a Room (Admin Only)
`DELETE /api/rooms/{id}`

Request body:
```json
{
  "user_id": "1"
}
```

**cURL Example:**
```bash
curl.exe -X DELETE {{baseURL}}/api/rooms/general ^
  -H "Content-Type: application/json" ^
  -d "{\"room_id\":\"general\",\"user_id\":\"1\",\"is_admin\":true}"
```

**Security:** Requires `is_admin: true`. If not an admin, returns `403 Forbidden` with error:
```json
{
  "error": "Unauthorized. Admin privileges required."
}
```

**Effect (Atomic Operation):**
1. **Database**: Deletes room record and all associated messages/reactions (cascading delete)
2. **Memory**: Removes room from active `Hub.rooms` map → prevents new socket connections
3. **Clients**: All connected clients receive:
   ```json
   {
     "type": "system",
     "content": "This chat room has been deleted by the admin."
   }
   ```
   Then connection is cleanly closed with:
   - `close(client.Send)` — breaks WritePump goroutine
   - `client.conn.Close()` — severs TCP socket

Response: Success confirmation
```json
{
  "status": "success",
  "message": "Room general and all its chat history were permanently deleted."
}
```

---

#### 7. Health Check
`GET /health`

**cURL Example:**
```bash
curl.exe -X GET {{baseURL}}/health
```

Response:
```json
{
  "status": "oooohhhhhhh yyyhhhhhh"
}
```

---

### WebSocket Connection & Real-Time Chat Flow

**Step 1: Authenticate User** 

**Step 2: Connect to WebSocket** (Browser Console or JavaScript Client)

Open your browser's developer console and execute:
```javascript

const userId = "1";
const username = "alice";
const roomId = "general";

const ws = new WebSocket(`ws://localhost:8080/ws?roomID=${roomId}&userID=${userId}&username=${username}`);

// Handle incoming messages
ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  console.log("Received:", msg);
  
  // Handles different message types(its optional)
  switch (msg.type) {
    case "message":
      console.log(`${msg.username}: ${msg.content}`);
      break;
    case "user_list":
      console.log("Active users:", msg.users);
      break;
    case "join":
      console.log(`${msg.username} joined the room`);
      break;
    case "leave":
      console.log(`${msg.username} left the room`);
      break;
    case "reaction":
      console.log(`${msg.username} reacted ${msg.emoji} to a message`);
      break;
    case "system":
      console.log(`SYSTEM: ${msg.content}`);
      break;
    case "error":
      console.error(`ERROR: ${msg.content}`);
      break;
  }
};

// Handle connection open
ws.onopen = () => {
  console.log("Connected to room!");
};

// Handle errors
ws.onerror = (error) => {
  console.error("WebSocket error:", error);
};

// Handle connection close
ws.onclose = () => {
  console.log("Disconnected from room");
};
```

**Step 3: Send a Message**

```javascript
// Send a chat message
ws.send(JSON.stringify({
  type: "message",
  content: "Hello everyone!"
}));
```

**Step 4: Add a Reaction**

```javascript
// React to a message with an emoji
ws.send(JSON.stringify({
  type: "reaction",
  message_id: "abc123def456",  // this should be copied from the response when you write or read a message 
  emoji: "👍"                   
}));
```

**Supported Emojis:** all emoji's

**Step 5: Handling Room Status Changes**

If an admin updates the room status to `archived`:

```javascript
// Server broadcasts system message
{
  "type": "system",
  "msg_type": "status_change",
  "content": "Room status changed to: archived",
  "room_id": "general"
}

// Now, attempting to send a message returns an error:
ws.send(JSON.stringify({
  type: "message",
  content: "This won't work"
}));

// Response (error):
{
  "type": "error",
  "content": "This room has been archived. You cannot send messages."
}
```

**Step 6: Room Deletion**

If an admin deletes the room:

```javascript
// Server broadcasts final system message to all clients
{
  "type": "system",
  "content": "This chat room has been deleted by the admin."
}

// Connection closes cleanly (browser receives close frame)
// ws.onclose fires, cleanup happens
```

---

### Security & Admin Features

#### Admin-Only Operations

1. **Update Room Status**
   - Only users with `is_admin: true` can change room status
   - Non-admins receive: `403 Forbidden` with `"Unauthorized. Admin privileges required."`
   - Changes are atomic: database + in-memory hub sync immediately

2. **Delete Room**
   - Only users with `is_admin: true` can delete rooms
   - Non-admins receive: `403 Forbidden` with `"Unauthorized. Admin privileges required."`
   - Deletion is atomic:
     - Database records deleted (cascading)
     - Room removed from memory → new connections blocked
     - Existing clients notified and disconnected cleanly

---
### Starting the Server

```bash
go run main.go
```
The server starts on `{{baseURL}}`