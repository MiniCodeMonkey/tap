// Package tunnel puts a running dev server on a public https URL through a
// Cloudflare Quick Tunnel.
//
// A Quick Tunnel needs no Cloudflare account, no login and no config: the
// cloudflared binary dials out, Cloudflare hands back a random
// *.trycloudflare.com name, and traffic to that name is forwarded to the
// local port. Nothing is registered, and the name is gone when the process
// stops.
//
// Two reasons a deck wants this. A phone on a different network can open
// it at all, and the page is https, so browser features that require a
// secure context -- the screen wake lock among them -- actually work.
package tunnel

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// ErrNotInstalled is returned by Start when cloudflared is not on PATH.
// Callers show InstallHint alongside it.
var ErrNotInstalled = errors.New("cloudflared is not installed")

// ErrNoURL is returned when cloudflared ran but never announced a URL.
var ErrNoURL = errors.New("cloudflared did not report a tunnel URL")

// urlPattern matches the hostname Cloudflare hands back. cloudflared draws
// it inside a banner box, so the line it sits on carries other characters.
var urlPattern = regexp.MustCompile(`https://[a-z0-9][a-z0-9-]*\.trycloudflare\.com`)

// startTimeout is how long to wait for that announcement. A Quick Tunnel
// normally comes up in a few seconds; this is slow-network headroom, not a
// target.
const startTimeout = 30 * time.Second

// InstallHint returns the one line to tell the user to run, for the
// platform this is running on.
func InstallHint() string {
	switch runtime.GOOS {
	case "darwin":
		return "brew install cloudflared"
	case "windows":
		return "winget install --id Cloudflare.cloudflared"
	default:
		return "see https://developers.cloudflare.com/cloudflare-one/connections/connect-networks/downloads/"
	}
}

// Available reports whether cloudflared can be found on PATH.
func Available() bool {
	_, err := exec.LookPath("cloudflared")
	return err == nil
}

// Tunnel is a running Quick Tunnel. Stop it when the server stops.
type Tunnel struct {
	url  string
	cmd  *exec.Cmd
	once sync.Once
}

// Start launches cloudflared against a local port and waits for the public
// URL. The returned Tunnel is running until Stop is called.
//
// ctx bounds the wait for the URL only. The tunnel itself outlives it, so
// a caller can pass a short-lived context here and still keep the tunnel.
func Start(ctx context.Context, port int) (*Tunnel, error) {
	binary, err := exec.LookPath("cloudflared")
	if err != nil {
		return nil, ErrNotInstalled
	}

	// --no-autoupdate: a dev server should not have its tunnel restarted
	// underneath it. The rest is the documented Quick Tunnel invocation.
	cmd := exec.Command(binary,
		"tunnel",
		"--no-autoupdate",
		"--url", fmt.Sprintf("http://127.0.0.1:%d", port),
	)

	// cloudflared writes its banner to stderr; stdout is read too, so a
	// future version that moves the line does not break this.
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("cloudflared stderr: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("cloudflared stdout: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting cloudflared: %w", err)
	}

	tunnel := &Tunnel{cmd: cmd}

	found := make(chan string, 1)
	go scanForURL(io.MultiReader(stderr, stdout), found)

	timeout := time.NewTimer(startTimeout)
	defer timeout.Stop()

	select {
	case url := <-found:
		if url == "" {
			_ = tunnel.Stop()
			return nil, ErrNoURL
		}
		tunnel.url = url
		return tunnel, nil
	case <-timeout.C:
		_ = tunnel.Stop()
		return nil, ErrNoURL
	case <-ctx.Done():
		_ = tunnel.Stop()
		return nil, ctx.Err()
	}
}

// scanForURL reads lines until one carries a trycloudflare.com URL, then
// sends it. It sends an empty string if the output ends without one, so a
// caller waiting on the channel is never left hanging.
func scanForURL(reader io.Reader, found chan<- string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		if url := ParseURL(scanner.Text()); url != "" {
			found <- url
			// Keep draining, so cloudflared never blocks on a full pipe.
			for scanner.Scan() { //nolint:revive // draining
			}
			return
		}
	}
	found <- ""
}

// ParseURL returns the trycloudflare.com URL on a line of cloudflared
// output, or "" when the line carries none.
func ParseURL(line string) string {
	return urlPattern.FindString(line)
}

// URL is the public https address the deck is reachable at.
func (t *Tunnel) URL() string {
	if t == nil {
		return ""
	}
	return t.url
}

// Host is the URL's hostname, which is what the dev server's Host header
// allow-list needs.
func (t *Tunnel) Host() string {
	return strings.TrimPrefix(t.URL(), "https://")
}

// Stop shuts the tunnel down. Safe to call more than once, and on nil.
func (t *Tunnel) Stop() error {
	if t == nil || t.cmd == nil {
		return nil
	}

	var err error
	t.once.Do(func() {
		if t.cmd.Process != nil {
			err = t.cmd.Process.Kill()
		}
		// Reap it, so no zombie is left behind. The error here is the
		// child's own exit status, which is meaningless after a kill.
		_ = t.cmd.Wait()
	})
	return err
}
