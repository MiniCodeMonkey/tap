package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// unreadLogApp is a tap --app process whose standard error is a pipe this
// test opens and never reads. The read end stays open for the whole run:
// closing it would make a blocked write fail instead of block, which is
// the one thing that must not rescue tap here. Standard output is drained
// throughout, so nothing these tests measure is about the event writer.
type unreadLogApp struct {
	t       *testing.T
	stdin   io.WriteCloser
	events  chan map[string]any
	exited  chan error
	backlog []map[string]any
	port    int
	token   string
	base    string
	// strayMu guards stray, the lines on standard output that were not one
	// JSON object. The protocol is one object per line and nothing else,
	// so any line here is a log line that reached the protocol stream.
	strayMu sync.Mutex
	stray   []string
}

// noteStray records a line of standard output that was not one JSON
// object.
func (app *unreadLogApp) noteStray(line string) {
	app.strayMu.Lock()
	app.stray = append(app.stray, line)
	app.strayMu.Unlock()
}

// requireCleanProtocol fails the test if anything but one JSON object per
// line reached standard output. Standard output is the protocol, and a
// log line landing in it is as bad as a log line blocking: the app stops
// being able to parse what its child is saying.
func (app *unreadLogApp) requireCleanProtocol() {
	app.t.Helper()
	app.strayMu.Lock()
	defer app.strayMu.Unlock()
	if len(app.stray) == 0 {
		return
	}
	shown := app.stray
	if len(shown) > 5 {
		shown = shown[:5]
	}
	app.t.Errorf("%d line(s) on standard output were not protocol objects, the first of them:\n%s", len(app.stray), strings.Join(shown, "\n"))
}

// startAppWithUnreadLog starts tap with args and its own settings folder,
// with standard error wedged from the first line. It does not wait for
// the ready line: whether the ready line arrives at all is what one of
// these tests measures.
func startAppWithUnreadLog(t *testing.T, args ...string) *unreadLogApp {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}
	command := exec.Command(buildTapBinaryForTest(t), args...)
	command.Env = append(os.Environ(), "XDG_CONFIG_HOME="+t.TempDir())
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	logRead, logWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = logWrite
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	app := &unreadLogApp{
		t:      t,
		stdin:  stdin,
		events: make(chan map[string]any, 1<<16),
		exited: make(chan error, 1),
	}
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 16<<20)
		for scanner.Scan() {
			var event map[string]any
			if json.Unmarshal(scanner.Bytes(), &event) != nil {
				app.noteStray(scanner.Text())
				continue
			}
			select {
			case app.events <- event:
			default:
			}
		}
		app.exited <- command.Wait()
		close(app.events)
	}()
	t.Cleanup(func() {
		select {
		case <-app.exited:
		default:
			_ = command.Process.Kill()
		}
		_ = logWrite.Close()
		_ = logRead.Close()
	})
	return app
}

// waitFor returns the next event of eventType, or false when none arrives
// within wait. Events it passes over stay in the backlog for a later call.
func (app *unreadLogApp) waitFor(eventType string, wait time.Duration) (map[string]any, bool) {
	app.t.Helper()
	for index, event := range app.backlog {
		if event["type"] == eventType {
			app.backlog = append(app.backlog[:index], app.backlog[index+1:]...)
			return event, true
		}
	}
	deadline := time.After(wait)
	for {
		select {
		case event, open := <-app.events:
			if !open {
				return nil, false
			}
			if event["type"] == eventType {
				return event, true
			}
			app.backlog = append(app.backlog, event)
		case <-deadline:
			return nil, false
		}
	}
}

// waitForReady reads the ready line and remembers the port and token.
func (app *unreadLogApp) waitForReady(wait time.Duration) bool {
	app.t.Helper()
	ready, arrived := app.waitFor(appEventReady, wait)
	if !arrived {
		return false
	}
	port, _ := ready["port"].(float64)
	app.port = int(port)
	app.token, _ = ready["token"].(string)
	app.base = fmt.Sprintf("http://127.0.0.1:%d", app.port)
	return true
}

func (app *unreadLogApp) send(line string) {
	app.t.Helper()
	if _, err := io.WriteString(app.stdin, line+"\n"); err != nil {
		app.t.Fatalf("writing %s: %v", line, err)
	}
}

// holdOneRequest opens a connection that sends a complete request head and
// only part of the body, and leaves it open. The handler is inside a read
// of the body, so the connection is active and a graceful HTTP shutdown
// waits for it. Nothing is injected: this is what an app that is mid-save
// when the user quits looks like.
func holdOneRequest(t *testing.T, port int, token, base string) net.Conn {
	t.Helper()
	connection, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = connection.Close() })
	body := `{"source":"# Held` + strings.Repeat(" ", 200)
	fmt.Fprintf(connection, "PUT /api/app/source HTTP/1.1\r\nHost: 127.0.0.1:%d\r\nAuthorization: Bearer %s\r\nOrigin: %s\r\nContent-Type: application/json\r\nContent-Length: 4000\r\n\r\n%s",
		port, token, base, body)
	// Long enough for the handler to have entered the body read, so the
	// connection counts as active rather than idle.
	time.Sleep(500 * time.Millisecond)
	return connection
}

