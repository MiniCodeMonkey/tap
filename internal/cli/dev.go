// Package cli provides the command-line interface for Tap.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Flags for the dev command
var (
	devPort              int
	devPresenterPassword string
)

// devCmd represents the dev command
var devCmd = &cobra.Command{
	Use:   "dev <file>",
	Short: "Start the development server",
	Long: `Start the development server to preview and present your slides.

The dev server provides:
  - Live preview of your presentation at http://localhost:<port>
  - Hot reload on file changes
  - Presenter view with speaker notes
  - Live code execution for supported drivers

Examples:
  tap dev slides.md                      # Start server on port 3000
  tap dev slides.md --port 8080          # Use custom port
  tap dev slides.md -p 8080              # Short form
  tap dev slides.md --presenter-password secret  # Protect presenter view`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Implement dev server
		// This will be implemented in a later user story (US-029, US-030, etc.)
		file := args[0]
		fmt.Printf("Starting dev server for %s...\n", file)
		fmt.Printf("  Port: %d\n", devPort)
		if devPresenterPassword != "" {
			fmt.Println("  Presenter view: password protected")
		}
		fmt.Printf("\n  Audience view:   http://localhost:%d\n", devPort)
		fmt.Printf("  Presenter view:  http://localhost:%d/presenter\n", devPort)
	},
}

func init() {
	// Register the dev command with root
	rootCmd.AddCommand(devCmd)

	// Command-specific flags
	devCmd.Flags().IntVarP(&devPort, "port", "p", 3000, "port for the dev server")
	devCmd.Flags().StringVar(&devPresenterPassword, "presenter-password", "", "password to protect the presenter view")
}
