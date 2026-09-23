package deckedit

import (
	"errors"
	"fmt"
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

func TestInsertIntoFileLeavesTheDeckUnchangedWhenTheWriteFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, which ignores directory permissions")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "talk.md")
	content := "# One\n\n---\n\n# Two\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// A read-only directory stops the temp file InsertIntoFile creates next
	// to the deck, simulating a write failure partway through.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("Failed to chmod dir: %v", err)
	}
	defer os.Chmod(dir, 0o755)

	if err := InsertIntoFile(path, 1, "![a](images/a.png)"); err == nil {
		t.Fatal("InsertIntoFile() returned no error, want an error from the read-only directory")
	}

	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("Failed to restore dir permissions: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read deck file: %v", err)
	}
	if string(got) != content {
		t.Errorf("deck file changed after a failed write:\ngot:  %q\nwant: %q", got, content)
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

func TestAppendSlideFollowsASymlinkToUpdateTheRealFile(t *testing.T) {
	dir := t.TempDir()
	realDir := filepath.Join(dir, "real")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	realPath := filepath.Join(realDir, "talk.md")
	if err := os.WriteFile(realPath, []byte("# One\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	linkDir := filepath.Join(dir, "link")
	if err := os.Mkdir(linkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(linkDir, "talk.md")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Fatal(err)
	}

	if err := AppendSlide(linkPath, "## Two\n"); err != nil {
		t.Fatalf("AppendSlide() error = %v", err)
	}

	linkInfo, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatal(err)
	}
	if linkInfo.Mode()&os.ModeSymlink == 0 {
		t.Fatal("AppendSlide() replaced the symlink with a regular file")
	}
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatal(err)
	}
	if target != realPath {
		t.Errorf("symlink now points to %q, want %q", target, realPath)
	}

	content, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "# One\n\n---\n\n## Two\n" {
		t.Errorf("real file = %q", content)
	}
}

func TestAppendSlideLeavesTheDeckUnchangedWhenTheWriteFails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, which ignores directory permissions")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "talk.md")
	content := "# One\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// A read-only directory stops the temp file AppendSlide creates next to
	// the deck, simulating a write failure partway through.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("Failed to chmod dir: %v", err)
	}
	defer os.Chmod(dir, 0o755)

	if err := AppendSlide(path, "## Two\n"); err == nil {
		t.Fatal("AppendSlide() returned no error, want an error from the read-only directory")
	}

	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatalf("Failed to restore dir permissions: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read deck file: %v", err)
	}
	if string(got) != content {
		t.Errorf("deck file changed after a failed append:\ngot:  %q\nwant: %q", got, content)
	}
}

func TestInsertIntoSlideTwiceKeepsAllSlideBoundaries(t *testing.T) {
	content := "# One\n\n---\n\n# Two\n\n---\n\n# Three\n"

	afterFirst, err := InsertIntoSlide(content, 0, "![d](images/d.png)")
	if err != nil {
		t.Fatalf("first InsertIntoSlide() error = %v", err)
	}
	if got := SlideBodies(afterFirst); len(got) != 3 {
		t.Fatalf("after first insert: %d slides, want 3: %q", len(got), got)
	}

	afterSecond, err := InsertIntoSlide(afterFirst, 1, "![d-2](images/d-2.png)")
	if err != nil {
		t.Fatalf("second InsertIntoSlide() error = %v", err)
	}

	bodies := SlideBodies(afterSecond)
	if len(bodies) != 3 {
		t.Fatalf("after second insert: %d slides, want 3: %q", len(bodies), bodies)
	}
	if !strings.Contains(bodies[0], "# One") || !strings.Contains(bodies[0], "![d](images/d.png)") {
		t.Errorf("slide 0 = %q, want it to keep # One and the first image", bodies[0])
	}
	if !strings.Contains(bodies[1], "# Two") || !strings.Contains(bodies[1], "![d-2](images/d-2.png)") {
		t.Errorf("slide 1 = %q, want it to keep # Two and the second image", bodies[1])
	}
	if bodies[2] != "# Three" {
		t.Errorf("slide 2 = %q, want %q untouched", bodies[2], "# Three")
	}
}

func TestInsertIntoSlideIsIdempotentOverManyEdits(t *testing.T) {
	content := "# One\n\n---\n\n# Two\n\n---\n\n# Three\n"
	current := content
	for round := 0; round < 10; round++ {
		slideIndex := round % 3
		var err error
		current, err = InsertIntoSlide(current, slideIndex, fmt.Sprintf("![r%d](images/r%d.png)", round, round))
		if err != nil {
			t.Fatalf("round %d: InsertIntoSlide() error = %v", round, err)
		}
		bodies := SlideBodies(current)
		if len(bodies) != 3 {
			t.Fatalf("round %d: %d slides, want 3: %q", round, len(bodies), bodies)
		}
	}
}

func TestInsertIntoSlideLeavesUntouchedSlidesByteIdentical(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		slideIndex   int
		markdown     string
		wantContains []string
		wantSlides   int
	}{
		{
			name:         "no trailing newline",
			content:      "# One\n\n---\n\n# Two",
			slideIndex:   1,
			markdown:     "![a](images/a.png)",
			wantContains: []string{"# One\n\n---\n\n"},
			wantSlides:   2,
		},
		{
			name:         "separators already use different spacing",
			content:      "# One\n\n---   \n\n# Two\n\n---\n\n# Three\n",
			slideIndex:   0,
			markdown:     "![a](images/a.png)",
			wantContains: []string{"---   \n\n# Two"},
			wantSlides:   3,
		},
		{
			name:         "separator inside a fenced code block is not a boundary",
			content:      "# One\n\n```yaml\n---\nkey: value\n```\n\n---\n\n# Two\n",
			slideIndex:   1,
			markdown:     "![a](images/a.png)",
			wantContains: []string{"```yaml\n---\nkey: value\n```"},
			wantSlides:   2,
		},
		{
			name:         "insertion into the first slide",
			content:      "# One\n\n---\n\n# Two\n\n---\n\n# Three\n",
			slideIndex:   0,
			markdown:     "![a](images/a.png)",
			wantContains: []string{"# Three\n"},
			wantSlides:   3,
		},
		{
			name:         "insertion into the last slide",
			content:      "# One\n\n---\n\n# Two\n\n---\n\n# Three\n",
			slideIndex:   2,
			markdown:     "![a](images/a.png)",
			wantContains: []string{"# One\n\n---\n\n# Two\n\n---\n\n"},
			wantSlides:   3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := InsertIntoSlide(tt.content, tt.slideIndex, tt.markdown)
			if err != nil {
				t.Fatalf("InsertIntoSlide() error = %v", err)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(got, want) {
					t.Errorf("InsertIntoSlide() = %q, want it to contain the untouched text %q", got, want)
				}
			}
			if bodies := SlideBodies(got); len(bodies) != tt.wantSlides {
				t.Errorf("InsertIntoSlide() produced %d slides, want %d: %q", len(bodies), tt.wantSlides, bodies)
			}
		})
	}
}

func TestAppendSlidePreservesThePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "talk.md")
	if err := os.WriteFile(path, []byte("# One\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := AppendSlide(path, "## Two\n"); err != nil {
		t.Fatalf("AppendSlide() error = %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("permissions = %v, want 0600", info.Mode().Perm())
	}
}
