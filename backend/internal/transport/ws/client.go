package ws

import (
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// writeWait is the maximum time allowed to write a message to the peer.
	writeWait = 10 * time.Second

	// pongWait is the maximum time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second

	// pingPeriod is the interval at which ping messages are sent to the peer.
	// Must be less than pongWait (30 seconds per ARCHITECTURE.md §7.3).
	pingPeriod = 30 * time.Second

	// maxMessageSize is the maximum message size allowed from peer (512KB).
	maxMessageSize = 512 * 1024

	// sendBufferSize is the capacity of the outbound message channel.
	sendBufferSize = 256
)

// Client is a middleman between the WebSocket connection and the Hub.
type Client struct {
	hub *Hub

	// The websocket connection.
	conn *websocket.Conn

	// Buffered channel of outbound messages.
	send chan []byte

	// marketID this client is subscribed to ("" or "all" for global broadcast).
	marketID string

	// onClose optional cleanup callback when connection terminates.
	onClose func()
}

// NewClient constructs a new WebSocket client instance.
func NewClient(hub *Hub, conn *websocket.Conn, marketID string) *Client {
	return &Client{
		hub:      hub,
		conn:     conn,
		send:     make(chan []byte, sendBufferSize),
		marketID: marketID,
	}
}

// SetOnClose registers a callback invoked when the client disconnects.
func (c *Client) SetOnClose(fn func()) {
	c.onClose = fn
}

// readPump pumps messages from the websocket connection to the hub.
// The application runs readPump in a per-connection goroutine.
func (c *Client) readPump() {
	defer func() {
		select {
		case c.hub.unregister <- c:
		case <-c.hub.shutdown:
		}
		c.conn.Close()
		if c.onClose != nil {
			c.onClose()
		}
	}()

	c.conn.SetReadLimit(maxMessageSize)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[DEBUG] WS unexpected close: %v", err)
			}
			break
		}
		// Client messages (e.g. heartbeat or client-initiated pings)
		_ = message
	}
}

// writePump pumps messages from the hub to the websocket connection.
// A goroutine running writePump is started for each connection.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// The hub closed the channel.
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			if _, err := w.Write(message); err != nil {
				return
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
