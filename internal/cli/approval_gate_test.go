package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/parser"
	"github.com/MiniCodeMonkey/tap/internal/server"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
	"github.com/MiniCodeMonkey/tap/internal/usersettings"
)

// askerAnswer is what a blockingAsker returns for one question.
type askerAnswer struct {
	err      error
	approved bool
}

// blockingAsker hands every question to the test and waits for the test
// to answer it, or for the question's context to end, the way the app
// and the terminal wait for a person.
type blockingAsker struct {
	requests chan approvalRequest
	answers  chan askerAnswer
}

func newBlockingAsker() *blockingAsker {
	return &blockingAsker{requests: make(chan approvalRequest, 16), answers: make(chan askerAnswer)}
}

func (asker *blockingAsker) askApproval(ctx context.Context, request approvalRequest) (bool, error) {
	asker.requests <- request
	select {
	case answer := <-asker.answers:
		return answer.approved, answer.err
	case <-ctx.Done():
		return false, context.Cause(ctx)
	}
}

// nextRequest returns the next question the gate asks.
func (asker *blockingAsker) nextRequest(t *testing.T) approvalRequest {
	t.Helper()
	select {
	case request := <-asker.requests:
		return request
	case <-time.After(5 * time.Second):
		t.Fatal("the gate asked nothing")
		return approvalRequest{}
	}
}

// noRequest fails when the gate asks anything within a short wait.
func (asker *blockingAsker) noRequest(t *testing.T) {
	t.Helper()
	select {
	case request := <-asker.requests:
		t.Fatalf("the gate asked %+v, want no question", request)
	case <-time.After(300 * time.Millisecond):
	}
}

func (asker *blockingAsker) answer(t *testing.T, approved bool) {
	t.Helper()
	select {
	case asker.answers <- askerAnswer{approved: approved}:
	case <-time.After(5 * time.Second):
		t.Fatal("nobody waits for an answer")
	}
}

// gateDeck returns a deck that declares drivers, with one live code block
// for each on its own slide, in name order.
func gateDeck(drivers map[string]config.DriverConfig) (*config.Config, *transformer.TransformedPresentation) {
	cfg := config.DefaultConfig()
	names := make([]string, 0, len(drivers))
	for name, settings := range drivers {
		cfg.Drivers[name] = settings
		names = append(names, name)
	}
	slices.Sort(names)
	parsed := &parser.Presentation{}
	for index, name := range names {
		parsed.Slides = append(parsed.Slides, parser.Slide{Index: index, CodeBlocks: []parser.CodeBlock{
			{Language: "text", Code: "run " + name, Meta: parser.CodeBlockMeta{Driver: name}},
		}})
	}
	return cfg, transformer.New(cfg).Transform(parsed)
}

var (
	shellDriver   = config.DriverConfig{}
	python3Driver = config.DriverConfig{Command: "python3", Args: []string{"-c"}}
	bashDriver    = config.DriverConfig{Command: "bash", Args: []string{"-c"}}
	rubyDriver    = config.DriverConfig{Command: "ruby"}
)

// gateHarness is a gate on a real deck file and settings file, with the
// policies it sets and the changes it reports recorded.
type gateHarness struct {
	gate     *liveCodeGate
	asker    *blockingAsker
	out      *syncBuffer
	input    approvalInput
	mu       sync.Mutex
	policy   server.LiveCodePolicy
	changes  int
	deckKey  usersettings.DeckKey
	settings string
}

// syncBuffer is a bytes.Buffer safe to write from the gate's goroutine
// while the test reads it.
type syncBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *syncBuffer) Write(data []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.Write(data)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}

// newGateHarness returns a gate for a fresh deck whose stored approval
// covers approved.
func newGateHarness(t *testing.T, approved ...usersettings.Driver) *gateHarness {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	harness := &gateHarness{asker: newBlockingAsker(), out: &syncBuffer{}}
	deck := makeDeckFile(t)
	harness.deckKey = resolveKey(t, deck)
	harness.settings = filepath.Join(t.TempDir(), "settings.yaml")
	if len(approved) > 0 {
		var settings usersettings.Settings
		settings.ApproveDrivers(harness.deckKey, approved, approvalNow)
		if err := usersettings.Save(harness.settings, settings); err != nil {
			t.Fatal(err)
		}
	}
	harness.input = approvalInput{
		Now:          func() time.Time { return approvalNow },
		Asker:        harness.asker,
		Out:          harness.out,
		SettingsPath: harness.settings,
		Deck:         deck,
		Interactive:  true,
		Context:      ctx,
	}
	harness.gate = newLiveCodeGate(harness.input)
	harness.gate.attach(func(policy server.LiveCodePolicy) {
		harness.mu.Lock()
		harness.policy = policy
		harness.mu.Unlock()
	}, func() {
		harness.mu.Lock()
		harness.changes++
		harness.mu.Unlock()
	})
	return harness
}

