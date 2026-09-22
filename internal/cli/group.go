// Package cli provides the command-line interface for Tap.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// runUnknownGroupSubcommand is the RunE for a command that only groups
// subcommands (export, slide, component, theme) and has no behavior of
// its own. With no args it prints help and exits 0, the same as --help.
// With an unrecognized subcommand, such as a typo, it fails instead of
// cobra's default of printing help and exiting 0.
func runUnknownGroupSubcommand(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		return cmd.Help()
	}
	return userError(codeUsage, fmt.Errorf("unknown command %q for %q", args[0], cmd.CommandPath()))
}
