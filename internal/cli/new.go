// Package cli provides the command-line interface for Tap.
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	newFolder string
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

With --folder <location>, tap makes a folder named after the title inside
<location>, with the deck (named the same) and an images/ folder, and
prints the deck's path. This is the mode for programs that create decks,
such as Tap Desktop.

Examples:
  tap new                          # Interactive mode
  tap new --theme terminal         # Create with the Terminal theme
  tap new --output my-talk.md      # Create with custom filename
  tap new -t terminal -o demo.md   # Combine options
  tap new my-talk.md --yes            # Write my-talk.md with no wizard
  tap new --yes --title "My Talk" --theme terminal --output talk.md   # No wizard
  tap new --folder ~/talks --title "My Talk" --theme terminal --json   # ~/talks/my-talk/my-talk.md`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if deck := firstArg(args); deck != "" {
			if cmd.Flags().Changed("output") {
				return userError(codeUsage, errors.New("give the deck path once: as the argument or with --output"))
			}
			newOutput = deck
		}

		if newFolder != "" {
			return runNewInFolder(cmd, firstArg(args))
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
	newCmd.Flags().StringVar(&newFolder, "folder", "", "make a folder named after the title inside this location, with the deck and an images/ folder (skips the wizard)")
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

// runNewInFolder writes the starter deck into a folder of its own inside
// newFolder, named by the title's slug (tui.FilenameFromTitle, the rule
// the wizard uses for a file name), with an images/ folder beside the
// deck for tap image add. A folder with that name that already exists
// gets -2, -3 and so on, so nothing is ever overwritten. It never runs
// the wizard: it exists for programs that create decks, such as the
// desktop app's New Deck sheet.
func runNewInFolder(cmd *cobra.Command, deckArg string) error {
	if deckArg != "" || cmd.Flags().Changed("output") {
		return userError(codeUsage, errors.New("--folder names the deck from the title: give no deck path and no --output"))
	}
	if newForce {
		return userError(codeUsage, errors.New("--force has no meaning with --folder: the folder is always new"))
	}
	info, err := os.Stat(newFolder)
	switch {
	case os.IsNotExist(err):
		return userError(codeFileNotFound, fmt.Errorf("the --folder location does not exist: %s", newFolder))
	case err != nil:
		return internalError(codeInternal, fmt.Errorf("failed to check %s: %w", newFolder, err))
	case !info.IsDir():
		return userError(codeUsage, fmt.Errorf("the --folder location is not a folder: %s", newFolder))
	}

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

	stem := strings.TrimSuffix(tui.FilenameFromTitle(title), ".md")
	folder, name, err := makeFreeFolder(newFolder, stem)
	if err != nil {
		return internalError(codeInternal, err)
	}
	if err := os.Mkdir(filepath.Join(folder, "images"), 0o755); err != nil {
		return internalError(codeInternal, fmt.Errorf("failed to create %s: %w", folder, err))
	}
	output := filepath.Join(folder, name+".md")
	content := tui.GenerateStarterMarkdown(title, theme, time.Now().Format("2006-01-02"), "Your Name")
	if err := os.WriteFile(output, []byte(content), 0o644); err != nil {
		return internalError(codeInternal, fmt.Errorf("failed to write %s: %w", output, err))
	}

	recordNewDeckApproval(cmd, output)

	if newJSON {
		return printJSONOK(cmd.OutOrStdout(), struct {
			Deck   string `json:"deck"`
			Folder string `json:"folder"`
		}{Deck: output, Folder: folder})
	}
	fmt.Fprintln(cmd.OutOrStdout(), output)
	return nil
}

// makeFreeFolder creates <location>/<stem>, or <location>/<stem>-2, -3
// and so on when that exists, and returns it with the name it settled
// on. The folder is made with os.Mkdir inside the loop, so two runs with
// one title at the same moment cannot both settle on the same folder: the
// second one's Mkdir fails with an existing folder and it tries the next.
func makeFreeFolder(location, stem string) (folder, name string, err error) {
	name = stem
	for attempt := 2; ; attempt++ {
		folder = filepath.Join(location, name)
		mkdirErr := os.Mkdir(folder, 0o755)
		if mkdirErr == nil {
			return folder, name, nil
		}
		if !os.IsExist(mkdirErr) {
			return "", "", fmt.Errorf("failed to create %s: %w", folder, mkdirErr)
		}
		name = fmt.Sprintf("%s-%d", stem, attempt)
	}
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
		approvalKey, err := usersettings.EnsureApprovalKey(settingsPath)
		if err != nil {
			return err
		}
		drivers := make([]usersettings.Driver, 0, len(cfg.Drivers))
		for _, name := range cfg.DeclaredDrivers() {
			drivers = append(drivers, newLiveDriver(name, cfg.Drivers[name]).stored(approvalKey))
		}
		settings.ApproveDrivers(key, drivers, now)
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
