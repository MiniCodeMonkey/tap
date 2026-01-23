// Package cli provides the command-line interface for Tap.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Flags for the new command
var (
	newTheme  string
	newOutput string
)

// newCmd represents the new command
var newCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new presentation",
	Long: `Create a new markdown presentation with the specified theme.

This command creates a new presentation file with frontmatter configuration
and example slides to help you get started quickly.

Examples:
  tap new                          # Interactive mode
  tap new --theme minimal          # Create with minimal theme
  tap new --output my-talk.md      # Create with custom filename
  tap new -t gradient -o demo.md   # Combine options`,
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Implement presentation creation
		// This will be implemented in a later user story (US-039)
		fmt.Println("Creating new presentation...")
		if newTheme != "" {
			fmt.Printf("  Theme: %s\n", newTheme)
		}
		if newOutput != "" {
			fmt.Printf("  Output: %s\n", newOutput)
		}
	},
}

func init() {
	// Register the new command with root
	rootCmd.AddCommand(newCmd)

	// Command-specific flags
	newCmd.Flags().StringVarP(&newTheme, "theme", "t", "", "theme for the new presentation (minimal, gradient, terminal, brutalist, keynote)")
	newCmd.Flags().StringVarP(&newOutput, "output", "o", "", "output filename for the presentation")
}
