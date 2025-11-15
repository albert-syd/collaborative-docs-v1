# Collaborative Document Editor

A real-time collaborative document editor built with Go and WebSockets. Multiple users can edit documents simultaneously with conflict-free concurrent editing using Operational Transformation.

## Features

- **Real-time collaboration** - See changes from other users instantly
- **Multiple documents** - Each document has isolated content and users
- **Operational Transformation** - Conflict-free concurrent editing
- **WebSocket-based** - Efficient bidirectional communication
- **Automatic reconnection** - Handles connection drops gracefully
- **Thread-safe** - Safe concurrent access to shared state

## Quick Start

### Run the Server

```bash
go run cmd/server/main.go
```

The server starts at `http://localhost:8080`

### Try It Out

1. Open `http://localhost:8080/doc/test-doc` in your browser
2. Start typing
3. Open the same URL in another browser tab
4. Watch both editors sync in real-time!

**Try different documents:**
- `http://localhost:8080/doc/my-notes`
- `http://localhost:8080/doc/team-doc`
- `http://localhost:8080/doc/project-ideas`

Each document is completely independent with its own content and user count.

### Run Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Verbose output
go test -v ./...
```

## Project Structure

```
collaborative-docs-v1/
├── cmd/
│   └── server/
│       └── main.go              # Server entry point (36 lines)
├── internal/
│   ├── server/                  # HTTP server & WebSocket handlers
│   │   ├── server.go
│   │   ├── handlers.go
│   │   ├── handlers_test.go
│   │   ├── integration_test.go
│   │   └── testutil/
│   ├── hub/                     # WebSocket connection manager
│   │   ├── hub.go
│   │   ├── client.go
│   │   ├── message.go
│   │   └── hub_test.go
│   ├── document/                # Document state management
│   │   ├── document.go
│   │   └── document_test.go
│   └── operations/              # Operational Transformation
│       ├── operation.go
│       ├── transform.go
│       ├── apply.go
│       └── ot_test.go
└── web/
    └── static/
        └── index.html           # Web UI
```

## How It Works

### Architecture

The application uses a **hub-and-spoke** pattern:

```
         ┌─────────┐
         │   Hub   │  ← Manages all WebSocket connections
         └────┬────┘
              │
      ┌───────┼───────┐
      │       │       │
   Client  Client  Client
      │       │       │
   Browser Browser Browser
```

### Request Flow

1. **User visits** `/doc/{documentID}`
2. **WebSocket connects** to `/ws/{documentID}`
3. **Client registers** with the Hub for that document
4. **User types** → Frontend sends operation
5. **Hub receives** → Applies to document → Broadcasts to all clients on same document
6. **Clients update** → Apply operation locally

### Key Components

**Server** (`internal/server/`)
- Handles HTTP requests and WebSocket upgrades
- Routes requests to specific documents
- Validates document IDs

**Hub** (`internal/hub/`)
- Manages WebSocket connections per document
- Broadcasts messages to clients editing the same document
- Tracks active user counts

**Document** (`internal/document/`)
- Thread-safe document state
- Applies operations and tracks versions
- Handles concurrent access

**Operations** (`internal/operations/`)
- Operational Transformation algorithms
- Transforms concurrent operations for conflict resolution
- Insert, delete, and retain operations

## Configuration

Server configuration in `cmd/server/main.go`:

```go
srv := server.New(server.Config{
    Port:       ":8080",
    StaticDir:  "static",
    LogEnabled: true,
})
```

## Testing

The project includes comprehensive tests:

- **Unit tests** - Test individual components in isolation
- **Integration tests** - Test WebSocket communication end-to-end
- **Table-driven tests** - Systematic test coverage

### Run All Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test -v ./...

# Run with coverage
go test -cover ./...
```

### Run Specific Test Types

```bash
# Run only unit tests for a specific package
go test ./internal/operations/
go test ./internal/hub/
go test ./internal/document/

# Run only integration tests
go test ./internal/server/ -run Integration

# Run specific integration test
go test ./internal/server/ -run TestWebSocketServer
go test ./internal/server/ -run TestMultipleClients

# Run benchmarks
go test -bench=. ./internal/document/
```

### Test Coverage

```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...

# View coverage in browser
go tool cover -html=coverage.out
```

All tests follow Go best practices with proper cleanup and error handling.

## Document IDs

Document IDs must:
- Contain only alphanumeric characters, hyphens, and underscores
- Be 1-100 characters long
- Be unique per document

Examples: `my-notes`, `team_doc`, `Project123`

## Production Considerations

For production deployment:

1. **Update CORS settings** - Restrict `CheckOrigin` in `handlers.go`
2. **Add persistence** - Currently documents are in-memory only
3. **Add authentication** - No user authentication currently
4. **Configure timeouts** - Review WebSocket timeout settings
5. **Add monitoring** - Implement metrics and logging

## Code Quality & Improvements

### Bug Fixes Applied (2025-11-15)

This section documents critical bugs that were discovered and fixed during code review and testing.

