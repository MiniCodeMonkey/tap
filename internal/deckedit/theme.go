package deckedit

import (
	"errors"
	"fmt"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/themes"
)

// ErrUnknownTheme means a slug names no built-in theme.
var ErrUnknownTheme = errors.New("unknown theme")

// SetTheme writes "theme: <slug>" into the deck's frontmatter, adding the
// frontmatter when the deck has none. An unknown slug leaves the file
// alone and returns an error that wraps ErrUnknownTheme.
func SetTheme(deckPath, slug string) error {
	if !themes.IsValid(slug) {
		return fmt.Errorf("%w %q", ErrUnknownTheme, slug)
	}
	return config.UpdateThemeInFile(deckPath, slug)
}
