// Package cli provides the command-line interface for Tap.
package cli

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

// Version is the release version. The release build sets it with
// -ldflags "-X github.com/MiniCodeMonkey/tap/internal/cli.Version=<version>",
// which only works on a variable, so this is not a constant.
var Version = "dev"

// displayVersion is Version as shown to a person: "v2.0.0-beta.2" for a
// release build, "dev" for a local one.
func displayVersion() string {
	if Version == "dev" {
		return Version
	}
	return "v" + Version
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:     "tap",
	Short:   "A markdown-based presentation tool",
	Version: Version,
	Long: `Tap is a markdown-based presentation tool for technical presentations
with beautiful defaults, live code execution, and developer-first experience.

Create stunning presentations using familiar markdown syntax, execute
code blocks live during your presentation, and enjoy instant hot reload
during development.`,
	// Silence Cobra's default error and usage output - we handle these ourselves
	SilenceErrors: true,
	SilenceUsage:  true,
}

func init() {
	// Customize version template to show "tap version X.Y.Z"
	rootCmd.SetVersionTemplate("tap version {{.Version}}\n")

	rootCmd.SetFlagErrorFunc(func(command *cobra.Command, err error) error {
		return userError(codeUsage, err)
	})
}

// Execute runs tap with the process arguments and returns its exit code.
func Execute() int {
	return execute(rootCmd, os.Args[1:], os.Stdout, os.Stderr)
}

// execute runs root with args and turns the result into an exit code. It
// is the only place that prints a failed command's error, so every
// command reports errors the same way.
func execute(root *cobra.Command, args []string, stdout, stderr io.Writer) int {
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	_, err := root.ExecuteC()
	if err == nil {
		return exitOK
	}

	exitCode, _, reported := classify(err)
	switch {
	case reported:
	case errors.Is(err, errInterrupted):
		fmt.Fprintln(stderr, "interrupted")
	default:
		errorColor.Fprintln(stderr, "Error:", err)
	}
	return exitCode
}
