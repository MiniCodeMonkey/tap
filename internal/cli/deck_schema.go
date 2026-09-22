package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/MiniCodeMonkey/tap/internal/config"
)

var deckSchemaJSON bool

// deckCmd groups the commands that describe what a deck file can hold.
var deckCmd = &cobra.Command{
	Use:   "deck",
	Short: "Describe what a deck file can hold",
}

var deckSchemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "List every frontmatter key tap understands",
	Long: `List every frontmatter key tap understands, with its type, its
default, its allowed values, and what it does. Nested keys are shown with
dots, and "<name>" stands for a name the deck picks, such as a driver's.

Editors and tools can build a form or completions from --json.

Examples:
  tap deck schema
  tap deck schema --json`,
	Args: cobra.NoArgs,
	RunE: runDeckSchema,
}

func init() {
	rootCmd.AddCommand(deckCmd)
	deckCmd.AddCommand(deckSchemaCmd)
	deckSchemaCmd.Flags().BoolVar(&deckSchemaJSON, "json", false, "print the schema as JSON")
}

func runDeckSchema(cmd *cobra.Command, args []string) error {
	keys := config.Schema()
	if deckSchemaJSON {
		return printJSONOK(cmd.OutOrStdout(), struct {
			Keys []config.SchemaKey `json:"keys"`
		}{Keys: keys})
	}

	table := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(table, "KEY\tTYPE\tDEFAULT\tVALUES\tDESCRIPTION")
	writeSchemaRows(table, keys, "")
	return table.Flush()
}

// writeSchemaRows writes one table row per key, then the rows of its
// nested keys, with the key path joined by dots.
func writeSchemaRows(w io.Writer, keys []config.SchemaKey, prefix string) {
	for _, key := range keys {
		path := prefix + key.Name
		defaultText := ""
		if key.Default != nil {
			defaultText = fmt.Sprint(key.Default)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", path, key.Type, defaultText, strings.Join(key.Values, ", "), key.Description)
		switch key.Type {
		case "object":
			writeSchemaRows(w, key.Keys, path+".")
		case "map":
			writeSchemaRows(w, key.Keys, path+".<name>.")
		}
	}
}
