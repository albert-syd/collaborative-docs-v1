package hub

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// TestNewHub verifies that NewHub creates a properly initialized hub.
func TestNewHub(t *testing.T) {
	h := NewHub()

	if h == nil {
		t.Fatal("NewHub() returned nil")
	}

	// Verify all fields are initialized
	if h.clients == nil {
		t.Error("clients map not initialized")
	}
	if h.broadcast == nil {
		t.Error("broadcast channel not initialized")
	}
	if h.register == nil {
		t.Error("register channel not initialized")
	}
	if h.unregister == nil {
		t.Error("unregister channel not initialized")
	}
}

// TestClientRegistration verifies basic client registration.
func TestClientRegistration(t *testing.T) {
	h := NewHub()
	go h.Run()

	// Create a mock WebSocket connection
	server := httptest.NewServer(nil)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http")
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Skip("skipping test that requires WebSocket server")
	}
	defer conn.Close()

	client := NewClient(h, conn, "test-doc")
	h.Register(client)

	// Allow hub to process registration
	time.Sleep(50 * time.Millisecond)

	// Verify client is registered
	h.mu.RLock()
	_, exists := h.clients[client]
	h.mu.RUnlock()

	if !exists {
		t.Error("client not registered in hub")
	}
}

// TestClientUnregistration verifies that unregistering removes clients properly.
func TestClientUnregistration(t *testing.T) {
	h := NewHub()
	go h.Run()

	// Create mock client without real connection
	client := &Client{
		hub:        h,
		conn:       nil,
		send:       make(chan []byte, 256),
		documentID: "test-doc",
	}

	// Register then unregister
	h.Register(client)
	time.Sleep(50 * time.Millisecond)

	h.mu.RLock()
	_, registered := h.clients[client]
	h.mu.RUnlock()
	if !registered {
		t.Fatal("client not registered initially")
	}

	h.Unregister(client)
	time.Sleep(50 * time.Millisecond)

	// Verify removal
	h.mu.RLock()
	_, exists := h.clients[client]
	h.mu.RUnlock()

	if exists {
		t.Error("client still registered after unregister")
	}
}

// TestBroadcast verifies that broadcast sends messages to all registered clients.
func TestBroadcast(t *testing.T) {
	h := NewHub()
	go h.Run()

	const numClients = 3
	clients := make([]*Client, numClients)

	for i := 0; i < numClients; i++ {
		clients[i] = &Client{
			hub:        h,
			conn:       nil,
			send:       make(chan []byte, 256),
			documentID: "test-doc",
		}
		h.Register(clients[i])
	}

	time.Sleep(200 * time.Millisecond)

	// Drain any initial system messages (USER_COUNT)
	for _, client := range clients {
		drainSystemMessages(t, client.send)
	}

	// Broadcast test message
	testMessage := []byte("Test broadcast message")
	h.Broadcast(testMessage, nil)

	time.Sleep(100 * time.Millisecond)

	// Verify all clients received the message
	for i, client := range clients {
		select {
		case msg := <-client.send:
			if string(msg) != string(testMessage) {
				t.Errorf("client %d: got %q, want %q", i, msg, testMessage)
			}
		case <-time.After(1 * time.Second):
			t.Errorf("client %d: did not receive broadcast message", i)
		}
	}
}

// TestClientCount verifies the ClientCount method returns accurate counts.
func TestClientCount(t *testing.T) {
	h := NewHub()
	go h.Run()

	if got := h.ClientCount(); got != 0 {
		t.Errorf("initial client count = %d, want 0", got)
	}

	// Register clients progressively and verify count
	tests := []struct {
		add  int
		want int
	}{
		{add: 1, want: 1},
		{add: 4, want: 5},
		{add: 5, want: 10},
	}

	var clients []*Client

	for _, tt := range tests {
		for i := 0; i < tt.add; i++ {
			client := &Client{
				hub:        h,
				conn:       nil,
				send:       make(chan []byte, 256),
				documentID: "test-doc",
			}
			h.Register(client)
			clients = append(clients, client)
		}

		time.Sleep(100 * time.Millisecond)

		if got := h.ClientCount(); got != tt.want {
			t.Errorf("after adding %d clients: count = %d, want %d", tt.add, got, tt.want)
		}
	}

	// Unregister some clients
	h.Unregister(clients[0])
	h.Unregister(clients[1])
	time.Sleep(100 * time.Millisecond)

	wantCount := len(clients) - 2
	if got := h.ClientCount(); got != wantCount {
		t.Errorf("after unregistering 2 clients: count = %d, want %d", got, wantCount)
	}
}

