package cli

import (
	"context"
	"fmt"
	"sync"

	"github.com/MiniCodeMonkey/tap/internal/server"
	"github.com/MiniCodeMonkey/tap/internal/tunnel"
)

// tunnelController owns the one tunnel a dev server may have, and keeps the
// Host header allow-list in step with it.
//
// The dev server refuses a request whose Host header is not local (its DNS
// rebinding defense), and a tunnel arrives as exactly such a Host:
// something.trycloudflare.com. So starting a tunnel has to add that name to
// the allow-list, and stopping one has to take it away again, or a stale
// name stays accepted for the rest of the session.
type tunnelController struct {
	mu sync.Mutex

	port        int
	baseOrigins []string
	srv         *server.Server
	hub         *server.WebSocketHub

	active *tunnel.Tunnel
}

func newTunnelController(port int, baseOrigins []string, srv *server.Server, hub *server.WebSocketHub) *tunnelController {
	return &tunnelController{port: port, baseOrigins: baseOrigins, srv: srv, hub: hub}
}

// Available reports whether a tunnel could be started at all.
func (c *tunnelController) Available() bool { return tunnel.Available() }

// InstallHint is the line to show when it is not.
func (c *tunnelController) InstallHint() string { return tunnel.InstallHint() }

// URL is the public address, or "" when no tunnel is running.
func (c *tunnelController) URL() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.active.URL()
}

// Start brings a tunnel up and returns its public URL. Starting one while
// one is already running returns the running one.
func (c *tunnelController) Start(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.active != nil {
		return c.active.URL(), nil
	}

	started, err := tunnel.Start(ctx, c.port)
	if err != nil {
		return "", err
	}

	c.active = started
	c.applyOriginsLocked()
	return started.URL(), nil
}

// Stop takes the tunnel down and narrows the allow-list back.
func (c *tunnelController) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.active == nil {
		return nil
	}

	err := c.active.Stop()
	c.active = nil
	c.applyOriginsLocked()
	return err
}

// applyOriginsLocked pushes the current allow-list to both the HTTP server
// and the websocket hub. Caller holds the lock.
func (c *tunnelController) applyOriginsLocked() {
	origins := make([]string, len(c.baseOrigins), len(c.baseOrigins)+2)
	copy(origins, c.baseOrigins)

	if c.active != nil {
		// Both forms: the Host header arrives bare, an Origin header
		// arrives with the scheme.
		origins = append(origins, c.active.Host(), c.active.URL())
	}

	tunnelHost := ""
	if c.active != nil {
		tunnelHost = c.active.Host()
	}
	if c.srv != nil {
		c.srv.SetAllowedOrigins(origins)
		c.srv.SetTunnelHost(tunnelHost)
	}
	if c.hub != nil {
		c.hub.SetAllowedOrigins(origins)
	}
}

// notInstalledError is what the user sees when --tunnel is passed on a
// machine without cloudflared. It names the one command to run.
func notInstalledError() error {
	return fmt.Errorf("--tunnel needs cloudflared, which is not installed.\n\n    %s\n\nThen run this again", tunnel.InstallHint())
}
