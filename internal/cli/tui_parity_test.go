package cli

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MiniCodeMonkey/tap/internal/tui"
)

// themeIndex is the position of slug in the TUI theme picker.
func themeIndex(t *testing.T, slug string) int {
	t.Helper()
	for index, theme := range tui.AvailableThemes {
		if theme.Name == slug {
			return index
		}
	}
	t.Fatalf("theme %q is not in the picker", slug)
	return -1
}

func TestThemeSetMatchesTheThemePickerKey(t *testing.T) {
	target := tui.AvailableThemes[len(tui.AvailableThemes)-1].Name
	if target == "base" {
		target = tui.AvailableThemes[0].Name
	}
	tuiDeck, commandDeck := twoDeckCopies(t, "talk.md", themeSetDeck)

	model := tea.Model(tui.NewDevModel(tui.DevConfig{MarkdownFile: tuiDeck, CurrentTheme: "base"}))
	model, _ = pressKeys(model, runeKey("t"))
	steps := themeIndex(t, target) - themeIndex(t, "base")
	model, _ = pressKeys(model, repeatKey(tea.KeyMsg{Type: tea.KeyDown}, steps)...)
	model, _ = pressKeys(model, repeatKey(tea.KeyMsg{Type: tea.KeyUp}, -steps)...)
	pressKeys(model, tea.KeyMsg{Type: tea.KeyEnter})

	if exitCode, _, stderr := runTap(t, "theme", "set", target, commandDeck); exitCode != exitOK {
		t.Fatalf("tap theme set exited %d: %s", exitCode, stderr)
	}
	requireSameFolders(t, tuiDeck, commandDeck)
}
