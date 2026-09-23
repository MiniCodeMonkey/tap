package cli

import (
	"io"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/fatih/color"
)

// appLogInUse is the bounded standard error writer of the --app run in
// progress, and nil outside --app mode. It outlives runDevServer on
// purpose: execute prints a failed command's error after the command has
// returned, and that print is the process's last line. A raw write there
// would be the one unbounded write of the run, on the statement before
// the exit, where a log pipe the app is not draining costs the process
// its exit. execute writes through this instead, and closes it after.
var (
	appLogMu    sync.Mutex
	appLogInUse *appLogWriter
)

// appLogForFinalOutput is the bounded writer standard error belongs to,
// or nil when this is not an --app run.
func appLogForFinalOutput() io.Writer {
	appLogMu.Lock()
	defer appLogMu.Unlock()
	if appLogInUse == nil {
		return nil
	}
	return appLogInUse
}

// closeAppLog flushes and closes the --app run's log writer, bounded at
// appLogCloseBound. It is the last thing the process does, and it is a
// no-op outside --app mode and on a second call.
func closeAppLog() {
	appLogMu.Lock()
	writer := appLogInUse
	appLogInUse = nil
	appLogMu.Unlock()
	if writer != nil {
		writer.close()
	}
}

// claimStdoutForApp gives standard output to the --app event writer
// alone, and returns the real standard output for it along with the
// bounded writer that owns standard error. Until restore runs, everything
// else that prints to standard output, fmt.Print and the color helpers
// alike, prints to standard error, which the app shows as the Tap Log. A
// write to a closed pipe returns an error instead of killing tap, so a
// recording is still finished when the app has gone.
//
// The color helpers are pointed at the bounded log writer rather than at
// standard error itself, so a warning printed while the app is not
// reading its log pipe is dropped like any other log line instead of
// holding the goroutine that printed it. Both of color's writers are
// redirected: Success, Muted and Info write to color.Output, while
// Warning and Error write to color.Error, and a warning is exactly the
// kind of line a render path produces by the screenful. os.Stdout has to
// stay a file, so a bare fmt.Print still reaches standard error directly;
// every fmt.Print in tap dev and tap present is in a branch --app does
// not run, and the render path and the quit path, where a blocked write
// costs the process its exit, print through the writer.
//
// restore hands standard output and both of color's writers back, but
// leaves the log writer open and registered, because the process has one
// more line to print after the command returns. execute closes it.
func claimStdoutForApp() (stdout *os.File, log *appLogWriter, restore func()) {
	stdout = os.Stdout
	log = newAppLogWriter(os.Stderr)
	previousColorOutput, previousColorError := color.Output, color.Error
	os.Stdout = os.Stderr
	color.Output = log
	color.Error = log

	appLogMu.Lock()
	appLogInUse = log
	appLogMu.Unlock()

	brokenPipes := make(chan os.Signal, 1)
	signal.Notify(brokenPipes, syscall.SIGPIPE)
	go func() {
		for range brokenPipes {
		}
	}()

	return stdout, log, func() {
		signal.Stop(brokenPipes)
		close(brokenPipes)
		os.Stdout = stdout
		color.Output = previousColorOutput
		color.Error = previousColorError
	}
}
