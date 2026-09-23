package cli

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fatih/color"
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

// appDescriptorChildEnv marks the re-execution of this test binary that
// stands in for a tap process whose descriptors an --app run has taken
// away. The mechanism takes the process's real standard output and
// standard error, so the only place to observe it is a process whose real
// standard output and standard error are the two streams an app gives
// tap. Swapping the os.Stdout and os.Stderr variables inside this test
// binary would measure two stand-ins instead, and a stand-in made by
// os.Pipe carries a registration in the runtime's poller that no
// descriptor an app hands tap has.
const appDescriptorChildEnv = "TAP_APP_DESCRIPTORS_UNDER_THE_REDIRECT"

// descriptorsUnderTheAppRedirect is the child half of the test below. Its
// real standard output is a file, which is where the protocol has to end
// up and nothing else may, and its real standard error is a pipe nobody
// reads, which is what an app that stopped draining its log panel looks
// like from the inside of tap. It reports why it failed on descriptor 3
// and exits non-zero, because after the redirect there is nowhere else
// left for it to say anything.
func descriptorsUnderTheAppRedirect() {
	report := os.NewFile(3, "report")
	fail := func(format string, args ...any) {
		fmt.Fprintf(report, format+"\n", args...)
		os.Exit(4)
	}
	// Independent handles on what the two descriptors name now, so the
	// child can ask later whether they still name it. A duplicate shares
	// the open file, which is what os.SameFile compares.
	protocolBefore, err := duplicateForComparison(1)
	if err != nil {
		fail("duplicating standard output: %v", err)
	}
	logBefore, err := duplicateForComparison(2)
	if err != nil {
		fail("duplicating standard error: %v", err)
	}

	protocol, appLog, restore, err := claimStdoutForApp()
	if err != nil {
		fail("claimStdoutForApp: %v", err)
	}
	events := newAppEventWriter(protocol, appLog)

	same, err := namesTheSameFile(os.Stdout, protocolBefore)
	if err != nil {
		fail("comparing standard output: %v", err)
	}
	if same {
		fail("standard output still names the file it named, so a raw write can still reach it")
	}
	same, err = namesTheSameFile(os.Stderr, logBefore)
	if err != nil {
		fail("comparing standard error: %v", err)
	}
	if same {
		fail("standard error still names the app's log pipe, so a raw write can still block on the app")
	}

	// Every spelling a guard that matches syntax has to be told about, and
	// more of them than the app's log pipe holds. The colour helpers and
	// the log package are in the list because they hold the process's own
	// standard output and standard error, captured at process start, and
	// those are exactly the descriptors the redirect takes.
	written := make(chan struct{})
	go func() {
		defer close(written)
		for index := range 4000 {
			fmt.Println("a bare print nobody is reading", index)
			_, _ = os.Stderr.WriteString("a method on the file nobody is reading\n")
			_, _ = io.WriteString(os.Stderr, "an io.WriteString nobody is reading\n")
			alias := os.Stderr
			fmt.Fprintln(alias, "a write through an alias nobody is reading")
			_, _ = color.New(color.FgRed).Println("a colour helper on standard output nobody is reading")
			_, _ = color.New(color.FgRed).Fprintln(color.Error, "a colour helper on standard error nobody is reading")
			log.Println("the log package nobody is reading")
		}
	}()
	select {
	case <-written:
	case <-time.After(appDescriptorFloodBound):
		fail("raw writes to standard output and standard error blocked on an app that is not reading its log pipe, so the descriptors were not taken away")
	}

	events.emit(appReadyEvent{Type: appEventReady, Port: 1, Token: "t", Launch: "l", Presenter: "p"})
	events.close()
	restore()
	closeAppLog()

	same, err = namesTheSameFile(os.Stdout, protocolBefore)
	if err != nil {
		fail("comparing standard output after restore: %v", err)
	}
	if !same {
		fail("restore did not give standard output back")
	}
	same, err = namesTheSameFile(os.Stderr, logBefore)
	if err != nil {
		fail("comparing standard error after restore: %v", err)
	}
	if !same {
		fail("restore did not give standard error back")
	}
	fmt.Fprintln(report, "every raw write returned and both descriptors came back")
	os.Exit(0)
}

