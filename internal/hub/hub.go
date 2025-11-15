package hub

import (
	"collaborative-docs/internal/document"
	"log"
	"sync"
)

// broadcastMessage pairs a message with its sender for broadcast routing
type broadcastMessage struct {
	message []byte
	sender  *Client
}

// Hub coordinates WebSocket connections and routes messages
// between clients editing the same document. It manages document-specific
// client groups, applies operations to shared document state, and broadcasts
// changes to connected clients.
type Hub struct {
	clients    map[*Client]bool
	broadcast  chan *broadcastMessage
	register   chan *Client
	unregister chan *Client
	documents  map[string]*document.Document
	mu         sync.RWMutex
	quit       chan struct{}
}

// NewHub creates and initializes a new Hub instance
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan *broadcastMessage),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		documents:  make(map[string]*document.Document),
		quit:       make(chan struct{}),
	}
}

// Run starts the hub's main event loop, processing client
// registration, unregistration, and message broadcasting.
// This method blocks and should be run in a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case <-h.quit:
			log.Println("hub shutting down, closing all clients")
			h.closeAllClients()
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("client registered, total: %d", len(h.clients))
			h.broadcastUserCount()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				log.Printf("client unregistered, total: %d", len(h.clients))
			}
			h.mu.Unlock()
			h.broadcastUserCount()

		case bm := <-h.broadcast:
			msg, err := MessageFromBytes(bm.message)
			if err != nil || IsLegacyContent(bm.message) {
				log.Printf("broadcasting legacy message to all clients")
				h.broadcastToAll(bm.message, nil)
				continue
			}

			documentID := msg.DocumentID
			if documentID == "" {
				log.Printf("no document ID in message, broadcasting to all")
				h.broadcastToAll(bm.message, nil)
				continue
			}

			doc := h.GetOrCreateDocument(documentID)

			switch msg.Type {
			case MsgTypeOperation:
				if msg.Operation != nil {
					log.Printf("applying operation to document %s: %s", documentID, msg.Operation.String())
					newContent, newVersion, err := doc.ApplyOperation(msg.Operation)
					if err != nil {
						log.Printf("operation failed: %v", err)
						continue
					}

					log.Printf("operation applied to document %s, version: %d, length: %d",
						documentID, newVersion, len(newContent))

					msg.Operation.Version = newVersion

					msgBytes, err := msg.ToBytes()
					if err != nil {
						log.Printf("serialization failed: %v", err)
						continue
					}
					h.broadcastToDocument(documentID, msgBytes, bm.sender)
				}

			case MsgTypeContent:
				if msg.Content != "" {
					doc.SetContent(msg.Content)
					msgBytes, _ := msg.ToBytes()
					h.broadcastToDocument(documentID, msgBytes, bm.sender)
				}

			default:
				h.broadcastToDocument(documentID, bm.message, bm.sender)
			}
		}
	}
}

// Register adds a client to the hub.
func (h *Hub) Register(client *Client) {
	h.register <- client
}

// Unregister removes a client from the hub.
func (h *Hub) Unregister(client *Client) {
	h.unregister <- client
}

// Broadcast sends a message to all connected clients.
// The sender parameter can be nil for system messages.
func (h *Hub) Broadcast(message []byte, sender *Client) {
	h.broadcast <- &broadcastMessage{
		message: message,
		sender:  sender,
	}
}

// ClientCount returns the current number of connected clients.
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// ClientCountForDocument returns the number of clients editing a specific document.
func (h *Hub) ClientCountForDocument(documentID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	count := 0
	for client := range h.clients {
		if client.documentID == documentID {
			count++
		}
	}
	return count
}

// GetOrCreateDocument retrieves an existing document or creates a new one.
func (h *Hub) GetOrCreateDocument(documentID string) *document.Document {
	h.mu.Lock()
	defer h.mu.Unlock()

	doc, exists := h.documents[documentID]
	if !exists {
		doc = document.NewDocument()
		h.documents[documentID] = doc
		log.Printf("created new document: %s", documentID)
	}
	return doc
}

// GetDocument retrieves a document by ID, returns nil if not found.
func (h *Hub) GetDocument(documentID string) *document.Document {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.documents[documentID]
}

// broadcastUserCount sends the current user count to all connected clients.
func (h *Hub) broadcastUserCount() {
	h.mu.RLock()
	defer h.mu.RUnlock()

	docCounts := make(map[string]int)
	for client := range h.clients {
		docCounts[client.documentID]++
	}

	for documentID, count := range docCounts {
		msg := NewUserCountMessage(count)
		msgBytes, err := msg.ToBytes()
		if err != nil {
			log.Printf("user count message creation failed: %v", err)
			continue
		}

		for client := range h.clients {
			if client.documentID == documentID {
				select {
				case client.send <- msgBytes:
				default:
				}
			}
		}

		log.Printf("broadcasted user count %d to document: %s", count, documentID)
	}
}

// broadcastToAll sends a message to all connected clients (legacy support).
// The exclude parameter can be nil to send to all clients.
func (h *Hub) broadcastToAll(message []byte, exclude *Client) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		// Skip the sender if exclude is provided
		if exclude != nil && client == exclude {
			continue
		}

		select {
		case client.send <- message:
		default:
			// Use Unregister channel instead of direct deletion to avoid race condition
			go h.Unregister(client)
			log.Printf("client marked for removal due to full send buffer")
		}
	}
}

// broadcastToDocument sends a message to all clients editing a specific document.
// The exclude parameter can be nil to send to all clients, or set to skip the sender.
func (h *Hub) broadcastToDocument(documentID string, message []byte, exclude *Client) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	sentCount := 0
	for client := range h.clients {
		if client.documentID == documentID {
			// Skip the sender if exclude is provided
			if exclude != nil && client == exclude {
				continue
			}

			select {
			case client.send <- message:
				sentCount++
			default:
				// Use Unregister channel instead of direct deletion to avoid race condition
				go h.Unregister(client)
				log.Printf("client marked for removal due to full send buffer")
			}
		}
	}

	log.Printf("broadcasted message to %d clients on document: %s", sentCount, documentID)
}

// Shutdown gracefully stops the hub and closes all client connections.
func (h *Hub) Shutdown() {
	close(h.quit)
}

// closeAllClients closes all client connections during shutdown.
func (h *Hub) closeAllClients() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for client := range h.clients {
		close(client.send)
		if err := client.conn.Close(); err != nil {
			log.Printf("error closing client connection: %v", err)
		}
	}
	h.clients = make(map[*Client]bool)
	log.Printf("all clients closed")
}
