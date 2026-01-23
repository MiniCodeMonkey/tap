package server

import (
	"fmt"
	"strings"
	"testing"
)

func TestGeneratePresenterURL(t *testing.T) {
	tests := []struct {
		name     string
		cfg      QRConfig
		contains []string
	}{
		{
			name: "basic URL without password",
			cfg: QRConfig{
				Port:          3000,
				PreferredHost: "192.168.1.100",
			},
			contains: []string{"http://192.168.1.100:3000/presenter"},
		},
		{
			name: "URL with password",
			cfg: QRConfig{
				Port:              3000,
				PreferredHost:     "192.168.1.100",
				PresenterPassword: "secret123",
			},
			contains: []string{"http://192.168.1.100:3000/presenter", "?key=secret123"},
		},
		{
			name: "different port",
			cfg: QRConfig{
				Port:          8080,
				PreferredHost: "10.0.0.1",
			},
			contains: []string{"http://10.0.0.1:8080/presenter"},
		},
		{
			name: "localhost host",
			cfg: QRConfig{
				Port:          3000,
				PreferredHost: "localhost",
			},
			contains: []string{"http://localhost:3000/presenter"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := GeneratePresenterURL(tt.cfg)
			if err != nil {
				t.Fatalf("GeneratePresenterURL() error = %v", err)
			}

			for _, expected := range tt.contains {
				if !strings.Contains(url, expected) {
					t.Errorf("GeneratePresenterURL() = %q, should contain %q", url, expected)
				}
			}
		})
	}
}

func TestGeneratePresenterURL_AutoDetectIP(t *testing.T) {
	cfg := QRConfig{
		Port: 3000,
		// No PreferredHost - should auto-detect
	}

	url, err := GeneratePresenterURL(cfg)
	if err != nil {
		t.Fatalf("GeneratePresenterURL() error = %v", err)
	}

	// Should contain either a local IP or localhost as fallback
	if !strings.Contains(url, "/presenter") {
		t.Errorf("GeneratePresenterURL() = %q, should contain '/presenter'", url)
	}
	if !strings.Contains(url, ":3000") {
		t.Errorf("GeneratePresenterURL() = %q, should contain ':3000'", url)
	}
}

func TestGenerateAudienceURL(t *testing.T) {
	tests := []struct {
		name     string
		cfg      QRConfig
		expected string
	}{
		{
			name: "basic URL",
			cfg: QRConfig{
				Port:          3000,
				PreferredHost: "192.168.1.100",
			},
			expected: "http://192.168.1.100:3000",
		},
		{
			name: "different port",
			cfg: QRConfig{
				Port:          8080,
				PreferredHost: "10.0.0.1",
			},
			expected: "http://10.0.0.1:8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := GenerateAudienceURL(tt.cfg)
			if err != nil {
				t.Fatalf("GenerateAudienceURL() error = %v", err)
			}

			if url != tt.expected {
				t.Errorf("GenerateAudienceURL() = %q, want %q", url, tt.expected)
			}
		})
	}
}

func TestGenerateQRCodePNG(t *testing.T) {
	url := "http://192.168.1.100:3000/presenter"
	png, err := GenerateQRCodePNG(url, 256)
	if err != nil {
		t.Fatalf("GenerateQRCodePNG() error = %v", err)
	}

	// Check that we got valid PNG data (starts with PNG magic bytes)
	if len(png) < 8 {
		t.Fatal("GenerateQRCodePNG() returned too little data")
	}
	// PNG magic bytes: 137 80 78 71 13 10 26 10
	pngMagic := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}
	for i, b := range pngMagic {
		if png[i] != b {
			t.Errorf("GenerateQRCodePNG() invalid PNG magic byte at %d: got %x, want %x", i, png[i], b)
		}
	}
}