// appDescriptorFloodBound is how long the flood of raw writes has to
// finish in. With the descriptors taken away every one of them lands in a
// pipe tap drains and the flood takes milliseconds; without them the
// first write that fills the app's log pipe waits for as long as the app
// lives, so anything short of the app's lifetime tells the two apart.
const appDescriptorFloodBound = 30 * time.Second

// TestAppModeTakesTheStandardDescriptorsAway is the one test worth
// keeping about the mechanism rather than about a symptom: it fails if
// the redirect is removed. The four tests above would each go on failing
// too, but they take a subprocess and a minute between them, and this one
// says in one place what the property is.
//
// It runs the test binary again with the two streams an app gives tap:
// standard output a file, so the test can read back exactly what arrived
// on it, and standard error a pipe nobody reads and nobody ever will.
// Those are the process's real descriptors, which is what the mechanism
// takes and what every writer in the process, including the ones that
// captured standard output and standard error at process start, is
// holding.
//
// Three things are checked. Standard output and standard error must no
// longer name the files they named, because a descriptor that is still
// there is a descriptor a raw write can reach. A raw write, in every
// spelling, must return promptly however much of it there is: with the
// descriptors taken away it lands in a pipe tap drains, and without them
// it blocks on the app forever. And the file standing in for the app's
// protocol stream must hold the ready line and nothing else, so a
// spelling that escaped the redirect and printed on standard output is
// caught even if it never blocked.
func TestAppModeTakesTheStandardDescriptorsAway(t *testing.T) {
	if os.Getenv(appDescriptorChildEnv) != "" {
		descriptorsUnderTheAppRedirect()
		return
	}
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}
	directory := t.TempDir()
	protocolFile, err := os.Create(filepath.Join(directory, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	reportFile, err := os.Create(filepath.Join(directory, "report"))
	if err != nil {
		t.Fatal(err)
	}
	// The app's log stream. The read end is held open and never read for
	// the whole run, which is what wedges it, and closing it early would
	// turn the wedge into a broken pipe instead.
	logRead, logWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command(os.Args[0], "-test.run=^TestAppModeTakesTheStandardDescriptorsAway$")
	command.Env = append(os.Environ(), appDescriptorChildEnv+"=1")
	command.Stdout = protocolFile
	command.Stderr = logWrite
	command.ExtraFiles = []*os.File{reportFile}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	_ = logWrite.Close()
	t.Cleanup(func() { _ = logRead.Close() })

	exited := make(chan error, 1)
	go func() { exited <- command.Wait() }()
	var waitErr error
	select {
	case waitErr = <-exited:
	case <-time.After(appDescriptorFloodBound + 30*time.Second):
		_ = command.Process.Kill()
		t.Fatal("the child never exited, so a raw write to a descriptor the redirect was supposed to have taken away is still waiting on an app that is not reading its log pipe")
	}
	reason, _ := os.ReadFile(reportFile.Name())
	if waitErr != nil {
		t.Fatalf("the process under the redirect failed: %v\n%s", waitErr, reason)
	}
	t.Logf("%s", reason)

	protocolText, err := os.ReadFile(protocolFile.Name())
	if err != nil {
		t.Fatal(err)
	}
	if string(protocolText) != `{"type":"ready","port":1,"token":"t","launch":"l","presenter":"p"}`+"\n" {
		t.Errorf("the protocol stream carried more than the ready line:\n%q", protocolText)
	}
}

// duplicateForComparison is a second descriptor on whatever the given
// descriptor names now, which keeps naming it when the descriptor itself
// is pointed somewhere else. It is close-on-exec so that nothing tap
// starts inherits a way back to the streams the redirect took.
func duplicateForComparison(descriptor int) (*os.File, error) {
	duplicate, err := unix.FcntlInt(uintptr(descriptor), unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(duplicate), "duplicate"), nil
}

// namesTheSameFile reports whether two descriptors name the same file,
// which is how this test asks whether a descriptor was replaced.
func namesTheSameFile(one, other *os.File) (bool, error) {
	oneInfo, err := one.Stat()
	if err != nil {
		return false, err
	}
	otherInfo, err := other.Stat()
	if err != nil {
		return false, err
	}
	return os.SameFile(oneInfo, otherInfo), nil
}
