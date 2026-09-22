package cli

import (
	"net/url"

	"github.com/MiniCodeMonkey/tap/internal/server"
)

// listenHost is the address tap dev and tap present listen on. They
// listen on loopback only, unless --lan opens them to the local network.
func listenHost(lan bool) string {
	if lan {
		return "0.0.0.0"
	}
	return "127.0.0.1"
}

// lanPresenterAddress returns the presenter URL on this machine's LAN
// address and a QR code of it, for a phone on the same network. ok is
// false when the machine has no LAN address, or when the QR code cannot
// be drawn.
func lanPresenterAddress(port int, presenterPassword string) (presenterURL, qrCode string, ok bool) {
	presenterURL, err := server.GeneratePresenterURL(server.QRConfig{Port: port, PresenterPassword: presenterPassword})
	if err != nil {
		return "", "", false
	}
	parsed, err := url.Parse(presenterURL)
	if err != nil || parsed.Hostname() == "localhost" {
		return "", "", false
	}
	qrCode, err = server.GenerateCompactQRCode(presenterURL)
	if err != nil {
		return presenterURL, "", true
	}
	return presenterURL, qrCode, true
}
