package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
			if json.Unmarshal(scanner.Bytes(), &event) == nil {
				select {
				case app.events <- event:
				default:
				}
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

// rawStandardErrorWrites are the functions in package cli that write
// straight to the raw standard error descriptor, each with the reason it
// is out of reach of --app mode. Everything tap does under --app goes
// through the bounded log writer instead, because a write to a pipe the
// app is not draining blocks for as long as the app lives, and a blocked
// write is how this branch has lost the process's exit before.
var rawStandardErrorWrites = map[string]string{
	"runBuild":        "tap build has no --app",
	"runExportPDF":    "tap export pdf has no --app",
	"runExportImages": "tap export images has no --app",
	"spinner.start":   "draws nothing unless standard error is a terminal, which an app pipe is not",
	"spinner.stop":    "erases what start drew, under the same terminal check",
}

// TestNoRawStandardErrorWriteIsReachableInAppMode reads package cli and
// requires every write to os.Stderr to be in a function that --app mode
// cannot reach. Patching the one call a reproduction happened to find
// leaves the others, so the rule is the writer, not the call site: a new
// raw write has to be named here, with the reason --app never runs it,
// before it can land.
func TestNoRawStandardErrorWriteIsReachableInAppMode(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		fileSet := token.NewFileSet()
		source, err := parser.ParseFile(fileSet, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, declaration := range source.Decls {
			function, isFunction := declaration.(*ast.FuncDecl)
			if !isFunction {
				continue
			}
			functionName := function.Name.Name
			if function.Recv != nil && len(function.Recv.List) == 1 {
				functionName = receiverTypeName(function.Recv.List[0].Type) + "." + functionName
			}
			ast.Inspect(function, func(node ast.Node) bool {
				call, isCall := node.(*ast.CallExpr)
				if !isCall || !writesToRawStandardError(call) {
					return true
				}
				if reason, known := rawStandardErrorWrites[functionName]; known {
					t.Logf("%s in %s writes to the raw descriptor on purpose: %s", functionName, name, reason)
					return true
				}
				var text strings.Builder
				if err := printer.Fprint(&text, fileSet, call); err != nil {
					t.Fatal(err)
				}
				t.Errorf("%s in %s writes straight to standard error, which --app mode must not reach:\n%s", functionName, name, text.String())
				return true
			})
		}
	}
}

// receiverTypeName is a method receiver's type name, without the pointer.
func receiverTypeName(expression ast.Expr) string {
	if star, isPointer := expression.(*ast.StarExpr); isPointer {
		expression = star.X
	}
	if name, isName := expression.(*ast.Ident); isName {
		return name.Name
	}
	return "?"
}

// writesToRawStandardError reports whether call hands os.Stderr to one of
// fmt's writers. A reference to os.Stderr that is not written to, such as
// the default value of a log writer or a terminal check, is not a write.
func writesToRawStandardError(call *ast.CallExpr) bool {
	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector {
		return false
	}
	qualifier, isName := selector.X.(*ast.Ident)
	if !isName || qualifier.Name != "fmt" || !strings.HasPrefix(selector.Sel.Name, "Fprint") {
		return false
	}
	if len(call.Args) == 0 {
		return false
	}
	target, isTarget := call.Args[0].(*ast.SelectorExpr)
	if !isTarget {
		return false
	}
	owner, isOwner := target.X.(*ast.Ident)
	return isOwner && owner.Name == "os" && target.Sel.Name == "Stderr"
}

// TestNoBarePrintInRunDevServerIsReachableInAppMode covers the one thing
// the redirect above cannot reach. os.Stdout has to stay a real file, so
// --app mode points it at standard error itself rather than at the
// bounded writer, and a bare fmt.Print is therefore the one write in tap
// dev and tap present that still goes to the raw descriptor. Every one of
// them has to sit in a branch --app does not run: the tunnel banner, the
// headless banner, the terminal interface. --app rejects --headless and
// starts no tunnel of its own, so the condition is the proof.
func TestNoBarePrintInRunDevServerIsReachableInAppMode(t *testing.T) {
	fileSet := token.NewFileSet()
	source, err := parser.ParseFile(fileSet, "dev.go", nil, 0)
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

	var ancestors []ast.Node
	ast.Inspect(runDevServer, func(node ast.Node) bool {
		if node == nil {
			ancestors = ancestors[:len(ancestors)-1]
			return false
		}
		ancestors = append(ancestors, node)
		call, isCall := node.(*ast.CallExpr)
		if !isCall || !isBarePrint(call) || guardedAgainstAppMode(fileSet, ancestors) {
			return true
		}
		var text strings.Builder
		if err := printer.Fprint(&text, fileSet, call); err != nil {
			t.Fatal(err)
		}
		t.Errorf("a bare print in runDevServer is not in a branch --app skips, so it writes to the raw descriptor:\n%s", text.String())
		return true
	})
}

// isBarePrint reports whether call is one of fmt's package-level prints,
// which write to os.Stdout rather than to a writer a caller chose.
func isBarePrint(call *ast.CallExpr) bool {
	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector {
		return false
	}
	qualifier, isName := selector.X.(*ast.Ident)
	if !isName || qualifier.Name != "fmt" {
		return false
	}
	name := selector.Sel.Name
	return name == "Print" || name == "Printf" || name == "Println"
}

// guardedAgainstAppMode reports whether any enclosing if statement asks
// about --app or about headless mode, which --app cannot be combined
// with.
func guardedAgainstAppMode(fileSet *token.FileSet, ancestors []ast.Node) bool {
	for _, ancestor := range ancestors {
		branch, isBranch := ancestor.(*ast.IfStmt)
		if !isBranch {
			continue
		}
		var condition strings.Builder
		if printer.Fprint(&condition, fileSet, branch.Cond) != nil {
			continue
		}
		if strings.Contains(condition.String(), "options.app") || strings.Contains(condition.String(), "headless") {
			return true
		}
	}
	return false
}
