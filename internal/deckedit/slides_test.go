package deckedit

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlideBodiesSkipsFrontmatterAndEmptySlides(t *testing.T) {
	content := "---\ntitle: Demo\n---\n\n# One\n\n---\n\n---\n\n# Two\n"
	got := SlideBodies(content)
	want := []string{"# One", "# Two"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("SlideBodies() = %q, want %q", got, want)
	}
}

func TestSlideBodiesKeepsSeparatorsInsideCodeBlocks(t *testing.T) {
	content := "# One\n\n```yaml\n---\nkey: value\n```\n\n---\n\n# Two\n"
	if got := SlideBodies(content); len(got) != 2 {
		t.Errorf("SlideBodies() found %d slides, want 2: %q", len(got), got)
	}
}

func TestInsertIntoSlideAddsAtTheEndOfThatSlide(t *testing.T) {
	content := "---\ntitle: Demo\n---\n\n# One\n\nText\n\n---\n\n# Two\n"
	got, err := InsertIntoSlide(content, 0, "![a](images/a.png)")
	if err != nil {
		t.Fatalf("InsertIntoSlide() error = %v", err)
	}
	if !strings.HasPrefix(got, "---\ntitle: Demo\n---\n") {
		t.Errorf("frontmatter lost:\n%s", got)
	}
	if !strings.Contains(got, "Text\n\n![a](images/a.png)\n") {
		t.Errorf("markdown not at the end of slide 1:\n%s", got)
	}
	if strings.Index(got, "![a]") > strings.Index(got, "# Two") {
		t.Errorf("markdown landed after slide 2:\n%s", got)
	}
}

func TestInsertIntoSlideOutOfRange(t *testing.T) {
	_, err := InsertIntoSlide("# One\n", 1, "x")
	var rangeError *SlideRangeError
	if !errors.As(err, &rangeError) {
		t.Fatalf("error = %v, want a *SlideRangeError", err)
	}
	if rangeError.Index != 1 || rangeError.Total != 1 {
		t.Errorf("rangeError = %+v, want Index 1, Total 1", rangeError)
	}
}

func TestInsertIntoFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "talk.md")
	if err := os.WriteFile(path, []byte("# One\n\n---\n\n# Two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := InsertIntoFile(path, 1, "![a](images/a.png)"); err != nil {
		t.Fatalf("InsertIntoFile() error = %v", err)
	}
	content, _ := os.ReadFile(path)
	if !strings.Contains(string(content), "# Two\n\n![a](images/a.png)\n") {
		t.Errorf("file = %q", content)
	}
}

func TestAppendSlideAddsTheSeparatorAndTheSlide(t *testing.T) {
	path := filepath.Join(t.TempDir(), "talk.md")
	if err := os.WriteFile(path, []byte("# One\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendSlide(path, "## Two\n"); err != nil {
		t.Fatalf("AppendSlide() error = %v", err)
	}
	content, _ := os.ReadFile(path)
	if string(content) != "# One\n\n---\n\n## Two\n" {
		t.Errorf("file = %q", content)
	}
}

func TestAppendSlideToAMissingFileFails(t *testing.T) {
	if err := AppendSlide(filepath.Join(t.TempDir(), "missing.md"), "## Two\n"); err == nil {
		t.Error("AppendSlide() on a missing file should fail")
	}
}
