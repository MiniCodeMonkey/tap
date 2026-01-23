// Package cli provides the command-line interface for Tap.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Flags for the build command
var (
	buildOutput string
)

// buildCmd represents the build command
var buildCmd = &cobra.Command{
	Use:   "build <file>",
	Short: "Build presentation to static HTML",
	Long: `Build a presentation to static HTML files for deployment.

The build command generates a self-contained static website from your
markdown presentation. The output can be deployed to any static hosting
service (GitHub Pages, Netlify, Vercel, etc.).

The generated files include:
  - index.html with embedded presentation
  - All referenced images and assets
  - Necessary JavaScript and CSS

Note: Live code execution is not available in static builds.

Examples:
  tap build slides.md                   # Build to dist/ directory
  tap build slides.md --output public   # Build to custom directory
  tap build slides.md -o ./build        # Short form`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Implement build logic
		// This will be implemented in a later user story (US-036, US-042)
		file := args[0]
		fmt.Printf("Building presentation from %s...\n", file)
		fmt.Printf("  Output directory: %s\n", buildOutput)
		fmt.Println("\nBuild complete!")
	},
}

func init() {
	// Register the build command with root
	rootCmd.AddCommand(buildCmd)

	// Command-specific flags
	buildCmd.Flags().StringVarP(&buildOutput, "output", "o", "dist", "output directory for static files")
}
