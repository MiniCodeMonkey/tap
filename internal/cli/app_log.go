package cli

import (
	"fmt"
	"io"
	"sync"
	"time"
)

// appLogQueueSize is how many log lines can wait for the writer. It
// matches appEventQueueSize: the two pipes fail the same way and there is
// no reason for one to hold more than the other.
const appLogQueueSize = appEventQueueSize

// appLogCloseBound is how long the log writer's own close waits for the
// lines already queued to reach standard error. It is short because
// waiting longer buys nothing: a pipe that is being read at all drains
// the queue in microseconds, and a pipe that is not will still not be
// read a second from now. What is at stake is a few advisory lines on a
// process that is one statement from exiting.
const appLogCloseBound = 500 * time.Millisecond

// appLogWriter is the only writer of standard error in --app mode, and
// the reason quit can report anything at all.
//
// An app reads its child's standard output because that is the protocol.
// Standard error is the Tap Log, a convenience the app shows in a panel,
// and an app whose own event loop is busy, or that simply never wired the
// pipe to anything, leaves it unread. A pipe nobody reads fills after
// about 64 KB, and a plain Fprintf to it then blocks for as long as the
// app lives. Every human-readable line the quit path writes goes here,
// including the report that says quit gave up waiting for something, so a
// blocking write would sit above the quit deadline itself: the deadline
// could not fire, because reporting the expiry is what is stuck. The
// event writer's own comment already names this shape for standard
// output -- the writer whose job is telling the app what happened must
// never be the thing that stops the app hearing anything -- and it is the
// same shape one pipe over.
//
// So standard error gets the same treatment as standard output, and for
// the same reason: one goroutine does the writing, a Write hands its line
// to that goroutine and never waits for room, and a line that finds the
// queue full is dropped and counted. What differs is where the drop is
// reported. A stuck standard output is reported on standard error,
// because standard error is by definition still working in that case.
// There is no third pipe to say a stuck standard error on, and an event
// on standard output would put a diagnostic about the log into the
// protocol stream the app parses. So the count is kept and spent on
// standard error itself the moment it drains again, which is the only
// place it could be read and the only moment it could be. A log line is
// advisory: the protocol is on standard output, and an advisory line
// losing its place in a queue is a smaller loss, every time, than a
// process that will not exit.
type appLogWriter struct {
	output io.Writer
	lines  chan []byte
	done   chan struct{}
	mu     sync.RWMutex
	closed bool
	// dropMu guards dropped, the length of the run of lines Write has
	// thrown away since the queue last had room for one.
	dropMu  sync.Mutex
	dropped int
}

var _ io.Writer = (*appLogWriter)(nil)

// newAppLogWriter starts the writer goroutine for output, which in --app
// mode is standard error.
func newAppLogWriter(output io.Writer) *appLogWriter {
	writer := &appLogWriter{
		output: output,
		lines:  make(chan []byte, appLogQueueSize),
		done:   make(chan struct{}),
	}
	go writer.run()
	return writer
}

func (writer *appLogWriter) run() {
	defer close(writer.done)
	failed := false
	for line := range writer.lines {
		if failed {
			continue
		}
		if _, err := writer.output.Write(line); err != nil {
			// Standard error has gone: a closed pipe, or a full disk on a
			// redirect. There is nowhere left to say so, so the rest of
			// the run's log is discarded rather than retried per line.
			failed = true
		}
	}
}

// Write queues one log line. It never blocks and never fails: a full
// queue means the writer goroutine is stuck inside a write to standard
// error, which happens when the app is not reading its child's log pipe.
// The line is dropped and counted instead. The return is always a
// complete write, because a caller that changed what it does on a
// dropped log line would be making the log matter more than the work,
// and fmt's writers report a short write as an error the caller then has
// to handle on a path that must not grow branches.
func (writer *appLogWriter) Write(data []byte) (int, error) {
	line := make([]byte, len(data))
	copy(line, data)

	writer.mu.RLock()
	defer writer.mu.RUnlock()
	if writer.closed {
		return len(data), nil
	}
	select {
	case writer.lines <- line:
		writer.noteQueued()
	default:
		writer.noteDropped()
	}
	return len(data), nil
}

// noteDropped counts one dropped line. Unlike the event writer's drop, it
// says nothing at the time: the pipe it would say it on is the one that
// is not being read.
func (writer *appLogWriter) noteDropped() {
	writer.dropMu.Lock()
	writer.dropped++
	writer.dropMu.Unlock()
}

// noteQueued ends a run of drops, queueing one line that says how many
// were lost, so a log that starts being read again says what is missing
// from it rather than silently skipping. The notice is queued the same
// non-blocking way as everything else, so a queue that is full again
// simply keeps counting.
func (writer *appLogWriter) noteQueued() {
	writer.dropMu.Lock()
	dropped := writer.dropped
	writer.dropped = 0
	writer.dropMu.Unlock()
	if dropped == 0 {
		return
	}
	notice := fmt.Appendf(nil, "Standard error was not being read: %d log lines were dropped.\n", dropped)
	select {
	case writer.lines <- notice:
	default:
		writer.dropMu.Lock()
		writer.dropped += dropped
		writer.dropMu.Unlock()
	}
}

// close stops taking lines and waits, for at most appLogCloseBound, for
// the ones already queued to reach standard error. It is safe to call
// more than once.
//
// The wait is written out here rather than going through quitJoiner,
// which is where every other wait in the quit path belongs, because the
// joiner's whole purpose is to report an expiry, and the only place it
// reports to is this writer. A blocked log cannot be used to announce
// that the log is blocked. What is given up on is some already-queued
// diagnostic lines, on a process that is one statement from exiting.
func (writer *appLogWriter) close() {
	writer.mu.Lock()
	if !writer.closed {
		writer.closed = true
		close(writer.lines)
	}
	writer.mu.Unlock()
	timer := time.NewTimer(appLogCloseBound)
	defer timer.Stop()
	select {
	case <-writer.done:
	case <-timer.C:
	}
}
