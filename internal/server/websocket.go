// Package server provides HTTP server functionality for the dev server.
package server

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// MessageType represents the type of WebSocket message.
type MessageType string

const (
	// MessageConnected is sent when a client connects.
	MessageConnected MessageType = "connected"
	// MessageReload signals clients to reload the page.
	MessageReload MessageType = "reload"
	// MessageSlide signals clients to navigate to a specific slide.
	MessageSlide MessageType = "slide"
)

// Message represents a WebSocket message sent between server and clients.
type Message struct {
	Type  MessageType `json:"type"`
	Slide int         `json:"slide,omitempty"`
}

// Client represents a connected WebSocket client.
type Client struct {
	hub  *WebSocketHub
	conn *websocket.Conn
	send chan []byte
}

// WebSocketHub manages WebSocket connections and message broadcasting.
type WebSocketHub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	done       chan struct{}
	mu         sync.RWMutex
}

// NewWebSocketHub creates a new WebSocket hub.
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		done:       make(chan struct{}),
	}
}

// Run starts the hub's main event loop.
// It should be started as a goroutine.
func (h *WebSocketHub) Run() {
	for {
		select {
		case <-h.done:
			// Close all client connections
			h.mu.Lock()
			for client := range h.clients {
				close(client.send)
				delete(h.clients, client)
			}
			h.mu.Unlock()
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// Client buffer is full, skip this message
				}
			}
			h.mu.RUnlock()
		}
	}
}

// Stop stops the hub's event loop.
func (h *WebSocketHub) Stop() {
	close(h.done)
}

// Broadcast sends a message to all connected clients.
func (h *WebSocketHub) Broadcast(msg Message) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	select {
	case h.broadcast <- data:
	default:
		// Broadcast channel is full, skip
	}

	return nil
}

// BroadcastReload sends a reload message to all clients.
func (h *WebSocketHub) BroadcastReload() error {
	return h.Broadcast(Message{Type: MessageReload})
}

// BroadcastSlide sends a slide navigation message to all clients.
func (h *WebSocketHub) BroadcastSlide(slideIndex int) error {
	return h.Broadcast(Message{Type: MessageSlide, Slide: slideIndex})
}

// ClientCount returns the number of connected clients.
func (h *WebSocketHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// HandleConnection handles a new WebSocket connection.
// It should be used as an HTTP handler.
func (h *WebSocketHub) HandleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// Allow connections from any origin in dev mode
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}

	client := &Client{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 256),
	}

	h.register <- client

	// Send connected message
	connectedMsg, _ := json.Marshal(Message{Type: MessageConnected})
	select {
	case client.send <- connectedMsg:
	default:
	}

	// Use a context that's independent of the HTTP request
	// The context will be canceled when the hub is stopped
	ctx, cancel := context.WithCancel(context.Background())

	// Start goroutines for reading and writing
	go client.writePump(ctx, cancel)
	client.readPump(ctx)
}

// readPump reads messages from the WebSocket connection.
// It handles ping/pong and client-initiated messages.
// This function blocks and runs in the HTTP handler goroutine.
func (c *Client) readPump(ctx context.Context) {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close(websocket.StatusNormalClosure, "")
	}()

	for {
		_, _, err := c.conn.Read(ctx)
		if err != nil {
			// Connection closed or error
			return
		}
		// For now, we don't process client messages, just keep the connection alive
	}
}

// writePump sends messages to the WebSocket connection.
func (c *Client) writePump(ctx context.Context, cancel context.CancelFunc) {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		cancel()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				// Channel closed
				return
			}

			writeCtx, writeCancel := context.WithTimeout(ctx, 10*time.Second)
			err := c.conn.Write(writeCtx, websocket.MessageText, message)
			writeCancel()
			if err != nil {
				return
			}

		case <-ticker.C:
			// Send ping to keep connection alive
			pingCtx, pingCancel := context.WithTimeout(ctx, 10*time.Second)
			err := c.conn.Ping(pingCtx)
			pingCancel()
			if err != nil {
				return
			}

		case <-ctx.Done():
			return
		}
	}
}
