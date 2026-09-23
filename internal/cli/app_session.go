package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/server"
	"github.com/MiniCodeMonkey/tap/internal/tui"
)

const (
	// appTunnelStartTimeout bounds a tunnel start, as the u key does.
	appTunnelStartTimeout = 45 * time.Second
	// appTunnelQRSize is the width of the tunnel QR code PNG, in pixels.
	appTunnelQRSize = 512
	// appKeepRecordingTimeout bounds the keep-recording question quit
	// asks, so quit always returns even when standard input stays open
	// and nothing answers. It is short on purpose: quit is a path whose
	// whole point is that the window is closing, so an app not watching
	// for the keep-recording event specifically sees silence for this
	// long and cannot tell it apart from a hang. The unanswered default
	// is "keep the recording" (safe, no data loss), so a short bound
	// costs little: a few seconds is plenty of room for a desktop app to
	// show its own dialog and reply, without exposing a long
	// unresponsive window on the way out.
	appKeepRecordingTimeout = 3 * time.Second
	// appTaskQueueSize matches appCommandQueueSize: readAppCommandLine
	// already refuses to queue more than that many commands, so a task
	// queue of the same size never has to make handle wait for room.
	appTaskQueueSize = appCommandQueueSize
	// appWorkerJoinTimeout bounds how long quit waits for the worker's
	// current task to finish. Reload, Saved, a recording action and a
	// tunnel stop are all otherwise synchronous with no bound of their
	// own, so without this, one that never returns would hang quit
	// itself, not just leak a goroutine. When it expires, the task is
	// abandoned (it may still be running, detached, on its own
	// goroutine) and shutdown continues without it -- finishing tidily
	// is preferable, never mandatory, since the process is on its way
	// out regardless.
	appWorkerJoinTimeout = 5 * time.Second
)

// appSessionOptions is what the --app control loop drives.
type appSessionOptions struct {
	Events    *appEventWriter
	Questions *appQuestions
	// Commands closes at the end of standard input (see readAppCommands).
	Commands <-chan appCommand
	Signals  <-chan os.Signal
	// Startup runs once on its own goroutine: the recording consent and
	// the live code approval. Its context ends when the run ends.
	Startup func(ctx context.Context)
	// Reload is the reload command: render the deck again and reload every
	// page, as r does. It takes the session's context and should honour
	// it: quit cancels this context, and a Reload that keeps working
	// past that is only saved from hanging quit by WorkerJoinTimeout, a
	// backstop of last resort, not a substitute for checking ctx.
	Reload func(ctx context.Context) error
	// Saved is the saved command. It is nil in tap present --app, which
	// has no buffer. Like Reload, it takes the session's context and
	// should honour it.
	Saved   func(ctx context.Context) error
	Tunnels tui.TunnelController
	// Present is the tap present run. It is nil in tap dev --app.
	Present    appPresentControl
	DiskStatus func() string
	// Log is where human-readable lines go: standard error.
	Log io.Writer
	// PresenterPassword rides along in the tunnel QR code.
	PresenterPassword string
	RecordingInterval time.Duration
	// StartTunnel starts the tunnel right away, for --tunnel.
	StartTunnel bool
	// KeepRecordingTimeout bounds how long quit waits for an answer to
	// the keep-recording question before giving up and keeping the
	// recording, so quit always returns even when standard input stays
	// open unanswered.
	KeepRecordingTimeout time.Duration
	// WorkerJoinTimeout bounds how long quit waits for the worker's
	// current task to finish before giving up on it and continuing
	// shutdown anyway, so a command that ignores cancellation cannot
	// hold the process open forever.
	WorkerJoinTimeout time.Duration
}

// keepRecordingPayload is the payload of a keep-recording question.
type keepRecordingPayload struct {
	Directory string `json:"directory"`
	Segments  int    `json:"segments"`
}

type appSession struct {
	options appSessionOptions
	// ctx ends when the run ends. Every goroutine handle starts is tied
	// to it, and its own timeouts (the tunnel start, the keep-recording
	// question) are derived from it, so quit reaches every one of them
	// instead of only the ones already listening on ctx.Done() directly.
	ctx         context.Context
	recordingMu sync.Mutex
	tunnelMu    sync.Mutex
	// tasks is the ordered queue handle feeds. A single worker goroutine
	// drains it, so commands are carried out in the order they were
	// sent, without making the command reader wait for one to finish.
	tasks      chan func()
	workerDone chan struct{}
}

