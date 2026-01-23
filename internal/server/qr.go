// Package server provides QR code generation for tap presentations.
package server

import (
	"encoding/base64"
	"fmt"
	"net"
	"strings"

	"github.com/skip2/go-qrcode"
)

// QRConfig holds configuration for QR code generation.
type QRConfig struct {
	PresenterPassword string
	PreferredHost     string // Optional: preferred host to use instead of auto-detecting
	Port              int
}

// GeneratePresenterURL generates the presenter URL for the given configuration.
// It auto-detects the local IP address if no preferred host is specified.
// If a presenter password is configured, it's included as a query parameter.
func GeneratePresenterURL(cfg QRConfig) (string, error) {
	host := cfg.PreferredHost
	if host == "" {
		// Auto-detect local IP
		ip, err := getLocalIP()
		if err != nil {
			// Fall back to localhost
			host = "localhost"
		} else {
			host = ip
		}
	}

	url := fmt.Sprintf("http://%s:%d/presenter", host, cfg.Port)
	if cfg.PresenterPassword != "" {
		url += "?key=" + cfg.PresenterPassword
	}
	return url, nil
}

// GenerateAudienceURL generates the audience URL for the given configuration.
// This is the main presentation view that audience members will see.
func GenerateAudienceURL(cfg QRConfig) (string, error) {
	host := cfg.PreferredHost
	if host == "" {
		ip, err := getLocalIP()
		if err != nil {
			host = "localhost"
		} else {
			host = ip
		}
	}

	return fmt.Sprintf("http://%s:%d", host, cfg.Port), nil
}

// GenerateQRCodePNG generates a QR code as a PNG image.
// The size parameter specifies the dimensions in pixels.
func GenerateQRCodePNG(url string, size int) ([]byte, error) {
	return qrcode.Encode(url, qrcode.Medium, size)
}

// GenerateQRCodeBase64 generates a QR code as a base64-encoded PNG.
// Useful for embedding in HTML pages.
func GenerateQRCodeBase64(url string, size int) (string, error) {
	png, err := GenerateQRCodePNG(url, size)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(png), nil
}

// GenerateASCIIQRCode generates an ASCII representation of a QR code.
// This can be displayed in the terminal.
func GenerateASCIIQRCode(url string) (string, error) {
	qr, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		return "", err
	}

	// Get the bitmap representation
	bitmap := qr.Bitmap()

	// Build ASCII representation using Unicode block characters
	// Each cell is represented as a 2x2 block to maintain aspect ratio
	var sb strings.Builder

	// We use full blocks and spaces
	// Black module = filled, White module = empty
	for y := 0; y < len(bitmap); y++ {
		for x := 0; x < len(bitmap[y]); x++ {
			if bitmap[y][x] {
				// Black module - use two full blocks for better visibility
				sb.WriteString("\u2588\u2588") // Full block (doubled for aspect ratio)
			} else {
				// White module - use spaces
				sb.WriteString("  ")
			}
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// getLocalIP returns the local IP address by attempting to connect to an external address.
// This doesn't actually send any traffic - it just determines the outbound interface.
func getLocalIP() (string, error) {
	// Use Google's DNS as a target (doesn't actually connect)
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer func() { _ = conn.Close() }()

	localAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return "", fmt.Errorf("unexpected address type")
	}
	return localAddr.IP.String(), nil
}

// GenerateQRCodeHTML generates an HTML page displaying the QR code.
// This is used for the /qr endpoint.
func GenerateQRCodeHTML(audienceURL, presenterURL string) (string, error) {
	// Generate QR codes for both URLs
	audienceQR, err := GenerateQRCodeBase64(audienceURL, 256)
	if err != nil {
		return "", fmt.Errorf("failed to generate audience QR code: %w", err)
	}

	presenterQR, err := GenerateQRCodeBase64(presenterURL, 256)
	if err != nil {
		return "", fmt.Errorf("failed to generate presenter QR code: %w", err)
	}

	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Tap QR Codes</title>
    <style>
        * { box-sizing: border-box; }
        body {
            font-family: system-ui, -apple-system, sans-serif;
            display: flex;
            flex-direction: column;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            margin: 0;
            padding: 2rem;
            background: linear-gradient(135deg, #667eea 0%%, #764ba2 100%%);
            color: white;
        }
        h1 {
            font-size: 2.5rem;
            margin-bottom: 2rem;
            text-shadow: 0 2px 10px rgba(0,0,0,0.2);
        }
        .qr-container {
            display: flex;
            gap: 3rem;
            flex-wrap: wrap;
            justify-content: center;
        }
        .qr-card {
            background: white;
            border-radius: 1rem;
            padding: 2rem;
            text-align: center;
            box-shadow: 0 10px 40px rgba(0,0,0,0.2);
            color: #1a1a1a;
        }
        .qr-card h2 {
            font-size: 1.5rem;
            margin: 0 0 1rem 0;
            color: #333;
        }
        .qr-card img {
            display: block;
            margin: 0 auto 1rem auto;
            border-radius: 0.5rem;
        }
        .qr-card .url {
            font-family: monospace;
            font-size: 0.9rem;
            color: #666;
            word-break: break-all;
            max-width: 256px;
        }
        .qr-card.audience h2 { color: #10B981; }
        .qr-card.presenter h2 { color: #7C3AED; }
    </style>
</head>
<body>
    <h1>Scan to Join</h1>
    <div class="qr-container">
        <div class="qr-card audience">
            <h2>Audience View</h2>
            <img src="data:image/png;base64,%s" alt="Audience QR Code" width="256" height="256">
            <p class="url">%s</p>
        </div>
        <div class="qr-card presenter">
            <h2>Presenter View</h2>
            <img src="data:image/png;base64,%s" alt="Presenter QR Code" width="256" height="256">
            <p class="url">%s</p>
        </div>
    </div>
</body>
</html>`, audienceQR, audienceURL, presenterQR, presenterURL)

	return html, nil
}
