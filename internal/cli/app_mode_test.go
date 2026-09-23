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
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/MiniCodeMonkey/tap/internal/recorder"
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
	t         *testing.T
	stdin     io.WriteCloser
	events    chan map[string]any
	done      chan struct{}
	exitErr   error
	stderr    *appLogBuffer
	base      string
	token     string
	launch    string
	presenter string
	port      int
	mu        sync.Mutex
	lines     []string
	backlog   []map[string]any
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
	process.presenter, _ = ready["presenter"].(string)
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

// remainingEvents drains and returns every event a process has emitted
// but a test has not yet consumed with next or nextWhere: the backlog,
// plus whatever is still buffered on the events channel. Call it only
// after waitForExit, once the channel is closed and nothing more will
// arrive, so the range below terminates instead of blocking.
func (process *appProcess) remainingEvents() []map[string]any {
	process.t.Helper()
	events := append([]map[string]any{}, process.backlog...)
	for event := range process.events {
		events = append(events, event)
	}
	return events
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

// dialAudienceWebSocket opens the WebSocket the way anyone with the
// address does: no app token, no presenter cookie. Viewing is open in
// --app mode, so this connection registers and receives every broadcast.
func (process *appProcess) dialAudienceWebSocket() (*websocket.Conn, context.Context) {
	process.t.Helper()
	return process.dialWebSocketWith(nil)
}

// dialPresenterWebSocket opens the WebSocket the way the app's own
// presenter window does: it loads /presenter with the presenter secret
// from the ready line, which answers with the presenter auth cookie, and
// carries that cookie into the connection.
func (process *appProcess) dialPresenterWebSocket() (*websocket.Conn, context.Context) {
	process.t.Helper()
	return process.dialWebSocketWith(http.Header{"Cookie": {server.PresenterAuthCookieName + "=" + process.presenterCookie()}})
}

// presenterCookie trades the presenter secret for the presenter auth
// cookie value, as a browser loading the presenter page does.
func (process *appProcess) presenterCookie() string {
	process.t.Helper()
	request, err := http.NewRequest(http.MethodGet, process.base+"/presenter?key="+url.QueryEscape(process.presenter), nil)
	if err != nil {
		process.t.Fatal(err)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	response, err := client.Do(request)
	if err != nil {
		process.t.Fatalf("loading the presenter page: %v", err)
	}
	defer response.Body.Close()
	for _, cookie := range response.Cookies() {
		if cookie.Name == server.PresenterAuthCookieName {
			return cookie.Value
		}
	}
	body, _ := io.ReadAll(response.Body)
	process.t.Fatalf("the presenter page set no auth cookie: %d %s", response.StatusCode, body)
	return ""
}

func (process *appProcess) dialWebSocketWith(header http.Header) (*websocket.Conn, context.Context) {
	process.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	process.t.Cleanup(cancel)
	conn, _, err := websocket.Dial(ctx, fmt.Sprintf("ws://127.0.0.1:%d/ws", process.port), &websocket.DialOptions{HTTPHeader: header})
	if err != nil {
		process.t.Fatalf("dialing the WebSocket: %v", err)
	}
	process.t.Cleanup(func() { _ = conn.Close(websocket.StatusNormalClosure, "") })
	readWebSocketUntil(process.t, ctx, conn, "connected")
	return conn, ctx
}

// sendSlide sends the message a client uses to drive every other client
// to a slide.
func sendSlide(t *testing.T, ctx context.Context, conn *websocket.Conn, index int) {
	t.Helper()
	message, err := json.Marshal(map[string]any{"type": "slide", "slideIndex": index})
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Write(ctx, websocket.MessageText, message); err != nil {
		t.Fatalf("sending a slide message: %v", err)
	}
}

// readSlideIndex reads until a relayed slide message arrives and returns
// the slide it names.
func readSlideIndex(t *testing.T, ctx context.Context, conn *websocket.Conn) int {
	t.Helper()
	message := readWebSocketUntil(t, ctx, conn, "slide")
	index, _ := message["slideIndex"].(float64)
	return int(index)
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

// TestAppDevLeavesViewingOpenAndClosesSteering covers the ruling that
// --app mode sets a presenter password of its own: the audience routes,
// the WebSocket among them, stay reachable by anyone with the address or
// the tunnel link, but driving the audience's deck needs the secret the
// ready line hands the app.
func TestAppDevLeavesViewingOpenAndClosesSteering(t *testing.T) {
	process := startAppProcess(t, t.TempDir(), "dev", "--app", copyAppFixture(t))
	if len(process.presenter) != 64 {
		t.Fatalf("ready line presenter secret %q, want 64 hex characters", process.presenter)
	}

	audience, audienceCtx := process.dialAudienceWebSocket()
	presenter, presenterCtx := process.dialPresenterWebSocket()

	// A client without the secret can watch, so it may not steer.
	sendSlide(t, audienceCtx, audience, 1)
	time.Sleep(250 * time.Millisecond)
	// A client with the secret steers, and the watcher sees where it went.
	sendSlide(t, presenterCtx, presenter, 2)
	if index := readSlideIndex(t, audienceCtx, audience); index != 2 {
		t.Errorf("the audience was driven to slide %d, want 2: a client without the presenter secret steered the deck", index)
	}

	process.send(`{"type":"quit"}`)
	if err := process.waitForExit(); err != nil {
		t.Fatalf("tap exited with %v:\n%s", err, process.stderr)
	}
}

func TestAppPresentAsksItsQuestionsOnStdout(t *testing.T) {
	configHome := t.TempDir()
	process := startAppProcess(t, configHome, "present", "--app", copyAppFixture(t))
	settingsPath := filepath.Join(configHome, "tap", "settings.yaml")

	if recorder.Supported() {
		consent := process.next(appEventQuestion)
		payload, _ := consent["payload"].(map[string]any)
		if consent["kind"] != appQuestionRecordConsent || payload["settingsPath"] != settingsPath {
			t.Fatalf("first question = %v, want record-consent", consent)
		}
		process.send(fmt.Sprintf(`{"type":"answer","id":%q,"value":false}`, consent["id"]))
	}
	approval := process.next(appEventQuestion)
	if approval["kind"] != appQuestionApproval {
		t.Fatalf("question = %v, want approval", approval)
	}
	process.send(fmt.Sprintf(`{"type":"answer","id":%q,"value":false}`, approval["id"]))
	if event := process.next(appEventRecording); event["state"] != "stopped" {
		t.Errorf("recording event = %v, want stopped", event)
	}

	process.send(`{"type":"quit"}`)
	if err := process.waitForExit(); err != nil {
		t.Errorf("exit: %v", err)
	}
	process.assertOnlyJSONLines()
	if recorder.Supported() {
		settings, err := usersettings.Load(settingsPath)
		if err != nil || settings.Present.Record == nil || *settings.Present.Record {
			t.Errorf("settings = %+v, %v; want present.record false", settings, err)
		}
	}
	for _, prompt := range []string{"(y/n)", "[y/N/s]", "Keep this recording?"} {
		if strings.Contains(process.stderr.String(), prompt) {
			t.Errorf("tap prompted on the terminal: %q", prompt)
		}
	}
}

func TestAppPresentReportsTheAudiencePosition(t *testing.T) {
	process := startAppProcess(t, t.TempDir(), "present", "--app", "--no-record", copyAppFixture(t))
	// tap present --app sets the same generated presenter password as tap
	// dev --app, so steering the deck needs the presenter secret, the way
	// the app's own presenter window carries it. A plain connection can only
	// watch.
	conn, ctx := process.dialPresenterWebSocket()
	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"type":"slide","slideIndex":2,"step":1}`)); err != nil {
		t.Fatal(err)
	}
	if event := process.next(appEventSlide); event["slide"] != float64(3) || event["step"] != float64(1) {
		t.Errorf("slide event = %v, want slide 3, step 1", event)
	}
}

func TestAppPresentShowsOnlyTheSavedDeck(t *testing.T) {
	deck := copyAppFixture(t)
	process := startAppProcess(t, t.TempDir(), "present", "--app", "--no-record", deck)
	if status, _ := process.putSource("# Unsaved"); status != http.StatusNotFound && status != http.StatusMethodNotAllowed {
		t.Errorf("PUT in tap present --app: status %d, want no such route", status)
	}
	process.send(`{"type":"saved"}`)
	process.nextWhere(appEventError, func(event map[string]any) bool { return event["code"] == appErrorNotEditing })

	onDisk, err := os.ReadFile(deck)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(deck, []byte(strings.Replace(string(onDisk), "# App Mode Fixture", "# Reloaded From Disk", 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(300 * time.Millisecond)
	if strings.Contains(process.presentation(), "Reloaded From Disk") {
		t.Error("tap present --app picked up a file change without a reload")
	}
	process.send(`{"type":"reload"}`)
	waitUntil(t, "the reload shows the file", func() bool { return strings.Contains(process.presentation(), "Reloaded From Disk") })
}

// appModeCommands are the tap commands that serve a running --app
// process. The property under test, that no HTTP route answers a
// question or touches recording, is about app mode as a whole, not
// about one command in it: the real PUT /api/app/source route, for
// instance, is registered only in tap dev --app's branch of
// runDevServer, so testing tap present --app alone would never see a
// rogue route added next to it. Add a mode here as its own entry, not a
// copy of the test.
var appModeCommands = []struct {
	name string
	args []string
}{
	{name: "dev", args: []string{"dev", "--app"}},
	{name: "present", args: []string{"present", "--app", "--no-record"}},
}

// TestAppModeHasNoRouteThatAnswersOrRecords sends everything a page on the
// same origin could send, with the token, while a question is open. None
// of it answers the question or reaches the recording, because questions
// and control commands travel only over standard input. It runs under
// every command in appModeCommands, since the property holds (or fails)
// per mode, not per test.
func TestAppModeHasNoRouteThatAnswersOrRecords(t *testing.T) {
	for _, appMode := range appModeCommands {
		t.Run(appMode.name, func(t *testing.T) {
			testAppModeHasNoRouteThatAnswersOrRecords(t, appMode.args)
		})
	}
}

func testAppModeHasNoRouteThatAnswersOrRecords(t *testing.T, args []string) {
	configHome := t.TempDir()
	process := startAppProcess(t, configHome, append(append([]string{}, args...), copyAppFixture(t))...)
	question := process.next(appEventQuestion)
	id, _ := question["id"].(string)

	body := []byte(fmt.Sprintf(`{"type":"answer","id":%q,"value":true,"action":"stop","start":true}`, id))
	for _, path := range []string{
		"/api/answer", "/api/app/answer", "/api/question", "/api/app/question",
		"/api/command", "/api/app/command", "/api/approval", "/api/app/approval",
		"/api/recording", "/api/app/recording", "/api/record", "/api/tunnel",
		"/api/quit", "/api/app/quit",
		// A case variant of an answer/recording path: an exact-string
		// route allowlist (the route-registry test) would catch a new
		// pattern in any case, but a request line is case sensitive, so
		// this black-box probe needs its own entries to catch the same
		// shape on its own.
		"/API/app/answer", "/api/APP/recording",
		// A different method on a path that is otherwise legitimately
		// registered (GET /api/presentation): the route-registry test
		// would catch a new pattern here too, but this probe list needs
		// its own entry so the black-box layer isn't only riding along.
		"/api/presentation",
	} {
		for _, method := range []string{http.MethodPost, http.MethodPut} {
			if status, _ := process.do(method, path, body, process.appHeader()); status != http.StatusNotFound && status != http.StatusMethodNotAllowed {
				t.Errorf("%s %s: status %d, want no such route", method, path, status)
			}
		}
	}

	conn, ctx := process.dialWebSocket()
	for _, message := range []string{
		fmt.Sprintf(`{"type":"answer","id":%q,"value":true}`, id),
		`{"type":"recording","action":"stop"}`,
		`{"type":"tunnel","start":true}`,
		`{"type":"quit"}`,
	} {
		if err := conn.Write(ctx, websocket.MessageText, []byte(message)); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(time.Second)

	if status, _ := process.do(http.MethodGet, "/api/presentation", nil, http.Header{"Authorization": {"Bearer " + process.token}}); status != http.StatusOK {
		t.Error("a page message stopped tap")
	}
	if _, err := os.Stat(filepath.Join(configHome, "tap", "settings.yaml")); !os.IsNotExist(err) {
		t.Errorf("a page saved an approval (stat error %v)", err)
	}

	// The question is still open: its answer on stdin is accepted.
	process.send(fmt.Sprintf(`{"type":"answer","id":%q,"value":false}`, id))
	process.send(`{"type":"quit"}`)
	if err := process.waitForExit(); err != nil {
		t.Errorf("exit: %v", err)
	}
	for _, event := range process.remainingEvents() {
		if event["type"] == appEventTunnel || event["code"] == appErrorUnknownQuestion {
			t.Errorf("a page message reached tap: %v", event)
		}
	}
}

// appServerRoutes is every HTTP route a composed --app server is allowed
// to have, per mode. It is an allow-list over the whole routing table, so
// a route registered anywhere, in SetupRoutes or in either of
// runDevServer's own branches, and whatever it is called, has to be added
// here before it can exist. That is the difference between this and the
// black-box probes below it: a probe can only look for paths somebody
// thought to name, and the route nobody names is exactly the one that
// gets through.
//
// The order here does not matter: the test sorts both sides before
// comparing them. Adding a route here is the deliberate step. Ask first whether it
// answers a question or controls the recording, because those travel only
// over standard input and output, and whether it belongs on the audience
// allow-list in internal/server (audienceRoutes) or behind the app token.
var appServerRoutes = map[string][]string{
	"dev": {
		"GET /api/custom-theme.css",
		"GET /api/presentation",
		"GET /assets/",
		"GET /components/",
		"GET /index.html",
		"GET /local/",
		"GET /presenter",
		"GET /presenter.html",
		"GET /presenter/",
		"GET /qr",
		"GET /ws",
		"GET /{$}",
		"POST /api/execute",
		"PUT " + server.AppSourcePath,
	},
	"present": {
		"GET /api/custom-theme.css",
		"GET /api/presentation",
		"GET /assets/",
		"GET /components/",
		"GET /index.html",
		"GET /local/",
		"GET /presenter",
		"GET /presenter.html",
		"GET /presenter/",
		"GET /qr",
		"GET /ws",
		"GET /{$}",
		"POST /api/execute",
	},
}

// TestAppModeServesOnlyTheRoutesOnTheAllowList reads the route table the
// running process reports and compares it, whole, against
// appServerRoutes. It runs under every command in appModeCommands, since
// the two modes register different routes and each one's table is its own
// property.
func TestAppModeServesOnlyTheRoutesOnTheAllowList(t *testing.T) {
	for _, appMode := range appModeCommands {
		t.Run(appMode.name, func(t *testing.T) {
			process := startAppProcess(t, t.TempDir(), append(append([]string{}, appMode.args...), copyAppFixture(t))...)
			// Sorted here rather than by hand: the comparison is
			// between two sets, and a legitimate route written in the
			// wrong place in the list above is a maintenance mistake,
			// not a rogue route.
			want := slices.Clone(appServerRoutes[appMode.name])
			slices.Sort(want)
			got := process.routes()
			if !slices.Equal(got, want) {
				t.Errorf("routes:\n got %v\nwant %v", got, want)
			}
		})
	}
}

// routes is the route table tap reports on standard error at startup,
// sorted. It waits for the line, because standard error is written by its
// own goroutine and may still be a moment behind the ready line.
func (process *appProcess) routes() []string {
	process.t.Helper()
	const prefix = "Routes: "
	var line string
	waitUntil(process.t, "the route table on standard error", func() bool {
		for _, candidate := range strings.Split(process.stderr.String(), "\n") {
			if after, found := strings.CutPrefix(candidate, prefix); found {
				line = after
				return true
			}
		}
		return false
	})
	routes := strings.Split(strings.TrimSpace(line), ", ")
	slices.Sort(routes)
	return routes
}