func (harness *gateHarness) currentPolicy() server.LiveCodePolicy {
	harness.mu.Lock()
	defer harness.mu.Unlock()
	return harness.policy
}

// allows reports whether the policy the gate set lets driver run with
// the command line buildDriverRegistry gives it for settings.
func (harness *gateHarness) allows(name string, settings config.DriverConfig) bool {
	cfg := config.DefaultConfig()
	cfg.Drivers[name] = settings
	return harness.currentPolicy().Allows(name, buildDriverRegistry(cfg, "").CommandLine(name))
}

func (harness *gateHarness) stored() usersettings.Settings {
	settings, _ := usersettings.Load(harness.settings)
	return settings
}

// start runs the startup check, which must ask nothing.
func (harness *gateHarness) start(t *testing.T, drivers map[string]config.DriverConfig) {
	t.Helper()
	cfg, pres := gateDeck(drivers)
	type started struct {
		err    error
		policy server.LiveCodePolicy
	}
	done := make(chan started, 1)
	go func() {
		policy, err := harness.gate.startup(cfg, pres)
		done <- started{err: err, policy: policy}
	}()
	var result started
	select {
	case result = <-done:
	case request := <-harness.asker.requests:
		t.Fatalf("startup asked %+v, want no question", request)
	case <-time.After(5 * time.Second):
		t.Fatal("startup never returned")
	}
	if result.err != nil {
		t.Fatal(result.err)
	}
	policy := result.policy
	harness.mu.Lock()
	harness.policy = policy
	harness.mu.Unlock()
}

func (harness *gateHarness) reload(drivers map[string]config.DriverConfig) {
	cfg, pres := gateDeck(drivers)
	harness.gate.reload(cfg, pres)
}

func driverNames(request approvalRequest) string {
	names := make([]string, len(request.Drivers))
	for index, entry := range request.Drivers {
		names[index] = entry.Name + "=" + entry.Command
	}
	return strings.Join(names, ",")
}

var (
	approvedShell   = usersettings.Driver{Name: "shell"}
	approvedPython3 = usersettings.Driver{Name: "python", Command: []string{"python3", "-c"}}
)

func TestGateReloadWithNoNewDriverAsksNothing(t *testing.T) {
	harness := newGateHarness(t, approvedShell, approvedPython3)
	harness.start(t, map[string]config.DriverConfig{"shell": shellDriver, "python": python3Driver})

	harness.reload(map[string]config.DriverConfig{"shell": shellDriver, "python": python3Driver})
	harness.reload(map[string]config.DriverConfig{"shell": shellDriver})
	harness.asker.noRequest(t)
	if !harness.allows("shell", shellDriver) {
		t.Error("the approved shell driver stopped running")
	}
}

func TestGateReloadAsksAboutANewDriverAndKeepsTheApprovedOnesRunning(t *testing.T) {
	harness := newGateHarness(t, approvedShell)
	harness.start(t, map[string]config.DriverConfig{"shell": shellDriver})

	harness.reload(map[string]config.DriverConfig{"shell": shellDriver, "python": python3Driver})
	request := harness.asker.nextRequest(t)
	if driverNames(request) != "python=python3 -c" || strings.Join(request.ApprovedBefore, ",") != "shell" {
		t.Errorf("request = %+v, want only python, with shell approved before", request)
	}
	if len(request.Blocks) != 1 || request.Blocks[0].Code != "run python" {
		t.Errorf("blocks = %+v, want the python block", request.Blocks)
	}
	if !harness.allows("shell", shellDriver) {
		t.Error("shell stopped running while python waits for an answer")
	}
	if harness.allows("python", python3Driver) {
		t.Error("python runs before anyone answered")
	}

	harness.asker.answer(t, true)
	waitUntil(t, "python runs", func() bool { return harness.allows("python", python3Driver) })
	if !harness.stored().Covers(harness.deckKey, approvedPython3) {
		t.Errorf("the approval was not stored: %+v", harness.stored())
	}
	if !harness.stored().Covers(harness.deckKey, approvedShell) {
		t.Errorf("storing python lost shell: %+v", harness.stored())
	}
	harness.mu.Lock()
	changes := harness.changes
	harness.mu.Unlock()
	if changes == 0 {
		t.Error("the gate did not report the change, so open pages keep showing Not approved")
	}
}

