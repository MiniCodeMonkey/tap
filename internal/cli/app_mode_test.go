package cli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/http/cookiejar"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/MiniCodeMonkey/tap/internal/server"
	"github.com/MiniCodeMonkey/tap/internal/usersettings"
)

// appLogBuffer collects a process's standard error from its own goroutine.
type appLogBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (log *appLogBuffer) Write(data []byte) (int, error) {
	log.mu.Lock()
	defer log.mu.Unlock()
	return log.buffer.Write(data)
}

func (log *appLogBuffer) String() string {
	log.mu.Lock()
	defer log.mu.Unlock()
	return log.buffer.String()
}

// appProcess is a tap --app process that a test started, with its
// standard output read line by line.
type appProcess struct {
	t       *testing.T
	stdin   io.WriteCloser
	events  chan map[string]any
	done    chan struct{}
	exitErr error
	stderr  *appLogBuffer
	base    string
	token   string
	launch  string
	port    int
	mu      sync.Mutex
	lines   []string
	backlog []map[string]any
}

// copyAppFixture copies testdata/app, the --app fixture, to a temporary
// folder and returns the path of its deck.
func copyAppFixture(t *testing.T) string {
	t.Helper()
	source := filepath.Join("testdata", "app")
	// The temporary folder is under a symlink on macOS (/var to
	// /private/var), and the file watcher reports the resolved path. The
	// fixture lives at the resolved path so a test can compare the paths
	// tap reports against the ones it passed in.
	destination, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, content, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(destination, "talk.md")
}

// startAppProcess starts tap with args and its own settings folder, reads
// the ready line, and checks that it is the first line on stdout.
func startAppProcess(t *testing.T, configHome string, args ...string) *appProcess {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}
	command := exec.Command(buildTapBinaryForTest(t), args...)
	command.Env = append(os.Environ(), "XDG_CONFIG_HOME="+configHome)
	stdin, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	process := &appProcess{
		t:      t,
		stdin:  stdin,
		events: make(chan map[string]any, 1024),
		done:   make(chan struct{}),
		stderr: &appLogBuffer{},
	}
	command.Stderr = process.stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	go func() {
		scanner := bufio.NewScanner(stdout)
		scanner.Buffer(make([]byte, 0, 64*1024), 16<<20)
		for scanner.Scan() {
			line := scanner.Text()
			process.mu.Lock()
			process.lines = append(process.lines, line)
			process.mu.Unlock()
			var event map[string]any
			if json.Unmarshal([]byte(line), &event) == nil {
				process.events <- event
			}
		}
		close(process.events)
		process.exitErr = command.Wait()
		close(process.done)
	}()
	t.Cleanup(func() {
		_ = stdin.Close()
		select {
		case <-process.done:
		case <-time.After(15 * time.Second):
			_ = command.Process.Kill()
			<-process.done
		}
	})

	ready := process.next(appEventReady)
	process.mu.Lock()
	first := process.lines[0]
	process.mu.Unlock()
	if !strings.HasPrefix(first, `{"type":"ready"`) {
		t.Fatalf("the first stdout line is %s, want the ready line", first)
	}
	port, _ := ready["port"].(float64)
	process.port = int(port)
	process.token, _ = ready["token"].(string)
	process.launch, _ = ready["launch"].(string)
	process.base = fmt.Sprintf("http://127.0.0.1:%d", process.port)
	return process
}

// next returns the next event of eventType. Events it passes over wait in
// the backlog for a later call.
func (process *appProcess) next(eventType string) map[string]any {
	process.t.Helper()
	return process.nextWhere(eventType, func(map[string]any) bool { return true })
}

// nextWhere returns the next event of eventType that match accepts.
func (process *appProcess) nextWhere(eventType string, match func(map[string]any) bool) map[string]any {
	process.t.Helper()
	wanted := func(event map[string]any) bool { return event["type"] == eventType && match(event) }
	for index, event := range process.backlog {
		if wanted(event) {
			process.backlog = append(process.backlog[:index], process.backlog[index+1:]...)
			return event
		}
	}
	timeout := time.After(30 * time.Second)
	for {
		select {
		case event, open := <-process.events:
			if !open {
				process.t.Fatalf("tap exited before a %s event:\n%s", eventType, process.stderr)
			}
			if wanted(event) {
				return event
			}
			process.backlog = append(process.backlog, event)
		case <-timeout:
			process.t.Fatalf("no %s event within 30 seconds:\n%s", eventType, process.stderr)
			return nil
		}
	}
}

