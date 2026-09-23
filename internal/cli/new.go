// Package cli provides the command-line interface for Tap.
package cli

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/themes"
	"github.com/MiniCodeMonkey/tap/internal/tui"
	"github.com/MiniCodeMonkey/tap/internal/usersettings"
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
	newJSON   bool
)

// newCmd represents the new command
var newCmd = &cobra.Command{
	Use:   "new [deck]",
	Short: "Create a new presentation",
	Long: `Create a new markdown presentation with the specified theme.

This command creates a new presentation file with frontmatter configuration
and example slides to help you get started quickly. [deck] is the path of
the new deck, the same as --output.

With no terminal attached to standard input, or with --yes, the wizard is
skipped: the file is written straight from --title, --theme and --output,
falling back to defaults for anything not given, and only the written
path is printed to standard output. An existing file at that path is left
alone unless --force is also given.

The new deck is approved to run live code with the drivers its frontmatter
declares.

Examples:
  tap new                          # Interactive mode
  tap new --theme terminal         # Create with the Terminal theme
  tap new --output my-talk.md      # Create with custom filename
  tap new -t terminal -o demo.md   # Combine options
  tap new my-talk.md --yes            # Write my-talk.md with no wizard
  tap new --yes --title "My Talk" --theme terminal --output talk.md   # No wizard`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if deck := firstArg(args); deck != "" {
			if cmd.Flags().Changed("output") {
				return userError(codeUsage, errors.New("give the deck path once: as the argument or with --output"))
			}
			newOutput = deck
		}

		if newYes || newJSON || !stdinIsTerminal() {
			return runNewNonInteractive(cmd)
		}

		result, err := tui.RunNewWizard(newTheme, newOutput)
		if err != nil {
			return internalError(codeInternal, fmt.Errorf("failed to create presentation: %w", err))
		}
		if result.Aborted {
			return errCancelled
		}
		recordNewDeckApproval(cmd, result.Filename)
		return nil
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
	newCmd.Flags().BoolVar(&newJSON, "json", false, "print the written deck as JSON (skips the wizard)")
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
func runNewNonInteractive(cmd *cobra.Command) error {
	title := newTitle
	if title == "" {
		title = tui.DefaultTitle
	}

	theme := newTheme
	if theme == "" {
		theme = tui.DefaultTheme()
	} else if !themes.IsValid(theme) {
		return userError(codeUnknownTheme, unknownThemeError(theme))
	}

	output := newOutput
	if output == "" {
		output = tui.FilenameFromTitle(title)
	} else if !strings.HasSuffix(output, ".md") {
		output += ".md"
	}

	if _, err := os.Stat(output); err == nil {
		if !newForce {
			return userError(codeExists, fmt.Errorf("%s already exists; use --force to overwrite", output))
		}
	} else if !os.IsNotExist(err) {
		return internalError(codeInternal, fmt.Errorf("failed to check %s: %w", output, err))
	}

	content := tui.GenerateStarterMarkdown(title, theme, time.Now().Format("2006-01-02"), "Your Name")
	if err := os.WriteFile(output, []byte(content), 0o644); err != nil {
		return internalError(codeInternal, fmt.Errorf("failed to write %s: %w", output, err))
	}

	recordNewDeckApproval(cmd, output)

	if newJSON {
		return printJSONOK(cmd.OutOrStdout(), struct {
			Deck string `json:"deck"`
		}{Deck: output})
	}
	fmt.Fprintln(cmd.OutOrStdout(), output)
	return nil
}

// approveNewDeck approves the deck tap new just wrote, with the drivers
// its frontmatter declares, so the person who made it is not asked about
// it.
func approveNewDeck(deck string, now time.Time) error {
	key, err := usersettings.ResolveDeck(deck)
	if err != nil {
		return err
	}
	cfg, err := config.Load(key.String())
	if err != nil {
		return err
	}
	settingsPath, err := usersettings.Path()
	if err != nil {
		return err
	}
	return usersettings.WithLock(settingsPath, func() error {
		settings, err := usersettings.Load(settingsPath)
		if err != nil {
			return err
		}
		settings.Approve(key, cfg.DeclaredDrivers(), now)
		return usersettings.Save(settingsPath, settings)
	})
}

// recordNewDeckApproval approves a new deck, and only warns when that
// fails: the deck is written either way.
func recordNewDeckApproval(cmd *cobra.Command, deck string) {
	if err := approveNewDeck(deck, time.Now()); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "warning: could not record the live code approval for %s: %v\n", deck, err)
	}
}
