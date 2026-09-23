package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/MiniCodeMonkey/tap/internal/slidelist"
)

var slideListJSON bool

var slideListCmd = &cobra.Command{
	Use:   "list [deck]",
	Short: "List a deck's slides with their line ranges",
	Long: `List every slide of a deck: its number, the lines it covers in the
file, its layout, title, step and fragment counts, whether it is skipped,
its errors, and its code blocks.

Line numbers are 1-based. A slide's range covers its text, including its
directive comment, without the blank lines around it. The "---" separator
lines and the frontmatter belong to no slide. Slide numbers count skipped
slides, so they match the numbers every other tap command uses.

Examples:
  tap slide list                 # The deck in this folder
  tap slide list talk.md
  tap slide list talk.md --json  # For editors and scripts`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSlideList,
}

func init() {
	slideCmd.AddCommand(slideListCmd)
	slideListCmd.Flags().BoolVar(&slideListJSON, "json", false, "print the slide list as JSON")
}

func runSlideList(cmd *cobra.Command, args []string) error {
	file, err := resolveDeck(firstArg(args))
	if err != nil {
		return err
	}
	absolute, err := filepath.Abs(file)
	if err != nil {
		return internalError(codeInternal, fmt.Errorf("failed to resolve file path: %w", err))
	}
	source, err := os.ReadFile(absolute)
	if err != nil {
		return userError(codeDeckNotFound, fmt.Errorf("failed to read %s: %w", file, err))
	}

	result, err := slidelist.Build(source, filepath.Dir(absolute))
	if err != nil {
		return internalError(codeInternal, err)
	}
	if slideListJSON {
		return printJSONOK(cmd.OutOrStdout(), result)
	}
	return printSlideList(cmd.OutOrStdout(), result)
}

// printSlideList writes the slide list as a table, one slide per row. The
// deck's own errors come first, and each slide's errors come after the
// table.
func printSlideList(w io.Writer, result slidelist.Result) error {
	for _, message := range result.Errors {
		fmt.Fprintf(w, "error: %s\n", message)
	}

	table := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "#\tLINES\tLAYOUT\tTITLE\tSTEPS\tFRAGMENTS\tNOTES")
	for _, slide := range result.Slides {
		fmt.Fprintf(table, "%d\t%d-%d\t%s\t%s\t%d\t%d\t%s\n",
			slide.Number, slide.StartLine, slide.EndLine, slide.Layout, slide.Title, slide.Steps, slide.Fragments, slideNotes(slide))
	}
	if err := table.Flush(); err != nil {
		return err
	}

	for _, slide := range result.Slides {
		for _, message := range slide.Errors {
			fmt.Fprintf(w, "slide %d: %s\n", slide.Number, message)
		}
	}
	return nil
}

// slideNotes sums up a slide for the NOTES column: whether it is skipped,
// the driver of each live code block, and how many errors it has.
func slideNotes(slide slidelist.Slide) string {
	var notes []string
	if slide.Skip {
		notes = append(notes, "skipped")
	}
	for _, block := range slide.CodeBlocks {
		if block.Live {
			notes = append(notes, "live: "+block.Driver)
		}
	}
	switch count := len(slide.Errors); {
	case count == 1:
		notes = append(notes, "1 error")
	case count > 1:
		notes = append(notes, fmt.Sprintf("%d errors", count))
	}
	return strings.Join(notes, ", ")
}
