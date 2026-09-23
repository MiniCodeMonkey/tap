package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/parser"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
	"github.com/MiniCodeMonkey/tap/internal/tui"
	"github.com/MiniCodeMonkey/tap/internal/usersettings"
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

func TestNewApprovesTheDeckItWrites(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	dir := t.TempDir()

	var deckPath string
	withWorkingDirectory(t, dir, func() {
		exitCode, _, stderr := runTap(t, "new", "talk.md", "--yes")
		if exitCode != exitOK {
			t.Fatalf("exit %d, stderr %q", exitCode, stderr)
		}
		deckPath, _ = filepath.Abs("talk.md")
	})

	settings, err := usersettings.Load(filepath.Join(configHome, "tap", "settings.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	key, err := usersettings.ResolveDeck(deckPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, found := settings.ApprovalFor(key); !found {
		t.Errorf("no approval for %s: %+v", deckPath, settings.Approvals)
	}
}

func TestNewJSONStaysJSONWhenTheApprovalCannotBeSaved(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	if err := os.MkdirAll(filepath.Join(configHome, "tap"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configHome, "tap", "settings.yaml"), []byte("approvals: [not: valid"), 0o600); err != nil {
		t.Fatal(err)
	}

	withWorkingDirectory(t, t.TempDir(), func() {
		exitCode, stdout, stderr := runTap(t, "new", "talk.md", "--json")
		if exitCode != exitOK || !strings.HasPrefix(stdout, "{") {
			t.Errorf("exit %d, stdout %q", exitCode, stdout)
		}
		if !strings.Contains(stderr, "could not record the live code approval") {
			t.Errorf("stderr = %q, want the warning", stderr)
		}
	})
}

func TestStarterDeclaresEveryDriverItUses(t *testing.T) {
	content := tui.GenerateStarterMarkdown("Title", tui.DefaultTheme(), "2026-09-22", "Author")
	deckPath := filepath.Join(t.TempDir(), "starter.md")
	if err := os.WriteFile(deckPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(deckPath)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parser.New().Parse([]byte(content))
	if err != nil {
		t.Fatal(err)
	}
	for _, slide := range transformer.New(cfg).Transform(parsed).Slides {
		for _, block := range slide.CodeBlocks {
			if block.Problem != "" {
				t.Errorf("slide %d: %s", slide.Index+1, block.Problem)
			}
		}
	}
}

func TestRunNewNonInteractiveDefaults(t *testing.T) {
	dir := t.TempDir()
	resetNewFlags()

	withWorkingDirectory(t, dir, func() {
		if err := runNewNonInteractive(newCmd); err != nil {
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

	if err := runNewNonInteractive(newCmd); err != nil {
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

	if err := runNewNonInteractive(newCmd); err != nil {
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

	err := runNewNonInteractive(newCmd)
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

	err := runNewNonInteractive(newCmd)
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

	if err := runNewNonInteractive(newCmd); err != nil {
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
