package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFileAtomicallyReplacesTheContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "deck.md")
	if err := os.WriteFile(path, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}

	if err := WriteFileAtomically(path, []byte("new"), 0o640); err != nil {
		t.Fatalf("WriteFileAtomically() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("file = %q, want %q", got, "new")
	}
}

func TestWriteFileAtomicallyFollowsASymlinkToUpdateTheRealFile(t *testing.T) {
	dir := t.TempDir()
	realDir := filepath.Join(dir, "real")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	realPath := filepath.Join(realDir, "deck.md")
	if err := os.WriteFile(realPath, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}

	linkDir := filepath.Join(dir, "link")
	if err := os.Mkdir(linkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(linkDir, "deck.md")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(linkPath)
	if err != nil {
		t.Fatal(err)
	}

	if err := WriteFileAtomically(linkPath, []byte("new"), info.Mode().Perm()); err != nil {
		t.Fatalf("WriteFileAtomically() error = %v", err)
	}

	linkInfo, err := os.Lstat(linkPath)
	if err != nil {
		t.Fatal(err)
	}
	if linkInfo.Mode()&os.ModeSymlink == 0 {
		t.Fatal("WriteFileAtomically() replaced the symlink with a regular file")
	}
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatal(err)
	}
	if target != realPath {
		t.Errorf("symlink now points to %q, want %q", target, realPath)
	}

	got, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("real file = %q, want %q", got, "new")
	}
}

func TestWriteFileAtomicallyFollowsAChainOfSymlinks(t *testing.T) {
	dir := t.TempDir()
	realPath := filepath.Join(dir, "deck.md")
	if err := os.WriteFile(realPath, []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}

	midPath := filepath.Join(dir, "mid.md")
	if err := os.Symlink(realPath, midPath); err != nil {
		t.Fatal(err)
	}
	outerPath := filepath.Join(dir, "outer.md")
	if err := os.Symlink(midPath, outerPath); err != nil {
		t.Fatal(err)
	}

	if err := WriteFileAtomically(outerPath, []byte("new"), 0o640); err != nil {
		t.Fatalf("WriteFileAtomically() error = %v", err)
	}

	for _, path := range []string{midPath, outerPath} {
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatalf("Lstat(%q) error = %v", path, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Errorf("%q is no longer a symlink", path)
		}
	}

	got, err := os.ReadFile(realPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("real file = %q, want %q", got, "new")
	}
}

func TestWriteFileAtomicallyOnABrokenSymlinkFailsWithoutCreatingAnything(t *testing.T) {
	dir := t.TempDir()
	linkPath := filepath.Join(dir, "deck.md")
	missingTarget := filepath.Join(dir, "missing.md")
	if err := os.Symlink(missingTarget, linkPath); err != nil {
		t.Fatal(err)
	}

	if err := WriteFileAtomically(linkPath, []byte("new"), 0o644); err == nil {
		t.Fatal("WriteFileAtomically() returned no error for a broken symlink")
	}

	if _, err := os.Lstat(missingTarget); !os.IsNotExist(err) {
		t.Errorf("target file was created at %q", missingTarget)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != "deck.md" {
			t.Errorf("stray file left behind: %s", entry.Name())
		}
	}
}

func TestWriteFileAtomicallyRemovesTheTempFileWhenTheRenameFails(t *testing.T) {
	dir := t.TempDir()
	// A directory can never be the destination of a rename from a regular
	// file, so this forces the rename itself to fail after the temp file
	// has already been created, written and chmoded.
	targetDir := filepath.Join(dir, "not-a-file")
	if err := os.Mkdir(targetDir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := WriteFileAtomically(targetDir, []byte("content"), 0o644); err == nil {
		t.Fatal("WriteFileAtomically() returned no error, want an error renaming onto a directory")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if entry.Name() != "not-a-file" {
			t.Errorf("stray temp file left behind: %s", entry.Name())
		}
	}
}
