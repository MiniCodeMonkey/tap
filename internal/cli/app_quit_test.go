package cli

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestAppDevQuitsWhenStandardErrorIsNotRead drives the half of the
// contract the event writer already keeps for standard output, against
// the other pipe. An app reads its child's standard output because that
// is the protocol; its log view is a convenience it may well not be
// draining, and a pipe nobody reads fills after about 64 KB. Every
// human-readable line tap writes, including the report that says quit
// gave up on something, goes to that pipe, so a blocking write there
// would sit above the quit deadline itself: the deadline could not fire
// because the thing that reports it is what is stuck.
//
// The harness is deliberately asymmetric. Standard output is drained
// throughout, so nothing here is about the event writer. Standard error
// goes to a pipe this test opens and never reads, which is the app whose
// log view has stopped consuming. The flood is unknown commands, each of
// which makes the control loop itself write one line, so the wedge lands
// on the goroutine that has to read the quit command.
func TestAppDevQuitsWhenStandardErrorIsNotRead(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}
	command := exec.Command(buildTapBinaryForTest(t), "dev", "--app", copyAppFixture(t))
	command.Env = append(os.Environ(), "XDG_CONFIG_HOME="+t.TempDir())
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	// The read end stays open and unread for the whole test. Closing it
	// would make the blocked write fail instead of block, which is the
	// one thing that must not rescue tap here.
	logRead, logWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stderr = logWrite
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan error, 1)
	events := make(chan map[string]any, 1<<16)
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 16<<20)
		for scanner.Scan() {
			var event map[string]any
			if json.Unmarshal(scanner.Bytes(), &event) == nil {
				select {
				case events <- event:
				default:
				}
			}
		}
		exited <- command.Wait()
		close(events)
	}()
	t.Cleanup(func() {
		select {
		case <-exited:
		default:
			_ = command.Process.Kill()
		}
		_ = logWrite.Close()
		_ = logRead.Close()
	})

	nextEvent := func(kind string) map[string]any {
		t.Helper()
		deadline := time.After(30 * time.Second)
		for {
			select {
			case event, open := <-events:
				if !open {
					t.Fatalf("tap exited before a %s event", kind)
				}
				if event["type"] == kind {
					return event
				}
			case <-deadline:
				t.Fatalf("no %s event within 30 seconds", kind)
			}
		}
	}

	nextEvent(appEventReady)
	question := nextEvent(appEventQuestion)
	id, _ := question["id"].(string)
	if _, err := fmt.Fprintf(stdin, "{\"type\":\"answer\",\"id\":%q,\"value\":false}\n", id); err != nil {
		t.Fatal(err)
	}

	// Well past the 64 KB a pipe buffers: each unknown command costs tap
	// one line of about thirty bytes on standard error.
	var flood strings.Builder
	for index := range 6000 {
		fmt.Fprintf(&flood, "{\"type\":\"flood-%d\"}\n", index)
	}
	if _, err := io.WriteString(stdin, flood.String()); err != nil {
		t.Fatal(err)
	}

	// Long enough for a control loop that is not wedged to drain the
	// flood, so the quit below lands on an empty command queue rather
	// than being dropped as one command too many.
	time.Sleep(2 * time.Second)
	if _, err := io.WriteString(stdin, "{\"type\":\"quit\"}\n"); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-exited:
		if err != nil {
			t.Errorf("exit: %v", err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("tap did not exit within 30 seconds of quit while standard error was not being read")
	}
}

// TestJoinQuitBoundsACleanupThatNeverReturns drives the helper the quit
// path's cleanup calls go through. The call under it is a stop that never
// returns, which is what a lock held by a goroutine quit already gave up
// on looks like from here.
func TestJoinQuitBoundsACleanupThatNeverReturns(t *testing.T) {
	var reported []string
	joiner := newQuitJoiner(func(code, _ string) { reported = append(reported, code) }, io.Discard, 100*time.Millisecond)
	stuck := make(chan struct{})
	defer close(stuck)

	started := time.Now()
	joinQuit(joiner, "stopping the tunnel", appErrorTunnelFailed, func() { <-stuck })
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Errorf("a cleanup call that never returns held quit for %s", elapsed)
	}
	if len(reported) != 1 || reported[0] != appErrorTunnelFailed {
		t.Errorf("reported %v, want one %s", reported, appErrorTunnelFailed)
	}
}

// TestJoinQuitSharesTheOneDeadline checks that the cleanup calls after the
// session share the session's deadline rather than each taking a fresh
// allowance. Three stuck stops in a row must cost one deadline, not three.
func TestJoinQuitSharesTheOneDeadline(t *testing.T) {
	joiner := newQuitJoiner(func(string, string) {}, io.Discard, 300*time.Millisecond)
	stuck := make(chan struct{})
	defer close(stuck)

	started := time.Now()
	for _, what := range []string{"stopping the file watcher", "stopping the tunnel", "finishing the recording"} {
		joinQuit(joiner, what, appErrorShutdownStuck, func() { <-stuck })
	}
	if elapsed := time.Since(started); elapsed > 300*time.Millisecond+3*appQuitJoinGrace+time.Second {
		t.Errorf("three stuck cleanup calls took %s, want one shared deadline", elapsed)
	}
}