// runAppSession is the control loop of tap dev --app and tap present
// --app. It runs the startup questions, carries out each command from
// standard input, and reports the recording. It returns once the run
// should end: a quit command, which asks keep-recording first when the run
// has a recording, the end of standard input, or a signal. The last two
// ask nothing and keep the recording.
func runAppSession(options appSessionOptions) {
	if options.Log == nil {
		options.Log = os.Stderr
	}
	if options.DiskStatus == nil {
		options.DiskStatus = func() string { return "ok" }
	}
	if options.RecordingInterval == 0 {
		options.RecordingInterval = appRecordingInterval
	}
	if options.KeepRecordingTimeout == 0 {
		options.KeepRecordingTimeout = appKeepRecordingTimeout
	}
	if options.WorkerJoinTimeout == 0 {
		options.WorkerJoinTimeout = appWorkerJoinTimeout
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	session := &appSession{
		options:    options,
		ctx:        ctx,
		tasks:      make(chan func(), appTaskQueueSize),
		workerDone: make(chan struct{}),
	}
	go func() {
		defer close(session.workerDone)
		session.runWorker()
	}()

	startupDone := make(chan struct{})
	go func() {
		defer close(startupDone)
		if options.Startup != nil {
			options.Startup(ctx)
		}
	}()

	reporterDone := make(chan struct{})
	if options.Present != nil {
		reporter := newRecordingReporter(options.Present, options.DiskStatus, options.Events)
		go func() {
			defer close(reporterDone)
			reporter.run(ctx, options.RecordingInterval)
		}()
	} else {
		close(reporterDone)
	}

	if options.StartTunnel {
		session.enqueue(func() { session.tunnel(true) })
	}

	end := func(askToKeep bool) {
		cancel()
		<-startupDone
		<-reporterDone
		workerJoined := true
		select {
		case <-session.workerDone:
		case <-time.After(options.WorkerJoinTimeout):
			workerJoined = false
			session.fail(appErrorCommandStuck, fmt.Sprintf("a command did not stop within %s of quit; leaving it running in the background and shutting down anyway", options.WorkerJoinTimeout))
		}
		session.stopTunnelAtExit()
		if workerJoined {
			session.finishRecording(askToKeep)
		} else {
			// The abandoned task may still hold the recording's own
			// lock, so finishRecording is skipped rather than risking
			// end() itself hanging on the same state finishRecording
			// would need to touch.
			fmt.Fprintln(options.Log, "Skipping the recording summary: a stuck command left its state unknown.")
		}
	}

	for {
		select {
		case command, open := <-options.Commands:
			if !open {
				fmt.Fprintln(options.Log, "Standard input closed, shutting down.")
				end(false)
				return
			}
			if command.Type == appCommandQuit {
				end(true)
				return
			}
			session.handle(command)
		case <-options.Signals:
			fmt.Fprintln(options.Log, "Interrupted, shutting down.")
			end(false)
			return
		}
	}
}

// handle queues one command for the worker goroutine. Commands run in the
// order handle queues them, on a single goroutine, so the loop that reads
// them stays free for quit without commands racing each other for it.
func (session *appSession) handle(command appCommand) {
	switch command.Type {
	case appCommandReload:
		session.enqueue(func() { session.runAction(appCommandReload, session.options.Reload) })
	case appCommandSaved:
		if session.options.Saved == nil {
			session.fail(appErrorNotEditing, "saved works only in tap dev --app; tap present --app reads the deck file on reload")
			return
		}
		session.enqueue(func() { session.runAction(appCommandSaved, session.options.Saved) })
	case appCommandTunnel:
		if command.Start == nil {
			session.fail(appErrorInvalidCommand, `tunnel needs "start": true or false`)
			return
		}
		start := *command.Start
		session.enqueue(func() { session.tunnel(start) })
	case appCommandRecording:
		action := command.Action
		session.enqueue(func() { session.recording(action) })
	default:
		session.fail(appErrorUnknownCommand, fmt.Sprintf("unknown command %q", command.Type))
	}
}

// enqueue queues task for the worker goroutine, or drops it once ctx has
// ended: by then nothing is left to carry it out, and the run is already
// on its way down.
func (session *appSession) enqueue(task func()) {
	select {
	case session.tasks <- task:
	case <-session.ctx.Done():
	}
}

// runWorker carries out queued commands one at a time, in the order they
// were queued, until ctx ends or the queue is closed.
func (session *appSession) runWorker() {
	for {
		select {
		case <-session.ctx.Done():
			return
		default:
		}
		select {
		case task, open := <-session.tasks:
			if !open {
				return
			}
			task()
		case <-session.ctx.Done():
			return
		}
	}
}

// runAction carries out a Reload or Saved command with session.ctx, so a
// quit already in progress reaches it the same way it reaches a tunnel
// start: through cancellation, not only through WorkerJoinTimeout giving
// up on the wait.
func (session *appSession) runAction(name string, action func(ctx context.Context) error) {
	if err := action(session.ctx); err != nil {
		session.fail(appErrorReloadFailed, fmt.Sprintf("%s: %v", name, err))
	}
}

// fail logs message and sends it as an error event.
func (session *appSession) fail(code, message string) {
	fmt.Fprintf(session.options.Log, "error: %s\n", message)
	session.options.Events.emit(appErrorEvent{Type: appEventError, Code: code, Message: message})
}

// recording is the recording command, as c does in tap present:
// new-segment starts a new segment, stopping the current one first, and
// stop stops the recording until the next new-segment.
func (session *appSession) recording(action string) {
	present := session.options.Present
	if present == nil {
		session.fail(appErrorNotPresenting, "recording commands work only in tap present --app")
		return
	}
	session.recordingMu.Lock()
	defer session.recordingMu.Unlock()

	var err error
	switch action {
	case "new-segment":
		if present.State() != tui.PresentNotRecording {
			err = present.Toggle()
		}
		if err == nil {
			err = present.Toggle()
		}
	case "stop":
		if present.State() != tui.PresentNotRecording {
			err = present.Toggle()
		}
	default:
		session.fail(appErrorInvalidCommand, fmt.Sprintf("unknown recording action %q: use new-segment or stop", action))
		return
	}
	if err != nil {
		session.fail(appErrorRecordingFailed, err.Error())
	}
}

// tunnel is the tunnel command, as u does.
func (session *appSession) tunnel(start bool) {
	session.tunnelMu.Lock()
	defer session.tunnelMu.Unlock()
	tunnels := session.options.Tunnels

	if !start {
		if err := tunnels.Stop(); err != nil {
			session.fail(appErrorTunnelFailed, "stopping the tunnel: "+err.Error())
		}
		session.options.Events.emit(appTunnelEvent{Type: appEventTunnel, State: "stopped"})
		return
	}
	if url := tunnels.URL(); url != "" {
		session.emitTunnelRunning(url)
		return
	}
	if !tunnels.Available() {
		session.fail(appErrorTunnelUnavailable, "the tunnel needs cloudflared: "+tunnels.InstallHint())
		return
	}

	session.options.Events.emit(appTunnelEvent{Type: appEventTunnel, State: "starting"})
	ctx, cancel := context.WithTimeout(session.ctx, appTunnelStartTimeout)
	defer cancel()
	url, err := tunnels.Start(ctx)
	if err != nil {
		session.fail(appErrorTunnelFailed, "starting the tunnel: "+err.Error())
		session.options.Events.emit(appTunnelEvent{Type: appEventTunnel, State: "stopped"})
		return
	}
	session.emitTunnelRunning(url)
}

// stopTunnelAtExit stops a tunnel still running when the session ends, so
// closing a session actually closes the deck's public exposure, not just
// the goroutine that started it. It deliberately bypasses tunnelMu:
// session.ctx is already cancelled by the time end() calls this, no new
// tunnel command can be enqueued, and if the abandoned worker task
// (WorkerJoinTimeout already gave up on it) happens to be holding tunnelMu
// itself, waiting for it here would reintroduce the same hang this fix
// removes. It is best effort, bounded the same way as the worker join: the
// process is exiting either way, and a call that will not return promptly
// is abandoned rather than allowed to hold up shutdown further.
//
// Every call into tunnels, including the URL() guard that decides whether
// there is anything to stop, runs inside the same bounded goroutine as
// Stop(). The concrete TunnelController serializes Start, Stop and URL on
// one internal mutex, so if the abandoned worker task is stuck inside
// Start holding that mutex, URL() blocks on it exactly as Stop() would;
// leaving the guard outside the bound would let that block hold up quit
// indefinitely, which is the same failure class WorkerJoinTimeout exists
// to close.
func (session *appSession) stopTunnelAtExit() {
	tunnels := session.options.Tunnels
	if tunnels == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		if tunnels.URL() == "" {
			return
		}
		if err := tunnels.Stop(); err != nil {
			session.fail(appErrorTunnelFailed, "stopping the tunnel at quit: "+err.Error())
			return
		}
		session.options.Events.emit(appTunnelEvent{Type: appEventTunnel, State: "stopped"})
	}()
	select {
	case <-done:
	case <-time.After(session.options.WorkerJoinTimeout):
		session.fail(appErrorTunnelFailed, fmt.Sprintf("stopping the tunnel did not finish within %s of quit; it may still be running", session.options.WorkerJoinTimeout))
	}
}

