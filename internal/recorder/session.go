package recorder

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// killGrace is how long a recorder gets to finalize its file after being
// interrupted, before it is killed. A variable so tests can shorten it.
var killGrace = 5 * time.Second

// Session is one running recorder process.
type Session struct {
	command   *exec.Cmd
	done      chan struct{}
	path      string
	startedAt time.Time

	mu       sync.Mutex
	exitErr  error
	stopOnce sync.Once
	stopErr  error
	result   Result
}

// Start launches the recorder. The returned Session is running until Stop
// is called or the process exits on its own.
func Start(options Options) (*Session, error) {
	command := exec.Command(options.command(), buildArgs(options)...) //nolint:gosec // the command and its flags are built here, not supplied by a deck

	if err := command.Start(); err != nil {
		return nil, fmt.Errorf("starting the recorder: %w", err)
	}

	session := &Session{
		command:   command,
		done:      make(chan struct{}),
		path:      options.OutputPath,
		startedAt: time.Now(),
	}

	go func() {
		err := command.Wait()
		session.mu.Lock()
		session.exitErr = err
		session.mu.Unlock()
		close(session.done)
	}()

	return session, nil
}

// Done closes when the recorder process has exited, whether it was stopped
// or ended on its own.
func (s *Session) Done() <-chan struct{} {
	return s.done
}

// ExitError is the recorder's exit error, or nil. It is only meaningful
// once Done has closed.
func (s *Session) ExitError() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.exitErr
}

// Path is the .mov being written.
func (s *Session) Path() string {
	return s.path
}

// Elapsed is how long the recorder has been running.
func (s *Session) Elapsed() time.Duration {
	return time.Since(s.startedAt)
}

// Stop interrupts the recorder and waits for it to finalize the file.
// Concurrent callers all receive the same result: sync.Once holds the
// later ones until the first has finished stopping.
func (s *Session) Stop() (Result, error) {
	s.stopOnce.Do(func() {
		result, err := s.stop()

		s.mu.Lock()
		s.result, s.stopErr = result, err
		s.mu.Unlock()
	})

	s.mu.Lock()
	defer s.mu.Unlock()
	return s.result, s.stopErr
}

// stop does the actual interrupting and waiting. screencapture writes a
// playable .mov on SIGINT; a recorder that has not exited within killGrace
// is killed, and the result is marked truncated. Stopping an already-exited
// session measures what it left behind. A recorder that exits on its own in
// the window between the done check and the signal makes Signal return
// os.ErrProcessDone; that is treated as already stopped, not as a failure.
func (s *Session) stop() (Result, error) {
	duration := s.Elapsed()
	truncated := false

	select {
	case <-s.done:
	default:
		if err := s.command.Process.Signal(syscall.SIGINT); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return Result{}, fmt.Errorf("interrupting the recorder: %w", err)
		}

		select {
		case <-s.done:
		case <-time.After(killGrace):
			_ = s.command.Process.Kill()
			<-s.done
			truncated = true
		}
	}

	result := Result{Path: s.path, Duration: duration, Truncated: truncated}
	if info, err := os.Stat(s.path); err == nil {
		result.Size = info.Size()
	}

	return result, nil
}
