// Package server provides HTTP server functionality for the dev server.
package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// PresenterAuthCookieName is the cookie handlePresenter sets once a client
// proves it knows the configured --presenter-password (see routes.go), and
// that HandleConnection reads back to decide whether a WebSocket connection
// may drive other clients (see WebSocketHub.SetPresenterPassword). Path=/
// on that cookie is what makes it ride along on the browser's /ws upgrade
// request too, alongside /presenter itself.
const PresenterAuthCookieName = "tap_presenter_key"

// MessageType represents the type of WebSocket message.
type MessageType string

const (
	// MessageConnected is sent when a client connects.
	MessageConnected MessageType = "connected"
	// MessageReload signals clients to reload the page.
	MessageReload MessageType = "reload"
	// MessageSlide signals clients to navigate to a specific slide.
	MessageSlide MessageType = "slide"
	// MessageTheme signals clients to switch to a specific theme.
	MessageTheme MessageType = "theme"
	// MessageRecording carries the recording's disk status to every
	// client, for the low disk badge.
	MessageRecording MessageType = "recording"
)

// Message represents a WebSocket message sent between server and clients.
// A slide message's SlideIndex, Fragment, Step and ScrollRevealed fields are
// optional: when absent, the message means "this slide, initial state"
// exactly as it did before these fields existed, which keeps an old client
// that ignores them working and lets a new client tell "no fragment
// revealed" (a present Fragment of -1) apart from "field not sent" (a nil
// Fragment). Pointers are required here because plain ints and bools can't
// distinguish a real zero value, such as slide index 0, fragment 0 or step
// 0, from an omitted field.
//
// Initial marks a slide message as the register-time state Run's register
// case sends a newly connected client (see lastSlideState below): only the
// hub itself ever sets it, and only on that one message. A client uses it,
// not message order, to tell the hub's late-joiner state apart from a live
// navigation broadcast by another client - the only message order actually
// guarantees is that this one, if sent at all, arrives before any broadcast
// concurrent with registration (see the register case in Run). Broadcast
// strips it from every message it sends out, so a client-sent "initial"
// (accidental or otherwise) never survives a relay to other clients.
//
// Revision is set only on the "connected" message Run's register case sends
// at register time, to the hub's current deck revision (see
// WebSocketHub.SetPresentationMeta). A client compares it across
// reconnects, on the same page load, to notice the deck changed while its
// socket was down and reload (see frontend/src/lib/stores/websocket.ts).
// Fields ordered by size for memory alignment.
type Message struct {
	Type     MessageType `json:"type"`
	Theme    string      `json:"theme,omitempty"`
	Revision string      `json:"revision,omitempty"`
	// Disk is set only on a "recording" message: "low", "full", or absent
	// when the disk is fine.
	Disk           string `json:"disk,omitempty"`
	SlideIndex     *int   `json:"slideIndex,omitempty"`
	Fragment       *int   `json:"fragment,omitempty"`
	Step           *int   `json:"step,omitempty"`
	ScrollRevealed *bool  `json:"scrollRevealed,omitempty"`
	Initial        bool   `json:"initial,omitempty"`
}

// Client represents a connected WebSocket client.
type Client struct {
	hub  *WebSocketHub
	conn *websocket.Conn
	send chan []byte
	// canSend is whether this client's relayed "slide" and "theme"
	// messages are broadcast to other clients (see readPump), rather than
	// silently dropped. True when no presenter password is configured (see
	// checkPresenterAuth), or when the connection carried a valid
	// PresenterAuthCookieName. A client with canSend false still registers
	// normally and receives every broadcast - it just cannot drive other
	// clients, so an audience window without the password still navigates
	// its own view locally without moving anyone else's.
	canSend bool
}

// ClientCountCallback is called when the number of connected clients changes.
type ClientCountCallback func(count int)

