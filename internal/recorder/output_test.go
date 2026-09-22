package recorder

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func startTime() time.Time {
	return time.Date(2026, 9, 20, 14, 32, 0, 0, time.UTC)
}

func TestOutputPathUsesSlugAndStartTime(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "recordings")

	got, err := OutputPath(dir, "My Talk: Geocoding!", startTime())
	if err != nil {
		t.Fatalf("OutputPath() returned %v", err)
	}

	want := filepath.Join(dir, "my-talk-geocoding-2026-09-20-1432.mov")
	if got != want {
		t.Errorf("OutputPath() = %q, want %q", got, want)
	}
}

func TestOutputPathCreatesTheDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "recordings")

	if _, err := OutputPath(dir, "Talk", startTime()); err != nil {
		t.Fatalf("OutputPath() returned %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("the output directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("the output path is not a directory")
	}
}

func TestOutputPathNeverOverwrites(t *testing.T) {
	dir := t.TempDir()

	first, err := OutputPath(dir, "Talk", startTime())
	if err != nil {
		t.Fatalf("OutputPath() returned %v", err)
	}
	if err := os.WriteFile(first, []byte("taken"), 0o600); err != nil {
		t.Fatal(err)
	}

	second, err := OutputPath(dir, "Talk", startTime())
	if err != nil {
		t.Fatalf("OutputPath() returned %v", err)
	}

	if second == first {
		t.Fatal("OutputPath() returned a path that already exists")
	}
	if !strings.HasSuffix(second, "-2.mov") {
		t.Errorf("OutputPath() = %q, want a -2 suffix", second)
	}
}

func TestOutputPathFallsBackForAnEmptyTitle(t *testing.T) {
	dir := t.TempDir()

	got, err := OutputPath(dir, "   ", startTime())
	if err != nil {
		t.Fatalf("OutputPath() returned %v", err)
	}

	if filepath.Base(got) != "talk-2026-09-20-1432.mov" {
		t.Errorf("OutputPath() = %q, want a talk- prefix", filepath.Base(got))
	}
}

func TestChapterPathSitsBesideTheMovie(t *testing.T) {
	got := ChapterPath("/tmp/recordings/my-talk-2026-09-20-1432.mov")
	want := "/tmp/recordings/my-talk-2026-09-20-1432.txt"

	if got != want {
		t.Errorf("ChapterPath() = %q, want %q", got, want)
	}
}

func TestRunDirCreatesAFolderNamedLikeARecording(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "recordings")

	dir, err := RunDir(parent, "My Talk", startTime())
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(parent, "my-talk-2026-09-20-1432"); dir != want {
		t.Errorf("RunDir = %q, want %q", dir, want)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		t.Errorf("RunDir did not create %q", dir)
	}
}

func TestRunDirNeverReusesAFolder(t *testing.T) {
	parent := t.TempDir()

	first, err := RunDir(parent, "My Talk", startTime())
	if err != nil {
		t.Fatal(err)
	}
	second, err := RunDir(parent, "My Talk", startTime())
	if err != nil {
		t.Fatal(err)
	}
	if second != first+"-2" {
		t.Errorf("second RunDir = %q, want %q", second, first+"-2")
	}
}
