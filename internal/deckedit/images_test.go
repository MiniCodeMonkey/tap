package deckedit

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, content string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAddImageCopiesIntoImages(t *testing.T) {
	deckDir := t.TempDir()
	deck := writeFile(t, filepath.Join(deckDir, "talk.md"), "# One\n")
	source := writeFile(t, filepath.Join(t.TempDir(), "diagram.png"), "png bytes")

	added, err := AddImage(deck, source)
	if err != nil {
		t.Fatalf("AddImage() error = %v", err)
	}
	if added.Path != filepath.Join("images", "diagram.png") || added.Markdown != "![diagram](images/diagram.png)" {
		t.Errorf("added = %+v", added)
	}
	copied, err := os.ReadFile(filepath.Join(deckDir, "images", "diagram.png"))
	if err != nil || string(copied) != "png bytes" {
		t.Errorf("copied file = (%q, %v)", copied, err)
	}
}

func TestAddImageNumbersAClash(t *testing.T) {
	deckDir := t.TempDir()
	deck := writeFile(t, filepath.Join(deckDir, "talk.md"), "# One\n")
	source := writeFile(t, filepath.Join(t.TempDir(), "diagram.png"), "png bytes")

	for _, want := range []string{"diagram.png", "diagram-2.png", "diagram-3.png"} {
		added, err := AddImage(deck, source)
		if err != nil {
			t.Fatalf("AddImage() error = %v", err)
		}
		if added.Path != filepath.Join("images", want) {
			t.Errorf("Path = %q, want images/%s", added.Path, want)
		}
	}
}

func TestAddImageOfAFileAlreadyInImagesDoesNotCopy(t *testing.T) {
	deckDir := t.TempDir()
	deck := writeFile(t, filepath.Join(deckDir, "talk.md"), "# One\n")
	source := writeFile(t, filepath.Join(deckDir, "images", "diagram.png"), "png bytes")

	added, err := AddImage(deck, source)
	if err != nil || added.Path != filepath.Join("images", "diagram.png") {
		t.Errorf("AddImage() = (%+v, %v), want the existing file", added, err)
	}
	entries, _ := os.ReadDir(filepath.Join(deckDir, "images"))
	if len(entries) != 1 {
		t.Errorf("images/ has %d files, want 1", len(entries))
	}
}

func TestAddImageRejectsANonImage(t *testing.T) {
	deck := writeFile(t, filepath.Join(t.TempDir(), "talk.md"), "# One\n")
	source := writeFile(t, filepath.Join(t.TempDir(), "notes.txt"), "text")
	if _, err := AddImage(deck, source); !errors.Is(err, ErrNotAnImage) {
		t.Errorf("AddImage() error = %v, want ErrNotAnImage", err)
	}
}

func TestAddImageMissingSource(t *testing.T) {
	deck := writeFile(t, filepath.Join(t.TempDir(), "talk.md"), "# One\n")
	if _, err := AddImage(deck, filepath.Join(t.TempDir(), "missing.png")); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("AddImage() error = %v, want fs.ErrNotExist", err)
	}
}

// TestAddImageSanitizesTheCopiedName covers the controller's ruling on
// Question 2 of the plan (docs/superpowers/plans/2026-09-22-commands-behind-tui-keys.md):
// tap image add replaces spaces, parentheses and other characters that
// need escaping in a markdown link with "-" in the copied file's name,
// then applies the -2, -3 clash rule. The link stays a plain relative
// path, never wrapped in angle brackets.
func TestAddImageSanitizesTheCopiedName(t *testing.T) {
	deckDir := t.TempDir()
	deck := writeFile(t, filepath.Join(deckDir, "talk.md"), "# One\n")

	tests := map[string]string{
		"my diagram.png":     "my-diagram.png",
		"chart (v2).png":     "chart-v2.png",
		`he said "hi".png`:   "he-said-hi.png",
		"it's a diagram.png": "it-s-a-diagram.png",
		"back`tick.png":      "back-tick.png",
		"diagram#1.png":      "diagram-1.png",
	}
	for sourceName, wantName := range tests {
		source := writeFile(t, filepath.Join(t.TempDir(), sourceName), "png bytes")
		added, err := AddImage(deck, source)
		if err != nil {
			t.Fatalf("AddImage(%q) error = %v", sourceName, err)
		}
		if added.Path != filepath.Join("images", wantName) {
			t.Errorf("AddImage(%q).Path = %q, want images/%s", sourceName, added.Path, wantName)
		}
		if _, err := os.Stat(filepath.Join(deckDir, "images", wantName)); err != nil {
			t.Errorf("copied file not found under sanitized name: %v", err)
		}
	}
}