// TestAppQuitShutdownBoundStaysInsideTheDeadline checks that the server
// shutdown is inside the quit deadline rather than five seconds of its
// own on top of it.
func TestAppQuitShutdownBoundStaysInsideTheDeadline(t *testing.T) {
	if got := appQuitShutdownBound(nil); got != appServerShutdownBound {
		t.Errorf("without a session the shutdown gets %s, want %s", got, appServerShutdownBound)
	}
	spent := newQuitJoiner(func(string, string) {}, io.Discard, 0)
	if got := appQuitShutdownBound(spent); got > appQuitJoinGrace {
		t.Errorf("with the quit deadline spent the shutdown gets %s, want no more than %s", got, appQuitJoinGrace)
	}
	fresh := newQuitJoiner(func(string, string) {}, io.Discard, time.Second)
	if got := appQuitShutdownBound(fresh); got > time.Second {
		t.Errorf("with a second of quit deadline left the shutdown gets %s, want no more than that second", got)
	}
}

// quitPathBoundedByDesign are the deferred calls in runDevServer that
// TestQuitPathJoinsThroughTheHelper leaves alone, each with the reason it
// needs no bound. Everything else that stops or finishes something in a
// defer waits on work the quit path does not own, and goes through
// joinQuit.
var quitPathBoundedByDesign = map[string]string{
	// A context cancel and a close of a channel: neither can block.
	"hub.Stop": "close of a channel",
	// Self-bounded at recorder.KillGrace, and in app mode it returns at
	// once because app mode drives the present recorder, not this
	// controller.
	"recordings.Stop": "self-bounded at recorder.KillGrace",
}

// TestQuitPathJoinsThroughTheHelper reads runDevServer and requires every
// deferred call that stops or finishes something to go through joinQuit,
// which is where the bound and the report of an expiry live. A join
// written by hand is a join whose bound is left to be remembered, and a
// deferred stop in this function is the easiest place in tap to forget
// one: it runs after the session has already established a deadline and
// given up on the goroutine that holds the lock the stop wants, so an
// unbounded wait here undoes that bound a few microseconds later.
func TestQuitPathJoinsThroughTheHelper(t *testing.T) {
	source, err := parser.ParseFile(token.NewFileSet(), "dev.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	var runDevServer *ast.FuncDecl
	for _, declaration := range source.Decls {
		if function, isFunction := declaration.(*ast.FuncDecl); isFunction && function.Name.Name == "runDevServer" {
			runDevServer = function
		}
	}
	if runDevServer == nil {
		t.Fatal("runDevServer is not in dev.go any more; move this test with it")
	}

	ast.Inspect(runDevServer, func(node ast.Node) bool {
		deferred, isDefer := node.(*ast.DeferStmt)
		if !isDefer {
			return true
		}
		var text strings.Builder
		if err := printer.Fprint(&text, token.NewFileSet(), deferred); err != nil {
			t.Fatal(err)
		}
		statement := text.String()
		if !strings.Contains(statement, ".Stop(") && !strings.Contains(statement, ".Finish(") && !strings.Contains(statement, ".Shutdown(") {
			return true
		}
		for call, reason := range quitPathBoundedByDesign {
			if strings.Contains(statement, call+"(") {
				t.Logf("%s is left unbounded on purpose: %s", call, reason)
				return true
			}
		}
		if !strings.Contains(statement, "joinQuit(") {
			t.Errorf("a deferred stop in the quit path does not go through joinQuit:\n%s", statement)
		}
		return true
	})
}

// TestAppLogWriterDropsRatherThanWaitForAStuckStandardError checks the
// property the whole quit path now rests on: a Write returns even when
// the goroutine behind it is inside a write that never does.
func TestAppLogWriterDropsRatherThanWaitForAStuckStandardError(t *testing.T) {
	output := &blockedWriter{entered: make(chan struct{}), release: make(chan struct{})}
	defer close(output.release)
	writer := newAppLogWriter(output)

	fmt.Fprintln(writer, "the first line, which blocks the writer goroutine")
	<-output.entered
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range appLogQueueSize * 2 {
			fmt.Fprintln(writer, "a line with nowhere to go")
		}
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("writing to a standard error that never drains blocked the caller")
	}
	writer.dropMu.Lock()
	dropped := writer.dropped
	writer.dropMu.Unlock()
	if dropped == 0 {
		t.Error("nothing was counted as dropped, so the queue swallowed the run silently")
	}
}

// pausedLog is a standard error that blocks on its first line until it is
// released, and collects every line it is given.
type pausedLog struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	mu      sync.Mutex
	buffer  bytes.Buffer
}

func (log *pausedLog) Write(data []byte) (int, error) {
	log.once.Do(func() {
		close(log.entered)
		<-log.release
	})
	log.mu.Lock()
	defer log.mu.Unlock()
	return log.buffer.Write(data)
}

func (log *pausedLog) String() string {
	log.mu.Lock()
	defer log.mu.Unlock()
	return log.buffer.String()
}

// TestAppLogWriterReportsTheRunOfDropsWhenTheLogIsReadAgain checks that a
// log that starts being read again says what is missing from it. There is
// no other pipe to say it on, so the count is spent here or not at all.
func TestAppLogWriterReportsTheRunOfDropsWhenTheLogIsReadAgain(t *testing.T) {
	output := &pausedLog{entered: make(chan struct{}), release: make(chan struct{})}
	writer := newAppLogWriter(output)
	fmt.Fprintln(writer, "the first line, which blocks the writer goroutine")
	<-output.entered
	for range appLogQueueSize * 2 {
		fmt.Fprintln(writer, "a line with nowhere to go")
	}

	// The log is read again: the queue drains, and the next line in makes
	// room for the notice.
	close(output.release)
	waitUntil(t, "the queue to drain", func() bool { return len(writer.lines) == 0 })
	fmt.Fprintln(writer, "a line that fits again")
	writer.close()

	if !strings.Contains(output.String(), "log lines were dropped") {
		t.Errorf("log = %q, want the run of drops named with its count", output.String())
	}
}
