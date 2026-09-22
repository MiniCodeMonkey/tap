package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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
			msg:  Message{Type: MessageSlide, SlideIndex: intPtr(5)},
			want: `{"type":"slide","slideIndex":5}`,
		},
		{
			name: "slide message with index 0 still encodes the index",
			msg:  Message{Type: MessageSlide, SlideIndex: intPtr(0)},
			want: `{"type":"slide","slideIndex":0}`,
		},
		{
			name: "slide message with fragment, step and scroll reveal",
			msg: Message{
				Type:           MessageSlide,
				SlideIndex:     intPtr(3),
				Fragment:       intPtr(1),
				Step:           intPtr(2),
				ScrollRevealed: boolPtr(true),
			},
			want: `{"type":"slide","slideIndex":3,"fragment":1,"step":2,"scrollRevealed":true}`,
		},
		{
			name: "slide message with fragment -1 and scroll revealed false still encode",
			msg: Message{
				Type:           MessageSlide,
				SlideIndex:     intPtr(0),
				Fragment:       intPtr(-1),
				Step:           intPtr(0),
				ScrollRevealed: boolPtr(false),
			},
			want: `{"type":"slide","slideIndex":0,"fragment":-1,"step":0,"scrollRevealed":false}`,
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

func intPtr(v int) *int {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}

// TestMessageRoundTripLegacySlide verifies that a slide message without the
// fragment, step or scroll reveal fields - exactly what an old client sends -
// decodes with those fields left nil, meaning "this slide, initial state".
func TestMessageRoundTripLegacySlide(t *testing.T) {
	legacy := `{"type":"slide","slideIndex":7}`

	var msg Message
	if err := json.Unmarshal([]byte(legacy), &msg); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if msg.SlideIndex == nil || *msg.SlideIndex != 7 {
		t.Errorf("SlideIndex = %v, want 7", msg.SlideIndex)
	}
	if msg.Fragment != nil {
		t.Errorf("Fragment = %v, want nil", msg.Fragment)
	}
	if msg.Step != nil {
		t.Errorf("Step = %v, want nil", msg.Step)
	}
	if msg.ScrollRevealed != nil {
		t.Errorf("ScrollRevealed = %v, want nil", msg.ScrollRevealed)
	}

	// Re-marshaling a message built from the decoded legacy fields must
	// still omit the new fields, so a legacy round trip stays legacy.
	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if string(data) != legacy {
		t.Errorf("json.Marshal() = %s, want %s", data, legacy)
	}
}

// TestMessageRoundTripFullState verifies that a slide message carrying
// fragment, step and scroll reveal state survives a marshal/unmarshal round
// trip unchanged, including a fragment index of -1 (no fragment revealed).
func TestMessageRoundTripFullState(t *testing.T) {
	original := Message{
		Type:           MessageSlide,
		SlideIndex:     intPtr(4),
		Fragment:       intPtr(-1),
		Step:           intPtr(1),
		ScrollRevealed: boolPtr(true),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.SlideIndex == nil || *decoded.SlideIndex != *original.SlideIndex {
		t.Errorf("SlideIndex = %v, want %v", decoded.SlideIndex, *original.SlideIndex)
	}
	if decoded.Fragment == nil || *decoded.Fragment != -1 {
		t.Errorf("Fragment = %v, want -1", decoded.Fragment)
	}
	if decoded.Step == nil || *decoded.Step != 1 {
		t.Errorf("Step = %v, want 1", decoded.Step)
	}
	if decoded.ScrollRevealed == nil || *decoded.ScrollRevealed != true {
		t.Errorf("ScrollRevealed = %v, want true", decoded.ScrollRevealed)
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

	// Registering queues a "connected" message to each client (see the
	// register case in Run); drain it before broadcasting so the reload
	// below is the next message read.
	<-client1.send
	<-client2.send

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

// TestWebSocketHubConnectedMessageCarriesRevision verifies that
// SetPresentationMeta's revision rides on the "connected" message sent at
// register time, and that a later connection sees a revision changed by a
// later SetPresentationMeta call (simulating a deck reload between the two
// connections).
func TestWebSocketHubConnectedMessageCarriesRevision(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	server := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	readConnected := func() Message {
		conn, _, err := websocket.Dial(ctx, wsURL, nil)
		if err != nil {
			t.Fatalf("websocket.Dial() error = %v", err)
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("conn.Read() error = %v", err)
		}
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		return msg
	}

	first := readConnected()
	if first.Revision != "" {
		t.Errorf("Revision = %q, want empty before SetPresentationMeta is ever called", first.Revision)
	}

	hub.SetPresentationMeta(3, "abc123")
	second := readConnected()
	if second.Revision != "abc123" {
		t.Errorf("Revision = %q, want %q", second.Revision, "abc123")
	}

	hub.SetPresentationMeta(3, "def456")
	third := readConnected()
	if third.Revision != "def456" {
		t.Errorf("Revision = %q, want %q", third.Revision, "def456")
	}
}

func TestWebSocketHubConnectedMessageCarriesPresentMode(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	server := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
	defer server.Close()
	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	readConnected := func() Message {
		conn, _, err := websocket.Dial(ctx, wsURL, nil)
		if err != nil {
			t.Fatalf("websocket.Dial() error = %v", err)
		}
		defer conn.Close(websocket.StatusNormalClosure, "")
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("conn.Read() error = %v", err)
		}
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		return msg
	}

	first := readConnected()
	if first.Mode != "" {
		t.Errorf("Mode = %q, want empty before SetPresentMode is ever called", first.Mode)
	}

	hub.SetPresentMode(true)
	second := readConnected()
	if second.Mode != "present" {
		t.Errorf("Mode = %q, want %q", second.Mode, "present")
	}

	hub.SetPresentMode(false)
	third := readConnected()
	if third.Mode != "" {
		t.Errorf("Mode = %q, want empty once present mode is turned back off", third.Mode)
	}
}

// TestWebSocketHubOriginCheck covers checkOrigin's rules: no Origin header,
// an Origin whose host matches the request's own Host header, and an Origin
// explicitly allowed via SetAllowedOrigins are all accepted; anything else
// is rejected with 403.
func TestWebSocketHubOriginCheck(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	hub.SetAllowedOrigins([]string{"http://localhost:5173"})

	server := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	httpURL := server.URL

	dial := func(origin string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		var header http.Header
		if origin != "" {
			header = http.Header{"Origin": []string{origin}}
		}
		conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: header})
		if err == nil {
			conn.Close(websocket.StatusNormalClosure, "")
		}
		return err
	}

	t.Run("no origin header is accepted", func(t *testing.T) {
		if err := dial(""); err != nil {
			t.Errorf("dial with no Origin header failed: %v", err)
		}
	})

	t.Run("origin matching the request host is accepted", func(t *testing.T) {
		sameHostOrigin := "http://" + strings.TrimPrefix(httpURL, "http://")
		if err := dial(sameHostOrigin); err != nil {
			t.Errorf("dial with same-host Origin %q failed: %v", sameHostOrigin, err)
		}
	})

	t.Run("origin allowed via SetAllowedOrigins is accepted", func(t *testing.T) {
		if err := dial("http://localhost:5173"); err != nil {
			t.Errorf("dial with allowed Origin failed: %v", err)
		}
	})

	t.Run("foreign origin is rejected", func(t *testing.T) {
		err := dial("http://evil.example.com")
		if err == nil {
			t.Fatal("dial with foreign Origin succeeded, want rejection")
		}
	})
}

// TestWebSocketHubCheckOrigin_RejectsDNSRebinding reproduces the
// DNS-rebinding gap directly against checkOrigin: a hostile domain an
// attacker controls can resolve to 127.0.0.1, so a request can carry
// Host: evil.example:3800 and Origin: http://evil.example:3800 - equal to
// each other, but neither one a host this hub should trust. The same-host
// compare alone must not be enough to accept it.
func TestWebSocketHubCheckOrigin_RejectsDNSRebinding(t *testing.T) {
	hub := NewWebSocketHub()

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Host = "evil.example:3800"
	req.Header.Set("Origin", "http://evil.example:3800")

	if hub.checkOrigin(req) {
		t.Error("checkOrigin accepted a same-host DNS-rebinding request, want rejection")
	}
}

// TestWebSocketHubHandleConnection_RejectsDisallowedHost checks
// HandleConnection's own Host allow-list, which runs even for a request
// with no Origin header at all (a non-browser client), where checkOrigin
// alone would otherwise accept it.
func TestWebSocketHubHandleConnection_RejectsDisallowedHost(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	req.Host = "evil.example:3800"
	w := httptest.NewRecorder()

	hub.HandleConnection(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
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

	if msg.SlideIndex == nil || *msg.SlideIndex != 10 {
		t.Errorf("slide = %v, want 10", msg.SlideIndex)
	}
}

// TestWebSocketHubBroadcastSlideZero verifies that slide index 0 survives
// the hub's readPump-unmarshal-then-rebroadcast round trip. A client sent
// message is decoded into a Message and re-encoded by hub.Broadcast, so a
// SlideIndex field that couldn't distinguish "zero" from "absent" would drop
// index 0 here even though BroadcastSlide never touches it directly.
func TestWebSocketHubBroadcastSlideZero(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	server := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sender, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket.Dial() sender error = %v", err)
	}
	defer sender.Close(websocket.StatusNormalClosure, "")

	receiver, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket.Dial() receiver error = %v", err)
	}
	defer receiver.Close(websocket.StatusNormalClosure, "")

	// Drain each connection's initial "connected" message.
	if _, _, err := sender.Read(ctx); err != nil {
		t.Fatalf("sender.Read() connected message error = %v", err)
	}
	if _, _, err := receiver.Read(ctx); err != nil {
		t.Fatalf("receiver.Read() connected message error = %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	slideZero := `{"type":"slide","slideIndex":0,"fragment":-1,"step":0,"scrollRevealed":false}`
	if err := sender.Write(ctx, websocket.MessageText, []byte(slideZero)); err != nil {
		t.Fatalf("sender.Write() error = %v", err)
	}

	_, data, err := receiver.Read(ctx)
	if err != nil {
		t.Fatalf("receiver.Read() broadcast error = %v", err)
	}

	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if msg.SlideIndex == nil || *msg.SlideIndex != 0 {
		t.Errorf("SlideIndex = %v, want 0", msg.SlideIndex)
	}
	if msg.Fragment == nil || *msg.Fragment != -1 {
		t.Errorf("Fragment = %v, want -1", msg.Fragment)
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

// TestWebSocketHubLateJoinerReceivesLastSlideState verifies that a client
// connecting after another client has already moved to a slide receives
// that slide state right after its "connected" message, instead of landing
// on slide 1 (or wherever its own URL hash points) - the scenario of a
// presenter window opened in the middle of a talk.
func TestWebSocketHubLateJoinerReceivesLastSlideState(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	server := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	viewer, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket.Dial() viewer error = %v", err)
	}
	defer viewer.Close(websocket.StatusNormalClosure, "")

	if _, _, err := viewer.Read(ctx); err != nil {
		t.Fatalf("viewer.Read() connected message error = %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	// The viewer moves to slide 3 (index 2).
	slideThree := `{"type":"slide","slideIndex":2,"fragment":-1,"step":0,"scrollRevealed":false}`
	if err := viewer.Write(ctx, websocket.MessageText, []byte(slideThree)); err != nil {
		t.Fatalf("viewer.Write() error = %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	// A presenter connects after the fact.
	presenter, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket.Dial() presenter error = %v", err)
	}
	defer presenter.Close(websocket.StatusNormalClosure, "")

	_, connectedData, err := presenter.Read(ctx)
	if err != nil {
		t.Fatalf("presenter.Read() connected message error = %v", err)
	}
	var connectedMsg Message
	if err := json.Unmarshal(connectedData, &connectedMsg); err != nil {
		t.Fatalf("json.Unmarshal() connected message error = %v", err)
	}
	if connectedMsg.Type != MessageConnected {
		t.Fatalf("first message type = %v, want %v", connectedMsg.Type, MessageConnected)
	}

	_, stateData, err := presenter.Read(ctx)
	if err != nil {
		t.Fatalf("presenter.Read() late-joiner state error = %v", err)
	}
	var stateMsg Message
	if err := json.Unmarshal(stateData, &stateMsg); err != nil {
		t.Fatalf("json.Unmarshal() late-joiner state error = %v", err)
	}
	if stateMsg.Type != MessageSlide {
		t.Fatalf("second message type = %v, want %v", stateMsg.Type, MessageSlide)
	}
	if stateMsg.SlideIndex == nil || *stateMsg.SlideIndex != 2 {
		t.Errorf("SlideIndex = %v, want 2", stateMsg.SlideIndex)
	}
	if !stateMsg.Initial {
		t.Errorf("Initial = %v, want true (this is the hub's register-time state)", stateMsg.Initial)
	}
}

// TestWebSocketHubForgetsStateOnceEveryoneDisconnects verifies that once
// every client has disconnected, the hub's remembered slide state is
// cleared: a client connecting after that gets no late-joiner state at all
// (only "connected"), the way a brand new hub does. Without this, a
// long-running server (or, in tests, one dev server shared by a whole
// Playwright run) would keep handing out a stale slide to every new,
// otherwise unrelated session.
func TestWebSocketHubForgetsStateOnceEveryoneDisconnects(t *testing.T) {
	hub := NewWebSocketHub()
	// Zero retention forgets immediately, same as this hub's behavior
	// before state retention existed; see the retention-window tests below
	// for the (now default) grace-period behavior.
	hub.SetStateRetention(0)
	go hub.Run()
	defer hub.Stop()

	server := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	viewer, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket.Dial() viewer error = %v", err)
	}
	if _, _, err := viewer.Read(ctx); err != nil {
		t.Fatalf("viewer.Read() connected message error = %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	slideThree := `{"type":"slide","slideIndex":2,"fragment":-1,"step":0,"scrollRevealed":false}`
	if err := viewer.Write(ctx, websocket.MessageText, []byte(slideThree)); err != nil {
		t.Fatalf("viewer.Write() error = %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	// Everyone disconnects.
	viewer.Close(websocket.StatusNormalClosure, "")
	time.Sleep(100 * time.Millisecond)
	if hub.ClientCount() != 0 {
		t.Fatalf("ClientCount() = %d, want 0", hub.ClientCount())
	}

	// A later, unrelated client connects.
	newClient, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket.Dial() error = %v", err)
	}
	defer newClient.Close(websocket.StatusNormalClosure, "")

	_, connectedData, err := newClient.Read(ctx)
	if err != nil {
		t.Fatalf("newClient.Read() connected message error = %v", err)
	}
	var connectedMsg Message
	if err := json.Unmarshal(connectedData, &connectedMsg); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if connectedMsg.Type != MessageConnected {
		t.Fatalf("message type = %v, want %v", connectedMsg.Type, MessageConnected)
	}

	// No second message should follow.
	readCtx, readCancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer readCancel()
	if _, _, err := newClient.Read(readCtx); err == nil {
		t.Error("expected no further message, but one arrived")
	}
}

// TestWebSocketHubRetainsStateWithinRetentionPeriod verifies that a client
// reconnecting shortly after the last one leaves - a reload of the only
// open window - still receives the deck's last-known slide state, instead
// of losing it the instant the hub reaches zero clients.
func TestWebSocketHubRetainsStateWithinRetentionPeriod(t *testing.T) {
	hub := NewWebSocketHub()
	hub.SetStateRetention(300 * time.Millisecond)
	go hub.Run()
	defer hub.Stop()

	server := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	viewer, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket.Dial() viewer error = %v", err)
	}
	if _, _, err := viewer.Read(ctx); err != nil {
		t.Fatalf("viewer.Read() connected message error = %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	slideFour := `{"type":"slide","slideIndex":3,"fragment":1,"step":0,"scrollRevealed":false}`
	if err := viewer.Write(ctx, websocket.MessageText, []byte(slideFour)); err != nil {
		t.Fatalf("viewer.Write() error = %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	// The only client disconnects (a reload briefly drops to zero clients
	// before the same window's new connection registers).
	viewer.Close(websocket.StatusNormalClosure, "")
	time.Sleep(50 * time.Millisecond)
	if hub.ClientCount() != 0 {
		t.Fatalf("ClientCount() = %d, want 0", hub.ClientCount())
	}

	// Reconnect well within the 300ms retention window.
	time.Sleep(50 * time.Millisecond)
	reconnected, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket.Dial() error = %v", err)
	}
	defer reconnected.Close(websocket.StatusNormalClosure, "")

	if _, _, err := reconnected.Read(ctx); err != nil {
		t.Fatalf("reconnected.Read() connected message error = %v", err)
	}

	readCtx, readCancel := context.WithTimeout(ctx, 1*time.Second)
	defer readCancel()
	_, data, err := reconnected.Read(readCtx)
	if err != nil {
		t.Fatalf("reconnected.Read() state message error = %v", err)
	}
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if msg.Type != MessageSlide || msg.SlideIndex == nil || *msg.SlideIndex != 3 {
		t.Fatalf("message = %+v, want a slide message for slide index 3", msg)
	}
	if msg.Fragment == nil || *msg.Fragment != 1 {
		t.Fatalf("message.Fragment = %v, want 1", msg.Fragment)
	}
	if !msg.Initial {
		t.Fatalf("Initial = %v, want true (this is the hub's register-time state)", msg.Initial)
	}
}

// TestWebSocketHubForgetsStateAfterRetentionExpires verifies that once the
// retention period has actually elapsed with nobody reconnecting, the hub
// forgets its state, same as the original (retention 0) behavior.
func TestWebSocketHubForgetsStateAfterRetentionExpires(t *testing.T) {
	hub := NewWebSocketHub()
	hub.SetStateRetention(100 * time.Millisecond)
	go hub.Run()
	defer hub.Stop()

	server := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	viewer, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket.Dial() viewer error = %v", err)
	}
	if _, _, err := viewer.Read(ctx); err != nil {
		t.Fatalf("viewer.Read() connected message error = %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	slideTwo := `{"type":"slide","slideIndex":1,"fragment":-1,"step":0,"scrollRevealed":false}`
	if err := viewer.Write(ctx, websocket.MessageText, []byte(slideTwo)); err != nil {
		t.Fatalf("viewer.Write() error = %v", err)
	}
	time.Sleep(20 * time.Millisecond)

	viewer.Close(websocket.StatusNormalClosure, "")
	time.Sleep(20 * time.Millisecond)

	// Wait well past the 100ms retention period before reconnecting.
	time.Sleep(300 * time.Millisecond)

	newClient, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket.Dial() error = %v", err)
	}
	defer newClient.Close(websocket.StatusNormalClosure, "")

	if _, _, err := newClient.Read(ctx); err != nil {
		t.Fatalf("newClient.Read() connected message error = %v", err)
	}

	readCtx, readCancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer readCancel()
	if _, _, err := newClient.Read(readCtx); err == nil {
		t.Error("expected no further message once the retention period has expired, but one arrived")
	}
}

// TestWebSocketHubNoLateJoinerStateBeforeAnySlideMessage verifies that a
// client connecting to a hub with no prior slide message receives only the
// "connected" message, so it initializes from its own URL hash as before.
func TestWebSocketHubNoLateJoinerStateBeforeAnySlideMessage(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	server := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket.Dial() error = %v", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("conn.Read() connected message error = %v", err)
	}
	var msg Message
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if msg.Type != MessageConnected {
		t.Fatalf("message type = %v, want %v", msg.Type, MessageConnected)
	}

	// No second message should follow within a short window.
	readCtx, readCancel := context.WithTimeout(ctx, 200*time.Millisecond)
	defer readCancel()
	if _, _, err := conn.Read(readCtx); err == nil {
		t.Error("expected no further message, but one arrived")
	}
}

// TestWebSocketHubRelayStripsInitialFromClientMessages verifies that a
// client cannot forge the register-time "initial" state by including
// "initial":true in its own message: Broadcast unconditionally clears
// Initial before marshaling and relaying, so only the hub's own register
// case (see Run) ever produces a message with Initial set.
func TestWebSocketHubRelayStripsInitialFromClientMessages(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	server := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sender, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket.Dial() sender error = %v", err)
	}
	defer sender.Close(websocket.StatusNormalClosure, "")
	if _, _, err := sender.Read(ctx); err != nil {
		t.Fatalf("sender.Read() connected message error = %v", err)
	}

	receiver, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket.Dial() receiver error = %v", err)
	}
	defer receiver.Close(websocket.StatusNormalClosure, "")
	if _, _, err := receiver.Read(ctx); err != nil {
		t.Fatalf("receiver.Read() connected message error = %v", err)
	}
	// The hub had no state yet, so no late-joiner message follows "connected".

	// Sender sends a message that itself claims to be the initial state.
	forged := `{"type":"slide","slideIndex":5,"fragment":-1,"step":0,"scrollRevealed":false,"initial":true}`
	if err := sender.Write(ctx, websocket.MessageText, []byte(forged)); err != nil {
		t.Fatalf("sender.Write() error = %v", err)
	}

	_, data, err := receiver.Read(ctx)
	if err != nil {
		t.Fatalf("receiver.Read() relayed message error = %v", err)
	}
	var relayed Message
	if err := json.Unmarshal(data, &relayed); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if relayed.Type != MessageSlide || relayed.SlideIndex == nil || *relayed.SlideIndex != 5 {
		t.Fatalf("relayed message = %+v, want a slide message for slide index 5", relayed)
	}
	if relayed.Initial {
		t.Error("relayed.Initial = true, want false - a client-forged initial flag must not survive a relay")
	}
	if strings.Contains(string(data), "initial") {
		t.Errorf("relayed JSON %s contains \"initial\", want it omitted entirely (omitempty, false)", data)
	}
}

// TestWebSocketHubRelayRejectsNegativeSlideIndex verifies that a relayed
// "slide" message with a negative slideIndex is dropped instead of
// broadcast to other clients or retained as lastSlideState.
func TestWebSocketHubRelayRejectsNegativeSlideIndex(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	server := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	sender, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket.Dial() sender error = %v", err)
	}
	defer sender.Close(websocket.StatusNormalClosure, "")
	if _, _, err := sender.Read(ctx); err != nil {
		t.Fatalf("sender.Read() connected message error = %v", err)
	}

	receiver, _, err := websocket.Dial(ctx, wsURL, nil)
	if err != nil {
		t.Fatalf("websocket.Dial() receiver error = %v", err)
	}
	defer receiver.Close(websocket.StatusNormalClosure, "")
	if _, _, err := receiver.Read(ctx); err != nil {
		t.Fatalf("receiver.Read() connected message error = %v", err)
	}

	negative := `{"type":"slide","slideIndex":-1}`
	if err := sender.Write(ctx, websocket.MessageText, []byte(negative)); err != nil {
		t.Fatalf("sender.Write() error = %v", err)
	}

	// A valid message afterward is what actually reaches the receiver;
	// if the negative one had been relayed too, it would arrive first.
	valid := `{"type":"slide","slideIndex":2}`
	if err := sender.Write(ctx, websocket.MessageText, []byte(valid)); err != nil {
		t.Fatalf("sender.Write() error = %v", err)
	}

	_, data, err := receiver.Read(ctx)
	if err != nil {
		t.Fatalf("receiver.Read() error = %v", err)
	}
	var relayed Message
	if err := json.Unmarshal(data, &relayed); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if relayed.SlideIndex == nil || *relayed.SlideIndex != 2 {
		t.Errorf("relayed message = %+v, want the negative slideIndex dropped and only slide 2 relayed", relayed)
	}
}

// TestWebSocketHubPresenterAuthGatesSending covers checkPresenterAuth end to
// end: with no password configured, a connection with no cookie can still
// send; with a password configured, a connection without the matching
// PresenterAuthCookieName is registered and receives broadcasts but its own
// messages are dropped, while a connection that carries the cookie sends
// normally.
func TestWebSocketHubPresenterAuthGatesSending(t *testing.T) {
	dial := func(t *testing.T, wsURL string, cookie string) *websocket.Conn {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		var header http.Header
		if cookie != "" {
			header = http.Header{"Cookie": []string{cookie}}
		}
		conn, _, err := websocket.Dial(ctx, wsURL, &websocket.DialOptions{HTTPHeader: header})
		if err != nil {
			t.Fatalf("websocket.Dial() error = %v", err)
		}
		ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel2()
		if _, _, err := conn.Read(ctx2); err != nil {
			t.Fatalf("conn.Read() connected message error = %v", err)
		}
		return conn
	}

	t.Run("no password configured: an uncookied connection can still send", func(t *testing.T) {
		hub := NewWebSocketHub()
		go hub.Run()
		defer hub.Stop()
		server := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
		defer server.Close()
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

		sender := dial(t, wsURL, "")
		defer sender.Close(websocket.StatusNormalClosure, "")
		receiver := dial(t, wsURL, "")
		defer receiver.Close(websocket.StatusNormalClosure, "")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := sender.Write(ctx, websocket.MessageText, []byte(`{"type":"slide","slideIndex":1}`)); err != nil {
			t.Fatalf("sender.Write() error = %v", err)
		}
		_, data, err := receiver.Read(ctx)
		if err != nil {
			t.Fatalf("receiver.Read() error = %v", err)
		}
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if msg.SlideIndex == nil || *msg.SlideIndex != 1 {
			t.Errorf("relayed message = %+v, want slideIndex 1", msg)
		}
	})

	t.Run("password configured: a connection without the auth cookie cannot send", func(t *testing.T) {
		hub := NewWebSocketHub()
		hub.SetPresenterPassword("secret")
		hub.SetPresenterSessionToken("session-token")
		go hub.Run()
		defer hub.Stop()
		server := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
		defer server.Close()
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

		unauthenticated := dial(t, wsURL, "")
		defer unauthenticated.Close(websocket.StatusNormalClosure, "")
		receiver := dial(t, wsURL, "")
		defer receiver.Close(websocket.StatusNormalClosure, "")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := unauthenticated.Write(ctx, websocket.MessageText, []byte(`{"type":"slide","slideIndex":1}`)); err != nil {
			t.Fatalf("unauthenticated.Write() error = %v", err)
		}
		// The dropped message never arrives; a subsequent authenticated
		// sender's message is what proves the receiver's pipe is still
		// live and nothing from the unauthenticated sender snuck through.
		authenticated := dial(t, wsURL, PresenterAuthCookieName+"=session-token")
		defer authenticated.Close(websocket.StatusNormalClosure, "")
		if err := authenticated.Write(ctx, websocket.MessageText, []byte(`{"type":"slide","slideIndex":2}`)); err != nil {
			t.Fatalf("authenticated.Write() error = %v", err)
		}
		_, data, err := receiver.Read(ctx)
		if err != nil {
			t.Fatalf("receiver.Read() error = %v", err)
		}
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if msg.SlideIndex == nil || *msg.SlideIndex != 2 {
			t.Errorf("relayed message = %+v, want the unauthenticated slide 1 dropped and only slide 2 relayed", msg)
		}
	})

	t.Run("password configured: a connection with the correct auth cookie can send", func(t *testing.T) {
		hub := NewWebSocketHub()
		hub.SetPresenterPassword("secret")
		hub.SetPresenterSessionToken("session-token")
		go hub.Run()
		defer hub.Stop()
		server := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
		defer server.Close()
		wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"

		sender := dial(t, wsURL, PresenterAuthCookieName+"=session-token")
		defer sender.Close(websocket.StatusNormalClosure, "")
		receiver := dial(t, wsURL, "")
		defer receiver.Close(websocket.StatusNormalClosure, "")

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := sender.Write(ctx, websocket.MessageText, []byte(`{"type":"slide","slideIndex":3}`)); err != nil {
			t.Fatalf("sender.Write() error = %v", err)
		}
		_, data, err := receiver.Read(ctx)
		if err != nil {
			t.Fatalf("receiver.Read() error = %v", err)
		}
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("json.Unmarshal() error = %v", err)
		}
		if msg.SlideIndex == nil || *msg.SlideIndex != 3 {
			t.Errorf("relayed message = %+v, want slideIndex 3", msg)
		}
	})
}

// TestWebSocketHubValidSlideIndex verifies validSlideIndex's rules: never
// negative; out of range only rejected once the hub knows the slide count.
func TestWebSocketHubValidSlideIndex(t *testing.T) {
	hub := NewWebSocketHub()

	if hub.validSlideIndex(-1) {
		t.Error("validSlideIndex(-1) = true, want false")
	}
	if !hub.validSlideIndex(99) {
		t.Error("validSlideIndex(99) = false, want true: slide count is unknown, so only negatives are rejected")
	}

	hub.SetPresentationMeta(5, "")
	if hub.validSlideIndex(5) {
		t.Error("validSlideIndex(5) = true, want false: only indexes 0-4 are in range for a 5-slide deck")
	}
	if !hub.validSlideIndex(4) {
		t.Error("validSlideIndex(4) = false, want true: the last slide is in range")
	}
}

// TestWebSocketHubBroadcastStripsThemeFromRetainedSlideState verifies that
// Broadcast never retains a Theme field on a "slide" message, even if the
// message it was given happened to carry one.
func TestWebSocketHubBroadcastStripsThemeFromRetainedSlideState(t *testing.T) {
	hub := NewWebSocketHub()
	slideIndex := 3
	if err := hub.Broadcast(Message{Type: MessageSlide, SlideIndex: &slideIndex, Theme: "dracula"}); err != nil {
		t.Fatalf("Broadcast() error = %v", err)
	}

	hub.mu.RLock()
	state := hub.lastSlideState
	hub.mu.RUnlock()

	if state == nil {
		t.Fatal("lastSlideState is nil, want the broadcast slide message retained")
	}
	if state.Theme != "" {
		t.Errorf("lastSlideState.Theme = %q, want empty", state.Theme)
	}
}

// TestWebSocketHubInitialStateOrdersBeforeConcurrentBroadcast is a stress
// test (run with -race) for the ordering guarantee the register case in Run
// relies on: a newly registered client's own "connected" and Initial
// late-joiner messages are queued to it before any live broadcast racing
// its registration can reach it, because both come from Run's single
// goroutine and a broadcast case cannot run until the register case's body
// (which includes those two sends) has returned. Talks directly to the hub's
// channels and a bare *Client, skipping real websocket connections, so a few
// hundred iterations run fast enough to reliably provoke any remaining race.
func TestWebSocketHubInitialStateOrdersBeforeConcurrentBroadcast(t *testing.T) {
	const iterations = 75
	seedIndex := 2
	liveIndex := 4

	for i := 0; i < iterations; i++ {
		hub := NewWebSocketHub()
		go hub.Run()

		// Seed the hub with existing state before any client of interest
		// exists, so lastSlideState is non-nil when the client below
		// registers.
		if err := hub.Broadcast(Message{Type: MessageSlide, SlideIndex: &seedIndex}); err != nil {
			t.Fatalf("iteration %d: Broadcast() seed error = %v", i, err)
		}

		client := &Client{hub: hub, send: make(chan []byte, 256)}

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			hub.register <- client
		}()
		go func() {
			defer wg.Done()
			_ = hub.Broadcast(Message{Type: MessageSlide, SlideIndex: &liveIndex})
		}()
		wg.Wait()

		// Collect whatever arrives within a short window. The live
		// broadcast above may or may not reach this client (it can lose the
		// race to even be registered in time), but whatever order the
		// messages arrive in must respect the guarantee under test.
		var messages []Message
		// A real delivery lands in microseconds; 30ms is generous headroom
		// without ballooning a 75-iteration loop's runtime waiting out a
		// message that, on some iterations, never arrives at all (the live
		// broadcast losing the race to even be queued before this client
		// registers).
		deadline := time.After(30 * time.Millisecond)
	collect:
		for {
			select {
			case data, ok := <-client.send:
				if !ok {
					break collect
				}
				var msg Message
				if err := json.Unmarshal(data, &msg); err != nil {
					t.Fatalf("iteration %d: json.Unmarshal() error = %v", i, err)
				}
				messages = append(messages, msg)
				if len(messages) >= 4 {
					break collect
				}
			case <-deadline:
				break collect
			}
		}

		hub.Stop()

		if len(messages) == 0 || messages[0].Type != MessageConnected {
			t.Fatalf("iteration %d: messages = %+v, want the first message to be \"connected\"", i, messages)
		}

		sawInitial := false
		for _, msg := range messages[1:] {
			if msg.Type != MessageSlide {
				continue
			}
			if msg.Initial {
				if sawInitial {
					t.Fatalf("iteration %d: more than one Initial message: %+v", i, messages)
				}
				// lastSlideState is set synchronously inside Broadcast, by
				// whichever goroutine's Broadcast call runs first - the seed
				// or the concurrent live one - so the Initial message can
				// legitimately carry either index depending on that race.
				// What must always hold is that it is one of the two, and
				// that it precedes any non-Initial message (checked below).
				if msg.SlideIndex == nil || (*msg.SlideIndex != seedIndex && *msg.SlideIndex != liveIndex) {
					t.Fatalf("iteration %d: Initial message slide index = %v, want %d or %d", i, msg.SlideIndex, seedIndex, liveIndex)
				}
				sawInitial = true
			} else {
				// A live (non-Initial) slide message must never precede the
				// Initial one, when both arrive. Its index can legitimately
				// be either seedIndex or liveIndex: the seed broadcast is
				// only guaranteed to have updated lastSlideState by the time
				// this loop starts, not to have already been drained off
				// the (buffered) broadcast channel to every then-registered
				// client, so it can itself still arrive here as an ordinary
				// live message if this client registered before Run got to
				// it. That is a property of this test's own seeding, not a
				// violation of the guarantee under test.
				if !sawInitial {
					t.Fatalf("iteration %d: a live message arrived before the Initial message: %+v", i, messages)
				}
				if msg.SlideIndex == nil || (*msg.SlideIndex != seedIndex && *msg.SlideIndex != liveIndex) {
					t.Fatalf("iteration %d: live message slide index = %v, want %d or %d", i, msg.SlideIndex, seedIndex, liveIndex)
				}
			}
		}
	}
}

// TestWebSocketHubHandleConnectionDuringShutdown verifies that a connection
// arriving after Stop() has closed h.done does not hang HandleConnection
// forever waiting on h.register, which nothing reads from once Run has
// returned.
func TestWebSocketHubHandleConnectionDuringShutdown(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()

	// Stop the hub and wait for Run to actually return before connecting,
	// so this test exercises the case under test (nothing reading from
	// h.register) rather than racing Run's own shutdown.
	hub.Stop()
	time.Sleep(20 * time.Millisecond)

	server := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/"
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		conn, _, err := websocket.Dial(ctx, wsURL, nil)
		if err == nil {
			conn.Close(websocket.StatusNormalClosure, "")
		}
		close(done)
	}()

	select {
	case <-done:
		// HandleConnection returned instead of blocking forever on h.register.
	case <-time.After(2 * time.Second):
		t.Fatal("HandleConnection did not return during hub shutdown within timeout")
	}
}

func TestSetOnSlideChangeFiresForSlideMessages(t *testing.T) {
	hub := NewWebSocketHub()
	hub.SetPresentationMeta(10, "rev1")

	received := make(chan int, 4)
	hub.SetOnSlideChange(func(slideIndex int) { received <- slideIndex })

	if err := hub.BroadcastSlide(3); err != nil {
		t.Fatalf("BroadcastSlide() returned %v", err)
	}

	select {
	case got := <-received:
		if got != 3 {
			t.Errorf("the listener saw slide %d, want 3", got)
		}
	case <-time.After(time.Second):
		t.Fatal("the listener never fired")
	}
}

func TestSetOnSlideChangeIgnoresOtherMessages(t *testing.T) {
	hub := NewWebSocketHub()

	received := make(chan int, 4)
	hub.SetOnSlideChange(func(slideIndex int) { received <- slideIndex })

	if err := hub.BroadcastTheme("nord"); err != nil {
		t.Fatalf("BroadcastTheme() returned %v", err)
	}
	if err := hub.BroadcastReload(); err != nil {
		t.Fatalf("BroadcastReload() returned %v", err)
	}

	select {
	case got := <-received:
		t.Fatalf("the listener fired with slide %d for a non-slide message", got)
	case <-time.After(100 * time.Millisecond):
	}
}

func TestCurrentSlideReportsTheLastBroadcast(t *testing.T) {
	hub := NewWebSocketHub()
	hub.SetPresentationMeta(10, "rev1")

	if _, ok := hub.CurrentSlide(); ok {
		t.Error("CurrentSlide() claims to know a slide before anything was broadcast")
	}

	if err := hub.BroadcastSlide(4); err != nil {
		t.Fatalf("BroadcastSlide() returned %v", err)
	}

	got, ok := hub.CurrentSlide()
	if !ok {
		t.Fatal("CurrentSlide() does not know the slide after a broadcast")
	}
	if got != 4 {
		t.Errorf("CurrentSlide() = %d, want 4", got)
	}
}

// dialDiskTestHub connects one client to a hub that already broadcast the
// given disk statuses, and returns it.
func dialDiskTestHub(t *testing.T, ctx context.Context, statuses ...string) *websocket.Conn {
	t.Helper()
	hub := NewWebSocketHub()
	go hub.Run()
	t.Cleanup(hub.Stop)

	for _, status := range statuses {
		if err := hub.BroadcastDiskStatus(status); err != nil {
			t.Fatal(err)
		}
	}

	server := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
	t.Cleanup(server.Close)

	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(server.URL, "http")+"/", nil)
	if err != nil {
		t.Fatalf("websocket.Dial() error = %v", err)
	}
	t.Cleanup(func() { conn.Close(websocket.StatusNormalClosure, "") })
	return conn
}

func readHubMessage(t *testing.T, ctx context.Context, conn *websocket.Conn) Message {
	t.Helper()
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	var message Message
	if err := json.Unmarshal(data, &message); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return message
}

func TestWebSocketHubSendsTheDiskStatusToALateJoiner(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn := dialDiskTestHub(t, ctx, "low")

	if first := readHubMessage(t, ctx, conn); first.Type != MessageConnected {
		t.Fatalf("first message = %+v, want connected", first)
	}
	if second := readHubMessage(t, ctx, conn); second.Type != MessageRecording || second.Disk != "low" {
		t.Errorf("second message = %+v, want a low disk status", second)
	}
}

func TestWebSocketHubSendsNoDiskStatusOnceTheDiskIsFine(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	conn := dialDiskTestHub(t, ctx, "low", "")

	if first := readHubMessage(t, ctx, conn); first.Type != MessageConnected {
		t.Fatalf("first message = %+v, want connected", first)
	}
	quiet, stop := context.WithTimeout(ctx, 200*time.Millisecond)
	defer stop()
	if _, data, err := conn.Read(quiet); err == nil {
		t.Errorf("got an unexpected message: %s", data)
	}
}

// dialHub connects to hub through a test server and reads the "connected"
// message, which it returns.
func dialHub(t *testing.T, hub *WebSocketHub) (*websocket.Conn, Message, context.Context) {
	t.Helper()
	testServer := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
	t.Cleanup(testServer.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/", nil)
	if err != nil {
		t.Fatalf("websocket.Dial() error = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(websocket.StatusNormalClosure, "") })

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("reading the connected message: %v", err)
	}
	var connected Message
	if err := json.Unmarshal(data, &connected); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return conn, connected, ctx
}

func TestWebSocketHubBroadcastUpdate(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	conn, _, ctx := dialHub(t, hub)
	time.Sleep(50 * time.Millisecond)

	if err := hub.BroadcastUpdate("r2", []int{2, 5}); err != nil {
		t.Fatalf("BroadcastUpdate() error = %v", err)
	}
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("conn.Read() error = %v", err)
	}
	if string(data) != `{"type":"update","revision":"r2","slides":[2,5]}` {
		t.Errorf("update message = %s", data)
	}
}

func TestWebSocketHubBroadcastUpdateWithNoChangedSlides(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	conn, _, ctx := dialHub(t, hub)
	time.Sleep(50 * time.Millisecond)

	if err := hub.BroadcastUpdate("r3", nil); err != nil {
		t.Fatalf("BroadcastUpdate() error = %v", err)
	}
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("conn.Read() error = %v", err)
	}
	if string(data) != `{"type":"update","revision":"r3","slides":[]}` {
		t.Errorf("update message = %s, want slides as an empty array", data)
	}
}

func TestWebSocketHubConnectedMessageCarriesTheVersion(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	_, before, _ := dialHub(t, hub)
	if before.Version != "" {
		t.Errorf("Version = %q before SetVersion, want empty", before.Version)
	}

	hub.SetVersion("v2.1.0")
	_, after, _ := dialHub(t, hub)
	if after.Version != "v2.1.0" {
		t.Errorf("Version = %q, want %q", after.Version, "v2.1.0")
	}
}

func TestWebSocketHubBroadcastFileChanged(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	conn, _, ctx := dialHub(t, hub)
	time.Sleep(50 * time.Millisecond)

	if err := hub.BroadcastFileChanged("/talks/talk.md"); err != nil {
		t.Fatalf("BroadcastFileChanged() error = %v", err)
	}
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("conn.Read() error = %v", err)
	}
	if string(data) != `{"type":"file-changed","path":"/talks/talk.md"}` {
		t.Errorf("message = %s", data)
	}
}

func TestWebSocketHubCurrentPosition(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	if _, _, known := hub.CurrentPosition(); known {
		t.Error("the position is known before any slide message")
	}

	slide, step := 3, 2
	if err := hub.Broadcast(Message{Type: MessageSlide, SlideIndex: &slide, Step: &step}); err != nil {
		t.Fatal(err)
	}
	if index, gotStep, known := hub.CurrentPosition(); !known || index != 3 || gotStep != 2 {
		t.Errorf("CurrentPosition() = %d, %d, %v; want 3, 2, true", index, gotStep, known)
	}

	next := 4
	if err := hub.Broadcast(Message{Type: MessageSlide, SlideIndex: &next}); err != nil {
		t.Fatal(err)
	}
	if index, gotStep, _ := hub.CurrentPosition(); index != 4 || gotStep != 0 {
		t.Errorf("a slide message without a step: CurrentPosition() = %d, %d; want 4, 0", index, gotStep)
	}
}
