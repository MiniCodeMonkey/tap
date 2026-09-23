package cli

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/tui"
)

// fakeTunnels starts a tunnel at once, unless startErr is set. When
// startBlock is set, Start waits for it to close, or for ctx to end,
// before completing; startEntered, when set, closes as soon as Start is
// called, and startReturned reports whether Start has returned yet. Both
// let a test observe a tunnel start goroutine's lifecycle from outside.
type fakeTunnels struct {
	mu            sync.Mutex
	startErr      error
	url           string
	available     bool
	startBlock    chan struct{}
	startEntered  chan struct{}
	startReturned atomic.Bool
}

var _ tui.TunnelController = (*fakeTunnels)(nil)

func (tunnels *fakeTunnels) Start(ctx context.Context) (string, error) {
	if tunnels.startEntered != nil {
		close(tunnels.startEntered)
	}
	defer tunnels.startReturned.Store(true)
	if tunnels.startBlock != nil {
		select {
		case <-tunnels.startBlock:
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	tunnels.mu.Lock()
	defer tunnels.mu.Unlock()
	if tunnels.startErr != nil {
		return "", tunnels.startErr
	}
	tunnels.url = "https://quiet-river.trycloudflare.com"
	return tunnels.url, nil
}

func (tunnels *fakeTunnels) Stop() error {
	tunnels.mu.Lock()
	defer tunnels.mu.Unlock()
	tunnels.url = ""
	return nil
}

func (tunnels *fakeTunnels) URL() string {
	tunnels.mu.Lock()
	defer tunnels.mu.Unlock()
	return tunnels.url
}

func (tunnels *fakeTunnels) Available() bool     { return tunnels.available }
func (tunnels *fakeTunnels) InstallHint() string { return "brew install cloudflared" }

// sessionHarness runs runAppSession with test channels.
type sessionHarness struct {
	commands  chan appCommand
	signals   chan os.Signal
	questions *appQuestions
	log       *eventLog
	done      chan struct{}
	closeOnce sync.Once
}

func startAppSessionForTest(t *testing.T, configure func(options *appSessionOptions)) *sessionHarness {
	t.Helper()
	events, log := newTestEvents(t)
	harness := &sessionHarness{
		commands:  make(chan appCommand, appCommandQueueSize),
		signals:   make(chan os.Signal, 1),
		questions: newAppQuestions(events),
		log:       log,
		done:      make(chan struct{}),
	}
	options := appSessionOptions{
		Events:            events,
		Questions:         harness.questions,
		Commands:          harness.commands,
		Signals:           harness.signals,
		Reload:            func() error { return nil },
		Tunnels:           &fakeTunnels{available: true},
		Log:               io.Discard,
		RecordingInterval: 10 * time.Millisecond,
	}
	if configure != nil {
		configure(&options)
	}
	go func() {
		runAppSession(options)
		close(harness.done)
	}()
	t.Cleanup(func() {
		harness.closeInput()
		harness.waitForEnd(t)
	})
	return harness
}

func (harness *sessionHarness) send(command appCommand) { harness.commands <- command }

// closeInput is the end of standard input.
func (harness *sessionHarness) closeInput() {
	harness.closeOnce.Do(func() {
		harness.questions.close()
		close(harness.commands)
	})
}

func (harness *sessionHarness) waitForEnd(t *testing.T) {
	t.Helper()
	select {
	case <-harness.done:
	case <-time.After(5 * time.Second):
		t.Fatal("the session did not end")
	}
}

func (harness *sessionHarness) answer(t *testing.T, question map[string]any, value string) {
	t.Helper()
	if err := harness.questions.answer(question["id"].(string), json.RawMessage(value)); err != nil {
		t.Fatal(err)
	}
}

// waitUntil polls condition for up to 10 seconds.
func waitUntil(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting until %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestAppSessionReloadCommand(t *testing.T) {
	var reloads atomic.Int32
	harness := startAppSessionForTest(t, func(options *appSessionOptions) {
		options.Reload = func() error { reloads.Add(1); return nil }
	})
	harness.send(appCommand{Type: appCommandReload})
	waitUntil(t, "the deck reloaded", func() bool { return reloads.Load() == 1 })
}

func TestAppSessionReportsAFailedReload(t *testing.T) {
	harness := startAppSessionForTest(t, func(options *appSessionOptions) {
		options.Reload = func() error { return errors.New("frontmatter: bad") }
	})
	harness.send(appCommand{Type: appCommandReload})
	event := harness.log.next(t, appEventError)
	if event["code"] != appErrorReloadFailed || !strings.Contains(event["message"].(string), "frontmatter: bad") {
		t.Errorf("event = %v", event)
	}
}

func TestAppSessionSavedCommand(t *testing.T) {
	var saves atomic.Int32
	harness := startAppSessionForTest(t, func(options *appSessionOptions) {
		options.Saved = func() error { saves.Add(1); return nil }
	})
	harness.send(appCommand{Type: appCommandSaved})
	waitUntil(t, "saved ran", func() bool { return saves.Load() == 1 })
}

func TestAppSessionSavedWithoutABufferIsAnError(t *testing.T) {
	harness := startAppSessionForTest(t, nil)
	harness.send(appCommand{Type: appCommandSaved})
	if event := harness.log.next(t, appEventError); event["code"] != appErrorNotEditing {
		t.Errorf("event = %v, want not_editing", event)
	}
}

func TestAppSessionStartsAndStopsTheTunnel(t *testing.T) {
	tunnels := &fakeTunnels{available: true}
	harness := startAppSessionForTest(t, func(options *appSessionOptions) {
		options.Tunnels = tunnels
		options.PresenterPassword = "secret"
	})
	start, stop := true, false

	harness.send(appCommand{Type: appCommandTunnel, Start: &start})
	if event := harness.log.next(t, appEventTunnel); event["state"] != "starting" {
		t.Errorf("first tunnel event = %v, want starting", event)
	}
	running := harness.log.next(t, appEventTunnel)
	if running["state"] != "running" || running["url"] != "https://quiet-river.trycloudflare.com" {
		t.Errorf("tunnel event = %v, want running with the URL", running)
	}
	qr, _ := running["qr"].(string)
	if png, err := base64.StdEncoding.DecodeString(qr); err != nil || !bytes.HasPrefix(png, []byte("\x89PNG")) {
		t.Errorf("qr is not a base64 PNG: %v", err)
	}

	harness.send(appCommand{Type: appCommandTunnel, Start: &stop})
	if event := harness.log.next(t, appEventTunnel); event["state"] != "stopped" || tunnels.URL() != "" {
		t.Errorf("tunnel event = %v, url %q; want stopped", event, tunnels.URL())
	}
}

func TestAppSessionReportsATunnelThatCannotStart(t *testing.T) {
	start := true
	unavailable := startAppSessionForTest(t, func(options *appSessionOptions) {
		options.Tunnels = &fakeTunnels{}
	})
	unavailable.send(appCommand{Type: appCommandTunnel, Start: &start})
	event := unavailable.log.next(t, appEventError)
	if event["code"] != appErrorTunnelUnavailable || !strings.Contains(event["message"].(string), "brew install cloudflared") {
		t.Errorf("event = %v", event)
	}

	failing := startAppSessionForTest(t, func(options *appSessionOptions) {
		options.Tunnels = &fakeTunnels{available: true, startErr: errors.New("no network")}
	})
	failing.send(appCommand{Type: appCommandTunnel, Start: &start})
	if event := failing.log.next(t, appEventError); event["code"] != appErrorTunnelFailed {
		t.Errorf("event = %v, want tunnel_failed", event)
	}
	if event := failing.log.nextWhere(t, appEventTunnel, func(event map[string]any) bool { return event["state"] != "starting" }); event["state"] != "stopped" {
		t.Errorf("tunnel event = %v, want stopped", event)
	}
}

func TestAppSessionTunnelWithoutStartIsAnError(t *testing.T) {
	harness := startAppSessionForTest(t, nil)
	harness.send(appCommand{Type: appCommandTunnel})
	if event := harness.log.next(t, appEventError); event["code"] != appErrorInvalidCommand {
		t.Errorf("event = %v, want invalid_command", event)
	}
}

func TestAppSessionStartsTheTunnelAtLaunch(t *testing.T) {
	harness := startAppSessionForTest(t, func(options *appSessionOptions) { options.StartTunnel = true })
	harness.log.nextWhere(t, appEventTunnel, func(event map[string]any) bool { return event["state"] == "running" })
}

func TestAppSessionRecordingCommands(t *testing.T) {
	present := &fakePresent{dir: "/talks/recordings/run"}
	harness := startAppSessionForTest(t, func(options *appSessionOptions) { options.Present = present })
	if event := harness.log.next(t, appEventRecording); event["state"] != "stopped" {
		t.Errorf("first recording event = %v, want stopped", event)
	}
	isState := func(state string, segment float64) func(map[string]any) bool {
		return func(event map[string]any) bool { return event["state"] == state && event["segment"] == segment }
	}

	harness.send(appCommand{Type: appCommandRecording, Action: "new-segment"})
	harness.log.nextWhere(t, appEventRecording, isState("recording", 1))
	harness.send(appCommand{Type: appCommandRecording, Action: "new-segment"})
	harness.log.nextWhere(t, appEventRecording, isState("recording", 2))
	harness.send(appCommand{Type: appCommandRecording, Action: "stop"})
	harness.log.nextWhere(t, appEventRecording, isState("stopped", 2))

	harness.send(appCommand{Type: appCommandRecording, Action: "pause"})
	if event := harness.log.next(t, appEventError); event["code"] != appErrorInvalidCommand {
		t.Errorf("event = %v, want invalid_command", event)
	}
}

func TestAppSessionRecordingCommandReportsTheReason(t *testing.T) {
	present := &fakePresent{toggleErr: errors.New("disk full: free space before recording")}
	harness := startAppSessionForTest(t, func(options *appSessionOptions) { options.Present = present })
	harness.send(appCommand{Type: appCommandRecording, Action: "new-segment"})
	event := harness.log.next(t, appEventError)
	if event["code"] != appErrorRecordingFailed || !strings.Contains(event["message"].(string), "disk full") {
		t.Errorf("event = %v", event)
	}
}

func TestAppSessionRecordingInTapDevIsAnError(t *testing.T) {
	harness := startAppSessionForTest(t, nil)
	harness.send(appCommand{Type: appCommandRecording, Action: "stop"})
	if event := harness.log.next(t, appEventError); event["code"] != appErrorNotPresenting {
		t.Errorf("event = %v, want not_presenting", event)
	}
}

func TestAppSessionQuitAsksToKeepARecording(t *testing.T) {
	present := &fakePresent{dir: "/talks/recordings/run", state: tui.PresentRecording, segment: 2, started: true, leftFirstSlide: true}
	harness := startAppSessionForTest(t, func(options *appSessionOptions) { options.Present = present })
	harness.send(appCommand{Type: appCommandQuit})

	question := harness.log.next(t, appEventQuestion)
	payload, _ := question["payload"].(map[string]any)
	if question["kind"] != appQuestionKeepRecording || payload["directory"] != "/talks/recordings/run" || payload["segments"] != float64(2) {
		t.Errorf("question = %v", question)
	}
	harness.answer(t, question, "false")
	harness.waitForEnd(t)
	if finished, kept := present.finishedWith(); !finished || kept {
		t.Errorf("finished %v, kept %v; want finished and deleted", finished, kept)
	}
	harness.log.nextWhere(t, appEventRecording, func(event map[string]any) bool { return event["state"] == "stopped" })
}

func TestAppSessionQuitWithoutARecordingAsksNothing(t *testing.T) {
	present := &fakePresent{}
	harness := startAppSessionForTest(t, func(options *appSessionOptions) { options.Present = present })
	harness.send(appCommand{Type: appCommandQuit})
	harness.waitForEnd(t)
	if finished, _ := present.finishedWith(); !finished {
		t.Error("the run was not finished")
	}
	if harness.log.drainHas(appEventQuestion) {
		t.Error("quit asked a question for a run with no recording")
	}
}

func TestAppSessionEndOfInputKeepsTheRecording(t *testing.T) {
	present := &fakePresent{state: tui.PresentRecording, segment: 1, started: true, leftFirstSlide: true}
	harness := startAppSessionForTest(t, func(options *appSessionOptions) { options.Present = present })
	harness.closeInput()
	harness.waitForEnd(t)
	if finished, kept := present.finishedWith(); !finished || !kept {
		t.Errorf("finished %v, kept %v; want kept", finished, kept)
	}
	if harness.log.drainHas(appEventQuestion) {
		t.Error("tap asked a question after standard input closed")
	}
}

func TestAppSessionSignalKeepsTheRecording(t *testing.T) {
	present := &fakePresent{state: tui.PresentRecording, segment: 1, started: true, leftFirstSlide: true}
	harness := startAppSessionForTest(t, func(options *appSessionOptions) { options.Present = present })
	harness.signals <- os.Interrupt
	harness.waitForEnd(t)
	if finished, kept := present.finishedWith(); !finished || !kept {
		t.Errorf("finished %v, kept %v; want kept", finished, kept)
	}
}

func TestAppSessionQuitEndsAStartupQuestion(t *testing.T) {
	answered := make(chan bool, 1)
	harness := startAppSessionForTest(t, func(options *appSessionOptions) {
		questions := options.Questions
		options.Startup = func(ctx context.Context) {
			_, gotAnswer := questions.ask(ctx, appQuestionApproval, map[string]string{})
			answered <- gotAnswer
		}
	})
	harness.log.next(t, appEventQuestion)
	harness.send(appCommand{Type: appCommandQuit})
	harness.waitForEnd(t)
	if <-answered {
		t.Error("the startup question was answered")
	}
}

func TestAppSessionUnknownCommand(t *testing.T) {
	harness := startAppSessionForTest(t, nil)
	harness.send(appCommand{Type: "dance"})
	if event := harness.log.next(t, appEventError); event["code"] != appErrorUnknownCommand {
		t.Errorf("event = %v, want unknown_command", event)
	}
}

// TestAppSessionQuitDoesNotHangOnAnUnansweredKeepRecordingQuestion reproduces
// the hang: quit asks keep-recording, standard input stays open, and
// nothing ever answers. Before the fix, ask() waits on context.Background()
// forever, so runAppSession never returns and this test fails when
// waitForEnd's five-second bound trips. After the fix, the ask is bounded
// by KeepRecordingTimeout, so the session ends on its own well inside that
// bound.
func TestAppSessionQuitDoesNotHangOnAnUnansweredKeepRecordingQuestion(t *testing.T) {
	present := &fakePresent{dir: "/talks/recordings/run", state: tui.PresentRecording, segment: 1, started: true, leftFirstSlide: true}
	harness := startAppSessionForTest(t, func(options *appSessionOptions) {
		options.Present = present
		options.KeepRecordingTimeout = 100 * time.Millisecond
	})

	harness.send(appCommand{Type: appCommandQuit})
	question := harness.log.next(t, appEventQuestion)
	if question["kind"] != appQuestionKeepRecording {
		t.Fatalf("question = %v, want keep-recording", question)
	}
	// Deliberately answer nothing and leave standard input open: the
	// harness's own t.Cleanup only closes input after this function
	// returns, so waitForEnd here can only succeed if the ask itself is
	// bounded.
	harness.waitForEnd(t)
}

// TestAppSessionQuitJoinsAnInProgressTunnelStart reproduces the second
// hang-shaped bug: quit returning while a tunnel-start goroutine is still
// running. Before the fix, the tunnel's own timeout context is rooted in
// context.Background(), so cancelling the session does not reach it, and
// runAppSession returns while fakeTunnels.Start is still blocked on
// startBlock, which this test never closes -- startReturned is still false
// right after waitForEnd. After the fix, the tunnel's context is derived
// from the session's, so quit cancels it, Start returns promptly, and
// runAppSession joins that goroutine before returning.
func TestAppSessionQuitJoinsAnInProgressTunnelStart(t *testing.T) {
	tunnels := &fakeTunnels{available: true, startBlock: make(chan struct{}), startEntered: make(chan struct{})}
	harness := startAppSessionForTest(t, func(options *appSessionOptions) { options.Tunnels = tunnels })
	start := true

	harness.send(appCommand{Type: appCommandTunnel, Start: &start})
	<-tunnels.startEntered
	harness.log.next(t, appEventTunnel) // "starting"

	harness.send(appCommand{Type: appCommandQuit})
	harness.waitForEnd(t)

	if !tunnels.startReturned.Load() {
		t.Error("runAppSession returned while the tunnel start goroutine was still running")
	}
}

// TestAppSessionCommandsRunInSendOrder forces the ordering question rather
// than hoping for it: reload is made artificially slower than saved, so
// per-command goroutines racing for the shared order slice would almost
// certainly record saved before reload in at least one of the ten pairs.
// A single worker draining commands in send order records every pair as
// reload-then-saved regardless of how long any one command takes.
func TestAppSessionCommandsRunInSendOrder(t *testing.T) {
	var mu sync.Mutex
	var order []string
	harness := startAppSessionForTest(t, func(options *appSessionOptions) {
		options.Reload = func() error {
			time.Sleep(30 * time.Millisecond)
			mu.Lock()
			order = append(order, "reload")
			mu.Unlock()
			return nil
		}
		options.Saved = func() error {
			mu.Lock()
			order = append(order, "saved")
			mu.Unlock()
			return nil
		}
	})

	const pairs = 10
	for i := 0; i < pairs; i++ {
		harness.send(appCommand{Type: appCommandReload})
		harness.send(appCommand{Type: appCommandSaved})
	}
	waitUntil(t, "every command ran", func() bool {
		mu.Lock()
		defer mu.Unlock()
		return len(order) == pairs*2
	})

	mu.Lock()
	defer mu.Unlock()
	for i := 0; i < pairs; i++ {
		if order[2*i] != "reload" || order[2*i+1] != "saved" {
			t.Fatalf("pair %d = %v, want [reload saved] (full order = %v)", i, order[2*i:2*i+2], order)
		}
	}
}

func TestSlideReporterSendsEachNewPosition(t *testing.T) {
	events, log := newTestEvents(t)
	slides := newSlideReporter(events)
	slides.report(2, 0)
	slides.report(2, 0)
	slides.report(2, 1)
	if first := log.next(t, appEventSlide); first["slide"] != float64(3) || first["step"] != float64(0) {
		t.Errorf("first = %v, want slide 3 step 0", first)
	}
	if second := log.next(t, appEventSlide); second["slide"] != float64(3) || second["step"] != float64(1) {
		t.Errorf("second = %v, want slide 3 step 1; the same position twice must send one event", second)
	}
}