// TestClientCountForDocument verifies per-document client counting.
func TestClientCountForDocument(t *testing.T) {
	h := NewHub()
	go h.Run()

	// Create clients for different documents
	documents := map[string]int{
		"doc1": 3,
		"doc2": 5,
		"doc3": 2,
	}

	for docID, count := range documents {
		for i := 0; i < count; i++ {
			client := &Client{
				hub:        h,
				conn:       nil,
				send:       make(chan []byte, 256),
				documentID: docID,
			}
			h.Register(client)
		}
	}

	time.Sleep(200 * time.Millisecond)

	// Verify counts for each document
	for docID, want := range documents {
		if got := h.ClientCountForDocument(docID); got != want {
			t.Errorf("ClientCountForDocument(%q) = %d, want %d", docID, got, want)
		}
	}

	// Verify count for non-existent document
	if got := h.ClientCountForDocument("nonexistent"); got != 0 {
		t.Errorf("ClientCountForDocument(nonexistent) = %d, want 0", got)
	}
}

// TestConcurrentRegistrations verifies thread-safe concurrent client registration.
func TestConcurrentRegistrations(t *testing.T) {
	h := NewHub()
	go h.Run()

	const numGoroutines = 20
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			client := &Client{
				hub:        h,
				conn:       nil,
				send:       make(chan []byte, 256),
				documentID: "test-doc",
			}
			h.Register(client)
		}()
	}

	wg.Wait()
	time.Sleep(200 * time.Millisecond)

	if got := h.ClientCount(); got != numGoroutines {
		t.Errorf("after concurrent registrations: count = %d, want %d", got, numGoroutines)
	}
}

// TestBroadcastToSelf verifies that clients receive their own broadcasts.
func TestBroadcastToSelf(t *testing.T) {
	h := NewHub()
	go h.Run()

	client := &Client{
		hub:        h,
		conn:       nil,
		send:       make(chan []byte, 256),
		documentID: "test-doc",
	}

	h.Register(client)
	time.Sleep(50 * time.Millisecond)

	// Drain initial system message
	drainSystemMessages(t, client.send)

	// Broadcast message
	testMessage := []byte("Self broadcast")
	h.Broadcast(testMessage, nil)

	// Verify client receives its own broadcast
	select {
	case msg := <-client.send:
		if string(msg) != string(testMessage) {
			t.Errorf("got %q, want %q", msg, testMessage)
		}
	case <-time.After(1 * time.Second):
		t.Error("client did not receive its own broadcast")
	}
}

// TestConcurrentBroadcasts verifies thread-safe concurrent broadcasting.
func TestConcurrentBroadcasts(t *testing.T) {
	h := NewHub()
	go h.Run()

	// Register a client
	client := &Client{
		hub:        h,
		conn:       nil,
		send:       make(chan []byte, 256),
		documentID: "test-doc",
	}
	h.Register(client)

	// Drain messages in background
	go func() {
		for range client.send {
			// Discard all messages
		}
	}()

	time.Sleep(50 * time.Millisecond)

	// Send many broadcasts concurrently
	const numBroadcasts = 100
	var wg sync.WaitGroup
	wg.Add(numBroadcasts)

	for i := 0; i < numBroadcasts; i++ {
		go func(id int) {
			defer wg.Done()
			h.Broadcast([]byte("message"), nil)
		}(i)
	}

	wg.Wait()
	// If we get here without panicking or deadlocking, the test passes
}

// BenchmarkBroadcast measures broadcast performance with varying client counts.
func BenchmarkBroadcast(b *testing.B) {
	clientCounts := []int{1, 10, 100}

	for _, numClients := range clientCounts {
		b.Run(string(rune('0'+numClients)), func(b *testing.B) {
			h := NewHub()
			go h.Run()

			// Register clients
			for i := 0; i < numClients; i++ {
				client := &Client{
					hub:        h,
					conn:       nil,
					send:       make(chan []byte, 256),
					documentID: "test-doc",
				}
				h.Register(client)

				// Drain messages to prevent channel blocking
				go func(c *Client) {
					for range c.send {
					}
				}(client)
			}

			time.Sleep(100 * time.Millisecond)
			message := []byte("Benchmark message")

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				h.Broadcast(message, nil)
			}
		})
	}
}

// BenchmarkRegisterUnregister measures client lifecycle performance.
func BenchmarkRegisterUnregister(b *testing.B) {
	h := NewHub()
	go h.Run()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		client := &Client{
			hub:        h,
			conn:       nil,
			send:       make(chan []byte, 256),
			documentID: "test-doc",
		}
		h.Register(client)
		h.Unregister(client)
	}
}

// drainSystemMessages drains USER_COUNT messages from a channel.
// This helper is used to clear system messages before testing content messages.
func drainSystemMessages(t *testing.T, ch chan []byte) {
	t.Helper()

	for {
		select {
		case msg := <-ch:
			var parsed Message
			if err := json.Unmarshal(msg, &parsed); err == nil {
				if parsed.Type == MsgTypeUserCount {
					continue
				}
			}
			if strings.HasPrefix(string(msg), "USER_COUNT:") {
				continue
			}
			// Non-system message found, stop draining
			return
		case <-time.After(50 * time.Millisecond):
			// Timeout - no more messages
			return
		}
	}
}
