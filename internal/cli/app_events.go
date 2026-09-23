package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/slidelist"
)

// Event types on standard output in --app mode.
const (
	appEventReady       = "ready"
	appEventFileChanged = "file-changed"
	appEventQuestion    = "question"
	appEventRecording   = "recording"
	appEventTunnel      = "tunnel"
	appEventSlide       = "slide"
	appEventError       = "error"
)

// Codes of the error events tap sends while it runs in --app mode. A fatal
// error uses the command's own error code instead (see classify).
const (
	appErrorInvalidCommand    = "invalid_command"
	appErrorUnknownCommand    = "unknown_command"
	appErrorUnknownQuestion   = "unknown_question"
	appErrorInvalidAnswer     = "invalid_answer"
	appErrorBusy              = "busy"
	appErrorNotPresenting     = "not_presenting"
	appErrorNotEditing        = "not_editing"
	appErrorReloadFailed      = "reload_failed"
	appErrorTunnelUnavailable = "tunnel_unavailable"
	appErrorTunnelFailed      = "tunnel_failed"
	appErrorRecordingFailed   = "recording_failed"
	appErrorRecordingBlocked  = "recording_blocked"
	appErrorCommandStuck      = "command_stuck"
	appErrorStartupStuck      = "startup_stuck"
	appErrorReporterStuck     = "reporter_stuck"
	appErrorShutdownStuck     = "shutdown_stuck"
)

// appEventQueueSize is how many events can wait for the writer.
const appEventQueueSize = 1024

// appReadyEvent is the first line on standard output. Presenter is the
// secret a client needs to drive the audience's deck, which the app
// passes as ?key= when it opens the presenter view; watching needs
// nothing.
//
//nolint:govet // fieldalignment: field order is the JSON output order
type appReadyEvent struct {
	Type      string `json:"type"`
	Port      int    `json:"port"`
	Token     string `json:"token"`
	Launch    string `json:"launch"`
	Presenter string `json:"presenter"`
}

// appFileChangedEvent reports a file that the watcher saw change and tap
// did not write. For a file other than the deck, such as a component whose
// steps export changed, it also carries the slide list of what tap renders
// now.
//
//nolint:govet // fieldalignment: field order is the JSON output order
type appFileChangedEvent struct {
	Type string `json:"type"`
	Path string `json:"path"`
	*slidelist.Result
}

// appQuestionEvent asks the app something. The answer comes back on
// standard input with the same id.
//
//nolint:govet // fieldalignment: field order is the JSON output order
type appQuestionEvent struct {
	Type    string `json:"type"`
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Payload any    `json:"payload"`
}

// appRecordingEvent is the recording state of a tap present run. Elapsed
// is the current segment's length in whole seconds.
//
//nolint:govet // fieldalignment: field order is the JSON output order
type appRecordingEvent struct {
	Type    string `json:"type"`
	State   string `json:"state"`
	Segment int    `json:"segment"`
	Elapsed int    `json:"elapsed"`
	Disk    string `json:"disk"`
}

// appTunnelEvent is the tunnel state: "starting", "running" with the
// public URL and a QR code of the presenter view (PNG, base64), or
// "stopped".
//
//nolint:govet // fieldalignment: field order is the JSON output order
type appTunnelEvent struct {
	Type  string `json:"type"`
	State string `json:"state"`
	URL   string `json:"url,omitempty"`
	QR    string `json:"qr,omitempty"`
}

// appSlideEvent is the audience position: a 1-based slide and its step.
//
//nolint:govet // fieldalignment: field order is the JSON output order
type appSlideEvent struct {
	Type  string `json:"type"`
	Slide int    `json:"slide"`
	Step  int    `json:"step"`
}

// appErrorEvent reports a fatal or reportable error.
//
//nolint:govet // fieldalignment: field order is the JSON output order
type appErrorEvent struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// appEventWriter is the only writer of standard output in --app mode. Any
// goroutine may emit an event. One goroutine writes each line whole, in
// the order the events were queued, so lines never interleave.
type appEventWriter struct {
	output io.Writer
	log    io.Writer
	lines  chan []byte
	done   chan struct{}
	mu     sync.RWMutex
	closed bool
	// dropMu guards dropped, the length of the run of events emit has
	// thrown away since the queue last had room for one.
	dropMu  sync.Mutex
	dropped int
}

