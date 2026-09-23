package cli

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// These four tests drive the writers that live outside package cli, in
// internal/parser, internal/config and internal/server. None of them is
// reached through a writer argument tap chose, and none of them can be:
// they are a package's own warning, written to the descriptor the process
// was started with. What keeps them bounded is that --app mode takes that
// descriptor away for the life of the run, so standard output and
// standard error are a pipe tap itself drains into the bounded log
// writer. Each test wedges the app's log pipe, which is what an app that
// is not draining its child's log panel looks like, and then asks tap to
// keep working.

// floodTheLog fills the app's log pipe, which holds about 64 KB, so that
// the next write to it blocks. One unknown command costs tap one log
// line, so the app's own input is enough and nothing is injected into
// tap. It then waits long enough for a control loop that is not wedged to
// have drained the commands.
func (app *unreadLogApp) floodTheLog() {
	app.t.Helper()
	var flood strings.Builder
	for index := range 6000 {
		fmt.Fprintf(&flood, "{\"type\":\"flood-%d\"}\n", index)
	}
	if _, err := io.WriteString(app.stdin, flood.String()); err != nil {
		app.t.Fatal(err)
	}
	time.Sleep(2 * time.Second)
}

// parserWarningDeck writes a deck whose every image carries a width that
// is not a CSS length. internal/parser warns about each one on every
// parse, so a render produces several times a pipe buffer of warnings
// without a single invalid layout. It is a deck someone is midway through
// editing, not a fault injected anywhere in tap.
func parserWarningDeck(t *testing.T) string {
	t.Helper()
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var deck strings.Builder
	deck.WriteString("---\ntitle: Parser Warning Deck\n---\n")
	for slide := range 600 {
		fmt.Fprintf(&deck, "\n---\n\n# Slide %d\n\n", slide)
		for image := range 6 {
			fmt.Fprintf(&deck, "![Picture %d of slide %d](picture.png){width=notavalidlength}\n\n", image, slide)
		}
	}
	path := filepath.Join(directory, "talk.md")
	if err := os.WriteFile(path, []byte(deck.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestAppDevReachesReadyWhenTheDeckWarnsFromTheParser drives the warning
// internal/parser writes on every parse. It is written before tap has
// decided anything about --app, from a package that knows nothing about
// the app protocol, so no writer argument can reach it. With the app not
// reading its log pipe and a deck whose warnings are larger than the
// pipe, a write straight to the descriptor blocks for as long as the app
// lives, and it blocks before tap ever binds a port, so the ready line
// never comes and the app waits on a child that will never speak.
func TestAppDevReachesReadyWhenTheDeckWarnsFromTheParser(t *testing.T) {
	app := startAppWithUnreadLog(t, "dev", "--app", parserWarningDeck(t))
	if !app.waitForReady(60 * time.Second) {
		t.Fatal("tap never reached the ready line on a deck whose parser warnings are larger than the log pipe")
	}
	app.requireCleanProtocol()
}

// unknownThemeDeck writes a small deck naming a theme that does not
// exist. internal/config warns about it through the log package, once per
// deck load, and the log package holds the standard error it was given at
// process start.
func unknownThemeDeck(t *testing.T) string {
	t.Helper()
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "talk.md")
	deck := "---\ntitle: Unknown Theme Deck\ntheme: nosuchtheme\n---\n\n# One\n\n---\n\n# Two\n"
	if err := os.WriteFile(path, []byte(deck), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestAppDevWatchesOnWhenTheThemeIsUnknownAndTheLogIsUnread drives the
// warning internal/config writes through the log package on every deck
// load. The watcher goroutine loads the deck, so a write that blocks
// there takes the watcher with it: the app is told about one change and
// then never about another, with no error and no drop count, which is the
// worst failure this branch has: silent.
func TestAppDevWatchesOnWhenTheThemeIsUnknownAndTheLogIsUnread(t *testing.T) {
	deck := unknownThemeDeck(t)
	app := startAppWithUnreadLog(t, "dev", "--app", deck)
	if !app.waitForReady(30 * time.Second) {
		t.Fatal("no ready line within 30 seconds")
	}
	app.floodTheLog()

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
			t.Fatalf("the file watcher stopped reporting changes after %d of them, on a deck naming a theme that does not exist", touch)
		}
	}
	app.requireCleanProtocol()
}

// probeRejectedUpgrade asks for a WebSocket upgrade with a Host the
// server does not allow, and returns the first line of the response. The
// WebSocket is an audience route, so this needs no credential at all.
func probeRejectedUpgrade(t *testing.T, port int, wait time.Duration) (string, bool) {
	t.Helper()
	connection, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = connection.Close() }()
	if err := connection.SetDeadline(time.Now().Add(wait)); err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(connection, "GET /ws HTTP/1.1\r\nHost: evil.example.com\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: AAAAAAAAAAAAAAAAAAAAAA==\r\n\r\n")
	status, err := bufio.NewReader(connection).ReadString('\n')
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(status), true
}

// TestAppDevAnswersRejectedUpgradesWhenTheLogIsUnread drives the two
// lines internal/server writes through the log package when it turns an
// upgrade away. They are written inside the request handler, before the
// 403 reaches the wire, so a write that blocks leaves the request
// unanswered forever and the handler goroutine parked, one per
// connection. A server that stops answering the requests it is rejecting
// is a server anyone can stop, and no credential is needed to try.
func TestAppDevAnswersRejectedUpgradesWhenTheLogIsUnread(t *testing.T) {
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
	app.floodTheLog()

	for probe := range 3 {
		status, answered := probeRejectedUpgrade(t, app.port, 8*time.Second)
		if !answered {
			t.Fatalf("rejected upgrade %d went unanswered while the log pipe was not being read", probe)
		}
		if !strings.Contains(status, "403") {
			t.Fatalf("rejected upgrade %d answered %q, want 403", probe, status)
		}
	}
	app.requireCleanProtocol()
}

// largeDeckWithoutWarnings writes a deck big enough that the
// presentation JSON does not fit in a socket buffer, so a client that
// hangs up while it is arriving makes the server's encode fail partway
// through. Nothing in it warns about anything: this test is about the
// print on the error path, not about how the deck was parsed.
func largeDeckWithoutWarnings(t *testing.T) string {
	t.Helper()
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var deck strings.Builder
	deck.WriteString("---\ntitle: Large Deck\n---\n")
	for slide := range 600 {
		fmt.Fprintf(&deck, "\n---\n\n# Slide %d\n\n%s\n", slide, strings.Repeat("Some ordinary prose on an ordinary slide. ", 12))
	}
	path := filepath.Join(directory, "talk.md")
	if err := os.WriteFile(path, []byte(deck.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// abortPresentationRequest asks for the presentation and hangs up in the
// middle of the answer, so the server's encode fails partway through. The
// route is an audience route: no credential is sent and none is needed.
func abortPresentationRequest(t *testing.T, port int) {
	t.Helper()
	connection, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(connection, "GET /api/presentation HTTP/1.1\r\nHost: 127.0.0.1:%d\r\n\r\n", port)
	if tcp, isTCP := connection.(*net.TCPConn); isTCP {
		// A reset rather than a polite close, so the server's write fails
		// while it is still encoding rather than after it has finished.
		_ = tcp.SetLinger(0)
	}
	time.Sleep(2 * time.Millisecond)
	_ = connection.Close()
}

// TestAppDevQuitsAfterAbortedPresentationRequestsWithTheLogUnread drives
// the bare print internal/server makes when encoding the presentation
// fails. A bare print writes to whatever standard output is, so in --app
// mode it is a raw write like any other, and it sits inside a request
// handler on a route that needs no credential: an audience page reloading
// at the wrong moment reaches it. A handler parked in that write holds
// its connection, and a connection that is never released is one the
// graceful shutdown waits out, so the quit that should take no time at
// all takes the whole shutdown bound.
func TestAppDevQuitsAfterAbortedPresentationRequestsWithTheLogUnread(t *testing.T) {
	app := startAppWithUnreadLog(t, "dev", "--app", largeDeckWithoutWarnings(t))
	if !app.waitForReady(60 * time.Second) {
		t.Fatal("no ready line within 60 seconds")
	}
	app.floodTheLog()

	for range 40 {
		abortPresentationRequest(t, app.port)
	}
	time.Sleep(time.Second)

	app.send(`{"type":"quit"}`)
	started := time.Now()
	select {
	case <-app.exited:
	case <-time.After(30 * time.Second):
		t.Fatal("tap did not exit within 30 seconds of quit after aborted presentation requests with the log pipe not being read")
	}
	if took := time.Since(started); took > 3*time.Second {
		t.Fatalf("quit took %s after aborted presentation requests, which means handler goroutines were parked in a write and the graceful shutdown had to wait them out", took)
	}
	app.requireCleanProtocol()
}

// TestAppModeTakesTheStandardDescriptorsAway is the one test worth
// keeping about the mechanism rather than about a symptom: it fails if
// the redirect is removed. The four tests above would each go on failing
// too, but they take a subprocess and a minute between them, and this one
// says in one place what the property is.
//
// It stands two files in for the streams an app reads. The protocol is a
// file, so the test can read back exactly what arrived on it. The log is
// a pipe nobody reads and nobody ever will, which is what an app that
// stopped draining its log panel looks like from the inside of tap.
//
// Two things are checked. Standard output and standard error must no
// longer name the files they named, because a descriptor that is still
// there is a descriptor a raw write can reach. And a raw write, in every
// spelling, must return promptly however much of it there is: with the
// descriptors taken away it lands in a pipe tap drains, and without them
// it blocks on the app forever.
//
// What this cannot show from inside the test binary is the color
// helpers and the log package, which hold the process's own standard
// output and standard error rather than the values of os.Stdout and
// os.Stderr, and the test cannot substitute the process's own without
// swallowing go test's output. In a real run those are precisely the
// descriptors that are taken away, and the subprocess tests above drive
// the log package through internal/config and internal/server.
func TestAppModeTakesTheStandardDescriptorsAway(t *testing.T) {
	directory := t.TempDir()
	protocolFile, err := os.Create(filepath.Join(directory, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	logRead, logWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = logWrite.Close()
		_ = logRead.Close()
	})
	realStandardOutput, realStandardError := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = protocolFile, logWrite
	t.Cleanup(func() { os.Stdout, os.Stderr = realStandardOutput, realStandardError })
	// Independent handles on what the two stand-ins point at now, so the
	// test can ask later whether the descriptors still point there. A
	// duplicate shares the open file, which is what os.SameFile compares.
	protocolBefore, logBefore := duplicateOf(t, protocolFile), duplicateOf(t, logWrite)

	protocol, log, restore, err := claimStdoutForApp()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(restore)
	t.Cleanup(closeAppLog)
	events := newAppEventWriter(protocol, log)

	if sameOpenFile(t, os.Stdout, protocolBefore) {
		t.Error("standard output still names the file it named, so a raw write can still reach it")
	}
	if sameOpenFile(t, os.Stderr, logBefore) {
		t.Error("standard error still names the app's log pipe, so a raw write can still block on the app")
	}

	// Every spelling a guard that matches syntax has to be told about,
	// and more of them than the app's log pipe holds. A pipe takes about
	// 64 KB before a write to it blocks.
	written := make(chan struct{})
	go func() {
		defer close(written)
		for index := range 4000 {
			fmt.Println("a bare print nobody is reading", index)
			_, _ = os.Stderr.WriteString("a method on the file nobody is reading\n")
			_, _ = io.WriteString(os.Stderr, "an io.WriteString nobody is reading\n")
			alias := os.Stderr
			fmt.Fprintln(alias, "a write through an alias nobody is reading")
		}
	}()
	select {
	case <-written:
	case <-time.After(30 * time.Second):
		t.Fatal("raw writes to standard output and standard error blocked on an app that is not reading its log pipe, so the descriptors were not taken away")
	}

	events.emit(appReadyEvent{Type: appEventReady, Port: 1, Token: "t", Launch: "l", Presenter: "p"})
	events.close()
	restore()
	closeAppLog()

	if !sameOpenFile(t, os.Stdout, protocolBefore) {
		t.Error("restore did not give standard output back")
	}
	if !sameOpenFile(t, os.Stderr, logBefore) {
		t.Error("restore did not give standard error back")
	}
	protocolText, err := os.ReadFile(protocolFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	if string(protocolText) != `{"type":"ready","port":1,"token":"t","launch":"l","presenter":"p"}`+"\n" {
		t.Errorf("the protocol stream carried more than the ready line:\n%q", protocolText)
	}
}

// duplicateOf is a second descriptor on whatever file names now, which
// keeps pointing there when file itself is pointed somewhere else.
func duplicateOf(t *testing.T, file *os.File) *os.File {
	t.Helper()
	descriptor, err := unix.Dup(int(file.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	duplicate := os.NewFile(uintptr(descriptor), file.Name())
	t.Cleanup(func() { _ = duplicate.Close() })
	return duplicate
}

// sameOpenFile reports whether two descriptors name the same file, which
// is how this test asks whether a descriptor was replaced.
func sameOpenFile(t *testing.T, one, other *os.File) bool {
	t.Helper()
	oneInfo, err := one.Stat()
	if err != nil {
		t.Fatal(err)
	}
	otherInfo, err := other.Stat()
	if err != nil {
		t.Fatal(err)
	}
	return os.SameFile(oneInfo, otherInfo)
}
