package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/MiniCodeMonkey/tap/internal/slidelist"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

// liveBlockReference names a live code block the way /api/execute does:
// the slide number and the block's number among the slide's live blocks.
type liveBlockReference struct {
	slide int
	block int
}

// undeclaredDriverWarnings returns one line per live code block whose
// driver the deck does not declare: the deck file, the block's line, and
// the message the page shows in the block.
func undeclaredDriverWarnings(file string, pres *transformer.TransformedPresentation) []string {
	type problem struct {
		reference liveBlockReference
		message   string
	}
	var problems []problem
	for _, slide := range pres.Slides {
		for _, block := range slide.CodeBlocks {
			if block.Problem != "" {
				problems = append(problems, problem{liveBlockReference{slide.Index + 1, block.Block}, block.Problem})
			}
		}
	}
	if len(problems) == 0 {
		return nil
	}

	lines := liveBlockLines(file)
	warnings := make([]string, 0, len(problems))
	for _, found := range problems {
		location := fmt.Sprintf("%s: slide %d", file, found.reference.slide)
		if line, known := lines[found.reference]; known {
			location = fmt.Sprintf("%s:%d", file, line)
		}
		warnings = append(warnings, fmt.Sprintf("warning: %s: %s", location, found.message))
	}
	return warnings
}

// liveBlockLines maps each live code block to its line in the deck file.
// It builds the slide list only when a warning needs it, because the list
// also builds the deck's component bundles.
func liveBlockLines(file string) map[liveBlockReference]int {
	source, err := os.ReadFile(file)
	if err != nil {
		return nil
	}
	result, err := slidelist.Build(source, filepath.Dir(file))
	if err != nil {
		return nil
	}
	lines := make(map[liveBlockReference]int)
	for _, slide := range result.Slides {
		block := 0
		for _, codeBlock := range slide.CodeBlocks {
			if !codeBlock.Live {
				continue
			}
			block++
			lines[liveBlockReference{slide.Number, block}] = codeBlock.Line
		}
	}
	return lines
}
