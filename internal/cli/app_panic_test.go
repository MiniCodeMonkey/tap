package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// appPanicChildEnv marks the re-execution of this test binary that stands
// in for a tap process panicking in the middle of an --app run. The panic
// has to be raised inside a process that has really called
// claimStdoutForApp: what is under test is where the runtime's own write
// to descriptor 2 goes, and the runtime does not take a writer.
const appPanicChildEnv = "TAP_APP_PANIC_UNDER_THE_REDIRECT"

// panicUnderTheAppRedirect is the child half of the test below. It takes
// the descriptors the way an --app run does, leaves the log unread the way
// an app with a busy event loop does, and panics with more goroutines
// alive than a stack trace of them fits in a pipe.
func panicUnderTheAppRedirect() {
	_, _, _, err := claimStdoutForApp()
	if err != nil {
		os.Exit(3)
	}
	// More than the app's log pipe holds and more than the bounded queue
	// behind it holds, so the writer goroutine is inside a write to the
	// app and every later line is dropped. This is the state an app that
	// stopped reading its log panel leaves tap in.
	for index := range 8000 {
		fmt.Fprintf(os.Stderr, "a log line nobody is reading, number %d\n", index)
	}
	// A real tap has hundreds of goroutines: the server, the watcher, the
	// recorder, the drivers. Under GOTRACEBACK=all, which is what an
	// integrator sets to find out why their engine died, a trace of them
	// is far larger than a pipe holds. A deep enough stack does it on the
	// default setting too. The channel is never closed, because they have
	// to be parked still when the runtime prints, and the process is one
	// statement from dying.
	parked := make(chan struct{})
	for range 4000 {
		go func() { <-parked }()
	}
	time.Sleep(200 * time.Millisecond)
	panic("a panic in the middle of an app mode run")
}

// TestAppModePanicDoesNotWaitForTheApp drives the crash path of an --app
// run. The runtime writes a panic message and stack trace with its own
// write to descriptor 2, which after the redirect is the pipe tap drains,
// and it writes it only after stopping every goroutine in the process. The
// goroutine draining that pipe is one of them, so during the write the
// pipe has no reader at all and cannot acquire one: a trace larger than
// the room left in the pipe waits for room that nothing will ever make.
//
// A tap that has already panicked and is still running is the failure this
// branch exists to remove, so the exit is what this test requires. What
// reaches the app is reported but not required: the drain cannot run, so
// the trace does not get out. That limit is written down in the --app
// reference.
func TestAppModePanicDoesNotWaitForTheApp(t *testing.T) {
	if os.Getenv(appPanicChildEnv) != "" {
		panicUnderTheAppRedirect()
		return
	}
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}
	command := exec.Command(os.Args[0], "-test.run=^TestAppModePanicDoesNotWaitForTheApp$")
	command.Env = append(os.Environ(), appPanicChildEnv+"=1", "GOTRACEBACK=all")
	// The app's two streams. Neither is read while the child runs, which
	// is what an app whose event loop is busy elsewhere looks like, and
	// neither may be what decides whether the child dies.
	protocolRead, protocolWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	logRead, logWrite, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	command.Stdout, command.Stderr = protocolWrite, logWrite
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	_ = protocolWrite.Close()
	_ = logWrite.Close()
	t.Cleanup(func() {
		_ = protocolRead.Close()
		_ = logRead.Close()
	})

	exited := make(chan error, 1)
	go func() { exited <- command.Wait() }()
	select {
	case <-exited:
	case <-time.After(20 * time.Second):
		_ = command.Process.Kill()
		t.Fatal("a panicked process was still alive 20 seconds later, waiting inside the runtime's own write for room in a pipe whose drain the panic has stopped")
	}

	// Read the log only now, which is the point: the exit above did not
	// depend on it.
	drained := make(chan string, 1)
	go func() {
		text, _ := io.ReadAll(logRead)
		drained <- string(text)
	}()
	var log string
	select {
	case log = <-drained:
	case <-time.After(5 * time.Second):
	}
	if !strings.Contains(log, "a panic in the middle of an app mode run") {
		t.Logf("the app was told nothing about the panic; its log held %d bytes", len(log))
	}
}
