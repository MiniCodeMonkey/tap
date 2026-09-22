package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// WriteFileAtomically writes content to a new temporary file, then renames
// it into place, so a crash, a full disk or a killed process mid-write
// leaves path holding either its old content or its new content, never a
// mix of both.
//
// path is resolved through any symlinks first, so a deck reached through a
// symlink (or a chain of them) is written by replacing the real file the
// link points at, not the link itself: the temporary file is created in
// the real file's directory, because a rename across filesystems is not
// atomic, and the rename replaces the real file, leaving the symlink in
// place. A broken symlink fails this call with an error and creates
// nothing.
//
// perm is applied to the temporary file before the rename, so path keeps
// its existing permissions; the caller reads them (with os.Stat, which
// itself follows symlinks) before deciding on new content to write.
func WriteFileAtomically(path string, content []byte, perm os.FileMode) error {
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return fmt.Errorf("failed to resolve file path: %w", err)
	}

	dir := filepath.Dir(target)
	tempFile, err := os.CreateTemp(dir, ".tap-*.tmp")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	removeTemp := true
	defer func() {
		if removeTemp {
			os.Remove(tempPath)
		}
	}()

	if _, err := tempFile.Write(content); err != nil {
		tempFile.Close()
		return fmt.Errorf("failed to write temp file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("failed to close temp file: %w", err)
	}
	if err := os.Chmod(tempPath, perm); err != nil {
		return fmt.Errorf("failed to set file permissions: %w", err)
	}
	if err := os.Rename(tempPath, target); err != nil {
		return fmt.Errorf("failed to replace file: %w", err)
	}
	removeTemp = false
	return nil
}
