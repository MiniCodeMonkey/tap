package cli

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MiniCodeMonkey/tap/internal/deckedit"
	"github.com/MiniCodeMonkey/tap/internal/gemini"
)

// writeDeckFile writes content to dir/name and returns the path.
func writeDeckFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// folderSnapshot maps every file under dir, by its slash-separated path
// relative to dir, to its content.
func folderSnapshot(t *testing.T, dir string) map[string]string {
	t.Helper()
	snapshot := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		snapshot[filepath.ToSlash(relative)] = string(content)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return snapshot
}

// twoDeckCopies writes the same deck into two new folders, one for the TUI
// key path and one for the command, and returns both deck paths.
func twoDeckCopies(t *testing.T, name, content string) (tuiDeck, commandDeck string) {
	t.Helper()
	return writeDeckFile(t, t.TempDir(), name, content), writeDeckFile(t, t.TempDir(), name, content)
}

// requireSameFolders fails the test unless the folders of the two decks
// hold the same files with the same bytes.
func requireSameFolders(t *testing.T, tuiDeck, commandDeck string) {
	t.Helper()
	tuiFiles := folderSnapshot(t, filepath.Dir(tuiDeck))
	commandFiles := folderSnapshot(t, filepath.Dir(commandDeck))
	names := func(files map[string]string) string {
		list := make([]string, 0, len(files))
		for name := range files {
			list = append(list, name)
		}
		sort.Strings(list)
		return strings.Join(list, ", ")
	}
	if names(tuiFiles) != names(commandFiles) {
		t.Fatalf("files differ:\n  TUI:     %s\n  command: %s", names(tuiFiles), names(commandFiles))
	}
	for name, tuiContent := range tuiFiles {
		if commandFiles[name] != tuiContent {
			t.Errorf("%s differs:\n--- TUI ---\n%s\n--- command ---\n%s", name, tuiContent, commandFiles[name])
		}
	}
}

// runeKey is a key press that types text.
func runeKey(text string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)}
}

// pressKeys sends each key to model in order and returns the final model
// and the command the last key returned.
func pressKeys(model tea.Model, keys ...tea.KeyMsg) (tea.Model, tea.Cmd) {
	var command tea.Cmd
	for _, key := range keys {
		model, command = model.Update(key)
	}
	return model, command
}

// repeatKey returns key count times. A negative count gives none.
func repeatKey(key tea.KeyMsg, count int) []tea.KeyMsg {
	keys := make([]tea.KeyMsg, 0, max(count, 0))
	for range max(count, 0) {
		keys = append(keys, key)
	}
	return keys
}

// fakeImageGenerator stands in for the Gemini API. Each image's bytes are
// derived from its prompt, so the same prompt gives the same file name.
type fakeImageGenerator struct {
	err     error
	prompts []string
}

func (f *fakeImageGenerator) GenerateImage(ctx context.Context, prompt string) (*gemini.ImageResult, error) {
	f.prompts = append(f.prompts, prompt)
	if f.err != nil {
		return nil, f.err
	}
	return &gemini.ImageResult{Data: []byte("png bytes for " + prompt), ContentType: "image/png"}, nil
}

// useFakeImageGenerator makes tap and the TUI use a fake generator for
// the rest of the test, and sets GEMINI_API_KEY so the TUI opens its
// image generator.
func useFakeImageGenerator(t *testing.T) *fakeImageGenerator {
	t.Helper()
	t.Setenv("GEMINI_API_KEY", "test-key")
	fake := &fakeImageGenerator{}
	original := deckedit.NewImageGenerator
	deckedit.NewImageGenerator = func(deckPath string) (deckedit.ImageGenerator, error) { return fake, nil }
	t.Cleanup(func() { deckedit.NewImageGenerator = original })
	return fake
}

// deliverCommand runs command and sends each message it produces to
// model, flattening batches. It does not run the commands those messages
// return, so a spinner tick does not loop.
func deliverCommand(model tea.Model, command tea.Cmd) tea.Model {
	if command == nil {
		return model
	}
	message := command()
	if batch, ok := message.(tea.BatchMsg); ok {
		for _, inner := range batch {
			model = deliverCommand(model, inner)
		}
		return model
	}
	model, _ = model.Update(message)
	return model
}
