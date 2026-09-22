// Package deckedit changes deck files on disk. The tap commands and the
// terminal UI keys both call it, so each change to a deck has one
// implementation.
package deckedit

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/MiniCodeMonkey/tap/internal/parser"
)

// SlideSeparator starts a new slide at the end of a deck.
const SlideSeparator = "\n---\n\n"

// frontmatterPattern matches YAML frontmatter at the start of a deck.
var frontmatterPattern = regexp.MustCompile(`(?s)^---\n.*?\n---\n?`)

// SlideRangeError reports a slide index outside the deck. Index is
// zero-based; Total is the number of slides.
type SlideRangeError struct {
	Index int
	Total int
}

func (e *SlideRangeError) Error() string {
	return fmt.Sprintf("invalid slide index: %d (have %d slides)", e.Index, e.Total)
}

// splitDeck splits content into its frontmatter and the chunks between
// slide separators. slideParts holds, for each slide in order, the index
// of its chunk in parts. Empty chunks are not slides.
func splitDeck(content string) (frontmatter string, parts []string, slideParts []int) {
	frontmatter = frontmatterPattern.FindString(content)
	parts = parser.SplitSlidesPreservingCodeBlocks(content[len(frontmatter):])
	for index, part := range parts {
		if strings.TrimSpace(part) != "" {
			slideParts = append(slideParts, index)
		}
	}
	return frontmatter, parts, slideParts
}

// SlideBodies returns the text of each slide in content, in order, with
// surrounding whitespace trimmed. The frontmatter and empty chunks between
// separators are not slides. Index 0 is the first slide; every slideIndex
// in this package counts the same way.
func SlideBodies(content string) []string {
	_, parts, slideParts := splitDeck(content)
	bodies := make([]string, len(slideParts))
	for index, partIndex := range slideParts {
		bodies[index] = strings.TrimSpace(parts[partIndex])
	}
	return bodies
}

// InsertIntoSlide returns content with markdown added at the end of the
// slide at slideIndex, after one blank line.
func InsertIntoSlide(content string, slideIndex int, markdown string) (string, error) {
	frontmatter, parts, slideParts := splitDeck(content)
	if slideIndex < 0 || slideIndex >= len(slideParts) {
		return "", &SlideRangeError{Index: slideIndex, Total: len(slideParts)}
	}

	partIndex := slideParts[slideIndex]
	parts[partIndex] = strings.TrimRight(parts[partIndex], " \t\n") + "\n\n" + markdown + "\n"

	var result strings.Builder
	result.WriteString(frontmatter)
	for index, part := range parts {
		result.WriteString(part)
		if index < len(parts)-1 {
			result.WriteString("---\n")
		}
	}
	return result.String(), nil
}

// InsertIntoFile adds markdown at the end of the slide at slideIndex in
// the deck file.
func InsertIntoFile(deckPath string, slideIndex int, markdown string) error {
	content, err := os.ReadFile(deckPath)
	if err != nil {
		return fmt.Errorf("failed to read markdown file: %w", err)
	}
	updated, err := InsertIntoSlide(string(content), slideIndex, markdown)
	if err != nil {
		return err
	}
	if err := os.WriteFile(deckPath, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("failed to write markdown file: %w", err)
	}
	return nil
}

// AppendSlide adds a slide at the end of the deck file: SlideSeparator,
// then slideMarkdown.
func AppendSlide(deckPath, slideMarkdown string) error {
	file, err := os.OpenFile(deckPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	if _, err := file.WriteString(SlideSeparator + slideMarkdown); err != nil {
		_ = file.Close()
		return fmt.Errorf("failed to write to file: %w", err)
	}
	return file.Close()
}