func TestAddImageSanitizingCanClash(t *testing.T) {
	deckDir := t.TempDir()
	deck := writeFile(t, filepath.Join(deckDir, "talk.md"), "# One\n")

	first := writeFile(t, filepath.Join(t.TempDir(), "my diagram.png"), "one")
	if added, err := AddImage(deck, first); err != nil || added.Path != filepath.Join("images", "my-diagram.png") {
		t.Fatalf("AddImage() = (%+v, %v)", added, err)
	}

	second := writeFile(t, filepath.Join(t.TempDir(), "my  diagram.png"), "two")
	added, err := AddImage(deck, second)
	if err != nil {
		t.Fatalf("AddImage() error = %v", err)
	}
	if added.Path != filepath.Join("images", "my-diagram-2.png") {
		t.Errorf("Path = %q, want images/my-diagram-2.png", added.Path)
	}
}

// TestAddImageLeavesOtherCharactersAlone covers characters the reviewer
// checked against goldmark's actual render but that do not need
// sanitizing: "%" and ";" pass through goldmark unchanged in a link
// destination, and a non-Latin name is not touched either.
func TestAddImageLeavesOtherCharactersAlone(t *testing.T) {
	deckDir := t.TempDir()
	deck := writeFile(t, filepath.Join(deckDir, "talk.md"), "# One\n")

	names := []string{"progress 50%.png", "diagram;v2.png", "图表.png"}
	for _, sourceName := range names {
		source := writeFile(t, filepath.Join(t.TempDir(), sourceName), "png bytes")
		added, err := AddImage(deck, source)
		if err != nil {
			t.Fatalf("AddImage(%q) error = %v", sourceName, err)
		}
		wantName := sanitizeForLink(sourceName)
		if sourceName != "progress 50%.png" && added.Path != filepath.Join("images", sourceName) {
			t.Errorf("AddImage(%q).Path = %q, want images/%s unchanged", sourceName, added.Path, sourceName)
		}
		if _, err := os.Stat(filepath.Join(deckDir, "images", wantName)); err != nil {
			t.Errorf("copied file not found under %q: %v", wantName, err)
		}
	}
}

// TestAddImageAlreadyInImagesWithAnUnsafeNameCopiesUnderASafeName covers
// the same-file fast path: a deck whose images folder already holds a
// file named before this sanitizing existed (with a raw space in it)
// must not have that raw name handed back as a link destination.
func TestAddImageAlreadyInImagesWithAnUnsafeNameCopiesUnderASafeName(t *testing.T) {
	deckDir := t.TempDir()
	deck := writeFile(t, filepath.Join(deckDir, "talk.md"), "# One\n")
	source := writeFile(t, filepath.Join(deckDir, "images", "my diagram.png"), "png bytes")

	added, err := AddImage(deck, source)
	if err != nil {
		t.Fatalf("AddImage() error = %v", err)
	}
	if added.Path != filepath.Join("images", "my-diagram.png") {
		t.Errorf("Path = %q, want images/my-diagram.png", added.Path)
	}
	if strings.Contains(added.Markdown, " ") {
		t.Errorf("Markdown = %q, still has a raw space", added.Markdown)
	}
	copied, err := os.ReadFile(filepath.Join(deckDir, "images", "my-diagram.png"))
	if err != nil || string(copied) != "png bytes" {
		t.Errorf("copied file = (%q, %v)", copied, err)
	}
	// The original, unsafely named file is left in place.
	if _, err := os.Stat(filepath.Join(deckDir, "images", "my diagram.png")); err != nil {
		t.Errorf("original file removed: %v", err)
	}
}

func TestImageMarkdown(t *testing.T) {
	tests := map[string]string{
		filepath.Join("images", "diagram.png"):    "![diagram](images/diagram.png)",
		filepath.Join("images", "my-diagram.png"): "![my-diagram](images/my-diagram.png)",
		filepath.Join("images", "chart-v2.png"):   "![chart-v2](images/chart-v2.png)",
		filepath.Join("images", "[draft].png"):    "![draft](images/[draft].png)",
	}
	for path, want := range tests {
		if got := ImageMarkdown(path); got != want {
			t.Errorf("ImageMarkdown(%q) = %q, want %q", path, got, want)
		}
	}
}

func TestEnsureImagesDirWhenImagesIsAFile(t *testing.T) {
	deckDir := t.TempDir()
	deck := writeFile(t, filepath.Join(deckDir, "talk.md"), "# One\n")
	writeFile(t, filepath.Join(deckDir, "images"), "not a folder")
	if _, err := EnsureImagesDir(deck); err == nil {
		t.Error("EnsureImagesDir() should fail when images is a file")
	}
}