#### 1. Race Condition in Hub Broadcast Functions
**Issue:** Writing to `h.clients` map while holding only RLock instead of Lock
**Location:** `internal/hub/hub.go:207-210, 226-229` (broadcastToAll, broadcastToDocument)
**Impact:** Data race that could cause crashes or undefined behavior under high load
**Root Cause:**
```go
// BEFORE (BUG):
h.mu.RLock()  // Read lock acquired
for client := range h.clients {
    select {
    case client.send <- message:
    default:
        delete(h.clients, client)  // ❌ Writing with read lock!
    }
}
h.mu.RUnlock()
```
**Fix Applied:** Use Unregister channel (actor model pattern) instead of direct map manipulation
```go
// AFTER (FIXED):
h.mu.RLock()
for client := range h.clients {
    select {
    case client.send <- message:
    default:
        go h.Unregister(client)  // ✅ Delegate to hub goroutine
    }
}
h.mu.RUnlock()
```

#### 2. Goroutine Leak - Missing Hub Shutdown
**Issue:** Hub.Run() goroutine had no way to stop gracefully
**Location:** `internal/hub/hub.go`, `internal/server/server.go`
**Impact:** Goroutine leak on server shutdown, preventing clean restarts
**Fix Applied:**
- Added `quit chan struct{}` to Hub struct
- Implemented `Shutdown()` method to signal hub to stop
- Updated `Run()` to handle quit signal and close all clients
- Modified `Server.Shutdown()` to call `hub.Shutdown()` before HTTP shutdown

#### 3. WebSocket Origin Security Vulnerability
**Issue:** CheckOrigin always returned true, allowing connections from any origin
**Location:** `internal/server/handlers.go:16-19`
**Impact:** CSRF vulnerability - malicious sites could connect to WebSocket
**Root Cause:**
```go
// BEFORE (VULNERABLE):
CheckOrigin: func(r *http.Request) bool {
    return true  // ❌ Accepts all origins!
}
```
**Fix Applied:** Implemented proper origin validation with allowlist
```go
// AFTER (FIXED):
CheckOrigin: checkOrigin,

func checkOrigin(r *http.Request) bool {
    origin := r.Header.Get("Origin")
    if origin == "" {
        return true  // Same-origin or non-browser
    }
    // Allowlist for development
    allowedOrigins := []string{
        "http://localhost:8080",
        "http://127.0.0.1:8080",
    }
    for _, allowed := range allowedOrigins {
        if origin == allowed {
            return true
        }
    }
    log.Printf("rejected connection from unauthorized origin: %s", origin)
    return false
}
```

#### 4. Operation Echo Bug - Sender Receives Own Operations
**Issue:** Operations broadcast to ALL clients including the sender, causing duplicate application
**Location:** `internal/hub/hub.go`, `internal/hub/client.go`
**Impact:** User types "hello" but sees "hheelllloo" - each character doubled
**Root Cause:**
1. User types → browser applies change locally
2. Frontend sends operation to server
3. Server broadcasts to ALL clients (including sender)
4. Sender receives and applies their own operation again

**Fix Applied:** Modified broadcast system to exclude sender
- Added `broadcastMessage` struct to pair messages with sender
- Updated `Broadcast()` to accept sender parameter
- Modified `broadcastToDocument()` and `broadcastToAll()` to skip sender
- Changed `client.ReadPump()` to pass client reference: `h.Broadcast(message, c)`

```go
// Key changes:
type broadcastMessage struct {
    message []byte
    sender  *Client  // Track who sent it
}

func (h *Hub) broadcastToDocument(documentID string, message []byte, exclude *Client) {
    for client := range h.clients {
        if client.documentID == documentID {
            if exclude != nil && client == exclude {
                continue  // ✅ Skip sender
            }
            client.send <- message
        }
    }
}
```

**Testing:** All fixes verified with:
- Unit tests pass ✅
- Integration tests pass ✅
- Race detector clean (`go test -race ./...`) ✅
- Manual testing with multiple browser instances ✅

### Known Issues & Future Improvements

**High Priority:**
- Unbounded `client.send` channel can cause memory issues with slow clients - recommend buffered channel with size limit
- Missing context propagation in ReadPump/WritePump goroutines - makes graceful shutdown difficult
- Potential goroutine leak if one client goroutine panics - recommend using errgroup pattern

**Medium Priority:**
- Error handling in message serialization (`hub.go:106`) silently ignores errors
- No WebSocket ping/pong health checks - dead connections not detected until write fails
- Magic numbers for timeouts should be configurable constants
- Documents never deleted from memory - need TTL or LRU eviction policy

**Low Priority:**
- Test coverage gaps for edge cases and error paths
- Standard log package lacks structured logging and log levels
- Configuration values not validated on startup
- No metrics for monitoring (active connections, operation throughput, etc.)

**Test Status:**
- All tests passing ✅
- Race detector clean ✅

## Dependencies

```
Go 1.21+
github.com/gorilla/websocket v1.5.3
```

Install dependencies:
```bash
go mod download
```

## Troubleshooting

**Port already in use:**
```bash
# Find process using port 8080
lsof -i :8080

# Kill the process
kill -9 <PID>
```

**WebSocket connection fails:**
- Ensure server is running
- Check browser console for errors
- Verify the document ID is valid (alphanumeric, `-`, `_` only)

**Tests failing:**
```bash
# Clear test cache and re-run
go clean -testcache
go test ./...
```

## Future Enhancements

Potential improvements:
- Document persistence (database or file storage)
- User authentication and identification
- Cursor position tracking for other users
- Document list and management UI
- Rich text editing support
- Document permissions and access control

## License

MIT License - See LICENSE file for details

---

Built with Go and WebSockets for real-time collaboration.
