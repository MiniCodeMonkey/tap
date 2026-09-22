package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

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
)

// appEventQueueSize is how many events can wait for the writer.
const appEventQueueSize = 1024

// appReadyEvent is the first line on standard output.
//
//nolint:govet // fieldalignment: field order is the JSON output order
type appReadyEvent struct {
	Type   string `json:"type"`
	Port   int    `json:"port"`
	Token  string `json:"token"`
	Launch string `json:"launch"`
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
	lines  chan []byte
	done   chan struct{}
	mu     sync.RWMutex
	closed bool
}

// newAppEventWriter starts the writer goroutine for output.
func newAppEventWriter(output io.Writer) *appEventWriter {
	writer := &appEventWriter{
		output: output,
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
			fmt.Fprintf(os.Stderr, "Standard output closed: %v\n", err)
		}
	}
}

// emit queues event as one JSON line. It does nothing after close.
func (writer *appEventWriter) emit(event any) {
	line, err := json.Marshal(event)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Encoding an app event: %v\n", err)
		return
	}
	line = append(line, '\n')

	writer.mu.RLock()
	defer writer.mu.RUnlock()
	if writer.closed {
		return
	}
	writer.lines <- line
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
