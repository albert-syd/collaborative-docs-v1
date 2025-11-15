package hub

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second           // Maximum time to write a message
	pongWait       = 60 * time.Second           // Time to wait for pong response
	pingPeriod     = (pongWait * 9) / 10        // Ping interval (must be < pongWait)
	maxMessageSize = 512 * 1024                 // Maximum message size (512KB)
)

// Client represents a WebSocket connection to a browser.
// It runs two concurrent goroutines: ReadPump for incoming
// messages and WritePump for outgoing messages.
type Client struct {
	hub        *Hub
	conn       *websocket.Conn
	send       chan []byte // Buffered channel for outbound messages
	documentID string
}

// NewClient creates a new Client instance.
func NewClient(hub *Hub, conn *websocket.Conn, documentID string) *Client {
	return &Client{
		hub:        hub,
		conn:       conn,
		send:       make(chan []byte, 256),
		documentID: documentID,
	}
}

// ReadPump reads messages from the WebSocket and forwards them to the hub.
// It runs until the connection closes, then unregisters the client.
func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("unexpected websocket close: %v", err)
			}
			break
		}

		c.hub.Broadcast(message, c)
	}
}

// WritePump sends messages from the hub to the WebSocket.
// It also sends periodic pings to detect disconnected clients.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))

			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
