package cli

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeDecks creates each named file in dir, one second apart in
// modification time so their order is fixed (newest last).
func writeDecks(t *testing.T, dir string, names ...string) {
	t.Helper()
	base := time.Now().Add(-time.Hour)
	for index, name := range names {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte("# "+name), 0o644); err != nil {
			t.Fatal(err)
		}
		modified := base.Add(time.Duration(index) * time.Second)
		if err := os.Chtimes(path, modified, modified); err != nil {
			t.Fatal(err)
		}
	}
}

func noPicker(t *testing.T) deckPicker {
	return func(candidates []string) (string, bool, error) {
		t.Fatalf("the picker should not open, candidates %v", candidates)
		return "", false, nil
	}
}

func TestResolveDeckUsesAFileArgument(t *testing.T) {
	dir := t.TempDir()
	writeDecks(t, dir, "talk.md")
	resolver := deckResolver{interactive: func() bool { return true }, pick: noPicker(t)}
	got, err := resolver.resolve(filepath.Join(dir, "talk.md"))
	if err != nil || got != filepath.Join(dir, "talk.md") {
		t.Errorf("resolve() = (%q, %v)", got, err)
	}
}

func TestResolveDeckMissingArgument(t *testing.T) {
	resolver := deckResolver{interactive: func() bool { return true }, pick: noPicker(t)}
	_, err := resolver.resolve(filepath.Join(t.TempDir(), "missing.md"))
	if _, code, _ := classify(err); code != codeDeckNotFound {
		t.Errorf("code = %q, want %q (err %v)", code, codeDeckNotFound, err)
	}
}

func TestResolveDeckTheOnlyDeckInAFolder(t *testing.T) {
	dir := t.TempDir()
	writeDecks(t, dir, "README.md", "talk.md", "notes.txt")
	resolver := deckResolver{interactive: func() bool { return false }, pick: noPicker(t)}
	got, err := resolver.resolve(dir)
	if err != nil || got != filepath.Join(dir, "talk.md") {
		t.Errorf("resolve() = (%q, %v), want the only deck, skipping README.md", got, err)
	}
}

func TestResolveDeckTheOnlyDeckInTheCurrentFolder(t *testing.T) {
	dir := t.TempDir()
	writeDecks(t, dir, "talk.md")
	resolver := deckResolver{interactive: func() bool { return false }, pick: noPicker(t)}
	withWorkingDirectory(t, dir, func() {
		got, err := resolver.resolve("")
		if err != nil || got != "talk.md" {
			t.Errorf("resolve(\"\") = (%q, %v), want talk.md", got, err)
		}
	})
}

func TestResolveDeckNoDeck(t *testing.T) {
	dir := t.TempDir()
	writeDecks(t, dir, "README.md")
	resolver := deckResolver{interactive: func() bool { return true }, pick: noPicker(t)}
	_, err := resolver.resolve(dir)
	if _, code, _ := classify(err); code != codeNoDeck {
		t.Errorf("code = %q, want %q (err %v)", code, codeNoDeck, err)
	}
}

func TestResolveDeckPicksOnATerminal(t *testing.T) {
	dir := t.TempDir()
	writeDecks(t, dir, "a.md", "b.md")
	var offered []string
	resolver := deckResolver{
		interactive: func() bool { return true },
		pick: func(candidates []string) (string, bool, error) {
			offered = candidates
			return candidates[1], false, nil
		},
	}
	got, err := resolver.resolve(dir)
	if err != nil {
		t.Fatalf("resolve() error = %v", err)
	}
	wantOffered := []string{filepath.Join(dir, "b.md"), filepath.Join(dir, "a.md")}
	if strings.Join(offered, ",") != strings.Join(wantOffered, ",") {
		t.Errorf("picker offered %v, want newest first %v", offered, wantOffered)
	}
	if got != filepath.Join(dir, "a.md") {
		t.Errorf("resolve() = %q, want the picked deck", got)
	}
}

func TestResolveDeckPickerCancelled(t *testing.T) {
	dir := t.TempDir()
	writeDecks(t, dir, "a.md", "b.md")
	resolver := deckResolver{
		interactive: func() bool { return true },
		pick:        func([]string) (string, bool, error) { return "", true, nil },
	}
	if _, err := resolver.resolve(dir); !errors.Is(err, errCancelled) {
		t.Errorf("resolve() error = %v, want errCancelled", err)
	}
}

func TestResolveDeckWithoutATerminalListsCandidates(t *testing.T) {
	dir := t.TempDir()
	writeDecks(t, dir, "a.md", "b.md")
	resolver := deckResolver{interactive: func() bool { return false }, pick: noPicker(t)}
	_, err := resolver.resolve(dir)
	exitCode, code, _ := classify(err)
	if exitCode != exitUserError || code != codeAmbiguousDeck {
		t.Fatalf("classify() = (%d, %q), want (1, %q)", exitCode, code, codeAmbiguousDeck)
	}
	for _, name := range []string{"a.md", "b.md"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error %q does not list %s", err, name)
		}
	}
}

func TestResolveDeckFolder(t *testing.T) {
	dir := t.TempDir()
	writeDecks(t, dir, "talk.md")
	tests := []struct {
		name string
		arg  string
		want string
	}{
		{"no argument is the current folder", "", "."},
		{"a folder is used as is", dir, dir},
		{"a file gives its folder", filepath.Join(dir, "talk.md"), dir},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveDeckFolder(tt.arg)
			if err != nil || got != tt.want {
				t.Errorf("resolveDeckFolder(%q) = (%q, %v), want %q", tt.arg, got, err, tt.want)
			}
		})
	}
	if _, err := resolveDeckFolder(filepath.Join(dir, "missing")); err == nil {
		t.Error("resolveDeckFolder() of a missing path should fail")
	}
}