// emitTunnelRunning sends the running tunnel's URL and a QR code of its
// presenter view, with the presenter password when there is one.
func (session *appSession) emitTunnelRunning(url string) {
	event := appTunnelEvent{Type: appEventTunnel, State: "running", URL: url}
	if qr, err := server.GenerateQRCodeBase64(tui.PresenterTarget(url, session.options.PresenterPassword), appTunnelQRSize); err == nil {
		event.QR = qr
	}
	session.options.Events.emit(event)
}

// finishRecording ends a tap present run. It asks keep-recording first
// when askToKeep is set and the run recorded past the first slide, as q
// does. No answer keeps the recording.
func (session *appSession) finishRecording(askToKeep bool) {
	present := session.options.Present
	if present == nil {
		return
	}
	keep := true
	if askToKeep && present.Started() && present.LeftFirstSlide() {
		payload := keepRecordingPayload{Directory: present.Dir(), Segments: present.Segment()}
		ctx, cancel := context.WithTimeout(context.Background(), session.options.KeepRecordingTimeout)
		defer cancel()
		if answer, answered := session.options.Questions.ask(ctx, appQuestionKeepRecording, payload); answered {
			keep = answer
		}
	}

	summary, err := present.Finish(keep)
	log := session.options.Log
	switch {
	case err != nil:
		session.fail(appErrorRecordingFailed, "finishing the recording: "+err.Error())
	case summary.Dir == "":
	case summary.NeverLeftFirstSlide:
		fmt.Fprintln(log, "Deleted the recording: the deck never left the first slide.")
	case summary.Deleted:
		fmt.Fprintln(log, "Deleted the recording.")
	default:
		fmt.Fprintf(log, "Recording kept: %s\n", summary.Dir)
	}
	session.options.Events.emit(sampleRecording(present, session.options.DiskStatus))
}

// slideReporter sends a slide event each time the audience position
// changes.
type slideReporter struct {
	events    *appEventWriter
	mu        sync.Mutex
	lastSlide int
	lastStep  int
	reported  bool
}

func newSlideReporter(events *appEventWriter) *slideReporter {
	return &slideReporter{events: events}
}

// report sends the position, a 0-based slide index and a step, unless it
// is the position sent last. The event's slide is 1-based.
func (reporter *slideReporter) report(slideIndex, step int) {
	reporter.mu.Lock()
	defer reporter.mu.Unlock()
	if reporter.reported && slideIndex == reporter.lastSlide && step == reporter.lastStep {
		return
	}
	reporter.lastSlide, reporter.lastStep, reporter.reported = slideIndex, step, true
	reporter.events.emit(appSlideEvent{Type: appEventSlide, Slide: slideIndex + 1, Step: step})
}
