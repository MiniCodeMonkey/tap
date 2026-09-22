package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/usersettings"
)

const liveCodeDeck = "---\ntitle: Live code\ndrivers:\n  shell: {}\n---\n\n# Run it\n\n```bash {driver: 'shell'}\necho hello from tap\n```\n"

func writeLiveCodeDeck(t *testing.T) string {
	t.Helper()
	deckPath := filepath.Join(t.TempDir(), "live.md")
	if err := os.WriteFile(deckPath, []byte(liveCodeDeck), 0o644); err != nil {
		t.Fatal(err)
	}
	return deckPath
}

// startDevForLiveCode starts a real tap dev --headless on deckPath with its
// own settings folder and extra arguments, waits until it serves the deck,
// and returns its base URL. It stops tap dev when the test ends.
func startDevForLiveCode(t *testing.T, deckPath, configHome string, extra ...string) string {
	t.Helper()
	binary := buildTapBinaryForTest(t)
	port := freePort(t)
	args := append([]string{"dev", deckPath, "--headless", "--port", fmt.Sprint(port)}, extra...)
	command := exec.Command(binary, args...)
	command.Env = append(os.Environ(), "XDG_CONFIG_HOME="+configHome)
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

	base := fmt.Sprintf("http://localhost:%d", port)
	deadline := time.Now().Add(20 * time.Second)
	for {
		response, err := http.Get(base + "/api/presentation")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				return base
			}
		}
		if time.Now().After(deadline) {
			_ = command.Process.Kill()
			_ = command.Wait()
			t.Fatalf("tap dev did not start:\n%s", output.String())
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// postExecute sends body to /api/execute the way the Run button does, and
// returns the status and the output or error text.
func postExecute(t *testing.T, base, body string) (int, string) {
	t.Helper()
	request, err := http.NewRequest(http.MethodPost, base+"/api/execute", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Origin", base)
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatalf("POST /api/execute: %v", err)
	}
	defer response.Body.Close()
	var result struct {
		Output string `json:"output"`
		Error  string `json:"error"`
	}
	_ = json.NewDecoder(response.Body).Decode(&result)
	return response.StatusCode, result.Output + result.Error
}

// TestDevRunsALiveShellBlockByReference starts a real tap dev with
// --allow-code and runs the deck's shell block by reference, as the Run
// button does.
func TestDevRunsALiveShellBlockByReference(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}
	base := startDevForLiveCode(t, writeLiveCodeDeck(t), t.TempDir(), "--allow-code")

	status, text := postExecute(t, base, `{"slide": 1, "block": 1}`)
	if status != http.StatusOK || !strings.Contains(text, "hello from tap") {
		t.Errorf("the deck's block: status %d, output %q, want 200 and \"hello from tap\"", status, text)
	}

	status, _ = postExecute(t, base, `{"driver": "shell", "code": "echo not in the deck"}`)
	if status != http.StatusBadRequest {
		t.Errorf("a code body: status %d, want 400", status)
	}
}

func TestDevKeepsLiveCodeOffForAnUnapprovedDeck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}
	configHome := t.TempDir()
	base := startDevForLiveCode(t, writeLiveCodeDeck(t), configHome)

	status, text := postExecute(t, base, `{"slide": 1, "block": 1}`)
	if status != http.StatusForbidden || !strings.Contains(text, "Not approved") {
		t.Errorf("status %d, output %q, want 403 Not approved", status, text)
	}
	if _, err := os.Stat(filepath.Join(configHome, "tap", "settings.yaml")); !os.IsNotExist(err) {
		t.Errorf("a run that did not ask saved the settings file (stat error %v)", err)
	}
}

func TestDevRunsAnApprovedDeckWithoutATerminal(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}
	deckPath := writeLiveCodeDeck(t)
	configHome := t.TempDir()
	deckKey, err := usersettings.ResolveDeck(deckPath)
	if err != nil {
		t.Fatal(err)
	}
	var settings usersettings.Settings
	settings.Approve(deckKey, []string{"shell"}, time.Now())
	if err := usersettings.Save(filepath.Join(configHome, "tap", "settings.yaml"), settings); err != nil {
		t.Fatal(err)
	}

	base := startDevForLiveCode(t, deckPath, configHome)
	status, text := postExecute(t, base, `{"slide": 1, "block": 1}`)
	if status != http.StatusOK || !strings.Contains(text, "hello from tap") {
		t.Errorf("status %d, output %q, want 200", status, text)
	}
}
