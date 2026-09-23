package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/MiniCodeMonkey/tap/internal/deckedit"
	"github.com/MiniCodeMonkey/tap/internal/themes"
)

var themeSetJSON bool

// themeSetCmd writes a theme into a deck's frontmatter.
var themeSetCmd = &cobra.Command{
	Use:   "set <slug> [deck]",
	Short: "Set a deck's theme",
	Long: `Set the theme: key in a deck's frontmatter, as the t key in tap dev
does. A deck with no frontmatter gets one.

[deck] is a deck file or folder. With no deck, tap uses the deck in the
current folder. See tap theme list for the slugs.

Examples:
  tap theme set terminal
  tap theme set blueprint talk.md
  tap theme set blueprint talk.md --json`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runThemeSet,
}

func init() {
	themeCmd.AddCommand(themeSetCmd)
	themeSetCmd.Flags().BoolVar(&themeSetJSON, "json", false, "print the result as JSON")
}

// themeSetResult is the --json result of tap theme set.
type themeSetResult struct {
	Deck  string `json:"deck"`
	Theme string `json:"theme"`
}

func runThemeSet(cmd *cobra.Command, args []string) error {
	slug := args[0]
	if !themes.IsValid(slug) {
		return userError(codeUnknownTheme, unknownThemeError(slug))
	}
	var deckArg string
	if len(args) > 1 {
		deckArg = args[1]
	}
	deck, err := resolveDeck(deckArg)
	if err != nil {
		return err
	}
	if err := deckedit.SetTheme(deck, slug); err != nil {
		return userError(codeInvalidDeck, fmt.Errorf("cannot set the theme of %s: %w", deck, err))
	}
	if themeSetJSON {
		return printJSONOK(cmd.OutOrStdout(), themeSetResult{Deck: deck, Theme: slug})
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Theme set to %s in %s\n", slug, deck)
	return nil
}
