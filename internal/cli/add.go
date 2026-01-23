// Package cli provides the command-line interface for Tap.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// addCmd represents the add command
var addCmd = &cobra.Command{
	Use:   "add [file]",
	Short: "Add a new slide interactively",
	Long: `Add a new slide to a presentation interactively.

This command launches an interactive TUI that guides you through creating
a new slide. You can select from various layouts and enter content for
each section of the slide.

If no file is specified, the command will look for a presentation file
in the current directory or prompt you to select one.

Examples:
  tap add                  # Add slide to presentation in current directory
  tap add slides.md        # Add slide to specific presentation file
  tap add my-talk.md       # Add slide to my-talk.md`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Implement interactive slide builder
		// This will be implemented in a later user story (US-041)
		file := ""
		if len(args) > 0 {
			file = args[0]
		}

		fmt.Println("Adding new slide interactively...")
		if file != "" {
			fmt.Printf("  File: %s\n", file)
		} else {
			fmt.Println("  Looking for presentation in current directory...")
		}
	},
}

func init() {
	// Register the add command with root
	rootCmd.AddCommand(addCmd)
}