func TestGenerateQRCodeBase64(t *testing.T) {
	url := "http://192.168.1.100:3000/presenter"
	base64Str, err := GenerateQRCodeBase64(url, 256)
	if err != nil {
		t.Fatalf("GenerateQRCodeBase64() error = %v", err)
	}

	// Base64 PNG starts with these characters (decoded: PNG magic)
	if !strings.HasPrefix(base64Str, "iVBOR") {
		t.Errorf("GenerateQRCodeBase64() should start with 'iVBOR', got prefix: %q", base64Str[:10])
	}

	// Should be non-empty
	if len(base64Str) == 0 {
		t.Error("GenerateQRCodeBase64() returned empty string")
	}
}

func TestGenerateASCIIQRCode(t *testing.T) {
	url := "http://192.168.1.100:3000"
	ascii, err := GenerateASCIIQRCode(url)
	if err != nil {
		t.Fatalf("GenerateASCIIQRCode() error = %v", err)
	}

	// Should have multiple lines
	lines := strings.Split(strings.TrimSpace(ascii), "\n")
	if len(lines) < 10 {
		t.Errorf("GenerateASCIIQRCode() should have at least 10 lines, got %d", len(lines))
	}

	// Should contain block characters or spaces
	if !strings.Contains(ascii, "\u2588") && !strings.Contains(ascii, " ") {
		t.Error("GenerateASCIIQRCode() should contain block characters or spaces")
	}
}

func TestGenerateQRCodeHTML(t *testing.T) {
	audienceURL := "http://192.168.1.100:3000"
	presenterURL := "http://192.168.1.100:3000/presenter?key=secret"

	html, err := GenerateQRCodeHTML(audienceURL, presenterURL)
	if err != nil {
		t.Fatalf("GenerateQRCodeHTML() error = %v", err)
	}

	// Check for required HTML elements
	requiredContent := []string{
		"<!DOCTYPE html>",
		"<html",
		"Audience View",
		"Presenter View",
		"data:image/png;base64,",
		audienceURL,
		presenterURL,
	}

	for _, required := range requiredContent {
		if !strings.Contains(html, required) {
			t.Errorf("GenerateQRCodeHTML() should contain %q", required)
		}
	}
}

func TestGeneratePresenterURL_PasswordURLEncoding(t *testing.T) {
	// Test that passwords are included correctly (basic case)
	cfg := QRConfig{
		Port:              3000,
		PreferredHost:     "localhost",
		PresenterPassword: "mypassword",
	}

	url, err := GeneratePresenterURL(cfg)
	if err != nil {
		t.Fatalf("GeneratePresenterURL() error = %v", err)
	}

	if !strings.HasSuffix(url, "?key=mypassword") {
		t.Errorf("GeneratePresenterURL() = %q, should end with '?key=mypassword'", url)
	}
}

func TestGeneratePresenterURL_EmptyPassword(t *testing.T) {
	cfg := QRConfig{
		Port:              3000,
		PreferredHost:     "localhost",
		PresenterPassword: "",
	}

	url, err := GeneratePresenterURL(cfg)
	if err != nil {
		t.Fatalf("GeneratePresenterURL() error = %v", err)
	}

	// Should not have query parameter when password is empty
	if strings.Contains(url, "?") {
		t.Errorf("GeneratePresenterURL() = %q, should not contain '?' when password is empty", url)
	}

	expected := "http://localhost:3000/presenter"
	if url != expected {
		t.Errorf("GeneratePresenterURL() = %q, want %q", url, expected)
	}
}

func TestQRCodeDifferentSizes(t *testing.T) {
	url := "http://test.local:3000"

	sizes := []int{128, 256, 512}
	for _, size := range sizes {
		size := size // capture for closure
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			png, err := GenerateQRCodePNG(url, size)
			if err != nil {
				t.Fatalf("GenerateQRCodePNG(size=%d) error = %v", size, err)
			}
			if len(png) == 0 {
				t.Errorf("GenerateQRCodePNG(size=%d) returned empty data", size)
			}
		})
	}
}