// noEvent fails the test when an event of eventType arrives within wait.
func (process *appProcess) noEvent(eventType string, wait time.Duration) {
	process.t.Helper()
	for _, event := range process.backlog {
		if event["type"] == eventType {
			process.t.Errorf("unexpected %s event: %v", eventType, event)
		}
	}
	timeout := time.After(wait)
	for {
		select {
		case event, open := <-process.events:
			if !open {
				return
			}
			if event["type"] == eventType {
				process.t.Errorf("unexpected %s event: %v", eventType, event)
				return
			}
			process.backlog = append(process.backlog, event)
		case <-timeout:
			return
		}
	}
}

// send writes one command line to tap's stdin.
func (process *appProcess) send(line string) {
	process.t.Helper()
	if _, err := io.WriteString(process.stdin, line+"\n"); err != nil {
		process.t.Fatalf("writing %s: %v", line, err)
	}
}

// waitForExit waits up to 15 seconds for tap to exit and returns its
// exit error.
func (process *appProcess) waitForExit() error {
	process.t.Helper()
	select {
	case <-process.done:
		return process.exitErr
	case <-time.After(15 * time.Second):
		process.t.Fatalf("tap did not exit:\n%s", process.stderr)
		return nil
	}
}

// assertOnlyJSONLines checks every stdout line tap printed: each one is a
// JSON object with a type.
func (process *appProcess) assertOnlyJSONLines() {
	process.t.Helper()
	process.mu.Lock()
	defer process.mu.Unlock()
	for _, line := range process.lines {
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			process.t.Errorf("stdout line is not JSON: %q", line)
			continue
		}
		if eventType, _ := event["type"].(string); eventType == "" {
			process.t.Errorf("stdout line has no type: %q", line)
		}
	}
}

// appHeader is what the app sends on a request that changes something.
func (process *appProcess) appHeader() http.Header {
	return http.Header{
		"Authorization": {"Bearer " + process.token},
		"Origin":        {process.base},
		"Content-Type":  {"application/json"},
	}
}

func (process *appProcess) do(method, path string, body []byte, header http.Header) (int, string) {
	process.t.Helper()
	request, err := http.NewRequest(method, process.base+path, bytes.NewReader(body))
	if err != nil {
		process.t.Fatal(err)
	}
	for name, values := range header {
		request.Header[name] = values
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		process.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer response.Body.Close()
	text, _ := io.ReadAll(response.Body)
	return response.StatusCode, string(text)
}

func (process *appProcess) putSource(source string) (int, string) {
	process.t.Helper()
	body, err := json.Marshal(map[string]string{"source": source})
	if err != nil {
		process.t.Fatal(err)
	}
	return process.do(http.MethodPut, server.AppSourcePath, body, process.appHeader())
}

// presentation is the /api/presentation response, as the app reads it.
func (process *appProcess) presentation() string {
	process.t.Helper()
	_, text := process.do(http.MethodGet, "/api/presentation", nil, http.Header{"Authorization": {"Bearer " + process.token}})
	return text
}

// revision is the deck revision /api/presentation reports, which
// /api/execute requires so a block reference cannot be run against a deck
// that has since changed.
func (process *appProcess) revision() string {
	process.t.Helper()
	var body struct {
		Revision string `json:"revision"`
	}
	if err := json.Unmarshal([]byte(process.presentation()), &body); err != nil {
		process.t.Fatalf("reading the revision: %v", err)
	}
	return body.Revision
}

// execute runs one live code block by reference, as a page does.
func (process *appProcess) execute(slide, block int) (int, string) {
	process.t.Helper()
	body, err := json.Marshal(server.ExecuteRequest{Slide: slide, Block: block, Revision: process.revision()})
	if err != nil {
		process.t.Fatal(err)
	}
	return process.do(http.MethodPost, "/api/execute", body, process.appHeader())
}

// statusForDeclaredLength sends only the headers of a request that
// declares a body of length bytes, and returns the status tap answers.
func (process *appProcess) statusForDeclaredLength(method, path string, length int) int {
	process.t.Helper()
	connection, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", process.port))
	if err != nil {
		process.t.Fatal(err)
	}
	defer connection.Close()
	fmt.Fprintf(connection, "%s %s HTTP/1.1\r\nHost: 127.0.0.1:%d\r\nAuthorization: Bearer %s\r\nOrigin: %s\r\nContent-Type: application/json\r\nContent-Length: %d\r\n\r\n",
		method, path, process.port, process.token, process.base, length)
	response, err := http.ReadResponse(bufio.NewReader(connection), nil)
	if err != nil {
		process.t.Fatalf("%s %s: %v", method, path, err)
	}
	response.Body.Close()
	return response.StatusCode
}

// dialWebSocket opens the WebSocket with the token and reads the
// connected message.
func (process *appProcess) dialWebSocket() (*websocket.Conn, context.Context) {
	process.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	process.t.Cleanup(cancel)
	conn, _, err := websocket.Dial(ctx, fmt.Sprintf("ws://127.0.0.1:%d/ws", process.port), &websocket.DialOptions{
		HTTPHeader: http.Header{"Authorization": {"Bearer " + process.token}},
	})
	if err != nil {
		process.t.Fatalf("dialing the WebSocket: %v", err)
	}
	process.t.Cleanup(func() { _ = conn.Close(websocket.StatusNormalClosure, "") })
	readWebSocketUntil(process.t, ctx, conn, "connected")
	return conn, ctx
}

// readWebSocketUntil reads messages until one of messageType arrives.
func readWebSocketUntil(t *testing.T, ctx context.Context, conn *websocket.Conn, messageType string) map[string]any {
	t.Helper()
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("waiting for a %s message: %v", messageType, err)
		}
		var message map[string]any
		if json.Unmarshal(data, &message) == nil && message["type"] == messageType {
			return message
		}
	}
}