// SlideChangeCallback is called with the slide index every time one is
// broadcast, whatever client moved it. The recorder uses it to build a
// chapter list; the hub itself knows nothing about recording.
type SlideChangeCallback func(slideIndex int)

// WebSocketHub manages WebSocket connections and message broadcasting.
type WebSocketHub struct {
	clients             map[*Client]bool
	broadcast           chan []byte
	register            chan *Client
	unregister          chan *Client
	done                chan struct{}
	onClientCountChange ClientCountCallback
	onSlideChange       SlideChangeCallback
	// lastSlideState is the most recently broadcast "slide" message. A
	// client that registers after the talk is underway (a presenter window
	// opened mid-talk, or a viewer that reconnects) is sent a copy of this,
	// marked Initial, right after it registers (see the register case in
	// Run), so it lands on the deck's current slide and fragment state
	// instead of slide 1. Stored as a Message, not pre-marshaled bytes,
	// because the copy sent to a new client needs Initial set while the
	// stored value itself (and anything broadcast) never has it set. nil
	// until the first slide message is broadcast, and reset to nil again
	// once stateRetention has passed since every client disconnected (see
	// scheduleForgetLocked and the unregister case in Run): with nobody
	// left watching, there is no live "current state" to hand off forever,
	// but a reload of the only open window (which briefly drops the hub to
	// zero clients before the same window reconnects) should not lose it
	// either, hence the grace period rather than forgetting immediately.
	lastSlideState *Message
	// stateRetention is how long lastSlideState survives after the last
	// client disconnects, before scheduleForgetLocked's timer clears it. A
	// zero or negative value forgets it immediately, with no retention.
	stateRetention time.Duration
	// retentionTimer is the pending "forget the state" timer started when
	// the last client disconnects, or nil when no such timer is pending
	// (nobody has disconnected down to zero clients since the timer last
	// fired or was canceled). A client reconnecting before it fires stops
	// it, canceling the forget.
	retentionTimer *time.Timer
	// slideCount is the presentation's current slide count, used to reject
	// a relayed "slide" message whose slideIndex is out of range. Zero
	// means "unknown" (no presentation set yet), in which case only a
	// negative slideIndex is rejected; the frontend already clamps an
	// in-range-but-stale index itself (see applyRemoteState in
	// frontend/src/lib/stores/websocket.ts).
	slideCount int
	// revision is the current deck's content hash (see ComputeRevision),
	// sent as the Revision field of the "connected" message at register
	// time. Empty until SetPresentationMeta is called at least once, in
	// which case "connected" carries no revision at all and a client never
	// reloads off its first connection (see the frontend's handling of an
	// absent revision in frontend/src/lib/stores/websocket.ts).
	revision string
	// diskStatus is the last disk status broadcast, sent again to each
	// client that connects later so a reloaded window still shows it.
	diskStatus string
	// allowedOrigins holds the extra origins a WebSocket upgrade is
	// accepted from, beyond same-host connections - the tap dev
	// --allow-origin flag, for a contributor's Vite dev server running on
	// another port (see checkOrigin). Keyed by the full origin string
	// (e.g. "http://localhost:5173") as sent in the Origin header.
	allowedOrigins map[string]struct{}
	// allowedHosts holds the same --allow-origin values, reduced to bare
	// hosts (see allowedHostsFromOrigins), checked by isAllowedHost against
	// a request's own Host header and an Origin header's host - the extra
	// defense against DNS rebinding a same-host compare alone cannot
	// provide (see checkOrigin and requireAllowedHost).
	allowedHosts map[string]struct{}
	// presenterPassword mirrors the dev server's --presenter-password (see
	// SetPresenterPassword): empty means nothing is protected, so every
	// connection can send. Non-empty means a connection may only send once
	// it presents PresenterAuthCookieName equal to presenterSessionToken
	// (see checkPresenterAuth), matching the cookie handlePresenter issues
	// after the presenter page's own ?key= check passes.
	presenterPassword string
	// presenterSessionToken is the random per-process token the presenter
	// auth cookie carries (see Server.GeneratePresenterSessionToken); the
	// dev command sets the same value here and on every candidate Server so
	// a cookie either of them issues validates.
	presenterSessionToken string
	mu                    sync.RWMutex
}