func TestGateAsksOneQuestionAtATime(t *testing.T) {
	harness := newGateHarness(t, approvedShell)
	harness.start(t, map[string]config.DriverConfig{"shell": shellDriver})

	harness.reload(map[string]config.DriverConfig{"shell": shellDriver, "python": python3Driver})
	harness.asker.nextRequest(t)
	harness.reload(map[string]config.DriverConfig{"shell": shellDriver, "python": python3Driver})
	harness.asker.noRequest(t)

	harness.reload(map[string]config.DriverConfig{"shell": shellDriver, "python": python3Driver, "ruby": rubyDriver})
	request := harness.asker.nextRequest(t)
	if driverNames(request) != "python=python3 -c,ruby=ruby" {
		t.Errorf("request = %s, want python and ruby in one question", driverNames(request))
	}
	harness.asker.noRequest(t)

	harness.asker.answer(t, true)
	waitUntil(t, "ruby runs", func() bool { return harness.allows("ruby", rubyDriver) })
	harness.asker.noRequest(t)
}

func TestGateWithdrawsAQuestionTheDeckNoLongerNeeds(t *testing.T) {
	harness := newGateHarness(t, approvedShell)
	harness.start(t, map[string]config.DriverConfig{"shell": shellDriver})

	harness.reload(map[string]config.DriverConfig{"shell": shellDriver, "python": python3Driver})
	harness.asker.nextRequest(t)
	harness.reload(map[string]config.DriverConfig{"shell": shellDriver})
	harness.asker.noRequest(t)

	// The withdrawn question counted as no answer, not as a no: python
	// coming back asks again.
	harness.reload(map[string]config.DriverConfig{"shell": shellDriver, "python": python3Driver})
	if request := harness.asker.nextRequest(t); driverNames(request) != "python=python3 -c" {
		t.Errorf("request = %s, want python asked again", driverNames(request))
	}
}

func TestGateDeclineKeepsTheNewDriverRefusedForTheRun(t *testing.T) {
	harness := newGateHarness(t, approvedShell)
	harness.start(t, map[string]config.DriverConfig{"shell": shellDriver})

	harness.reload(map[string]config.DriverConfig{"shell": shellDriver, "python": python3Driver})
	harness.asker.nextRequest(t)
	harness.asker.answer(t, false)
	waitUntil(t, "the no is recorded", func() bool {
		return strings.Contains(harness.out.String(), "Live code is off for python in this run.")
	})
	if harness.allows("python", python3Driver) || !harness.allows("shell", shellDriver) {
		t.Errorf("policy = %+v, want shell only", harness.currentPolicy())
	}

	harness.reload(map[string]config.DriverConfig{"shell": shellDriver, "python": python3Driver})
	harness.asker.noRequest(t)
	if harness.stored().Covers(harness.deckKey, approvedPython3) {
		t.Error("a no stored an approval")
	}

	harness.reload(map[string]config.DriverConfig{"shell": shellDriver, "python": bashDriver})
	if request := harness.asker.nextRequest(t); driverNames(request) != "python=bash -c" {
		t.Errorf("request = %s, want python asked again with its new command", driverNames(request))
	}
}

func TestGateDeclineAtStartupHoldsForReloads(t *testing.T) {
	harness := newGateHarness(t, approvedShell)
	cfg, pres := gateDeck(map[string]config.DriverConfig{"shell": shellDriver, "python": python3Driver})
	started := make(chan server.LiveCodePolicy, 1)
	go func() {
		policy, _ := harness.gate.startup(cfg, pres)
		started <- policy
	}()
	harness.asker.nextRequest(t)
	harness.asker.answer(t, false)
	<-started

	harness.reload(map[string]config.DriverConfig{"shell": shellDriver, "python": python3Driver})
	harness.asker.noRequest(t)
}

func TestGateAsksAgainWhenAnApprovedCommandChanges(t *testing.T) {
	harness := newGateHarness(t, approvedShell, approvedPython3)
	harness.start(t, map[string]config.DriverConfig{"shell": shellDriver, "python": python3Driver})
	if !harness.allows("python", python3Driver) {
		t.Fatal("the approved python driver does not run")
	}

	harness.reload(map[string]config.DriverConfig{"shell": shellDriver, "python": bashDriver})
	request := harness.asker.nextRequest(t)
	if driverNames(request) != "python=bash -c" || strings.Join(request.ApprovedBefore, ",") != "shell" {
		t.Errorf("request = %+v, want python with its new command", request)
	}
	if harness.allows("python", bashDriver) || harness.allows("python", python3Driver) {
		t.Errorf("policy = %+v, want python refused until the new command is approved", harness.currentPolicy())
	}

	harness.asker.answer(t, true)
	waitUntil(t, "python runs bash", func() bool { return harness.allows("python", bashDriver) })
	stored := harness.stored()
	if !stored.Covers(harness.deckKey, usersettings.Driver{Name: "python", Command: []string{"bash", "-c"}}) || stored.Covers(harness.deckKey, approvedPython3) {
		t.Errorf("stored = %+v, want the bash command in place of python3", stored)
	}
}

