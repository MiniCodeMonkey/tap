package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/slidelist"
)

// eventLog collects the JSON lines an appEventWriter writes, one event per
// line, for tests.
type eventLog struct {
	lines chan map[string]any
}

func (log *eventLog) Write(data []byte) (int, error) {
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return 0, fmt.Errorf("not a JSON line: %q", line)
		}
		log.lines <- event
	}
	return len(data), nil
}

// newTestEvents returns an event writer whose lines go to an eventLog. The
// writer closes when the test ends.
func newTestEvents(t *testing.T) (*appEventWriter, *eventLog) {
	t.Helper()
	log := &eventLog{lines: make(chan map[string]any, 256)}
	events := newAppEventWriter(log, io.Discard)
	t.Cleanup(events.close)
	return events, log
}

// next returns the next event of eventType, and drops the events before it.
func (log *eventLog) next(t *testing.T, eventType string) map[string]any {
	t.Helper()
	return log.nextWhere(t, eventType, func(map[string]any) bool { return true })
}

// nextWhere returns the next event of eventType that match accepts, and
// drops the events before it.
func (log *eventLog) nextWhere(t *testing.T, eventType string, match func(map[string]any) bool) map[string]any {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case event := <-log.lines:
			if event["type"] == eventType && match(event) {
				return event
			}
		case <-timeout:
			t.Fatalf("no matching %s event within 5 seconds", eventType)
			return nil
		}
	}
}

// drainHas reports whether any event already written has eventType.
func (log *eventLog) drainHas(eventType string) bool {
	time.Sleep(50 * time.Millisecond)
	found := false
	for {
		select {
		case event := <-log.lines:
			if event["type"] == eventType {
				found = true
			}
		default:
			return found
		}
	}
}

func TestAppEventWriterWritesWholeLinesFromManyGoroutines(t *testing.T) {
	var output bytes.Buffer
	writer := newAppEventWriter(&output, io.Discard)
	var group sync.WaitGroup
	for sender := range 20 {
		group.Add(1)
		go func() {
			defer group.Done()
			for index := range 50 {
				writer.emit(appErrorEvent{Type: appEventError, Code: "test", Message: strings.Repeat("x", 200) + fmt.Sprint(sender, index)})
			}
		}()
	}
	group.Wait()
	writer.close()

	lines := strings.Split(strings.TrimSuffix(output.String(), "\n"), "\n")
	if len(lines) != 1000 {
		t.Fatalf("%d lines, want 1000", len(lines))
	}
	for _, line := range lines {
		var event appErrorEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil || event.Type != appEventError {
			t.Fatalf("line %q is not one whole event: %v", line, err)
		}
	}
}

func TestAppEventWriterDropsEventsAfterClose(t *testing.T) {
	var output bytes.Buffer
	writer := newAppEventWriter(&output, io.Discard)
	writer.emit(appSlideEvent{Type: appEventSlide, Slide: 1})
	writer.close()
	writer.emit(appSlideEvent{Type: appEventSlide, Slide: 2})
	writer.close()
	if output.String() != `{"type":"slide","slide":1,"step":0}`+"\n" {
		t.Errorf("output = %q, want only the event before close", output.String())
	}
}

type failingWriter struct{ writes int }

func (writer *failingWriter) Write([]byte) (int, error) {
	writer.writes++
	return 0, errors.New("broken pipe")
}

func TestAppEventWriterStopsWritingAfterAFailedWrite(t *testing.T) {
	output := &failingWriter{}
	writer := newAppEventWriter(output, io.Discard)
	for range 3 {
		writer.emit(appSlideEvent{Type: appEventSlide, Slide: 1})
	}
	writer.close()
	if output.writes != 1 {
		t.Errorf("%d writes, want 1: the app is gone after the first failure", output.writes)
	}
}

