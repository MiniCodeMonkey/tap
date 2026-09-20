package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// fakeTunnel stands in for the CLI's controller.
type fakeTunnel struct {
	url         string
	startErr    error
	available   bool
	startCalls  int
	stopCalls   int
	installHint string
}

func (f *fakeTunnel) Start(context.Context) (string, error) {
	f.startCalls++
	if f.startErr != nil {
		return "", f.startErr
	}
	return f.url, nil
}

func (f *fakeTunnel) Stop() error {
	f.stopCalls++
	return nil
}

func (f *fakeTunnel) URL() string         { return f.url }
func (f *fakeTunnel) Available() bool     { return f.available }
func (f *fakeTunnel) InstallHint() string { return f.installHint }

func pressU(t *testing.T, m *DevModel) tea.Cmd {
	t.Helper()
	_, cmd := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	return cmd
}

func TestTunnelKeyStartsOne(t *testing.T) {
	fake := &fakeTunnel{url: "https://calm-river.trycloudflare.com", available: true}
	m := NewDevModel(DevConfig{})
	m.SetTunnelController(fake)

	cmd := pressU(t, m)
	if cmd == nil {
		t.Fatal("pressing u returned no command, so nothing would start")
	}
	if !m.tunnelStarting {
		t.Error("the model does not show a tunnel starting")
	}

	msg, ok := cmd().(tunnelMsg)
	if !ok {
		t.Fatalf("command returned %T, want tunnelMsg", cmd())
	}
	m.applyTunnelMsg(msg)

	if fake.startCalls != 1 {
		t.Errorf("Start called %d times, want 1", fake.startCalls)
	}
	if m.tunnelURL != fake.url {
		t.Errorf("tunnelURL = %q, want %q", m.tunnelURL, fake.url)
	}
	if m.tunnelStarting {
		t.Error("the model still shows a tunnel starting after it came up")
	}
	if m.tunnelQR == "" {
		t.Error("no QR code was rendered for the tunnel URL")
	}
}

func TestTunnelKeyStopsARunningOne(t *testing.T) {
	fake := &fakeTunnel{url: "https://calm-river.trycloudflare.com", available: true}
	m := NewDevModel(DevConfig{TunnelURL: fake.url})
	m.SetTunnelController(fake)

	cmd := pressU(t, m)
	if cmd == nil {
		t.Fatal("pressing u with a tunnel up returned no command")
	}
	cmd()

	if fake.stopCalls != 1 {
		t.Errorf("Stop called %d times, want 1", fake.stopCalls)
	}
	if m.tunnelURL != "" {
		t.Errorf("tunnelURL = %q after stopping, want an empty string", m.tunnelURL)
	}
	if m.tunnelQR != "" {
		t.Error("the QR code outlived the tunnel")
	}
}

func TestTunnelKeySaysWhatToInstall(t *testing.T) {
	fake := &fakeTunnel{available: false, installHint: "brew install cloudflared"}
	m := NewDevModel(DevConfig{})
	m.SetTunnelController(fake)

	if cmd := pressU(t, m); cmd != nil {
		t.Error("pressing u without cloudflared returned a command, want none")
	}
	if fake.startCalls != 0 {
		t.Error("Start was called without cloudflared available")
	}

	var found bool
	for _, event := range m.state.RecentEvents {
		if strings.Contains(event.Message, "brew install cloudflared") {
			found = true
		}
	}
	if !found {
		t.Errorf("no event told the user what to install; events: %v", m.state.RecentEvents)
	}
}

func TestTunnelFailureIsReportedAndRecoverable(t *testing.T) {
	fake := &fakeTunnel{available: true, startErr: errors.New("no route to host")}
	m := NewDevModel(DevConfig{})
	m.SetTunnelController(fake)

	cmd := pressU(t, m)
	m.applyTunnelMsg(cmd().(tunnelMsg))

	if m.tunnelStarting {
		t.Error("the model is still starting after a failure, so u would do nothing again")
	}
	if m.tunnelURL != "" {
		t.Errorf("tunnelURL = %q after a failure, want an empty string", m.tunnelURL)
	}

	var found bool
	for _, event := range m.state.RecentEvents {
		if strings.Contains(event.Message, "no route to host") {
			found = true
		}
	}
	if !found {
		t.Error("the failure was not reported to the user")
	}
}

func TestTunnelKeyIgnoredWhileStarting(t *testing.T) {
	fake := &fakeTunnel{url: "https://calm-river.trycloudflare.com", available: true}
	m := NewDevModel(DevConfig{})
	m.SetTunnelController(fake)

	pressU(t, m)
	if cmd := pressU(t, m); cmd != nil {
		t.Error("a second u while starting returned a command, want none")
	}
}

func TestTunnelKeyWithoutAControllerDoesNothing(t *testing.T) {
	m := NewDevModel(DevConfig{})
	if cmd := pressU(t, m); cmd != nil {
		t.Error("pressing u with no controller returned a command, want none")
	}
}

func TestConfiguredTunnelURLIsShownWithItsQR(t *testing.T) {
	m := NewDevModel(DevConfig{TunnelURL: "https://calm-river.trycloudflare.com"})

	if m.tunnelURL == "" {
		t.Error("a tunnel started by --tunnel is not reflected in the model")
	}
	if m.qrCode() == "" {
		t.Error("a tunnel started by --tunnel has no QR code")
	}
}

func TestQRCodeFallsBackToTheConfiguredOne(t *testing.T) {
	m := NewDevModel(DevConfig{QRCodeASCII: "██\n██"})
	if got, want := m.qrCode(), "██\n██"; got != want {
		t.Errorf("qrCode() = %q, want the configured code %q", got, want)
	}
}

// The TUI used to take every other line to make a tall code fit, which
// leaves something that still looks like a QR code and cannot be scanned.
func TestQRCodeIsRenderedWholeOrNotAtAll(t *testing.T) {
	m := NewDevModel(DevConfig{TunnelURL: "https://advert-howard-sys-blocks.trycloudflare.com"})

	code := m.qrCode()
	if code == "" {
		t.Fatal("no QR code was rendered for the tunnel URL")
	}

	view := m.viewQRCode()
	for _, line := range strings.Split(strings.TrimRight(code, "\n"), "\n") {
		if !strings.Contains(view, line) {
			t.Fatalf("a module row is missing from the rendered view:\n%q", line)
		}
	}
}

func TestQRHeightGateLeavesRoomForTheRestOfTheScreen(t *testing.T) {
	code := strings.Repeat("x\n", 21)

	got := qrMinimumHeight(code)
	if got <= 21 {
		t.Errorf("qrMinimumHeight = %d for a 21-line code, want more than the code's own height", got)
	}
}