// newAppEventWriter starts the writer goroutine for output. log is where
// the writer reports its own trouble, which in --app mode is standard
// error: an event saying standard output is broken has nowhere to go.
func newAppEventWriter(output, log io.Writer) *appEventWriter {
	writer := &appEventWriter{
		output: output,
		log:    log,
		lines:  make(chan []byte, appEventQueueSize),
		done:   make(chan struct{}),
	}
	go writer.run()
	return writer
}

func (writer *appEventWriter) run() {
	defer close(writer.done)
	failed := false
	for line := range writer.lines {
		if failed {
			continue
		}
		if _, err := writer.output.Write(line); err != nil {
			// The app has gone. tap quits when its standard input closes,
			// so the remaining events have nowhere to go.
			failed = true
			fmt.Fprintf(writer.log, "Standard output closed: %v\n", err)
		}
	}
}

// emit queues event as one JSON line. It does nothing after close, and it
// never blocks: a full queue means the writer goroutine is stuck inside a
// write to standard output, which happens when the app has stopped
// reading its child's pipe. Waiting for room there would stop whoever
// emitted, and the quit path emits, so the process that is trying to
// leave would be held by the writer whose whole job is telling the app
// what happened. The event is dropped instead, and the drop is named on
// standard error.
//
// The queue holds appEventQueueSize lines, which a healthy run never
// fills, so a drop always means the app is not reading.
func (writer *appEventWriter) emit(event any) {
	line, err := json.Marshal(event)
	if err != nil {
		fmt.Fprintf(writer.log, "Encoding an app event: %v\n", err)
		return
	}
	line = append(line, '\n')

	writer.mu.RLock()
	defer writer.mu.RUnlock()
	if writer.closed {
		return
	}
	select {
	case writer.lines <- line:
		writer.noteQueued()
	default:
		writer.noteDropped()
	}
}

// noteDropped counts one dropped event and names the run on standard
// error once, at its first drop. A stuck standard output drops every
// event that follows, so a line per drop would bury the reason under
// thousands of copies of itself. One line opens the run and one closes it
// (see noteQueued), whatever its length.
func (writer *appEventWriter) noteDropped() {
	writer.dropMu.Lock()
	writer.dropped++
	first := writer.dropped == 1
	writer.dropMu.Unlock()
	if first {
		fmt.Fprintln(writer.log, "Standard output is not being read: dropping app events.")
	}
}

// noteQueued ends a run of drops, reporting how many events were lost, so
// the app's log says what it missed rather than only that it missed
// something.
func (writer *appEventWriter) noteQueued() {
	writer.dropMu.Lock()
	dropped := writer.dropped
	writer.dropped = 0
	writer.dropMu.Unlock()
	if dropped > 0 {
		fmt.Fprintf(writer.log, "Standard output is being read again: %d app events were dropped.\n", dropped)
	}
}

// close stops taking events, and returns once every queued line is
// written. It is safe to call more than once.
func (writer *appEventWriter) close() {
	writer.mu.Lock()
	if !writer.closed {
		writer.closed = true
		close(writer.lines)
	}
	writer.mu.Unlock()
	<-writer.done
}

// appEventCloseBound is how long closing the event writer waits for the
// lines already queued to reach standard output.
const appEventCloseBound = 2 * time.Second

// closeAppEventWriter closes events the way the session's quit path waits
// for everything else it does not control: through quitJoiner, so the wait
// is bounded and its expiry is loud. The writer's own close waits for its
// goroutine to finish draining the queue, and that goroutine writes to
// standard output. An app that has stopped reading its child's pipe leaves
// that write blocked for as long as the app lives, so an unbounded wait
// here would hang the last step of a quit whose every other step is
// already bounded.
//
// The bound is its own rather than what is left of the session's quit
// deadline: this also runs on the path that reports a startup failure,
// where no session ever ran and there is no deadline to inherit. Nothing
// here waits on a recorder either, so the deadline derived from the
// recorder's kill grace is not the right length. All that is left is some
// queued lines reaching a pipe, which takes microseconds unless the pipe
// is not being read at all.
//
// An expiry is loud on standard error alone. The app not reading standard
// output is the only way to get here, so an error event would have nowhere
// to go even if the writer were still taking them.
func closeAppEventWriter(events *appEventWriter, log io.Writer) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		events.close()
	}()
	joiner := newQuitJoiner(func(code, message string) {
		fmt.Fprintf(log, "error: %s: %s\n", code, message)
	}, log, appEventCloseBound)
	joiner.join("writing the last events", appErrorShutdownStuck, done)
}