func TestGateNeverAsksOnReloadWithoutATerminal(t *testing.T) {
	harness := newGateHarness(t, approvedShell)
	harness.input.Interactive = false
	harness.gate = newLiveCodeGate(harness.input)
	harness.gate.attach(func(policy server.LiveCodePolicy) {
		harness.mu.Lock()
		harness.policy = policy
		harness.mu.Unlock()
	}, func() {})
	harness.start(t, map[string]config.DriverConfig{"shell": shellDriver})

	harness.reload(map[string]config.DriverConfig{"shell": shellDriver, "python": python3Driver})
	harness.reload(map[string]config.DriverConfig{"shell": shellDriver, "python": python3Driver})
	harness.asker.noRequest(t)
	if harness.allows("python", python3Driver) || !harness.allows("shell", shellDriver) {
		t.Errorf("policy = %+v, want shell only", harness.currentPolicy())
	}
	if count := strings.Count(harness.out.String(), "not approved to run python"); count != 1 {
		t.Errorf("output = %q, want the refusal named once", harness.out.String())
	}
}

func TestGateAllowCodeAllowsEveryReloadedDriver(t *testing.T) {
	harness := newGateHarness(t)
	harness.input.AllowCode = true
	harness.gate = newLiveCodeGate(harness.input)
	harness.gate.attach(func(policy server.LiveCodePolicy) {
		harness.mu.Lock()
		harness.policy = policy
		harness.mu.Unlock()
	}, func() {})
	harness.start(t, map[string]config.DriverConfig{"shell": shellDriver})
	harness.reload(map[string]config.DriverConfig{"shell": shellDriver, "python": bashDriver})
	harness.asker.noRequest(t)
	if !harness.allows("python", bashDriver) {
		t.Error("--allow-code refused a driver added by a reload")
	}
}

func TestGateAsksOnlyOnceAnAskerIsSet(t *testing.T) {
	harness := newGateHarness(t, approvedShell)
	harness.start(t, map[string]config.DriverConfig{"shell": shellDriver})
	harness.gate.setAsker(nil)

	harness.reload(map[string]config.DriverConfig{"shell": shellDriver, "python": python3Driver})
	harness.asker.noRequest(t)

	harness.gate.setAsker(harness.asker)
	if request := harness.asker.nextRequest(t); driverNames(request) != "python=python3 -c" {
		t.Errorf("request = %s, want python asked once the asker is set", driverNames(request))
	}
}

func TestStartupAsksAgainForARecordStoredWithoutACommand(t *testing.T) {
	harness := newGateHarness(t, usersettings.Driver{Name: "python"}, approvedShell)
	cfg, pres := gateDeck(map[string]config.DriverConfig{"shell": shellDriver, "python": python3Driver})
	started := make(chan server.LiveCodePolicy, 1)
	go func() {
		policy, _ := harness.gate.startup(cfg, pres)
		started <- policy
	}()
	request := harness.asker.nextRequest(t)
	if driverNames(request) != "python=python3 -c" || strings.Join(request.ApprovedBefore, ",") != "shell" {
		t.Errorf("request = %+v, want python asked about its command", request)
	}
	harness.asker.answer(t, true)
	policy := <-started
	if !policy.Allows("python", []string{"python3", "-c"}) {
		t.Errorf("policy = %+v, want python allowed after a yes", policy)
	}
	if !harness.stored().Covers(harness.deckKey, approvedPython3) {
		t.Errorf("stored = %+v, want python stored with its command", harness.stored())
	}
}

func TestAppApprovalAskerWithdrawsAQuestionWhenItsContextIsWithdrawn(t *testing.T) {
	events, log := newTestEvents(t)
	questions := newAppQuestions(events)
	asker := appApprovalAsker{questions: questions}
	ctx, withdraw := context.WithCancelCause(context.Background())
	outcome := make(chan error, 1)
	go func() {
		_, err := asker.askApproval(ctx, approvalRequest{Deck: "/talks/talk.md"})
		outcome <- err
	}()
	question := log.next(t, appEventQuestion)
	withdraw(errQuestionWithdrawn)

	closed := log.next(t, appEventQuestionClosed)
	if closed["id"] != question["id"] {
		t.Errorf("question-closed = %v, want the id of %v", closed, question)
	}
	if err := <-outcome; !errors.Is(err, errQuestionWithdrawn) {
		t.Errorf("error = %v, want the question withdrawn", err)
	}
	if err := questions.answer(fmt.Sprint(question["id"]), json.RawMessage("true")); !errors.Is(err, errUnknownQuestion) {
		t.Errorf("answering a withdrawn question: %v, want errUnknownQuestion", err)
	}
}
