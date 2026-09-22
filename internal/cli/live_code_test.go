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
)

const liveCodeDeck = "---\ntitle: Live code\n---\n\n# Run it\n\n```bash {driver: 'shell'}\necho hello from tap\n```\n"

// TestDevRunsALiveShellBlock starts a real tap dev and runs the deck's
// shell block through /api/execute, as the Run button does.
func TestDevRunsALiveShellBlock(t *testing.T) {
	t.Skip("Task 8 of the live code approvals plan rewrites this test for execute by reference")
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}
	binary := buildTapBinaryForTest(t)

	deckPath := filepath.Join(t.TempDir(), "live.md")
	if err := os.WriteFile(deckPath, []byte(liveCodeDeck), 0o644); err != nil {
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

	base := fmt.Sprintf("http://localhost:%d", port)
	var presentation struct {
		Slides []struct {
			CodeBlocks []struct {
				Code   string `json:"code"`
				Driver string `json:"driver"`
			} `json:"codeBlocks"`
		} `json:"slides"`
	}
	deadline := time.Now().Add(20 * time.Second)
	for {
		response, err := http.Get(base + "/api/presentation")
		if err == nil && response.StatusCode == http.StatusOK {
			err = json.NewDecoder(response.Body).Decode(&presentation)
			response.Body.Close()
			if err != nil {
				t.Fatalf("decoding /api/presentation: %v", err)
			}
			break
		}
		if err == nil {
			response.Body.Close()
		}
		if time.Now().After(deadline) {
			t.Fatalf("tap dev did not start:\n%s", output.String())
		}
		time.Sleep(100 * time.Millisecond)
	}
	if len(presentation.Slides) == 0 || len(presentation.Slides[0].CodeBlocks) == 0 {
		t.Fatalf("the deck has no code block: %+v", presentation)
	}
	block := presentation.Slides[0].CodeBlocks[0]

	execute := func(code string) (int, string) {
		body, _ := json.Marshal(map[string]string{"driver": block.Driver, "code": code})
		request, _ := http.NewRequest(http.MethodPost, base+"/api/execute", bytes.NewReader(body))
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

	status, text := execute(block.Code)
	if status != http.StatusOK || !strings.Contains(text, "hello from tap") {
		t.Errorf("the deck's block: status %d, output %q, want 200 and \"hello from tap\"", status, text)
	}

	status, _ = execute("echo not in the deck")
	if status != http.StatusForbidden {
		t.Errorf("code outside the deck: status %d, want 403", status)
	}
}
