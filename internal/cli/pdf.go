// Package cli provides the command-line interface for Tap.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Flags for the pdf command
var (
	pdfOutput  string
	pdfContent string
)

// pdfCmd represents the pdf command
var pdfCmd = &cobra.Command{
	Use:   "pdf <file>",
	Short: "Export presentation to PDF",
	Long: `Export a presentation to a PDF file.

The pdf command renders your presentation and exports it to a PDF document.
This is useful for sharing your slides offline or printing handouts.

You can choose what content to include in the PDF:
  - slides: Only the slide content (default)
  - notes:  Only the speaker notes
  - both:   Slides with speaker notes below

Examples:
  tap pdf slides.md                        # Export to slides.pdf
  tap pdf slides.md --output handout.pdf   # Custom output filename
  tap pdf slides.md -o talk.pdf            # Short form
  tap pdf slides.md --content notes        # Export only speaker notes
  tap pdf slides.md --content both         # Slides with notes`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Implement PDF export logic
		// This will be implemented in a later user story (US-037, US-044)
		file := args[0]
		fmt.Printf("Exporting presentation from %s to PDF...\n", file)
		fmt.Printf("  Output: %s\n", pdfOutput)
		fmt.Printf("  Content: %s\n", pdfContent)
		fmt.Println("\nPDF export complete!")
	},
}

func init() {
	// Register the pdf command with root
	rootCmd.AddCommand(pdfCmd)

	// Command-specific flags
	pdfCmd.Flags().StringVarP(&pdfOutput, "output", "o", "", "output PDF file path (default: <input>.pdf)")
	pdfCmd.Flags().StringVar(&pdfContent, "content", "slides", "content to include: slides, notes, or both")
}
