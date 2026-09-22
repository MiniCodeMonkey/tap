package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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

func TestSlideAddLayoutAppendsTheTemplate(t *testing.T) {
	deck := writeDeckFile(t, t.TempDir(), "talk.md", "# One\n")
	exitCode, stdout, stderr := runTap(t, "slide", "add", deck, "--layout", "big-stat")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}
	if stdout != "Added a big-stat slide to "+deck+"\n" {
		t.Errorf("stdout = %q", stdout)
	}
	content, _ := os.ReadFile(deck)
	want := "# One\n\n---\n\n<!--\nlayout: big-stat\n-->\n\n# 100%\n\nDescription\n"
	if string(content) != want {
		t.Errorf("deck = %q, want %q", content, want)
	}
}

func TestSlideAddLayoutNeedsNoTerminal(t *testing.T) {
	original := stdinIsTerminal
	stdinIsTerminal = func() bool { return false }
	t.Cleanup(func() { stdinIsTerminal = original })

	deck := writeDeckFile(t, t.TempDir(), "talk.md", "# One\n")
	if exitCode, _, stderr := runTap(t, "slide", "add", deck, "--layout", "quote"); exitCode != exitOK {
		t.Errorf("exit code = %d, stderr %q", exitCode, stderr)
	}
}

func TestSlideAddPrintWritesNothing(t *testing.T) {
	dir := t.TempDir()
	deck := writeDeckFile(t, dir, "talk.md", "# One\n")
	exitCode, stdout, _ := runTap(t, "slide", "add", deck, "--layout", "sidebar", "--print")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d", exitCode)
	}
	if !strings.HasPrefix(stdout, "<!--\nlayout: sidebar\n-->") || strings.HasPrefix(stdout, "\n---") {
		t.Errorf("stdout = %q, want the template with no separator", stdout)
	}
	content, _ := os.ReadFile(deck)
	if string(content) != "# One\n" {
		t.Errorf("--print changed the deck to %q", content)
	}
}

func TestSlideAddPrintNeedsNoDeck(t *testing.T) {
	withWorkingDirectory(t, t.TempDir(), func() {
		exitCode, stdout, stderr := runTap(t, "slide", "add", "--layout", "title", "--print")
		if exitCode != exitOK || stdout != "# Title\n" {
			t.Errorf("(%d, %q, %q), want (0, \"# Title\\n\", \"\")", exitCode, stdout, stderr)
		}
	})
}

func TestSlideAddPrintJSON(t *testing.T) {
	exitCode, stdout, _ := runTap(t, "slide", "add", "--layout", "blank", "--print", "--json")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d", exitCode)
	}
	var output map[string]any
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if output["ok"] != true || output["layout"] != "blank" || output["markdown"] != "<!--\nlayout: blank\n-->\n\nContent\n" {
		t.Errorf("output = %v", output)
	}
	if _, hasDeck := output["deck"]; hasDeck {
		t.Errorf("--print --json should have no deck field: %v", output)
	}
}

func TestSlideAddUnknownLayout(t *testing.T) {
	exitCode, stdout, _ := runTap(t, "slide", "add", "--layout", "bogus", "--print", "--json")
	if exitCode != exitUserError || !strings.Contains(stdout, `"code": "unknown_layout"`) || !strings.Contains(stdout, "split-media") {
		t.Errorf("(%d, %q), want exit 1, unknown_layout, and the list of layouts", exitCode, stdout)
	}
}

func TestSlideAddPrintNeedsALayout(t *testing.T) {
	exitCode, _, stderr := runTap(t, "slide", "add", "--print")
	if exitCode != exitUserError || !strings.Contains(stderr, "--print needs --layout") {
		t.Errorf("(%d, %q), want exit 1 and a message about --layout", exitCode, stderr)
	}
}
