// Package cli provides the command-line interface for Tap.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version information
const (
	Version = "0.1.0"
)

// Global flags
var verbose bool

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

	// Global flags available to all subcommands
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
}

// Execute runs the root command and returns exit code
func Execute() int {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// Verbose returns true if verbose mode is enabled
func Verbose() bool {
	return verbose
}