// readDeckChange reads until a page is told the deck changed, and returns
// how: "update" patches the open pages in place, keeping the audience's
// position and whatever a slide is holding, and "reload" throws that away
// and loads every page again. Which of the two a change takes is the whole
// point of the distinction, so the tests read the message that arrives
// rather than waiting for the one they expect.
func readDeckChange(t *testing.T, ctx context.Context, conn *websocket.Conn) string {
	t.Helper()
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("waiting for a deck change: %v", err)
		}
		var message map[string]any
		if json.Unmarshal(data, &message) != nil {
			continue
		}
		switch message["type"] {
		case "update", "reload":
			messageType, _ := message["type"].(string)
			return messageType
		}
	}
}

func TestAppDevStartsWithTheReadyLine(t *testing.T) {
	process := startAppProcess(t, t.TempDir(), "dev", "--app", copyAppFixture(t))
	if process.port == 0 || len(process.token) != 64 || len(process.launch) != 64 || process.token == process.launch {
		t.Errorf("ready line: port %d, token %q, launch %q", process.port, process.token, process.launch)
	}
	process.send(`{"type":"quit"}`)
	if err := process.waitForExit(); err != nil {
		t.Errorf("tap exited with %v after quit, want 0:\n%s", err, process.stderr)
	}
	process.assertOnlyJSONLines()
	if strings.Contains(process.stderr.String(), "Press Ctrl+C") {
		t.Error("tap printed the headless banner in --app mode")
	}
}

func TestAppDevListensOnLoopbackOnly(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}
	lanAddress := firstLANAddress(t)
	process := startAppProcess(t, t.TempDir(), "dev", "--app", copyAppFixture(t))
	connection, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", lanAddress, process.port), time.Second)
	if err == nil {
		connection.Close()
		t.Errorf("tap dev --app answered on %s:%d, want 127.0.0.1 only", lanAddress, process.port)
	}
}

// Every route outside the audience set needs the token: the routes an
// audience browser or a phone remote reaches, whatever address it came in
// on, are the only ones open (see server.audienceRoutes). The app's own
// pages carry the token as a cookie they got for the launch code.
func TestAppDevNeedsTheTokenOnEveryRequest(t *testing.T) {
	process := startAppProcess(t, t.TempDir(), "dev", "--app", copyAppFixture(t))
	for _, test := range []struct {
		name   string
		method string
		path   string
		header http.Header
		want   int
	}{
		{"no token", http.MethodPost, "/api/execute", nil, http.StatusUnauthorized},
		{"the wrong token", http.MethodPost, "/api/execute", http.Header{"Authorization": {"Bearer " + strings.Repeat("0", 64)}}, http.StatusUnauthorized},
		{"no token on the buffer route", http.MethodPut, server.AppSourcePath, nil, http.StatusUnauthorized},
		{"no token on an unknown route", http.MethodGet, "/api/whatever", nil, http.StatusUnauthorized},
		{"the audience deck", http.MethodGet, "/api/presentation", nil, http.StatusOK},
		{"the token", http.MethodGet, "/api/presentation", http.Header{"Authorization": {"Bearer " + process.token}}, http.StatusOK},
	} {
		if status, _ := process.do(test.method, test.path, nil, test.header); status != test.want {
			t.Errorf("%s: %s %s status %d, want %d", test.name, test.method, test.path, status, test.want)
		}
	}
	process.dialWebSocket()
}