// DefaultStateRetention is how long the hub keeps the last-known slide
// state after the last client disconnects, before forgetting it, unless
// SetStateRetention overrides it.
const DefaultStateRetention = 10 * time.Minute

// NewWebSocketHub creates a new WebSocket hub with the default state
// retention period (DefaultStateRetention).
func NewWebSocketHub() *WebSocketHub {
	return &WebSocketHub{
		clients:        make(map[*Client]bool),
		broadcast:      make(chan []byte, 256),
		register:       make(chan *Client),
		unregister:     make(chan *Client),
		done:           make(chan struct{}),
		stateRetention: DefaultStateRetention,
	}
}

// SetStateRetention sets how long the hub keeps its last-known slide state
// after the last client disconnects. Safe to call at any time, including
// while a forget is already pending (it does not itself reschedule a
// pending timer, only affects the next time the hub reaches zero clients).
// A zero or negative duration makes the hub forget the state immediately
// on disconnect.
func (h *WebSocketHub) SetStateRetention(d time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.stateRetention = d
}

// SetPresentationMeta tells the hub about the deck it is currently serving:
// slideCount, so a relayed "slide" message with an out-of-range slideIndex
// can be rejected instead of broadcast, and revision (see ComputeRevision),
// sent as the Revision field of every "connected" message from then on so a
// reconnecting client can tell the deck changed while its socket was down.
// Safe to call at any time, including before Run starts or while clients
// are connected (tap dev calls it again on every reload).
func (h *WebSocketHub) SetPresentationMeta(slideCount int, revision string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.slideCount = slideCount
	h.revision = revision
}

// validSlideIndex reports whether slideIndex is acceptable in a relayed
// "slide" message: never negative, and, when the hub knows the slide
// count, not past the last slide either.
func (h *WebSocketHub) validSlideIndex(slideIndex int) bool {
	if slideIndex < 0 {
		return false
	}
	h.mu.RLock()
	count := h.slideCount
	h.mu.RUnlock()
	if count > 0 && slideIndex >= count {
		return false
	}
	return true
}

// SetAllowedOrigins sets the extra origins the hub accepts a WebSocket
// upgrade from, beyond an origin that already matches the request's own
// Host header (see checkOrigin). Each entry is a full origin such as
// "http://localhost:5173", matched exactly against the incoming Origin
// header. This is what tap dev's repeatable --allow-origin flag feeds, for
// a contributor's Vite dev server proxying to this hub from another port.
func (h *WebSocketHub) SetAllowedOrigins(origins []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.allowedOrigins = make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		h.allowedOrigins[origin] = struct{}{}
	}
	h.allowedHosts = allowedHostsFromOrigins(origins)
}

// checkOrigin reports whether r is an acceptable WebSocket upgrade request
// given its Origin header:
//   - No Origin header at all: accepted. Only a browser sends one, so this
//     covers non-browser clients and tests. (HandleConnection separately
//     enforces the Host allow-list on every request, browser or not.)
//   - An Origin whose host (host and port) equals the request's own Host
//     header, and that host is itself on the allow-list (see
//     isAllowedHost): accepted. This covers localhost, 127.0.0.1, the LAN
//     address a presenter opens tap dev from on a phone or second laptop,
//     and whatever fallback port tap dev bound when its default was busy -
//     all of them addressed with the same host the browser used to reach
//     this server in the first place. The extra isAllowedHost check is
//     what stops DNS rebinding: without it, a hostile domain that
//     resolves to 127.0.0.1 would make Origin and Host equal for any name
//     an attacker picks.
//   - An Origin explicitly listed via SetAllowedOrigins: accepted. This is
//     the --allow-origin flag's list, for a dev server proxying in from
//     elsewhere (the Vite dev server contributors run on another port).
//
// Anything else is rejected.
func (h *WebSocketHub) checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	h.mu.RLock()
	allowedHosts := h.allowedHosts
	_, exactlyAllowed := h.allowedOrigins[origin]
	h.mu.RUnlock()

	if originURL, err := url.Parse(origin); err == nil && originURL.Host == r.Host && isAllowedHost(r.Host, allowedHosts) {
		return true
	}

	return exactlyAllowed
}