func TestAppEventJSON(t *testing.T) {
	for _, test := range []struct {
		event any
		want  string
	}{
		{appReadyEvent{Type: appEventReady, Port: 49152, Token: "t", Launch: "l", Presenter: "p"}, `{"type":"ready","port":49152,"token":"t","launch":"l","presenter":"p"}`},
		{appFileChangedEvent{Type: appEventFileChanged, Path: "/talks/talk.md"}, `{"type":"file-changed","path":"/talks/talk.md"}`},
		{
			appFileChangedEvent{Type: appEventFileChanged, Path: "/talks/slides/Counter.jsx", Result: &slidelist.Result{Slides: []slidelist.Slide{}, Errors: []string{}}},
			`{"type":"file-changed","path":"/talks/slides/Counter.jsx","slides":[],"errors":[]}`,
		},
		{appQuestionEvent{Type: appEventQuestion, ID: "q1", Kind: "approval", Payload: map[string]string{"deck": "/talks/talk.md"}}, `{"type":"question","id":"q1","kind":"approval","payload":{"deck":"/talks/talk.md"}}`},
		{appRecordingEvent{Type: appEventRecording, State: "recording", Segment: 2, Elapsed: 75, Disk: "ok"}, `{"type":"recording","state":"recording","segment":2,"elapsed":75,"disk":"ok"}`},
		{appTunnelEvent{Type: appEventTunnel, State: "stopped"}, `{"type":"tunnel","state":"stopped"}`},
		{appSlideEvent{Type: appEventSlide, Slide: 3, Step: 1}, `{"type":"slide","slide":3,"step":1}`},
		{appErrorEvent{Type: appEventError, Code: "busy", Message: "m"}, `{"type":"error","code":"busy","message":"m"}`},
	} {
		encoded, err := json.Marshal(test.event)
		if err != nil || string(encoded) != test.want {
			t.Errorf("json = %s, %v; want %s", encoded, err, test.want)
		}
	}
}

func TestClaimStdoutForAppSendsEverythingElseToStderr(t *testing.T) {
	directory := t.TempDir()
	fakeStdout, err := os.Create(filepath.Join(directory, "stdout"))
	if err != nil {
		t.Fatal(err)
	}
	fakeStderr, err := os.Create(filepath.Join(directory, "stderr"))
	if err != nil {
		t.Fatal(err)
	}
	realStdout, realStderr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = fakeStdout, fakeStderr
	t.Cleanup(func() { os.Stdout, os.Stderr = realStdout, realStderr })

	stdout, log, restore := claimStdoutForApp()
	writer := newAppEventWriter(stdout, log)
	fmt.Println("plain text")
	Success("colored text\n")
	Info("more text\n")
	writer.emit(appReadyEvent{Type: appEventReady, Port: 1, Token: "t", Launch: "l", Presenter: "p"})
	writer.close()
	restore()

	if os.Stdout != fakeStdout {
		t.Error("restore did not give standard output back")
	}
	stdoutText, _ := os.ReadFile(fakeStdout.Name())
	stderrText, _ := os.ReadFile(fakeStderr.Name())
	if string(stdoutText) != `{"type":"ready","port":1,"token":"t","launch":"l","presenter":"p"}`+"\n" {
		t.Errorf("stdout = %q, want only the ready line", stdoutText)
	}
	for _, want := range []string{"plain text", "colored text", "more text"} {
		if !strings.Contains(string(stderrText), want) {
			t.Errorf("stderr lacks %q: %q", want, stderrText)
		}
	}
}

// blockedWriter is standard output when the app has stopped reading its
// child's pipe: the write never returns.
type blockedWriter struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (writer *blockedWriter) Write(data []byte) (int, error) {
	writer.once.Do(func() { close(writer.entered) })
	<-writer.release
	return len(data), nil
}

func TestCloseAppEventWriterGivesUpOnStandardOutputThatNeverDrains(t *testing.T) {
	output := &blockedWriter{entered: make(chan struct{}), release: make(chan struct{})}
	defer close(output.release)
	events := newAppEventWriter(output, io.Discard)
	events.emit(appErrorEvent{Type: appEventError, Code: appErrorBusy, Message: "the first line, which blocks"})
	<-output.entered
	events.emit(appErrorEvent{Type: appEventError, Code: appErrorBusy, Message: "the line that never gets out"})

	var log bytes.Buffer
	returned := make(chan struct{})
	go func() {
		defer close(returned)
		closeAppEventWriter(events, &log)
	}()
	select {
	case <-returned:
	case <-time.After(appEventCloseBound + 5*time.Second):
		t.Fatal("closing the event writer never returned, so quit would hang on a standard output nothing reads")
	}
	if !strings.Contains(log.String(), "writing the last events") {
		t.Errorf("log = %q, want the expiry named on standard error", log.String())
	}
}