// The app's buffer route writes what the presenter's audience sees, so it
// is behind the token like every other route outside the audience set. The
// route is registered on the same mux as the rest, so it reaches its
// handler only through the front door that checks the token, caps the body
// and refuses a cross-site or non-JSON write.
func TestAppDevGuardsTheSourceRouteLikeEveryOtherWrite(t *testing.T) {
	process := startAppProcess(t, t.TempDir(), "dev", "--app", copyAppFixture(t))
	body := []byte(`{"source": "# Unauthenticated\n"}`)

	noToken := process.appHeader()
	noToken.Del("Authorization")
	if status, _ := process.do(http.MethodPut, server.AppSourcePath, body, noToken); status != http.StatusUnauthorized {
		t.Errorf("a PUT without the token: status %d, want 401", status)
	}
	wrongToken := process.appHeader()
	wrongToken.Set("Authorization", "Bearer "+strings.Repeat("0", 64))
	if status, _ := process.do(http.MethodPut, server.AppSourcePath, body, wrongToken); status != http.StatusUnauthorized {
		t.Errorf("a PUT with the wrong token: status %d, want 401", status)
	}
	if strings.Contains(process.presentation(), "Unauthenticated") {
		t.Error("a PUT without the token reached the deck")
	}
}

func TestAppDevTradesTheLaunchCodeForACookie(t *testing.T) {
	process := startAppProcess(t, t.TempDir(), "dev", "--app", copyAppFixture(t))
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	page := &http.Client{Jar: jar}

	response, err := page.Get(process.base + "/api/presentation?launch=" + process.launch)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK || response.Request.URL.RawQuery != "" {
		t.Errorf("launch URL: status %d at %s, want 200 without the code", response.StatusCode, response.Request.URL)
	}
	found := false
	for _, cookie := range jar.Cookies(response.Request.URL) {
		found = found || (cookie.Name == server.AppSessionCookieName(process.port) && cookie.Value == process.token)
	}
	if !found {
		t.Errorf("cookies = %v, want the session cookie", jar.Cookies(response.Request.URL))
	}
	again, err := page.Get(process.base + "/api/presentation")
	if err != nil {
		t.Fatal(err)
	}
	again.Body.Close()
	if again.StatusCode != http.StatusOK {
		t.Errorf("with the cookie: status %d, want 200", again.StatusCode)
	}

	reused, err := http.Get(process.base + "/?launch=" + process.launch)
	if err != nil {
		t.Fatal(err)
	}
	reused.Body.Close()
	if reused.StatusCode != http.StatusForbidden {
		t.Errorf("the launch code a second time: status %d, want 403", reused.StatusCode)
	}
}

