package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/MiniCodeMonkey/tap/internal/tui"
)

// slideCmd groups the commands that work on a deck's slides.
var slideCmd = &cobra.Command{
	Use:   "slide",
	Short: "Work with the slides in a deck",
	Args:  cobra.ArbitraryArgs,
	RunE:  runUnknownGroupSubcommand,
}

// slideAddCmd appends a slide through the interactive wizard.
var slideAddCmd = &cobra.Command{
	Use:   "add [deck]",
	Short: "Add a slide to a deck interactively",
	Long: `Add a slide to the end of a deck with an interactive wizard.

The wizard asks for a layout and the content of each section of the slide.
It needs a terminal.

Examples:
  tap slide add              # The deck in this folder
  tap slide add talk.md      # A specific deck`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if !stdinIsTerminal() {
			return userError(codeNeedsTerminal, errors.New("tap slide add runs a wizard and needs a terminal"))
		}
		file, err := resolveDeck(firstArg(args))
		if err != nil {
			return err
		}
		result, err := tui.RunAddWizard(file)
		if err != nil {
			return internalError(codeInternal, err)
		}
		if result.Aborted {
			return errCancelled
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(slideCmd)
	slideCmd.AddCommand(slideAddCmd)
}
