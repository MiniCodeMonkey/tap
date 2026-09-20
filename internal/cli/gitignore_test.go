package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func gitRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestGitignoreStateWantsAnEntryInARepo(t *testing.T) {
	dir := gitRepo(t)

	needed, path := gitignoreState(dir, "recordings/")
	if !needed {
		t.Error("a repo with no .gitignore does not want the entry")
	}
	if path != filepath.Join(dir, ".gitignore") {
		t.Errorf("gitignoreState() = %q, want the repo's .gitignore", path)
	}
}

func TestGitignoreStateIsQuietOutsideARepo(t *testing.T) {
	if needed, _ := gitignoreState(t.TempDir(), "recordings/"); needed {
		t.Error("a plain directory wants a .gitignore entry")
	}
}

func TestGitignoreStateIsQuietWhenAlreadyIgnored(t *testing.T) {
	dir := gitRepo(t)
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules\nrecordings/\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if needed, _ := gitignoreState(dir, "recordings/"); needed {
		t.Error("an already-ignored directory wants the entry again")
	}
}

func TestGitignoreStateFindsTheRepoRootAbove(t *testing.T) {
	dir := gitRepo(t)
	deck := filepath.Join(dir, "talks", "2026")
	if err := os.MkdirAll(deck, 0o755); err != nil {
		t.Fatal(err)
	}

	needed, path := gitignoreState(deck, "recordings/")
	if !needed {
		t.Error("a deck in a subdirectory does not find the repo")
	}
	if path != filepath.Join(dir, ".gitignore") {
		t.Errorf("gitignoreState() = %q, want the repo root's .gitignore", path)
	}
}

func TestAppendGitignoreEntryKeepsWhatIsThere(t *testing.T) {
	dir := gitRepo(t)
	path := filepath.Join(dir, ".gitignore")
	if err := os.WriteFile(path, []byte("node_modules"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := appendGitignoreEntry(path, "recordings/"); err != nil {
		t.Fatalf("appendGitignoreEntry() returned %v", err)
	}

	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "node_modules") {
		t.Error("the existing entry was lost")
	}
	if !strings.HasSuffix(string(written), "recordings/\n") {
		t.Errorf("the file ends with %q, want the entry on its own final line", written)
	}
}

func TestAppendGitignoreEntryCreatesTheFile(t *testing.T) {
	path := filepath.Join(gitRepo(t), ".gitignore")

	if err := appendGitignoreEntry(path, "recordings/"); err != nil {
		t.Fatalf("appendGitignoreEntry() returned %v", err)
	}

	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != "recordings/\n" {
		t.Errorf("the file holds %q", written)
	}
}
