package deckedit

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSetThemeReplacesTheTheme(t *testing.T) {
	path := filepath.Join(t.TempDir(), "talk.md")
	if err := os.WriteFile(path, []byte("---\ntitle: Demo\ntheme: base\n---\n\n# One\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetTheme(path, "terminal"); err != nil {
		t.Fatalf("SetTheme() error = %v", err)
	}
	content, _ := os.ReadFile(path)
	if string(content) != "---\ntitle: Demo\ntheme: terminal\n---\n\n# One\n" {
		t.Errorf("file = %q", content)
	}
}

func TestSetThemeAddsFrontmatter(t *testing.T) {
	path := filepath.Join(t.TempDir(), "talk.md")
	if err := os.WriteFile(path, []byte("# One\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetTheme(path, "terminal"); err != nil {
		t.Fatalf("SetTheme() error = %v", err)
	}
	content, _ := os.ReadFile(path)
	if string(content) != "---\ntheme: terminal\n---\n# One\n" {
		t.Errorf("file = %q", content)
	}
}

func TestSetThemeRejectsAnUnknownThemeAndLeavesTheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "talk.md")
	original := "---\ntheme: base\n---\n\n# One\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SetTheme(path, "no-such-theme"); !errors.Is(err, ErrUnknownTheme) {
		t.Errorf("SetTheme() error = %v, want ErrUnknownTheme", err)
	}
	content, _ := os.ReadFile(path)
	if string(content) != original {
		t.Errorf("file changed to %q", content)
	}
}
