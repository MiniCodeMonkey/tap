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
	// page, as r does.
	Reload func() error
	// Saved is the saved command. It is nil in tap present --app, which
	// has no buffer.
	Saved   func() error
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
}

// keepRecordingPayload is the payload of a keep-recording question.
type keepRecordingPayload struct {
	Directory string `json:"directory"`
	Segments  int    `json:"segments"`
}

type appSession struct {
	options     appSessionOptions
	recordingMu sync.Mutex
	tunnelMu    sync.Mutex
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
	session := &appSession{options: options}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

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
		go session.tunnel(true)
	}

	end := func(askToKeep bool) {
		cancel()
		<-startupDone
		<-reporterDone
		session.finishRecording(askToKeep)
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

// handle starts one command. Commands that take time run on their own
// goroutine, so the loop stays free for quit.
func (session *appSession) handle(command appCommand) {
	switch command.Type {
	case appCommandReload:
		go session.runAction(appCommandReload, session.options.Reload)
	case appCommandSaved:
		if session.options.Saved == nil {
			session.fail(appErrorNotEditing, "saved works only in tap dev --app; tap present --app reads the deck file on reload")
			return
		}
		go session.runAction(appCommandSaved, session.options.Saved)
	case appCommandTunnel:
		if command.Start == nil {
			session.fail(appErrorInvalidCommand, `tunnel needs "start": true or false`)
			return
		}
		go session.tunnel(*command.Start)
	case appCommandRecording:
		go session.recording(command.Action)
	default:
		session.fail(appErrorUnknownCommand, fmt.Sprintf("unknown command %q", command.Type))
	}
}

func (session *appSession) runAction(name string, action func() error) {
	if err := action(); err != nil {
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
	ctx, cancel := context.WithTimeout(context.Background(), appTunnelStartTimeout)
	defer cancel()
	url, err := tunnels.Start(ctx)
	if err != nil {
		session.fail(appErrorTunnelFailed, "starting the tunnel: "+err.Error())
		session.options.Events.emit(appTunnelEvent{Type: appEventTunnel, State: "stopped"})
		return
	}
	session.emitTunnelRunning(url)
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
		if answer, answered := session.options.Questions.ask(context.Background(), appQuestionKeepRecording, payload); answered {
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
