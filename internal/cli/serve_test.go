package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// tap serve --json prints one ready line and nothing else on standard
// output, so a program (the desktop app's Preview button) learns the port
// without parsing text meant for a person.
func TestServeJSONPrintsOneReadyLine(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>built</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	server, listener, err := startServe(dir, 0, true, true, &out)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer func() { _ = server.Shutdown(shutdown) }()
	go func() { _ = server.Serve(listener) }()

	lines := strings.Split(strings.TrimRight(out.String(), "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("stdout = %q, want one line", out.String())
	}
	var ready struct {
		OK   bool   `json:"ok"`
		Dir  string `json:"dir"`
		Port int    `json:"port"`
		URL  string `json:"url"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &ready); err != nil {
		t.Fatalf("the ready line is not JSON: %v\n%s", err, lines[0])
	}
	if !ready.OK || ready.Dir != dir || ready.Port == 0 || ready.URL != fmt.Sprintf("http://127.0.0.1:%d", ready.Port) {
		t.Errorf("ready = %+v, want a 127.0.0.1 URL, the address bound", ready)
	}
	// The site is for the person's own browser: loopback, as --app binds.
	if address, ok := listener.Addr().(*net.TCPAddr); !ok || !address.IP.IsLoopback() {
		t.Errorf("--json bound %v, want 127.0.0.1", listener.Addr())
	}

	response, err := http.Get(ready.URL + "/index.html")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	_ = response.Body.Close()
	if !strings.Contains(string(body), "built") {
		t.Errorf("served %q", body)
	}
	// Shutdown returns once every handler has returned, so a request log
	// written after the response would be in out by now.
	if err := server.Shutdown(shutdown); err != nil {
		t.Fatal(err)
	}
	if out.String() != lines[0]+"\n" {
		t.Errorf("a request was logged to stdout in --json mode: %q", out.String())
	}
}

func TestServeWithoutJSONPrintsTheBanner(t *testing.T) {
	dir := t.TempDir()
	var out bytes.Buffer
	server, listener, err := startServe(dir, 0, true, false, &out)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	defer func() { _ = server.Shutdown(shutdown) }()
	if !strings.Contains(out.String(), "Serving presentation from") || !strings.Contains(out.String(), "http://localhost:") {
		t.Errorf("banner = %q", out.String())
	}
	if address, ok := listener.Addr().(*net.TCPAddr); !ok || address.IP.IsLoopback() {
		t.Errorf("the human mode bound %v, want every interface as before", listener.Addr())
	}
}

// A failure with --json is one line too, so a program reads the outcome,
// ready or not, from the first line of stdout.
func TestServeJSONFailureIsOneLine(t *testing.T) {
	exitCode, stdout, _ := runTap(t, "serve", filepath.Join(t.TempDir(), "missing"), "--port", "0", "--json")
	if exitCode != exitUserError {
		t.Fatalf("exit %d, want 1", exitCode)
	}
	if strings.Count(stdout, "\n") != 1 || !strings.HasPrefix(stdout, `{"ok":false,"error":{"code":"`+codeDeckNotFound+`"`) {
		t.Errorf("stdout = %q, want one compact error line", stdout)
	}
}

// A program that starts tap serve --json hands it a pipe; when the program
// goes, the pipe closes and the server exits, the contract --app has, so a
// preview server never outlives the app that opened it.
func TestServeJSONExitsWhenStdinCloses(t *testing.T) {
	dir := t.TempDir()
	reader, writer := io.Pipe()
	original := serveStandardInput
	serveStandardInput = reader
	t.Cleanup(func() { serveStandardInput = original })

	exited := make(chan int, 1)
	go func() {
		exitCode, _, _ := runTap(t, "serve", dir, "--port", "0", "--json")
		exited <- exitCode
	}()
	time.Sleep(300 * time.Millisecond)
	_ = writer.Close()
	select {
	case exitCode := <-exited:
		if exitCode != exitOK {
			t.Errorf("exit %d after stdin closed, want 0", exitCode)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("tap serve --json kept running after its stdin closed")
	}
}
