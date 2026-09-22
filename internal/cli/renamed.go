package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// renamedCommand is a hidden stand-in for a removed command name. It
// accepts any arguments and flags, prints one line that names the new
// command, and exits 1.
func renamedCommand(name string, replacement func(args []string) (oldName, newName string)) *cobra.Command {
	return &cobra.Command{
		Use:                name,
		Hidden:             true,
		DisableFlagParsing: true,
		SilenceUsage:       true,
		RunE: func(cmd *cobra.Command, args []string) error {
			oldName, newName := replacement(args)
			err := fmt.Errorf("%s was renamed: use %s", oldName, newName)
			fmt.Fprintln(cmd.ErrOrStderr(), err)
			return reportedError(codeRenamed, err)
		},
	}
}

func init() {
	rootCmd.AddCommand(renamedCommand("pdf", func([]string) (string, string) {
		return "tap pdf", "tap export pdf"
	}))
	rootCmd.AddCommand(renamedCommand("screenshot", func([]string) (string, string) {
		return "tap screenshot", "tap export images"
	}))
	rootCmd.AddCommand(renamedCommand("add", func(args []string) (string, string) {
		if len(args) > 0 && args[0] == "component" {
			return "tap add component", "tap component new"
		}
		return "tap add", "tap slide add"
	}))
}
