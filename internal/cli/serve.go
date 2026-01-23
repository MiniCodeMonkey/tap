// Package cli provides the command-line interface for Tap.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Flags for the serve command
var (
	servePort int
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve [dir]",
	Short: "Serve a built presentation",
	Long: `Serve a previously built presentation from static files.

This command starts a simple HTTP file server to preview your built
presentation locally before deploying. It's useful for testing your
static build output.

The serve command is intended for previewing static builds. For live
development with hot reload and code execution, use 'tap dev' instead.

Examples:
  tap serve                    # Serve from dist/ on port 3000
  tap serve public             # Serve from custom directory
  tap serve --port 8080        # Use custom port
  tap serve ./build -p 8080    # Both options together`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Implement serve logic
		// This will be implemented in a later user story (US-043)
		dir := "dist"
		if len(args) > 0 {
			dir = args[0]
		}
		fmt.Printf("Serving presentation from %s...\n", dir)
		fmt.Printf("  Port: %d\n", servePort)
		fmt.Printf("\n  Local: http://localhost:%d\n", servePort)
	},
}

func init() {
	// Register the serve command with root
	rootCmd.AddCommand(serveCmd)

	// Command-specific flags
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 3000, "port for the server")
}
