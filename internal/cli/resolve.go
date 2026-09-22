package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MiniCodeMonkey/tap/internal/tui"
)

// notDeckNames are markdown files that are never a deck, compared in
// lower case.
var notDeckNames = map[string]bool{
	"readme.md":       true,
	"changelog.md":    true,
	"contributing.md": true,
	"license.md":      true,
}

// deckPicker asks the person to choose one of several decks. cancelled is
// true when they closed the picker without choosing.
type deckPicker func(candidates []string) (chosen string, cancelled bool, err error)

// deckResolver picks the deck a command acts on. interactive and pick are
// fields so a test can stand in for the terminal.
type deckResolver struct {
	interactive func() bool
	pick        deckPicker
}

var defaultDeckResolver = deckResolver{
	interactive: func() bool { return stdinIsTerminal() },
	pick: func(candidates []string) (string, bool, error) {
		result, err := tui.RunFilePickerWith(candidates)
		if err != nil {
			return "", false, err
		}
		return result.File, result.Aborted, nil
	},
}

// resolveDeck returns the deck file for a command's optional [deck]
// argument, which is a file or a folder. The order is: the file itself;
// the only deck in the folder (the current folder when arg is empty); a
// picker when standard input is a terminal; otherwise an error that lists
// the candidates.
func resolveDeck(arg string) (string, error) {
	return defaultDeckResolver.resolve(arg)
}

func (r deckResolver) resolve(arg string) (string, error) {
	folder := "."
	if arg != "" {
		info, err := os.Stat(arg)
		if os.IsNotExist(err) {
			return "", userError(codeDeckNotFound, fmt.Errorf("deck not found: %s", arg))
		}
		if err != nil {
			return "", userError(codeDeckNotFound, fmt.Errorf("cannot read %s: %w", arg, err))
		}
		if !info.IsDir() {
			return arg, nil
		}
		folder = arg
	}

	candidates, err := deckCandidates(folder)
	if err != nil {
		return "", userError(codeDeckNotFound, err)
	}
	switch len(candidates) {
	case 0:
		return "", userError(codeNoDeck, fmt.Errorf("no deck found in %s: name a deck file, or create one with tap new", describeFolder(folder)))
	case 1:
		return candidates[0], nil
	}

	if !r.interactive() {
		return "", userError(codeAmbiguousDeck, fmt.Errorf("more than one deck in %s, name one of: %s", describeFolder(folder), strings.Join(candidates, ", ")))
	}
	chosen, cancelled, err := r.pick(candidates)
	if err != nil {
		return "", internalError(codeInternal, fmt.Errorf("deck picker: %w", err))
	}
	if cancelled || chosen == "" {
		return "", errCancelled
	}
	return chosen, nil
}

// deckCandidates lists the decks in folder, newest first: every .md file
// except the ones in notDeckNames. Each path is joined to folder.
func deckCandidates(folder string) ([]string, error) {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", folder, err)
	}

	type candidate struct {
		path     string
		modified int64
	}
	var found []candidate
	for _, entry := range entries {
		name := entry.Name()
		lower := strings.ToLower(name)
		if entry.IsDir() || !strings.HasSuffix(lower, ".md") || notDeckNames[lower] {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		found = append(found, candidate{path: filepath.Join(folder, name), modified: info.ModTime().UnixNano()})
	}

	sort.Slice(found, func(i, j int) bool {
		if found[i].modified != found[j].modified {
			return found[i].modified > found[j].modified
		}
		return found[i].path < found[j].path
	})
	paths := make([]string, len(found))
	for index, deck := range found {
		paths[index] = deck.path
	}
	return paths, nil
}

// resolveDeckFolder returns the folder a deck lives in, for commands that
// need only the folder: the argument when it is a folder, the folder of a
// deck file, or the current folder when arg is empty.
func resolveDeckFolder(arg string) (string, error) {
	if arg == "" {
		return ".", nil
	}
	info, err := os.Stat(arg)
	if os.IsNotExist(err) {
		return "", userError(codeDeckNotFound, fmt.Errorf("deck not found: %s", arg))
	}
	if err != nil {
		return "", userError(codeDeckNotFound, fmt.Errorf("cannot read %s: %w", arg, err))
	}
	if info.IsDir() {
		return arg, nil
	}
	return filepath.Dir(arg), nil
}

// describeFolder names a folder in a message.
func describeFolder(folder string) string {
	if folder == "." {
		return "the current folder"
	}
	return folder
}

// firstArg returns the first positional argument, or "" when there is
// none.
func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}
