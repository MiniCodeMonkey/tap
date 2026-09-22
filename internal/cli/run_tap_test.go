package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// runTap runs the real root command in-process with args, and returns the
// exit code and what it wrote. Flag values live in package variables that
// cobra never resets, so every flag goes back to its default afterward.
func runTap(t *testing.T, args ...string) (exitCode int, stdout, stderr string) {
	t.Helper()
	t.Cleanup(func() { resetAllFlags(rootCmd) })
	var out, errOut bytes.Buffer
	exitCode = execute(rootCmd, args, &out, &errOut)
	return exitCode, out.String(), errOut.String()
}

// resetAllFlags sets every flag of command and its children back to its
// default value.
func resetAllFlags(command *cobra.Command) {
	reset := func(flag *pflag.Flag) {
		if sliceValue, ok := flag.Value.(pflag.SliceValue); ok {
			_ = sliceValue.Replace(nil)
		} else {
			_ = flag.Value.Set(flag.DefValue)
		}
		flag.Changed = false
	}
	command.Flags().VisitAll(reset)
	command.PersistentFlags().VisitAll(reset)
	for _, child := range command.Commands() {
		resetAllFlags(child)
	}
}
