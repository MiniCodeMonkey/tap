package cli

import (
	"path/filepath"
	"testing"
)

func TestSlideAddCommandShape(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"slide", "add"})
	if err != nil || command.Name() != "add" || command.Parent().Name() != "slide" {
		t.Fatalf("tap slide add not found: %v", err)
	}
	if command.Use != "add [deck]" {
		t.Errorf("Use = %q, want %q", command.Use, "add [deck]")
	}
}

func TestSlideAddWithoutATerminalIsAUserError(t *testing.T) {
	original := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() { stdinIsTerminal = original })

	dir := t.TempDir()
	writeDecks(t, dir, "talk.md")
	exitCode, _, stderr := runTap(t, "slide", "add", filepath.Join(dir, "talk.md"))
	if exitCode != exitUserError {
		t.Errorf("exit code = %d, want %d (stderr %q)", exitCode, exitUserError, stderr)
	}
}
