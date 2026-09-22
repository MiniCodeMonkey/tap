package cli

import "github.com/spf13/cobra"

// exportCmd groups the commands that render a deck to files.
var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export a deck to a PDF or to images",
	Args:  cobra.ArbitraryArgs,
	RunE:  runUnknownGroupSubcommand,
}

func init() {
	rootCmd.AddCommand(exportCmd)
}
