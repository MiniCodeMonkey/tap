package cli

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/themes"
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

// The desktop app's theme grid has a Default cell: picking it removes the
// theme line, so the deck renders with tap's default, the state a deck
// that never named a theme is in.
// Each phase runs in its own subtest so runTap's flag reset, which fires
// on subtest cleanup, happens between them; otherwise --json from the
// first phase would leak into the plain-output check at the end (pflag
// leaves an unset bool flag at whatever a previous parse left it at).
func TestThemeSetDefaultRemovesTheTheme(t *testing.T) {
	deck := writeDeckFile(t, t.TempDir(), "talk.md", "---\ntitle: Demo\ntheme: terminal\ntransition: fade\n---\n\n# One\n")

	t.Run("json", func(t *testing.T) {
		exitCode, stdout, stderr := runTap(t, "theme", "set", "default", deck, "--json")
		if exitCode != exitOK {
			t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
		}
		var output struct {
			OK    bool   `json:"ok"`
			Deck  string `json:"deck"`
			Theme string `json:"theme"`
		}
		if err := json.Unmarshal([]byte(stdout), &output); err != nil || !output.OK || output.Theme != "default" || output.Deck != deck {
			t.Errorf("output = %+v (%v)", output, err)
		}
		content, _ := os.ReadFile(deck)
		if string(content) != "---\ntitle: Demo\ntransition: fade\n---\n\n# One\n" {
			t.Errorf("deck = %q, want the theme line gone and nothing else changed", content)
		}
	})

	// A deck with no theme line, and one with no frontmatter, are left as they are.
	for _, text := range []string{"---\ntitle: Demo\n---\n\n# One\n", "# One\n"} {
		text := text
		t.Run(text, func(t *testing.T) {
			plain := writeDeckFile(t, t.TempDir(), "talk.md", text)
			if exitCode, _, stderr := runTap(t, "theme", "set", "default", plain); exitCode != exitOK {
				t.Fatalf("exit %d, stderr %q", exitCode, stderr)
			}
			if after, _ := os.ReadFile(plain); string(after) != text {
				t.Errorf("deck = %q, want %q", after, text)
			}
		})
	}

	t.Run("plain output", func(t *testing.T) {
		exitCode, stdout, _ := runTap(t, "theme", "set", "default", deck)
		if exitCode != exitOK || stdout != "Theme set to tap's default in "+deck+"\n" {
			t.Errorf("plain output = %q", stdout)
		}
	})
}

func TestDefaultIsNotAThemeSlug(t *testing.T) {
	if themes.IsValid("default") {
		t.Fatal("a theme is named default: tap theme set default could not mean tap's default")
	}
}
