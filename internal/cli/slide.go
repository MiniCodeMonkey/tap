package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/MiniCodeMonkey/tap/internal/deckedit"
	"github.com/MiniCodeMonkey/tap/internal/layouts"
	"github.com/MiniCodeMonkey/tap/internal/tui"
)

// Flags for tap slide add.
var (
	slideAddLayout string
	slideAddPrint  bool
	slideAddJSON   bool
)

// slideCmd groups the commands that work on a deck's slides.
var slideCmd = &cobra.Command{
	Use:   "slide",
	Short: "Work with the slides in a deck",
	Args:  cobra.ArbitraryArgs,
	RunE:  runUnknownGroupSubcommand,
}

// slideAddCmd appends a slide, through the wizard or from a layout's
// template.
var slideAddCmd = &cobra.Command{
	Use:   "add [deck]",
	Short: "Add a slide to a deck",
	Long: `Add a slide to the end of a deck.

Without flags, an interactive wizard asks for a layout and the content of
each section of the slide. It needs a terminal.

With --layout, tap appends that layout's template without asking. With
--print as well, it prints the template and writes nothing; the template
has no "---" separator in front of it, and no deck is needed.

Layouts: ` + strings.Join(layouts.Names(), ", ") + `.

Examples:
  tap slide add                               # The wizard, for the deck in this folder
  tap slide add talk.md --layout quote        # Append a quote slide
  tap slide add --layout big-stat --print     # Print the big-stat template
  tap slide add --layout big-stat --print --json`,
	Args: cobra.MaximumNArgs(1),
	RunE: runSlideAdd,
}

func init() {
	rootCmd.AddCommand(slideCmd)
	slideCmd.AddCommand(slideAddCmd)

	slideAddCmd.Flags().StringVar(&slideAddLayout, "layout", "", "append this layout's template instead of running the wizard")
	slideAddCmd.Flags().BoolVar(&slideAddPrint, "print", false, "print the template and write nothing (needs --layout)")
	slideAddCmd.Flags().BoolVar(&slideAddJSON, "json", false, "print the result as JSON (needs --layout)")
}

// slideAddResult is the --json result of tap slide add --layout. Deck is
// empty with --print.
type slideAddResult struct {
	Deck     string `json:"deck,omitempty"`
	Layout   string `json:"layout"`
	Markdown string `json:"markdown"`
}

// layoutListEntry is one layout in the --json result of tap slide add
// --print --json with no --layout.
type layoutListEntry struct {
	Name     string `json:"name"`
	Template string `json:"template"`
}

// layoutListResult is the --json result of tap slide add --print --json
// with no --layout: every layout's template, in the wizard's order, so
// the app's layout gallery hard-codes no layout names.
type layoutListResult struct {
	Layouts []layoutListEntry `json:"layouts"`
}

func runSlideAdd(cmd *cobra.Command, args []string) error {
	if slideAddLayout != "" {
		return addSlideFromTemplate(cmd, args)
	}
	if slideAddPrint && slideAddJSON {
		return printAllLayouts(cmd)
	}
	if slideAddPrint {
		return userError(codeUsage, errors.New("--print needs --layout"))
	}
	if slideAddJSON {
		return userError(codeUsage, errors.New("--json needs --layout: the wizard has no JSON output"))
	}

	if !stdinIsTerminal() {
		return userError(codeNeedsTerminal, errors.New("tap slide add runs a wizard and needs a terminal; pass --layout to add a slide without it"))
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
}

// printAllLayouts prints every layout's template, in the wizard's order:
// the controller's ruling for tap slide add --print --json with no
// --layout, so the app's layout gallery reads it instead of hard-coding
// layout names.
func printAllLayouts(cmd *cobra.Command) error {
	templates := layouts.Templates()
	entries := make([]layoutListEntry, len(templates))
	for i, template := range templates {
		body, err := layouts.RenderSlide(template.Name, nil)
		if err != nil {
			return internalError(codeInternal, err)
		}
		entries[i] = layoutListEntry{Name: template.Name, Template: body}
	}
	return printJSONOK(cmd.OutOrStdout(), layoutListResult{Layouts: entries})
}

// addSlideFromTemplate prints or appends the template of --layout.
func addSlideFromTemplate(cmd *cobra.Command, args []string) error {
	body, err := layouts.RenderSlide(slideAddLayout, nil)
	if err != nil {
		return userError(codeUnknownLayout, err)
	}

	if slideAddPrint {
		if slideAddJSON {
			return printJSONOK(cmd.OutOrStdout(), slideAddResult{Layout: slideAddLayout, Markdown: body})
		}
		_, err := fmt.Fprint(cmd.OutOrStdout(), body)
		return err
	}

	deck, err := resolveDeck(firstArg(args))
	if err != nil {
		return err
	}
	if err := deckedit.AppendSlide(deck, body); err != nil {
		return userError(codeInvalidDeck, fmt.Errorf("cannot add a slide to %s: %w", deck, err))
	}
	if slideAddJSON {
		return printJSONOK(cmd.OutOrStdout(), slideAddResult{Deck: deck, Layout: slideAddLayout, Markdown: body})
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Added a %s slide to %s\n", slideAddLayout, deck)
	return nil
}