func TestAppDevRendersTheBufferUntilSaved(t *testing.T) {
	deck := copyAppFixture(t)
	process := startAppProcess(t, t.TempDir(), "dev", "--app", deck)
	onDisk, err := os.ReadFile(deck)
	if err != nil {
		t.Fatal(err)
	}
	conn, ctx := process.dialWebSocket()

	edited := strings.Replace(string(onDisk), "# App Mode Fixture", "# Edited In The App", 1)
	status, body := process.putSource(edited)
	if status != http.StatusOK {
		t.Fatalf("PUT: status %d: %s", status, body)
	}
	var list slideListResponse
	if err := json.Unmarshal([]byte(body), &list); err != nil {
		t.Fatalf("the PUT answer is not a slide list: %v\n%s", err, body)
	}
	if !list.OK || len(list.Slides) != 4 || list.Slides[0].Title != "Edited In The App" {
		t.Fatalf("slide list = %+v", list)
	}
	if len(list.Slides[1].CodeBlocks) != 1 || list.Slides[1].CodeBlocks[0].Driver != "shell" || list.Slides[3].Steps != 2 {
		t.Errorf("slides 2 and 4 = %+v, %+v", list.Slides[1], list.Slides[3])
	}
	// An ordinary edit patches the open pages in place. A reload here
	// would throw away the audience's position and everything the slides
	// are holding, for a change to one heading.
	if change := readDeckChange(t, ctx, conn); change != "update" {
		t.Errorf("an edit sent %q, want update so the pages keep their state", change)
	}
	if !strings.Contains(process.presentation(), "Edited In The App") {
		t.Error("/api/presentation does not show the buffer")
	}
	if now, _ := os.ReadFile(deck); string(now) != string(onDisk) {
		t.Error("PUT wrote the deck file")
	}

	// The app saves its buffer, then says so. That is not a change made
	// somewhere else.
	if err := os.WriteFile(deck, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	process.send(`{"type":"saved"}`)
	process.noEvent(appEventFileChanged, 2*time.Second)
	if !strings.Contains(process.presentation(), "Edited In The App") {
		t.Error("the saved deck does not show")
	}

	process.send(`{"type":"quit"}`)
	if err := process.waitForExit(); err != nil {
		t.Errorf("exit: %v", err)
	}
	process.assertOnlyJSONLines()
}

func TestAppDevReportsAChangeMadeElsewhere(t *testing.T) {
	deck := copyAppFixture(t)
	process := startAppProcess(t, t.TempDir(), "dev", "--app", deck)
	onDisk, err := os.ReadFile(deck)
	if err != nil {
		t.Fatal(err)
	}
	conn, ctx := process.dialWebSocket()

	if status, body := process.putSource(strings.Replace(string(onDisk), "# App Mode Fixture", "# Unsaved In The App", 1)); status != http.StatusOK {
		t.Fatalf("PUT: status %d: %s", status, body)
	}
	outside := strings.Replace(string(onDisk), "# App Mode Fixture", "# Changed In Another Editor", 1)
	if err := os.WriteFile(deck, []byte(outside), 0o644); err != nil {
		t.Fatal(err)
	}
	event := process.next(appEventFileChanged)
	if event["path"] != deck {
		t.Errorf("file-changed path = %v, want %s", event["path"], deck)
	}
	if _, carriesSlides := event["slides"]; carriesSlides {
		t.Error("a change to the deck carries a slide list")
	}
	if message := readWebSocketUntil(t, ctx, conn, "file-changed"); message["path"] != deck {
		t.Errorf("WebSocket file-changed = %v", message)
	}
	if !strings.Contains(process.presentation(), "Unsaved In The App") {
		t.Error("the buffer stopped showing before saved")
	}

	process.send(`{"type":"saved"}`)
	waitUntil(t, "the deck file shows", func() bool { return strings.Contains(process.presentation(), "Changed In Another Editor") })

	// With no buffer, a change made elsewhere is reported and shown.
	again := strings.Replace(outside, "# Changed In Another Editor", "# Changed Again", 1)
	if err := os.WriteFile(deck, []byte(again), 0o644); err != nil {
		t.Fatal(err)
	}
	process.next(appEventFileChanged)
	waitUntil(t, "the second change shows", func() bool { return strings.Contains(process.presentation(), "Changed Again") })
}

func TestAppDevSendsTheSlideListWhenAComponentChanges(t *testing.T) {
	deck := copyAppFixture(t)
	process := startAppProcess(t, t.TempDir(), "dev", "--app", deck)
	component := filepath.Join(filepath.Dir(deck), "slides", "Counter.jsx")
	source, err := os.ReadFile(component)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(component, bytes.Replace(source, []byte("steps = 2"), []byte("steps = 3"), 1), 0o644); err != nil {
		t.Fatal(err)
	}

	event := process.next(appEventFileChanged)
	if event["path"] != component {
		t.Errorf("path = %v, want %s", event["path"], component)
	}
	slides, _ := event["slides"].([]any)
	if len(slides) != 4 {
		t.Fatalf("slides = %v, want the four slides", event["slides"])
	}
	if steps := slides[3].(map[string]any)["steps"]; steps != float64(3) {
		t.Errorf("slide 4 steps = %v, want 3", steps)
	}
}

func TestAppDevReloadCommandReloadsEveryPage(t *testing.T) {
	process := startAppProcess(t, t.TempDir(), "dev", "--app", copyAppFixture(t))
	conn, ctx := process.dialWebSocket()
	process.send(`{"type":"reload"}`)
	// The reload command is the one path that deliberately throws the
	// pages' state away, which is what makes it worth having next to the
	// in-place update an edit takes.
	if change := readDeckChange(t, ctx, conn); change != "reload" {
		t.Errorf("the reload command sent %q, want reload", change)
	}
	process.send(`{"type":"quit"}`)
	if err := process.waitForExit(); err != nil {
		t.Errorf("exit: %v", err)
	}
	process.assertOnlyJSONLines()
}

func TestAppDevExitsWhenStandardInputCloses(t *testing.T) {
	process := startAppProcess(t, t.TempDir(), "dev", "--app", copyAppFixture(t))
	if err := process.stdin.Close(); err != nil {
		t.Fatal(err)
	}
	if err := process.waitForExit(); err != nil {
		t.Errorf("exit: %v, want 0", err)
	}
	process.assertOnlyJSONLines()
}

func TestAppDevAsksForLiveCodeApprovalOnStdout(t *testing.T) {
	configHome := t.TempDir()
	deck := copyAppFixture(t)
	process := startAppProcess(t, configHome, "dev", "--app", deck)

	question := process.next(appEventQuestion)
	payload, _ := question["payload"].(map[string]any)
	if question["kind"] != appQuestionApproval || payload["deck"] != deck {
		t.Fatalf("question = %v", question)
	}
	if status, _ := process.execute(2, 1); status != http.StatusForbidden {
		t.Errorf("before the answer: status %d, want 403", status)
	}

	process.send(fmt.Sprintf(`{"type":"answer","id":%q,"value":true}`, question["id"]))
	waitUntil(t, "the block runs", func() bool {
		status, body := process.execute(2, 1)
		return status == http.StatusOK && strings.Contains(body, "hello from the fixture")
	})
	deckKey, err := usersettings.ResolveDeck(deck)
	if err != nil {
		t.Fatal(err)
	}
	settings, err := usersettings.Load(filepath.Join(configHome, "tap", "settings.yaml"))
	if err != nil || !settings.Approved(deckKey, []string{"shell"}) {
		t.Errorf("settings = %+v, %v; want the deck approved for shell", settings, err)
	}
	if strings.Contains(process.stderr.String(), "[y/N/s]") {
		t.Error("tap asked on the terminal")
	}
}

func TestAppDevLimitsRequestBodies(t *testing.T) {
	process := startAppProcess(t, t.TempDir(), "dev", "--app", copyAppFixture(t))
	if status := process.statusForDeclaredLength(http.MethodPut, server.AppSourcePath, server.AppSourceBodyLimit+1); status != http.StatusRequestEntityTooLarge {
		t.Errorf("a buffer over 8 MB: status %d, want 413", status)
	}
	if status := process.statusForDeclaredLength(http.MethodPost, "/api/execute", server.RequestBodyLimit+1); status != http.StatusRequestEntityTooLarge {
		t.Errorf("an execute body over 64 KB: status %d, want 413", status)
	}

	crossSite := process.appHeader()
	crossSite.Set("Origin", "https://evil.example")
	if status, _ := process.do(http.MethodPut, server.AppSourcePath, []byte(`{"source": "# Hi"}`), crossSite); status != http.StatusForbidden {
		t.Errorf("a PUT from another site: status %d, want 403", status)
	}
	plainText := process.appHeader()
	plainText.Set("Content-Type", "text/plain")
	if status, _ := process.do(http.MethodPut, server.AppSourcePath, []byte(`{"source": "# Hi"}`), plainText); status != http.StatusUnsupportedMediaType {
		t.Errorf("a text/plain PUT: status %d, want 415", status)
	}
}

func TestAppDevReportsAStartupFailureAsAnErrorEvent(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}
	deck := filepath.Join(t.TempDir(), "broken.md")
	if err := os.WriteFile(deck, []byte("---\naspectRatio: \"7:3\"\n---\n\n# Broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	command := exec.Command(buildTapBinaryForTest(t), "dev", "--app", deck)
	command.Env = append(os.Environ(), "XDG_CONFIG_HOME="+t.TempDir())
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err == nil {
		t.Fatal("tap dev --app started on a deck with an invalid aspectRatio")
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	var event map[string]any
	if len(lines) != 1 || json.Unmarshal([]byte(lines[0]), &event) != nil || event["type"] != appEventError || event["code"] == "" {
		t.Errorf("stdout = %q, want one error event\nstderr: %s", stdout.String(), stderr.String())
	}
}
