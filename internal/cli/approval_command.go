package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/MiniCodeMonkey/tap/internal/usersettings"
)

var (
	approvalListJSON   bool
	approvalRevokeJSON bool
)

var approvalCmd = &cobra.Command{
	Use:   "approval",
	Short: "List and revoke the decks allowed to run live code",
	Long: `A deck with live code runs nothing until you approve it. tap dev and
tap present ask once, in the terminal, and remember the answer in
~/.config/tap/settings.yaml, keyed by the deck's path and the drivers you
allowed. A moved deck, or a new driver, asks again.`,
}

var approvalListCmd = &cobra.Command{
	Use:   "list",
	Short: "List the decks allowed to run live code",
	Long: `List the decks allowed to run live code, with the drivers each may use.

Examples:
  tap approval list
  tap approval list --json`,
	Args: cobra.NoArgs,
	RunE: runApprovalList,
}

var approvalRevokeCmd = &cobra.Command{
	Use:   "revoke <deck>",
	Short: "Stop a deck from running live code until you approve it again",
	Long: `Remove a deck's approval. tap asks again the next time it opens the deck.

<deck> is the deck file or its folder. A deck that was moved or deleted
can be revoked by its old path.

Examples:
  tap approval revoke talk.md
  tap approval revoke ~/talks/old-place/talk.md`,
	Args: cobra.ExactArgs(1),
	RunE: runApprovalRevoke,
}

func init() {
	rootCmd.AddCommand(approvalCmd)
	approvalCmd.AddCommand(approvalListCmd)
	approvalCmd.AddCommand(approvalRevokeCmd)
	approvalListCmd.Flags().BoolVar(&approvalListJSON, "json", false, "print the approvals as JSON")
	approvalRevokeCmd.Flags().BoolVar(&approvalRevokeJSON, "json", false, "print the result as JSON")
}

// loadApprovalSettings returns the settings file path and its contents.
func loadApprovalSettings() (string, usersettings.Settings, error) {
	settingsPath, err := usersettings.Path()
	if err != nil {
		return "", usersettings.Settings{}, internalError(codeInternal, err)
	}
	settings, err := usersettings.Load(settingsPath)
	if err != nil {
		return "", usersettings.Settings{}, userError(codeInvalidSettings, err)
	}
	return settingsPath, settings, nil
}

func runApprovalList(cmd *cobra.Command, args []string) error {
	_, settings, err := loadApprovalSettings()
	if err != nil {
		return err
	}
	approvals := make([]usersettings.Approval, 0, len(settings.Approvals))
	for _, approval := range settings.Approvals {
		if approval.Drivers == nil {
			approval.Drivers = []string{}
		}
		approvals = append(approvals, approval)
	}

	out := cmd.OutOrStdout()
	if approvalListJSON {
		return printJSONOK(out, struct {
			Approvals []usersettings.Approval `json:"approvals"`
		}{Approvals: approvals})
	}
	if len(approvals) == 0 {
		fmt.Fprintln(out, "No deck is approved to run live code.")
		return nil
	}
	for _, approval := range approvals {
		drivers := strings.Join(approval.Drivers, ", ")
		if drivers == "" {
			drivers = "none"
		}
		fmt.Fprintln(out, approval.Deck)
		fmt.Fprintf(out, "  drivers:  %s\n", drivers)
		fmt.Fprintf(out, "  approved: %s\n", approval.ApprovedAt.UTC().Format(time.RFC3339))
	}
	return nil
}

func runApprovalRevoke(cmd *cobra.Command, args []string) error {
	deck, key, resolved, err := resolveApprovalTarget(args[0])
	if err != nil {
		return err
	}
	settingsPath, settings, err := loadApprovalSettings()
	if err != nil {
		return err
	}

	// A deck still on disk is revoked by its DeckKey, resolved fresh so
	// the store never sees an unresolved path. A deck that no longer
	// exists cannot be resolved at all, so RevokeStoredPath, the one
	// function the approval store still takes a plain string for, is the
	// only way to reach its approval: it was keyed by that stored path
	// when it was approved, and there is nothing left on disk to resolve
	// again.
	var revoked bool
	if resolved {
		revoked = settings.Revoke(key)
	} else {
		revoked = settings.RevokeStoredPath(deck)
	}
	if !revoked {
		return userError(codeNotApproved, fmt.Errorf("%s is not approved to run live code", deck))
	}
	if err := usersettings.Save(settingsPath, settings); err != nil {
		return internalError(codeInternal, err)
	}

	out := cmd.OutOrStdout()
	if approvalRevokeJSON {
		return printJSONOK(out, struct {
			Deck string `json:"deck"`
		}{Deck: deck})
	}
	fmt.Fprintf(out, "Revoked: %s. tap asks again the next time it opens this deck.\n", deck)
	return nil
}

// resolveApprovalTarget turns the revoke argument into the settings key
// an approval is stored under, and reports whether the deck could be
// resolved. arg is a deck file or its folder.
//
// A deck that still exists goes through the same two steps every real
// approval does: resolveDeck picks the file (naming a folder resolves to
// the one deck in it, exactly as tap dev and tap present would open it),
// and usersettings.ResolveDeck turns that into the DeckKey the store
// compares and keys by. resolved is true, and key is usable.
//
// A deck that no longer exists, such as one that was moved or deleted,
// cannot be resolved at all: ResolveDeck needs a real file to canonicalize
// against. deck falls back to arg's absolute path, unresolved, which is
// exactly the spelling RevokeStoredPath needs, since that is what the
// approval was stored under. resolved is false, and key is the zero
// value.
func resolveApprovalTarget(arg string) (deck string, key usersettings.DeckKey, resolved bool, err error) {
	if _, statErr := os.Stat(arg); statErr == nil {
		path, err := resolveDeck(arg)
		if err != nil {
			return "", usersettings.DeckKey{}, false, err
		}
		resolvedKey, err := usersettings.ResolveDeck(path)
		if err != nil {
			return "", usersettings.DeckKey{}, false, internalError(codeInternal, err)
		}
		return resolvedKey.String(), resolvedKey, true, nil
	}
	absolute, err := filepath.Abs(arg)
	if err != nil {
		return "", usersettings.DeckKey{}, false, internalError(codeInternal, err)
	}
	return absolute, usersettings.DeckKey{}, false, nil
}
