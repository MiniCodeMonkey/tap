package cli

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/fatih/color"
)

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
// holding the goroutine that printed it. os.Stdout has to stay a file, so
// a bare fmt.Print still reaches standard error directly; nothing on the
// quit path prints that way, and the quit path is where a blocked write
// costs the process its exit.
func claimStdoutForApp() (stdout *os.File, log *appLogWriter, restore func()) {
	stdout = os.Stdout
	log = newAppLogWriter(os.Stderr)
	previousColorOutput := color.Output
	os.Stdout = os.Stderr
	color.Output = log

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
		log.close()
	}
}