func TestCloseAppEventWriterWritesTheQueuedLines(t *testing.T) {
	output := &appLogBuffer{}
	events := newAppEventWriter(output, io.Discard)
	events.emit(appErrorEvent{Type: appEventError, Code: appErrorBusy, Message: "on its way out"})
	var stderr bytes.Buffer
	closeAppEventWriter(events, &stderr)
	if !strings.Contains(output.String(), "on its way out") {
		t.Errorf("output = %q, want the queued event", output.String())
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q, want nothing", stderr.String())
	}
}

// TestAppEventWriterDropsEventsWhenStandardOutputIsBlocked holds the
// writer goroutine inside one Write, fills the queue behind it, and then
// emits more. Every emit has to return: a writer whose job is telling the
// app what happened must never be the thing that stops the app hearing
// anything, and the quit path itself emits.
func TestAppEventWriterDropsEventsWhenStandardOutputIsBlocked(t *testing.T) {
	output := &blockedWriter{entered: make(chan struct{}), release: make(chan struct{})}
	defer close(output.release)
	log := &appLogBuffer{}
	writer := newAppEventWriter(output, log)

	writer.emit(appErrorEvent{Type: appEventError, Code: appErrorBusy, Message: "the line the writer is stuck on"})
	<-output.entered

	const extra = 5
	emitted := make(chan struct{})
	go func() {
		defer close(emitted)
		for index := range appEventQueueSize + extra {
			writer.emit(appErrorEvent{Type: appEventError, Code: appErrorBusy, Message: fmt.Sprintf("line %d", index)})
		}
	}()
	select {
	case <-emitted:
	case <-time.After(3 * time.Second):
		t.Fatal("emit did not return on a full queue, so every caller of emit hangs once the app stops reading standard output")
	}

	if count := strings.Count(log.String(), "\n"); count != 1 {
		t.Errorf("standard error has %d lines, want one line for the whole run of drops:\n%s", count, log)
	}
	if !strings.Contains(log.String(), "dropping app events") {
		t.Errorf("standard error = %q, want the drops named", log)
	}
}

// TestAppEventWriterReportsHowManyEventsItDropped checks the other end of
// a run of drops: one line when the drops start, one when standard output
// takes events again, and the count of what was lost in between.
func TestAppEventWriterReportsHowManyEventsItDropped(t *testing.T) {
	output := &blockedWriter{entered: make(chan struct{}), release: make(chan struct{})}
	log := &appLogBuffer{}
	writer := newAppEventWriter(output, log)

	writer.emit(appErrorEvent{Type: appEventError, Code: appErrorBusy, Message: "the line the writer is stuck on"})
	<-output.entered
	const extra = 5
	for index := range appEventQueueSize + extra {
		writer.emit(appErrorEvent{Type: appEventError, Code: appErrorBusy, Message: fmt.Sprintf("line %d", index)})
	}

	// Let the queue drain, so the next emit finds room and ends the run.
	close(output.release)
	deadline := time.Now().Add(10 * time.Second)
	for len(writer.lines) > 0 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	writer.emit(appErrorEvent{Type: appEventError, Code: appErrorBusy, Message: "after the block"})
	writer.close()

	want := fmt.Sprintf("%d app events were dropped", extra)
	if !strings.Contains(log.String(), want) {
		t.Errorf("standard error = %q, want %q", log, want)
	}
	if count := strings.Count(log.String(), "\n"); count != 2 {
		t.Errorf("standard error has %d lines, want two: the drops starting and the count once they stop:\n%s", count, log)
	}
}
