# theChat
Build a simple real time room based backend where users can join with a username, enter a room, and 
interact live through messages and reactions. Keep the REST API minimal, just enough for creating rooms
and fetching message history, and use WebSockets for real time interactions like sending messages,
broadcasting reactions, and tracking when users join or leave. Messages should be stored in a database,
while active users in each room can be managed in memory. The focus is not on complexity but on getting
the fundamentals right, clean API design, sensible data modeling, and a solid understanding of event
driven systems and the WebSocket lifecycle.