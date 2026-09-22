package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// TestDevSendsAnUpdateWhenASlideChanges starts a real tap dev, edits the
// second slide on disk, and expects an "update" message that names slide
// 2, not a "reload".
func TestDevSendsAnUpdateWhenASlideChanges(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}
	binary := buildTapBinaryForTest(t)

	deckPath := filepath.Join(t.TempDir(), "talk.md")
	if err := os.WriteFile(deckPath, []byte("# One\n\n---\n\n# Two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	port := freePort(t)
	command := exec.Command(binary, "dev", deckPath, "--headless", "--port", fmt.Sprint(port))
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Signal(syscall.SIGINT)
		_ = command.Wait()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var conn *websocket.Conn
	for {
		var err error
		conn, _, err = websocket.Dial(ctx, fmt.Sprintf("ws://127.0.0.1:%d/ws", port), nil)
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			t.Fatalf("tap dev never accepted a WebSocket: %v\n%s", err, output.String())
		}
		time.Sleep(100 * time.Millisecond)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	readMessage := func() map[string]any {
		t.Helper()
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("conn.Read() error = %v\n%s", err, output.String())
		}
		var message map[string]any
		if err := json.Unmarshal(data, &message); err != nil {
			t.Fatalf("json.Unmarshal(%s) error = %v", data, err)
		}
		return message
	}

	if connected := readMessage(); connected["type"] != "connected" {
		t.Fatalf("first message = %v, want connected", connected)
	}

	if err := os.WriteFile(deckPath, []byte("# One\n\n---\n\n# Two, edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for {
		message := readMessage()
		switch message["type"] {
		case "reload":
			t.Fatal("tap dev sent reload for a markdown edit, want update")
		case "update":
			if fmt.Sprint(message["slides"]) != "[2]" {
				t.Errorf("slides = %v, want [2]", message["slides"])
			}
			if revision, _ := message["revision"].(string); revision == "" {
				t.Error("the update carries no revision")
			}
			return
		}
	}
}
