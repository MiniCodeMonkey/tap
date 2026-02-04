// Package cli provides the command-line interface for Tap.
package cli

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/MiniCodeMonkey/tap/internal/tui"
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
		file := ""
		if len(args) > 0 {
			file = args[0]
		} else {
			// Look for a presentation file in the current directory
			file = findPresentationFile()
		}

		if file == "" {
			Error("No presentation file found. Please specify a file or create one with 'tap new'.")
			os.Exit(1)
		}

		// Verify file exists
		if _, err := os.Stat(file); os.IsNotExist(err) {
			Error("File not found: %s\n", file)
			os.Exit(1)
		}

		result, err := tui.RunAddWizard(file)
		if err != nil {
			Error("Error: %v\n", err)
			os.Exit(1)
		}

		if result.Aborted {
			// User cancelled, exit silently
			os.Exit(0)
		}
	},
}

// findPresentationFile looks for a markdown presentation file in the current directory.
func findPresentationFile() string {
	// Common presentation file names
	commonNames := []string{
		"presentation.md",
		"slides.md",
		"talk.md",
		"deck.md",
	}

	// First check common names
	for _, name := range commonNames {
		if _, err := os.Stat(name); err == nil {
			return name
		}
	}

	// Look for any .md file in the current directory
	entries, err := os.ReadDir(".")
	if err != nil {
		return ""
	}

	var mdFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			// Skip common non-presentation files
			name := strings.ToLower(entry.Name())
			if name == "readme.md" || name == "changelog.md" || name == "contributing.md" || name == "license.md" {
				continue
			}
			mdFiles = append(mdFiles, entry.Name())
		}
	}

	// If exactly one markdown file found, use it
	if len(mdFiles) == 1 {
		return mdFiles[0]
	}

	// If multiple found, prefer one that looks like a presentation
	for _, f := range mdFiles {
		lower := strings.ToLower(f)
		if strings.Contains(lower, "slide") || strings.Contains(lower, "present") || strings.Contains(lower, "talk") || strings.Contains(lower, "deck") {
			return f
		}
	}

	// Return first one if any exist
	if len(mdFiles) > 0 {
		return mdFiles[0]
	}

	return ""
}
func init() {
	// Register the add command with root
	rootCmd.AddCommand(addCmd)
}
