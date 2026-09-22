package cli

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/fatih/color"
)

// claimStdoutForApp gives standard output to the --app event writer alone,
// and returns the real standard output for it. Until restore runs,
// everything else that prints to standard output, fmt.Print and the color
// helpers alike, prints to standard error, which the app shows as the Tap
// Log. A write to a closed pipe returns an error instead of killing tap,
// so a recording is still finished when the app has gone.
func claimStdoutForApp() (stdout *os.File, restore func()) {
	stdout = os.Stdout
	previousColorOutput := color.Output
	os.Stdout = os.Stderr
	color.Output = os.Stderr

	brokenPipes := make(chan os.Signal, 1)
	signal.Notify(brokenPipes, syscall.SIGPIPE)
	go func() {
		for range brokenPipes {
		}
	}()

	return stdout, func() {
		signal.Stop(brokenPipes)
		close(brokenPipes)
		os.Stdout = stdout
		color.Output = previousColorOutput
	}
}
