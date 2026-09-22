package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resetNewFlags clears the package-level flag variables the new command
// tests mutate, so tests don't leak state into each other.
func resetNewFlags() {
	newTitle = ""
	newTheme = ""
	newOutput = ""
	newYes = false
	newForce = false
	newJSON = false
}

func TestNewRejectsADeckArgumentAndOutputTogether(t *testing.T) {
	exitCode, _, stderr := runTap(t, "new", "a.md", "--output", "b.md", "--yes")
	if exitCode != exitUserError {
		t.Errorf("exit code = %d, want %d (stderr %q)", exitCode, exitUserError, stderr)
	}
}

func TestNewDeckArgumentIsTheOutputPath(t *testing.T) {
	dir := t.TempDir()
	withWorkingDirectory(t, dir, func() {
		exitCode, _, stderr := runTap(t, "new", "talk.md", "--yes")
		if exitCode != exitOK {
			t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
		}
	})
	if _, err := os.Stat(filepath.Join(dir, "talk.md")); err != nil {
		t.Errorf("talk.md was not written: %v", err)
	}
}

func TestRunNewNonInteractiveDefaults(t *testing.T) {
	dir := t.TempDir()
	resetNewFlags()

	withWorkingDirectory(t, dir, func() {
		if err := runNewNonInteractive(); err != nil {
			t.Fatalf("runNewNonInteractive: %v", err)
		}
	})

	// Defaults: title "My Presentation" -> filename "my-presentation.md",
	// theme the wizard's first (default) theme.
	outputPath := filepath.Join(dir, "my-presentation.md")
	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("expected %s to be written: %v", outputPath, err)
	}
	if !strings.Contains(string(content), `title: "My Presentation"`) {
		t.Errorf("expected default title in content, got:\n%s", content)
	}
}

func TestRunNewNonInteractiveFlags(t *testing.T) {
	dir := t.TempDir()
	resetNewFlags()
	newTitle = "My Talk"
	newTheme = "terminal"
	newOutput = filepath.Join(dir, "talk.md")

	if err := runNewNonInteractive(); err != nil {
		t.Fatalf("runNewNonInteractive: %v", err)
	}

	content, err := os.ReadFile(newOutput)
	if err != nil {
		t.Fatalf("expected %s to be written: %v", newOutput, err)
	}
	if !strings.Contains(string(content), `title: "My Talk"`) {
		t.Errorf("expected title \"My Talk\" in content, got:\n%s", content)
	}
	if !strings.Contains(string(content), "theme: terminal") {
		t.Errorf("expected theme terminal in content, got:\n%s", content)
	}
}

func TestRunNewNonInteractiveAppendsMdSuffix(t *testing.T) {
	dir := t.TempDir()
	resetNewFlags()
	newOutput = filepath.Join(dir, "talk")

	if err := runNewNonInteractive(); err != nil {
		t.Fatalf("runNewNonInteractive: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "talk.md")); err != nil {
		t.Fatalf("expected talk.md to be written: %v", err)
	}
}

func TestRunNewNonInteractiveUnknownTheme(t *testing.T) {
	dir := t.TempDir()
	resetNewFlags()
	newOutput = filepath.Join(dir, "talk.md")
	newTheme = "not-a-real-theme"

	err := runNewNonInteractive()
	if err == nil {
		t.Fatal("expected an error for an unknown theme, got nil")
	}
	if !strings.Contains(err.Error(), "unknown theme") {
		t.Errorf("expected an unknown theme error, got: %v", err)
	}
}

func TestRunNewNonInteractiveRefusesToOverwrite(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "talk.md")
	if err := os.WriteFile(outputPath, []byte("existing content"), 0o644); err != nil {
		t.Fatalf("failed to seed existing file: %v", err)
	}

	resetNewFlags()
	newOutput = outputPath

	err := runNewNonInteractive()
	if err == nil {
		t.Fatal("expected an error refusing to overwrite an existing file, got nil")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Errorf("expected the error to name --force, got: %v", err)
	}

	content, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		t.Fatalf("failed to read %s: %v", outputPath, readErr)
	}
	if string(content) != "existing content" {
		t.Errorf("expected the existing file to be left alone, got:\n%s", content)
	}
}

func TestRunNewNonInteractiveForceOverwrites(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "talk.md")
	if err := os.WriteFile(outputPath, []byte("existing content"), 0o644); err != nil {
		t.Fatalf("failed to seed existing file: %v", err)
	}

	resetNewFlags()
	newOutput = outputPath
	newForce = true

	if err := runNewNonInteractive(); err != nil {
		t.Fatalf("runNewNonInteractive: %v", err)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read %s: %v", outputPath, err)
	}
	if strings.Contains(string(content), "existing content") {
		t.Errorf("expected the file to be overwritten, got:\n%s", content)
	}
}
