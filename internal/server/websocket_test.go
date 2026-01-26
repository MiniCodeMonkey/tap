package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
)

func TestNewWebSocketHub(t *testing.T) {
	hub := NewWebSocketHub()

	if hub == nil {
		t.Fatal("NewWebSocketHub() returned nil")
	}

	if hub.clients == nil {
		t.Error("clients map is nil")
	}

	if hub.broadcast == nil {
		t.Error("broadcast channel is nil")
	}

	if hub.register == nil {
		t.Error("register channel is nil")
	}

	if hub.unregister == nil {
		t.Error("unregister channel is nil")
	}

	if hub.done == nil {
		t.Error("done channel is nil")
	}
}

func TestWebSocketHubClientCount(t *testing.T) {
	hub := NewWebSocketHub()

	if hub.ClientCount() != 0 {
		t.Errorf("ClientCount() = %d, want 0", hub.ClientCount())
	}
}

func TestWebSocketHubRunAndStop(t *testing.T) {
	hub := NewWebSocketHub()

	// Start the hub
	done := make(chan struct{})
	go func() {
		hub.Run()
		close(done)
	}()

	// Give it a moment to start
	time.Sleep(10 * time.Millisecond)

	// Stop the hub
	hub.Stop()

	// Wait for it to finish
	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Hub did not stop within timeout")
	}
}

func TestMessageTypes(t *testing.T) {
	tests := []struct {
		msgType MessageType
		want    string
	}{
		{MessageConnected, "connected"},
		{MessageReload, "reload"},
		{MessageSlide, "slide"},
	}

	for _, tt := range tests {
		if string(tt.msgType) != tt.want {
			t.Errorf("MessageType %v = %q, want %q", tt.msgType, tt.msgType, tt.want)
		}
	}
}

