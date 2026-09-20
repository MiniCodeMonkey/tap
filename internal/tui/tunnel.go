package tui

import (
	"context"
	"net/url"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MiniCodeMonkey/tap/internal/server"
)

// TunnelController is the slice of tunnel handling the dev TUI needs. The
// CLI supplies the real one; the exec and the allow-list live there, so
// this file stays about keys and rendering.
type TunnelController interface {
	Start(ctx context.Context) (string, error)
	Stop() error
	URL() string
	Available() bool
	InstallHint() string
}

// tunnelStartTimeout bounds the wait a keypress can cause. A Quick Tunnel
// is usually up in a few seconds.
const tunnelStartTimeout = 45 * time.Second

// tunnelMsg reports the outcome of a start or stop to the update loop.
type tunnelMsg struct {
	url     string
	err     error
	stopped bool
}

// SetTunnelController gives the TUI a way to start and stop a tunnel.
func (m *DevModel) SetTunnelController(controller TunnelController) {
	m.tunnels = controller
}

// toggleTunnel is the U key: start one, or take the running one down.
func (m *DevModel) toggleTunnel() (*DevModel, tea.Cmd) {
	if m.tunnels == nil || m.tunnelStarting {
		return m, nil
	}

	if m.tunnelURL != "" {
		controller := m.tunnels
		m.tunnelURL = ""
		m.tunnelQR = ""
		m.addEvent(DevEvent{
			Type:      "action",
			Message:   "Tunnel stopped",
			Timestamp: time.Now(),
		})
		return m, func() tea.Msg {
			err := controller.Stop()
			return tunnelMsg{stopped: true, err: err}
		}
	}

	if !m.tunnels.Available() {
		m.addEvent(DevEvent{
			Type:      "error",
			Message:   "Tunnel needs cloudflared: " + m.tunnels.InstallHint(),
			Timestamp: time.Now(),
		})
		return m, nil
	}

	m.tunnelStarting = true
	m.addEvent(DevEvent{
		Type:      "action",
		Message:   "Starting tunnel...",
		Timestamp: time.Now(),
	})

	controller := m.tunnels
	return m, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), tunnelStartTimeout)
		defer cancel()

		url, err := controller.Start(ctx)
		return tunnelMsg{url: url, err: err}
	}
}

// applyTunnelMsg folds a start or stop outcome back into the model.
func (m *DevModel) applyTunnelMsg(msg tunnelMsg) *DevModel {
	m.tunnelStarting = false

	if msg.err != nil {
		m.addEvent(DevEvent{
			Type:      "error",
			Message:   "Tunnel failed: " + msg.err.Error(),
			Timestamp: time.Now(),
		})
		return m
	}

	if msg.stopped {
		return m
	}

	m.tunnelURL = msg.url
	m.tunnelQR = tunnelQRCode(presenterTarget(msg.url, m.config.PresenterPassword))
	m.addEvent(DevEvent{
		Type:      "action",
		Message:   "Tunnel up: " + msg.url,
		Timestamp: time.Now(),
	})
	return m
}

// qrCode is the QR block to draw: the tunnel's, or the one the config
// carried in. Scanning a LAN URL off the terminal is just as useful as
// scanning a tunnel URL, so both are honoured.
func (m *DevModel) qrCode() string {
	if m.tunnelQR != "" {
		return m.tunnelQR
	}
	return m.config.QRCodeASCII
}

// presenterTarget is where a scanned QR code should land: the presenter
// view, which carries the speaker notes and the controls, and which links
// on to the slides themselves. A password rides along, since the view is
// gated without it.
func presenterTarget(tunnelURL, presenterPassword string) string {
	if tunnelURL == "" {
		return ""
	}

	target := strings.TrimSuffix(tunnelURL, "/") + "/presenter"
	if presenterPassword != "" {
		target += "?key=" + url.QueryEscape(presenterPassword)
	}
	return target
}

// tunnelQRCode renders a URL as a scannable block-character QR code, or ""
// when there is no URL or it cannot be drawn. A QR code that fails to
// render is not worth an error: the URL itself is still on screen.
func tunnelQRCode(url string) string {
	if url == "" {
		return ""
	}

	code, err := server.GenerateCompactQRCode(url)
	if err != nil {
		return ""
	}
	return strings.TrimRight(code, "\n")
}

// qrMinimumHeight is the window height below which a QR code is left out
// entirely: its own lines, plus room for everything else on the screen.
// Showing a code that scrolls off the top is no better than showing none.
func qrMinimumHeight(code string) int {
	// Measured against the real screen: title, file, four URL lines, three
	// status lines, the section's own label, the help footer, and the
	// blank lines between them. The events list shrinks on its own, so it
	// is not counted.
	const chromeLines = 16
	return strings.Count(code, "\n") + chromeLines
}
