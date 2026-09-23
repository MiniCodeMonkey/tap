package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

const themeSetDeck = "---\ntitle: Demo\ntheme: base\n---\n\n# One\n"

func TestThemeSetWritesTheTheme(t *testing.T) {
	deck := writeDeckFile(t, t.TempDir(), "talk.md", themeSetDeck)
	exitCode, stdout, stderr := runTap(t, "theme", "set", "terminal", deck)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}
	if stdout != "Theme set to terminal in "+deck+"\n" {
		t.Errorf("stdout = %q", stdout)
	}
	content, _ := os.ReadFile(deck)
	if !strings.Contains(string(content), "\ntheme: terminal\n") {
		t.Errorf("deck = %q", content)
	}
}

func TestThemeSetJSON(t *testing.T) {
	deck := writeDeckFile(t, t.TempDir(), "talk.md", themeSetDeck)
	exitCode, stdout, _ := runTap(t, "theme", "set", "terminal", deck, "--json")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d", exitCode)
	}
	var output struct {
		OK    bool   `json:"ok"`
		Deck  string `json:"deck"`
		Theme string `json:"theme"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if !output.OK || output.Deck != deck || output.Theme != "terminal" {
		t.Errorf("output = %+v", output)
	}
}

func TestThemeSetUnknownThemeListsTheThemes(t *testing.T) {
	deck := writeDeckFile(t, t.TempDir(), "talk.md", themeSetDeck)
	exitCode, stdout, _ := runTap(t, "theme", "set", "no-such-theme", deck, "--json")
	if exitCode != exitUserError {
		t.Errorf("exit code = %d, want %d", exitCode, exitUserError)
	}
	if !strings.Contains(stdout, `"code": "unknown_theme"`) || !strings.Contains(stdout, "terminal") {
		t.Errorf("stdout = %q, want unknown_theme and the list of themes", stdout)
	}
	content, _ := os.ReadFile(deck)
	if string(content) != themeSetDeck {
		t.Errorf("deck changed to %q", content)
	}
}

func TestThemeSetUsesTheDeckInTheCurrentFolder(t *testing.T) {
	dir := t.TempDir()
	writeDeckFile(t, dir, "talk.md", themeSetDeck)
	withWorkingDirectory(t, dir, func() {
		if exitCode, _, stderr := runTap(t, "theme", "set", "terminal"); exitCode != exitOK {
			t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
		}
		content, _ := os.ReadFile("talk.md")
		if !strings.Contains(string(content), "\ntheme: terminal\n") {
			t.Errorf("deck = %q", content)
		}
	})
}
