// Package cli provides the command-line interface for Tap.
package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/themes"
	"github.com/MiniCodeMonkey/tap/internal/tui"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// Flags for the new command
var (
	newTitle  string
	newTheme  string
	newOutput string
	newYes    bool
	newForce  bool
)

// newCmd represents the new command
var newCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new presentation",
	Long: `Create a new markdown presentation with the specified theme.

This command creates a new presentation file with frontmatter configuration
and example slides to help you get started quickly.

With no terminal attached to standard input, or with --yes, the wizard is
skipped: the file is written straight from --title, --theme and --output,
falling back to defaults for anything not given, and only the written
path is printed to standard output. An existing file at that path is left
alone unless --force is also given.

Examples:
  tap new                          # Interactive mode
  tap new --theme terminal         # Create with the Terminal theme
  tap new --output my-talk.md      # Create with custom filename
  tap new -t terminal -o demo.md   # Combine options
  tap new --yes --title "My Talk" --theme terminal --output talk.md   # No wizard`,
	Run: func(cmd *cobra.Command, args []string) {
		if newYes || !stdinIsTerminal() {
			if err := runNewNonInteractive(); err != nil {
				Errorln("Error:", err)
				os.Exit(1)
			}
			return
		}

		result, err := tui.RunNewWizard(newTheme, newOutput)
		if err != nil {
			Error("Failed to create presentation: %v", err)
			os.Exit(1)
		}

		if result.Aborted {
			os.Exit(0)
		}
	},
}

func init() {
	// Register the new command with root
	rootCmd.AddCommand(newCmd)

	// Command-specific flags
	newCmd.Flags().StringVar(&newTitle, "title", "", "title for the new presentation (default: \"My Presentation\")")
	newCmd.Flags().StringVarP(&newTheme, "theme", "t", "", "theme for the new presentation")
	newCmd.Flags().StringVarP(&newOutput, "output", "o", "", "output filename for the presentation")
	newCmd.Flags().BoolVarP(&newYes, "yes", "y", false, "skip the interactive wizard and write the file from flags and defaults")
	newCmd.Flags().BoolVar(&newForce, "force", false, "overwrite --output if it already exists (non-interactive mode only)")
}

// stdinIsTerminal reports whether standard input is a terminal. A var, not
// a direct isatty call, so a test can force either path without depending
// on how the test binary itself happens to be run.
var stdinIsTerminal = func() bool {
	return isatty.IsTerminal(os.Stdin.Fd())
}

// runNewNonInteractive writes the starter deck from flags and defaults,
// with no wizard, and prints the written path (and nothing else) to
// standard output. It is used both for --yes and for the case where
// standard input is not a terminal, so scripts, CI and LLM agents that
// invoke tap new never hang waiting on the TUI.
func runNewNonInteractive() error {
	title := newTitle
	if title == "" {
		title = tui.DefaultTitle
	}

	theme := newTheme
	if theme == "" {
		theme = tui.DefaultTheme()
	} else if !themes.IsValid(theme) {
		return unknownThemeError(theme)
	}

	output := newOutput
	if output == "" {
		output = tui.FilenameFromTitle(title)
	} else if !strings.HasSuffix(output, ".md") {
		output += ".md"
	}

	if _, err := os.Stat(output); err == nil {
		if !newForce {
			return fmt.Errorf("%s already exists; use --force to overwrite", output)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("failed to check %s: %w", output, err)
	}

	content := tui.GenerateStarterMarkdown(title, theme, time.Now().Format("2006-01-02"), "Your Name")
	if err := os.WriteFile(output, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write %s: %w", output, err)
	}

	fmt.Println(output)
	return nil
}