// SetPresenterPassword tells the hub the dev server's current
// --presenter-password, so HandleConnection can decide whether a new
// connection may send (see checkPresenterAuth). An empty password (the
// default, and tap dev's default) leaves every connection able to send,
// with nothing gated. Safe to call at any time.
func (h *WebSocketHub) SetPresenterPassword(password string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.presenterPassword = password
}

// SetPresenterSessionToken sets the per-process token checkPresenterAuth
// compares a connection's cookie against (see
// Server.GeneratePresenterSessionToken). The dev command calls this with
// the same token it sets on every candidate Server.
func (h *WebSocketHub) SetPresenterSessionToken(token string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.presenterSessionToken = token
}

// checkPresenterAuth reports whether r may send navigation messages once
// connected: always true when no presenter password is configured, and
// otherwise true only when r carries PresenterAuthCookieName equal to the
// current presenter session token - proof this browser already passed the
// same check the presenter page's ?key= requires (see handlePresenter in
// routes.go). The compare runs in constant time, and against the random
// session token rather than the password itself, since Go's cookie jar
// sanitizes cookie values and would silently change a password containing
// a semicolon, quote, backslash, space, or non-ASCII character before it
// ever reached this compare. A connection that fails this still registers
// and receives every broadcast; it just cannot send one (see
// Client.canSend).
func (h *WebSocketHub) checkPresenterAuth(r *http.Request) bool {
	h.mu.RLock()
	password := h.presenterPassword
	sessionToken := h.presenterSessionToken
	h.mu.RUnlock()

	if password == "" {
		return true
	}
	if sessionToken == "" {
		return false
	}

	cookie, err := r.Cookie(PresenterAuthCookieName)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(sessionToken)) == 1
}

// scheduleForgetLocked arranges for the hub to forget lastSlideState
// after stateRetention, unless a client reconnects first (the register
// case in Run stops this timer). Must be called with h.mu held, and only
// once the hub has reached zero clients.
func (h *WebSocketHub) scheduleForgetLocked() {
	if h.retentionTimer != nil {
		h.retentionTimer.Stop()
		h.retentionTimer = nil
	}
	if h.stateRetention <= 0 {
		h.lastSlideState = nil
		return
	}
	h.retentionTimer = time.AfterFunc(h.stateRetention, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		// A client may have reconnected just as this fired, racing the
		// register case's Stop() call; only forget if the hub is still at
		// zero clients under this same lock.
		if len(h.clients) == 0 {
			h.lastSlideState = nil
		}
		h.retentionTimer = nil
	})
}

