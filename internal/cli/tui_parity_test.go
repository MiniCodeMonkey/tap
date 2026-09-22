package cli

import (
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MiniCodeMonkey/tap/internal/layouts"
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

func TestSlideAddLayoutMatchesTheWizardKey(t *testing.T) {
	for index, template := range layouts.Templates() {
		t.Run(template.Name, func(t *testing.T) {
			tuiDeck, commandDeck := twoDeckCopies(t, "talk.md", "# One\n")

			model := tea.Model(tui.NewDevModel(tui.DevConfig{MarkdownFile: tuiDeck}))
			model, _ = pressKeys(model, runeKey("a"))
			model, _ = pressKeys(model, repeatKey(tea.KeyMsg{Type: tea.KeyDown}, index)...)
			pressKeys(model, tea.KeyMsg{Type: tea.KeyEnter}, tea.KeyMsg{Type: tea.KeyCtrlD})

			if exitCode, _, stderr := runTap(t, "slide", "add", commandDeck, "--layout", template.Name); exitCode != exitOK {
				t.Fatalf("tap slide add exited %d: %s", exitCode, stderr)
			}
			requireSameFolders(t, tuiDeck, commandDeck)
		})
	}
}

func TestImageGenerateMatchesTheImageKey(t *testing.T) {
	useFakeImageGenerator(t)
	tuiDeck, commandDeck := twoDeckCopies(t, "talk.md", "---\ntheme: paper\n---\n\n# One\n\n---\n\n# Two\n\nText\n")

	model := tea.Model(tui.NewDevModel(tui.DevConfig{MarkdownFile: tuiDeck}))
	model, _ = pressKeys(model, runeKey("i"), tea.KeyMsg{Type: tea.KeyDown}, tea.KeyMsg{Type: tea.KeyEnter}, runeKey("a red fox"))
	model, submit := pressKeys(model, tea.KeyMsg{Type: tea.KeyCtrlD})
	deliverCommand(model, submit)

	if exitCode, _, stderr := runTap(t, "image", "generate", commandDeck, "--slide", "2", "--prompt", "a red fox"); exitCode != exitOK {
		t.Fatalf("tap image generate exited %d: %s", exitCode, stderr)
	}
	requireSameFolders(t, tuiDeck, commandDeck)
	if len(folderSnapshot(t, filepath.Dir(tuiDeck))) != 2 {
		t.Error("the TUI path wrote no image: the parity check compared two unchanged folders")
	}
}
