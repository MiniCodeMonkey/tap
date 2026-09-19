package parser

import (
	"fmt"
	"regexp"
	"strings"
)

// DefaultSlot is the name of the slot that holds content before the first marker.
const DefaultSlot = "default"

// slotMarkerPattern matches a slot marker line such as "::caption".
var slotMarkerPattern = regexp.MustCompile(`^::([a-z][a-z0-9-]*)\s*$`)

// slotSection is the raw markdown of one slot. StartLine is the 1-based
// line, within the content passed to splitSlots, where Content begins
// (leading blank lines already skipped), so a caller can turn a local
// line number inside Content into a real line number in the deck file.
type slotSection struct {
	Name      string
	Content   string
	StartLine int
}

// splitSlots splits slide markdown into slot sections at marker lines.
// Marker lines inside fenced code blocks are content. Empty sections are dropped.
func splitSlots(content string) ([]slotSection, error) {
	lines := strings.Split(content, "\n")
	sections := []slotSection{}
	seen := map[string]bool{}
	currentName := DefaultSlot
	var currentLines []string
	fenceLength := 0
	sectionStartLine := 1

	flush := func() {
		joined := strings.Join(currentLines, "\n")
		text := strings.TrimSpace(joined)
		if text != "" {
			sections = append(sections, slotSection{
				Name:      currentName,
				Content:   text,
				StartLine: sectionStartLine + leadingWhitespaceLines(joined),
			})
		}
		currentLines = nil
	}

	for lineIndex, line := range lines {
		backticks := countLeadingBackticks(line)
		if fenceLength == 0 && backticks >= 3 {
			fenceLength = backticks
		} else if fenceLength > 0 && backticks >= fenceLength {
			fenceLength = 0
		} else if fenceLength == 0 {
			if match := slotMarkerPattern.FindStringSubmatch(line); match != nil {
				flush()
				name := match[1]
				if seen[name] {
					return nil, fmt.Errorf("duplicate slot %q on line %d", name, lineIndex+1)
				}
				seen[name] = true
				currentName = name
				sectionStartLine = lineIndex + 2
				continue
			}
		}
		currentLines = append(currentLines, line)
	}
	flush()
	return sections, nil
}

// pausePart is one piece of a slot's content split at pause markers.
// StartLine is the 1-based line, within the content passed to
// splitOnPauseWithLines, where Text begins.
type pausePart struct {
	Text      string
	StartLine int
}

// splitOnPauseWithLines splits content at pause markers the same way
// pausePattern.Split(content, -1) does, but also returns each piece's
// starting line within content, so a caller can turn a local line number
// inside a piece into a real line number in the deck file.
func splitOnPauseWithLines(content string) []pausePart {
	matches := pausePattern.FindAllStringIndex(content, -1)
	parts := make([]pausePart, 0, len(matches)+1)
	previousEnd := 0
	startLine := 1
	for _, match := range matches {
		parts = append(parts, pausePart{Text: content[previousEnd:match[0]], StartLine: startLine})
		startLine += strings.Count(content[previousEnd:match[1]], "\n")
		previousEnd = match[1]
	}
	parts = append(parts, pausePart{Text: content[previousEnd:], StartLine: startLine})
	return parts
}

// leadingWhitespaceLines counts the newlines inside the leading whitespace
// strings.TrimSpace would remove from text, so a caller who already knows
// the file line text's first byte sits on can compute the file line of the
// first byte of strings.TrimSpace(text).
func leadingWhitespaceLines(text string) int {
	trimmed := strings.TrimLeft(text, " \t\n\r\v\f")
	return strings.Count(text[:len(text)-len(trimmed)], "\n")
}

// renderSlot renders one slot to HTML. Content after each pause marker is
// wrapped in a fragment element. nextFragmentIndex, nextCodeBlockIndex, and
// nextComponentIndex are the first indices to use (threaded the same way
// across slots and fragments within a slide), and the function returns the
// next free fragment index, the next free code block index, the next free
// component index, and the CodeBlock and Component values found in this
// slot in document order. chunkStartLine is the real deck file line that
// content's first line sits on; lineNumbersExact says whether that mapping
// still holds after every transform applied to content upstream (a
// mid-content rewrite, such as notes comment extraction or the asciinema
// metadata rewrite, can shift line numbers in a way this package does not
// track, in which case a component props error falls back to naming the
// file instead of a line).
func (p *Parser) renderSlot(content string, nextFragmentIndex int, nextCodeBlockIndex int, nextComponentIndex int, chunkStartLine int, lineNumbersExact bool) (string, int, int, int, []CodeBlock, []Component, error) {
	parts := splitOnPauseWithLines(content)
	var builder strings.Builder
	var codeBlocks []CodeBlock
	var comps []Component
	for partIndex, part := range parts {
		trimmed := strings.TrimSpace(part.Text)
		if trimmed == "" {
			continue
		}
		partFileStartLine := chunkStartLine + (part.StartLine - 1) + leadingWhitespaceLines(part.Text)
		html, updatedCodeBlockIndex, updatedComponentIndex, blocks, partComps, err := p.renderHTMLWithCodeBlocks([]byte(trimmed), nextCodeBlockIndex, nextComponentIndex, partFileStartLine, lineNumbersExact)
		if err != nil {
			return "", nextFragmentIndex, nextCodeBlockIndex, nextComponentIndex, nil, nil, err
		}
		nextCodeBlockIndex = updatedCodeBlockIndex
		nextComponentIndex = updatedComponentIndex
		codeBlocks = append(codeBlocks, blocks...)
		comps = append(comps, partComps...)
		if partIndex == 0 {
			builder.WriteString(html)
			continue
		}
		builder.WriteString(`<div class="fragment fragment-hidden" data-fragment-index="` + intToString(nextFragmentIndex) + `">`)
		builder.WriteString(html)
		builder.WriteString(`</div>`)
		nextFragmentIndex++
	}
	return builder.String(), nextFragmentIndex, nextCodeBlockIndex, nextComponentIndex, codeBlocks, comps, nil
}