// Run starts the hub's main event loop.
// It should be started as a goroutine.
func (h *WebSocketHub) Run() {
	for {
		select {
		case <-h.done:
			// Close all client connections
			h.mu.Lock()
			if h.retentionTimer != nil {
				h.retentionTimer.Stop()
				h.retentionTimer = nil
			}
			for client := range h.clients {
				close(client.send)
				delete(h.clients, client)
			}
			h.mu.Unlock()
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			// A reconnect within the retention window cancels the pending
			// forget: the state it's about to be sent below is exactly what
			// would otherwise be erased.
			if h.retentionTimer != nil {
				h.retentionTimer.Stop()
				h.retentionTimer = nil
			}
			var initialData []byte
			if h.lastSlideState != nil {
				initialMsg := *h.lastSlideState
				initialMsg.Initial = true
				initialData, _ = json.Marshal(initialMsg)
			}
			revision := h.revision
			var diskData []byte
			if h.diskStatus != "" {
				diskData, _ = json.Marshal(Message{Type: MessageRecording, Disk: h.diskStatus})
			}
			h.notifyClientCountChange()
			h.mu.Unlock()

			// Queue "connected" and, if any, the late-joiner state (marked
			// Initial) from this same goroutine, before this case returns to
			// select: Run is the only writer of client.send once a client is
			// registered (HandleConnection never writes to it directly), so
			// nothing sent here can ever be interleaved with, or arrive
			// after, a broadcast concurrent with this registration - the
			// broadcast case below can't run until this case's body
			// finishes. That's what guarantees a client always receives its
			// own late-joiner state before any live navigation broadcast by
			// another client racing its connection.
			//
			// Revision rides on this same "connected" message rather than a
			// separate one: a client only needs to compare it across
			// distinct connections (see hasSeenFirstRevision in the
			// frontend), and "connected" already fires exactly once per
			// connection.
			connectedMsg, _ := json.Marshal(Message{Type: MessageConnected, Revision: revision})
			select {
			case client.send <- connectedMsg:
			default:
			}
			if initialData != nil {
				select {
				case client.send <- initialData:
				default:
				}
			}
			if diskData != nil {
				select {
				case client.send <- diskData:
				default:
				}
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
				h.notifyClientCountChange()
				// Once nobody is watching, the state isn't handed off
				// immediately, but it isn't kept forever either: after
				// stateRetention with nobody reconnecting, there is no
				// "current state" left to hand off, so scheduleForgetLocked
				// clears it and the next connection starts from its own URL
				// hash instead of wherever the deck was left days ago.
				if len(h.clients) == 0 {
					h.scheduleForgetLocked()
				}
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

// SetOnClientCountChange sets a callback to be called when the client count changes.
func (h *WebSocketHub) SetOnClientCountChange(callback ClientCountCallback) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onClientCountChange = callback
}

// notifyClientCountChange calls the callback with the current client count.
// Must be called with the lock held.
func (h *WebSocketHub) notifyClientCountChange() {
	if h.onClientCountChange != nil {
		count := len(h.clients)
		// Call callback without lock to avoid deadlocks
		callback := h.onClientCountChange
		go callback(count)
	}
}

// SetOnSlideChange sets a callback to be called when a slide is broadcast.
func (h *WebSocketHub) SetOnSlideChange(callback SlideChangeCallback) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onSlideChange = callback
}

// CurrentSlide is the slide the deck is on, and whether that is known at
// all. It is not known before the first slide message, or after the hub has
// forgotten the state because nobody was connected.
func (h *WebSocketHub) CurrentSlide() (int, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.lastSlideState == nil || h.lastSlideState.SlideIndex == nil {
		return 0, false
	}
	return *h.lastSlideState.SlideIndex, true
}

// Broadcast sends a message to all connected clients. Initial is always
// cleared first, whether this call originated internally (BroadcastSlide,
// BroadcastTheme) or from relaying a client's own message (readPump): only
// the register case in Run, sending a client its own late-joiner state, is
// allowed to set it, so a client can trust Initial as "the hub's state on
// register", never "a peer happened to send this flag".
func (h *WebSocketHub) Broadcast(msg Message) error {
	msg.Initial = false

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	if msg.Type == MessageSlide {
		stateCopy := msg
		// A slide message carries no theme of its own; strip any Theme a
		// relayed client message happened to set so the retained state
		// never claims one.
		stateCopy.Theme = ""
		h.mu.Lock()
		h.lastSlideState = &stateCopy
		callback := h.onSlideChange
		h.mu.Unlock()

		// The listener runs on its own goroutine: a slow one must not
		// hold up the broadcast that puts the slide on screen. That also
		// means two slide changes in quick succession give the listener no
		// ordering guarantee at all; recorder.Chapters.Add's clamp against
		// an out-of-order timestamp exists only because of this.
		if callback != nil && stateCopy.SlideIndex != nil {
			slideIndex := *stateCopy.SlideIndex
			go callback(slideIndex)
		}
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
	return h.Broadcast(Message{Type: MessageSlide, SlideIndex: &slideIndex})
}

// BroadcastTheme sends a theme change message to all clients.
func (h *WebSocketHub) BroadcastTheme(themeName string) error {
	return h.Broadcast(Message{Type: MessageTheme, Theme: themeName})
}

// BroadcastDiskStatus tells every client how full the recordings disk is,
// and remembers it for clients that connect later.
func (h *WebSocketHub) BroadcastDiskStatus(status string) error {
	h.mu.Lock()
	h.diskStatus = status
	h.mu.Unlock()
	return h.Broadcast(Message{Type: MessageRecording, Disk: status})
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
	h.mu.RLock()
	allowedHosts := h.allowedHosts
	h.mu.RUnlock()
	if !isAllowedHost(r.Host, allowedHosts) {
		log.Printf("rejected websocket connection with Host %q: not a local, private, or allowed host", r.Host)
		http.Error(w, "Forbidden: host not allowed; use --allow-origin to allow it", http.StatusForbidden)
		return
	}

	if !h.checkOrigin(r) {
		log.Printf("rejected websocket connection from origin %q: not this server's host and not allowed by --allow-origin", r.Header.Get("Origin"))
		http.Error(w, "Forbidden: origin not allowed", http.StatusForbidden)
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// checkOrigin above already enforced the origin rules tap dev
		// wants; skip the library's own, less flexible host-pattern check.
		InsecureSkipVerify: true,
	})
	if err != nil {
		return
	}

	client := &Client{
		hub:     h,
		conn:    conn,
		send:    make(chan []byte, 256),
		canSend: h.checkPresenterAuth(r),
	}

	// Registering also queues the "connected" message and, if the hub has
	// one, the late-joiner state (marked Initial) - see the register case
	// in Run. Both happen from Run's own goroutine, before this call
	// returns control here, which is what guarantees they reach the client
	// ahead of any live broadcast racing this registration: writing them
	// here instead, from this goroutine, would race the broadcast case in
	// Run writing to the same client.send from a different goroutine, with
	// no guarantee which arrived first. A brand new hub with no slide
	// message broadcast yet sends only "connected", leaving this client to
	// initialize from its own URL hash.
	//
	// Selecting on h.done alongside the send matters once the hub has
	// stopped: Run's loop has returned by then, so nothing ever receives
	// from h.register again, and an unconditional send would block this
	// goroutine forever. A connection arriving during shutdown gets no
	// "connected" message and its readPump/writePump never start; the
	// deferred conn.Close below still runs to tell the client goodbye.
	select {
	case h.register <- client:
	case <-h.done:
		conn.Close(websocket.StatusGoingAway, "server shutting down")
		return
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
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			// Connection closed or error
			return
		}

		// Parse and broadcast client messages to other clients
		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			continue // Ignore invalid JSON
		}

		// A client without canSend (a presenter password is configured and
		// this connection never proved it) still receives every broadcast,
		// but its own slide and theme messages are dropped here instead of
		// relayed - it can navigate its own view locally, but never drives
		// anyone else's.
		if !c.canSend {
			continue
		}

		// Broadcast slide and theme messages to all clients. A "slide"
		// message with a negative index, or (when the hub knows the slide
		// count) an index past the last slide, is dropped instead of
		// relayed and retained as lastSlideState.
		switch msg.Type {
		case MessageSlide:
			if msg.SlideIndex == nil || !c.hub.validSlideIndex(*msg.SlideIndex) {
				continue
			}
			_ = c.hub.Broadcast(msg)
		case MessageTheme:
			_ = c.hub.Broadcast(msg)
		}
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
