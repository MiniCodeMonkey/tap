package deckedit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/gemini"
)

func pngImage(content string) gemini.ImageResult {
	return gemini.ImageResult{Data: []byte(content), ContentType: "image/png"}
}

func TestPlaceGeneratedImageAddsToTheSlide(t *testing.T) {
	deckDir := t.TempDir()
	deck := writeFile(t, filepath.Join(deckDir, "talk.md"), "---\ntheme: paper\n---\n\n# One\n\n---\n\n# Two\n")

	image := pngImage("new image")
	placed, err := PlaceGeneratedImage(Placement{DeckPath: deck, SlideIndex: 1, Prompt: "a red fox"}, image)
	if err != nil {
		t.Fatalf("PlaceGeneratedImage() error = %v", err)
	}
	wantPath := filepath.Join("images", GenerateImageFilename(image.Data, image.ContentType))
	if placed.Path != wantPath || placed.Markdown != AIImageMarkdown("a red fox", wantPath) {
		t.Errorf("placed = %+v", placed)
	}
	saved, err := os.ReadFile(filepath.Join(deckDir, wantPath))
	if err != nil || string(saved) != "new image" {
		t.Errorf("saved image = (%q, %v)", saved, err)
	}
	content, _ := os.ReadFile(deck)
	if !strings.Contains(string(content), "# Two\n\n"+placed.Markdown+"\n") {
		t.Errorf("deck = %q", content)
	}
	if !strings.HasPrefix(string(content), "---\ntheme: paper\n---\n") {
		t.Errorf("frontmatter lost: %q", content)
	}
}

func TestPlaceGeneratedImageReplacesInPlaceAndDeletesTheOldFile(t *testing.T) {
	deckDir := t.TempDir()
	oldPath := writeFile(t, filepath.Join(deckDir, "images", "generated-old00000.png"), "old image")
	deck := writeFile(t, filepath.Join(deckDir, "talk.md"),
		"# One\n\nBefore\n\n<!-- ai-prompt: a blue whale -->\n![](images/generated-old00000.png)\n\nAfter\n")

	replacing := AIImage{Prompt: "a blue whale", ImagePath: "images/generated-old00000.png"}
	placed, err := PlaceGeneratedImage(Placement{DeckPath: deck, Prompt: "a green whale", Replacing: &replacing}, pngImage("new image"))
	if err != nil {
		t.Fatalf("PlaceGeneratedImage() error = %v", err)
	}
	if placed.DeletedPath != "images/generated-old00000.png" || placed.DeleteError != nil {
		t.Errorf("placed = %+v", placed)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Error("the old image should be deleted")
	}
	content, _ := os.ReadFile(deck)
	want := "# One\n\nBefore\n\n" + placed.Markdown + "\n\nAfter\n"
	if string(content) != want {
		t.Errorf("deck = %q, want %q", content, want)
	}
}

func TestPlaceGeneratedImageKeepsAFileWithTheSameName(t *testing.T) {
	deckDir := t.TempDir()
	image := pngImage("same bytes")
	name := GenerateImageFilename(image.Data, image.ContentType)
	writeFile(t, filepath.Join(deckDir, "images", name), "same bytes")
	deck := writeFile(t, filepath.Join(deckDir, "talk.md"), "# One\n\n<!-- ai-prompt: a cat -->\n![](images/"+name+")\n")

	replacing := AIImage{Prompt: "a cat", ImagePath: "images/" + name}
	placed, err := PlaceGeneratedImage(Placement{DeckPath: deck, Prompt: "a cat", Replacing: &replacing}, image)
	if err != nil {
		t.Fatalf("PlaceGeneratedImage() error = %v", err)
	}
	if placed.DeletedPath != "" {
		t.Errorf("DeletedPath = %q, want nothing deleted", placed.DeletedPath)
	}
	if _, err := os.Stat(filepath.Join(deckDir, "images", name)); err != nil {
		t.Errorf("the new image was deleted: %v", err)
	}
}

func TestPlaceGeneratedImageWritesADollarSignAsTyped(t *testing.T) {
	deckDir := t.TempDir()
	deck := writeFile(t, filepath.Join(deckDir, "talk.md"), "# One\n\n<!-- ai-prompt: old -->\n![](images/old.png)\n")
	replacing := AIImage{Prompt: "old", ImagePath: "images/old.png"}
	if _, err := PlaceGeneratedImage(Placement{DeckPath: deck, Prompt: "costs $1 a day", Replacing: &replacing}, pngImage("x")); err != nil {
		t.Fatal(err)
	}
	content, _ := os.ReadFile(deck)
	if !strings.Contains(string(content), "<!-- ai-prompt: costs $1 a day -->") {
		t.Errorf("deck = %q", content)
	}
}

func TestPlaceGeneratedImageOldFileAlreadyMissingIsNotAnError(t *testing.T) {
	deckDir := t.TempDir()
	deck := writeFile(t, filepath.Join(deckDir, "talk.md"),
		"# One\n\n<!-- ai-prompt: a blue whale -->\n![](images/generated-old00000.png)\n")

	replacing := AIImage{Prompt: "a blue whale", ImagePath: "images/generated-old00000.png"}
	placed, err := PlaceGeneratedImage(Placement{DeckPath: deck, Prompt: "a green whale", Replacing: &replacing}, pngImage("new image"))
	if err != nil {
		t.Fatalf("PlaceGeneratedImage() error = %v", err)
	}
	if placed.DeleteError != nil {
		t.Errorf("DeleteError = %v, want nil for an already-missing file", placed.DeleteError)
	}
}

func TestPlaceGeneratedImageSlideOutOfRange(t *testing.T) {
	deck := writeFile(t, filepath.Join(t.TempDir(), "talk.md"), "# One\n")
	if _, err := PlaceGeneratedImage(Placement{DeckPath: deck, SlideIndex: 4, Prompt: "x"}, pngImage("x")); err == nil {
		t.Error("PlaceGeneratedImage() on a missing slide should fail")
	}
}

func TestSaveImageNamesTheFileByItsContent(t *testing.T) {
	deckDir := t.TempDir()
	deck := writeFile(t, filepath.Join(deckDir, "talk.md"), "# One\n")
	image := gemini.ImageResult{Data: []byte("jpeg bytes"), ContentType: "image/jpeg"}
	path, err := SaveImage(deck, image)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(path, filepath.Join("images", "generated-")) || !strings.HasSuffix(path, ".jpg") {
		t.Errorf("path = %q", path)
	}
	info, err := os.Stat(filepath.Join(deckDir, path))
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Errorf("saved file = (%v, %v), want mode 0644", info, err)
	}
}
