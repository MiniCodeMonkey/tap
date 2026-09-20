package tunnel

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// A real banner, as cloudflared draws it: the URL sits inside a box, on a
// line with other characters either side.
const banner = `2026-09-20T14:12:02Z INF Requesting new quick Tunnel on trycloudflare.com...
2026-09-20T14:12:04Z INF +--------------------------------------------------------------------------------------------+
2026-09-20T14:12:04Z INF |  Your quick Tunnel has been created! Visit it at (it may take some time to be reachable):   |
2026-09-20T14:12:04Z INF |  https://plain-shoes-arrive-lately.trycloudflare.com                                       |
2026-09-20T14:12:04Z INF +--------------------------------------------------------------------------------------------+`

func TestParseURL(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{
			name: "inside the banner box",
			line: "2026-09-20T14:12:04Z INF |  https://plain-shoes-arrive-lately.trycloudflare.com    |",
			want: "https://plain-shoes-arrive-lately.trycloudflare.com",
		},
		{
			name: "on its own",
			line: "https://calm-river-1234.trycloudflare.com",
			want: "https://calm-river-1234.trycloudflare.com",
		},
		{
			name: "an ordinary log line",
			line: "2026-09-20T14:12:02Z INF Requesting new quick Tunnel on trycloudflare.com...",
			want: "",
		},
		{
			name: "a different cloudflare domain is not a tunnel",
			line: "see https://developers.cloudflare.com/cloudflare-one/",
			want: "",
		},
		{
			name: "http is not accepted, the tunnel is always https",
			line: "http://plain-shoes.trycloudflare.com",
			want: "",
		},
		{
			name: "empty",
			line: "",
			want: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ParseURL(test.line); got != test.want {
				t.Errorf("ParseURL(%q) = %q, want %q", test.line, got, test.want)
			}
		})
	}
}

func TestScanForURLFindsItInABanner(t *testing.T) {
	found := make(chan string, 1)
	go scanForURL(strings.NewReader(banner), found)

	got := <-found
	want := "https://plain-shoes-arrive-lately.trycloudflare.com"
	if got != want {
		t.Errorf("scanForURL = %q, want %q", got, want)
	}
}

func TestScanForURLReportsOutputWithNoURL(t *testing.T) {
	found := make(chan string, 1)
	go scanForURL(strings.NewReader("failed to connect\nexiting\n"), found)

	if got := <-found; got != "" {
		t.Errorf("scanForURL = %q, want an empty string", got)
	}
}

func TestStartWithoutCloudflaredSaysSo(t *testing.T) {
	// An empty PATH makes the lookup fail the way a machine without
	// cloudflared does.
	t.Setenv("PATH", t.TempDir())

	_, err := Start(context.Background(), 3000)
	if err != ErrNotInstalled {
		t.Errorf("Start error = %v, want ErrNotInstalled", err)
	}
}

func TestAvailableFollowsPATH(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the stub below is a shell script")
	}

	dir := t.TempDir()
	t.Setenv("PATH", dir)

	if Available() {
		t.Error("Available() = true with an empty PATH, want false")
	}

	stub := filepath.Join(dir, "cloudflared")
	if err := os.WriteFile(stub, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("writing the stub: %v", err)
	}

	if !Available() {
		t.Error("Available() = false with cloudflared on PATH, want true")
	}
}

func TestInstallHintIsNotEmpty(t *testing.T) {
	if InstallHint() == "" {
		t.Error("InstallHint() is empty; every platform needs something to tell the user")
	}
}

func TestHostStripsTheScheme(t *testing.T) {
	tunnel := &Tunnel{url: "https://plain-shoes.trycloudflare.com"}
	if got, want := tunnel.Host(), "plain-shoes.trycloudflare.com"; got != want {
		t.Errorf("Host() = %q, want %q", got, want)
	}
}

func TestNilTunnelIsSafe(t *testing.T) {
	var tunnel *Tunnel
	if got := tunnel.URL(); got != "" {
		t.Errorf("URL() on a nil tunnel = %q, want an empty string", got)
	}
	if err := tunnel.Stop(); err != nil {
		t.Errorf("Stop() on a nil tunnel = %v, want nil", err)
	}
}

func TestStopIsIdempotent(t *testing.T) {
	tunnel := &Tunnel{}
	if err := tunnel.Stop(); err != nil {
		t.Errorf("first Stop() = %v, want nil", err)
	}
	if err := tunnel.Stop(); err != nil {
		t.Errorf("second Stop() = %v, want nil", err)
	}
}