func TestMessageJSON(t *testing.T) {
	tests := []struct {
		name string
		msg  Message
		want string
	}{
		{
			name: "connected message",
			msg:  Message{Type: MessageConnected},
			want: `{"type":"connected"}`,
		},
		{
			name: "reload message",
			msg:  Message{Type: MessageReload},
			want: `{"type":"reload"}`,
		},
		{
			name: "slide message",
			msg:  Message{Type: MessageSlide, SlideIndex: 5},
			want: `{"type":"slide","slideIndex":5}`,
		},
		{
			name: "slide message with zero",
			msg:  Message{Type: MessageSlide, SlideIndex: 0},
			want: `{"type":"slide"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.msg)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}

			if string(data) != tt.want {
				t.Errorf("json.Marshal() = %s, want %s", data, tt.want)
			}
		})
	}
}

func TestWebSocketHubBroadcast(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	// Give the hub time to start
	time.Sleep(10 * time.Millisecond)

	// Broadcast should not error even with no clients
	err := hub.Broadcast(Message{Type: MessageReload})
	if err != nil {
		t.Errorf("Broadcast() error = %v", err)
	}
}

func TestWebSocketHubBroadcastReload(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	time.Sleep(10 * time.Millisecond)

	err := hub.BroadcastReload()
	if err != nil {
		t.Errorf("BroadcastReload() error = %v", err)
	}
}

func TestWebSocketHubBroadcastSlide(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	time.Sleep(10 * time.Millisecond)

	err := hub.BroadcastSlide(5)
	if err != nil {
		t.Errorf("BroadcastSlide() error = %v", err)
	}
}

func TestWebSocketHubClientRegistration(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	time.Sleep(10 * time.Millisecond)

	// Create a mock client
	client := &Client{
		hub:  hub,
		conn: nil,
		send: make(chan []byte, 256),
	}

	// Register the client
	hub.register <- client

	// Give it time to process
	time.Sleep(10 * time.Millisecond)

	if hub.ClientCount() != 1 {
		t.Errorf("ClientCount() = %d, want 1", hub.ClientCount())
	}

	// Unregister the client
	hub.unregister <- client

	// Give it time to process
	time.Sleep(10 * time.Millisecond)

	if hub.ClientCount() != 0 {
		t.Errorf("ClientCount() = %d, want 0", hub.ClientCount())
	}
}

func TestWebSocketHubBroadcastToClients(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	time.Sleep(10 * time.Millisecond)

	// Create mock clients
	client1 := &Client{
		hub:  hub,
		conn: nil,
		send: make(chan []byte, 256),
	}
	client2 := &Client{
		hub:  hub,
		conn: nil,
		send: make(chan []byte, 256),
	}

	// Register clients
	hub.register <- client1
	hub.register <- client2

	time.Sleep(10 * time.Millisecond)

	// Broadcast a message
	err := hub.BroadcastReload()
	if err != nil {
		t.Fatalf("BroadcastReload() error = %v", err)
	}

	// Give it time to broadcast
	time.Sleep(10 * time.Millisecond)

	// Check both clients received the message
	select {
	case msg := <-client1.send:
		var m Message
		if err := json.Unmarshal(msg, &m); err != nil {
			t.Errorf("client1: json.Unmarshal() error = %v", err)
		}
		if m.Type != MessageReload {
			t.Errorf("client1: message type = %v, want %v", m.Type, MessageReload)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("client1 did not receive message")
	}

	select {
	case msg := <-client2.send:
		var m Message
		if err := json.Unmarshal(msg, &m); err != nil {
			t.Errorf("client2: json.Unmarshal() error = %v", err)
		}
		if m.Type != MessageReload {
			t.Errorf("client2: message type = %v, want %v", m.Type, MessageReload)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("client2 did not receive message")
	}
}

func TestWebSocketHubHandleConnection(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
	defer server.Close()

	// Connect via WebSocket
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket.Dial() error = %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Give time for registration
	time.Sleep(50 * time.Millisecond)

	// Check client is registered
	if hub.ClientCount() != 1 {
		t.Errorf("ClientCount() = %d, want 1", hub.ClientCount())
	}

	// Read the connected message
	msgType, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("conn.Read() error = %v", err)
	}

	if msgType != websocket.MessageText {
		t.Errorf("message type = %v, want %v", msgType, websocket.MessageText)
	}

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if msg.Type != MessageConnected {
		t.Errorf("message type = %v, want %v", msg.Type, MessageConnected)
	}
}

func TestWebSocketHubBroadcastToRealConnection(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
	defer server.Close()

	// Connect via WebSocket
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket.Dial() error = %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	// Read the connected message first
	_, _, err = conn.Read(ctx)
	if err != nil {
		t.Fatalf("conn.Read() connected message error = %v", err)
	}

	// Give time for registration
	time.Sleep(50 * time.Millisecond)

	// Broadcast a slide message
	err = hub.BroadcastSlide(10)
	if err != nil {
		t.Fatalf("BroadcastSlide() error = %v", err)
	}

	// Read the broadcast message
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("conn.Read() broadcast error = %v", err)
	}

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if msg.Type != MessageSlide {
		t.Errorf("message type = %v, want %v", msg.Type, MessageSlide)
	}

	if msg.SlideIndex != 10 {
		t.Errorf("slide = %d, want 10", msg.SlideIndex)
	}
}

func TestWebSocketHubClientDisconnect(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
	defer server.Close()

	// Connect via WebSocket
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket.Dial() error = %v", err)
	}

	// Give time for registration
	time.Sleep(50 * time.Millisecond)

	if hub.ClientCount() != 1 {
		t.Errorf("ClientCount() before disconnect = %d, want 1", hub.ClientCount())
	}

	// Close the connection
	conn.Close(websocket.StatusNormalClosure, "")

	// Give time for unregistration
	time.Sleep(100 * time.Millisecond)

	if hub.ClientCount() != 0 {
		t.Errorf("ClientCount() after disconnect = %d, want 0", hub.ClientCount())
	}
}

func TestWebSocketHubMultipleConnections(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Connect multiple clients
	var conns []*websocket.Conn
	for i := 0; i < 3; i++ {
		conn, _, err := websocket.Dial(ctx, wsURL, nil)
		if err != nil {
			t.Fatalf("websocket.Dial() connection %d error = %v", i, err)
		}
		conns = append(conns, conn)
	}

	defer func() {
		for _, conn := range conns {
			conn.Close(websocket.StatusNormalClosure, "")
		}
	}()

	// Give time for all registrations
	time.Sleep(100 * time.Millisecond)

	if hub.ClientCount() != 3 {
		t.Errorf("ClientCount() = %d, want 3", hub.ClientCount())
	}

	// Close one connection
	conns[0].Close(websocket.StatusNormalClosure, "")

	// Give time for unregistration
	time.Sleep(100 * time.Millisecond)

	if hub.ClientCount() != 2 {
		t.Errorf("ClientCount() after one disconnect = %d, want 2", hub.ClientCount())
	}
}