// TestAppDevQuitsWithARequestInFlightAndStandardErrorUnread drives the
// last write the process makes. A graceful HTTP shutdown that runs out of
// time returns a deadline error, and a failed command's error is printed
// once, at the top level, after runDevServer has returned. That print
// must go through the bounded log writer like every line before it: an
// app that is not draining its child's log pipe would otherwise hold the
// process open on its very last statement.
func TestAppDevQuitsWithARequestInFlightAndStandardErrorUnread(t *testing.T) {
	app := startAppWithUnreadLog(t, "dev", "--app", copyAppFixture(t))
	if !app.waitForReady(30 * time.Second) {
		t.Fatal("no ready line within 30 seconds")
	}
	question, asked := app.waitFor(appEventQuestion, 30*time.Second)
	if !asked {
		t.Fatal("no live code question within 30 seconds")
	}
	id, _ := question["id"].(string)
	app.send(fmt.Sprintf(`{"type":"answer","id":%q,"value":false}`, id))

	// The pipe buffers about 64 KB before a write to it blocks, so the
	// log is filled first: one unknown command costs tap one line. This
	// is the state an app that stopped draining its log panel a while ago
	// is in, and it is the state the last write of the process meets.
	var flood strings.Builder
	for index := range 6000 {
		fmt.Fprintf(&flood, "{\"type\":\"flood-%d\"}\n", index)
	}
	if _, err := io.WriteString(app.stdin, flood.String()); err != nil {
		t.Fatal(err)
	}
	// Long enough for a control loop that is not wedged to drain the
	// flood, so the quit below lands on an empty command queue.
	time.Sleep(2 * time.Second)

	holdOneRequest(t, app.port, app.token, app.base)
	app.send(`{"type":"quit"}`)

	select {
	case <-app.exited:
	case <-time.After(45 * time.Second):
		t.Fatal("tap did not exit within 45 seconds of quit with one request in flight and standard error not being read")
	}
}

// TestAppDevExitsZeroOnACleanQuitWithARequestInFlight checks what the app
// is told about an ordinary quit that happened to catch a request. A
// graceful shutdown that runs out of time is expected on that quit, not a
// failure: the listener is closed and the process is leaving either way.
// An exit code of 1 with "Error: context deadline exceeded" in the log
// panel is a crash as far as the desktop app is concerned.
func TestAppDevExitsZeroOnACleanQuitWithARequestInFlight(t *testing.T) {
	deck := copyAppFixture(t)
	process := startAppProcess(t, t.TempDir(), "dev", "--app", deck)
	question := process.next(appEventQuestion)
	id, _ := question["id"].(string)
	process.send(fmt.Sprintf(`{"type":"answer","id":%q,"value":false}`, id))

	holdOneRequest(t, process.port, process.token, process.base)
	process.send(`{"type":"quit"}`)

	if err := process.waitForExit(); err != nil {
		t.Errorf("a clean quit with one request in flight exited %v, want 0:\n%s", err, process.stderr)
	}
	if log := process.stderr.String(); strings.Contains(log, "Error: context deadline exceeded") {
		t.Errorf("a clean quit reported a failure in the log panel:\n%s", log)
	}
}

// warningDeck writes a deck whose every slide names a layout that does
// not exist, so every render produces well over a pipe buffer of warning
// lines. It is the shape of a deck someone is midway through editing, not
// a fault injected anywhere in tap.
func warningDeck(t *testing.T) string {
	t.Helper()
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var deck strings.Builder
	deck.WriteString("---\ntitle: Warning Deck\n---\n")
	for slide := range 600 {
		fmt.Fprintf(&deck, "\n---\n\n<!--\nlayout: no-such-layout-%d\n-->\n\n# Slide %d\n", slide, slide)
	}
	path := filepath.Join(directory, "talk.md")
	if err := os.WriteFile(path, []byte(deck.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestAppDevRendersWhenStandardErrorIsNotReadAndTheDeckWarns drives the
// render path rather than the quit path. The layout and component
// warnings a render produces are log lines like any other, so a deck that
// warns must not be able to stop tap starting, or stop the file watcher
// reporting, when the app is not draining its log pipe. A hang on quit is
// at least visible; a watcher that has silently stopped is not.
func TestAppDevRendersWhenStandardErrorIsNotReadAndTheDeckWarns(t *testing.T) {
	deck := warningDeck(t)
	app := startAppWithUnreadLog(t, "dev", "--app", deck)
	if !app.waitForReady(60 * time.Second) {
		t.Fatal("tap never reached the ready line on a warning deck while standard error was not being read")
	}

	source, err := os.ReadFile(deck)
	if err != nil {
		t.Fatal(err)
	}
	for touch := range 4 {
		changed := append(source, fmt.Appendf(nil, "\n---\n\n# Touch %d\n", touch)...)
		if err := os.WriteFile(deck, changed, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, reported := app.waitFor(appEventFileChanged, 30*time.Second); !reported {
			t.Fatalf("the file watcher stopped reporting changes after %d of them", touch)
		}
	}
}
