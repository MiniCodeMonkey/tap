package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/parser"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

// TestExampleDecksDeclareTheirDrivers keeps every shipped deck runnable:
// a live code block whose driver the deck does not declare never runs.
func TestExampleDecksDeclareTheirDrivers(t *testing.T) {
	var decks []string
	for _, pattern := range []string{"../../examples/*.md", "../../docs/examples/*.md", "../../testdata/*.md"} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Fatal(err)
		}
		decks = append(decks, matches...)
	}
	if len(decks) == 0 {
		t.Fatal("no example decks found")
	}

	for _, deck := range decks {
		t.Run(filepath.Base(deck), func(t *testing.T) {
			content, err := os.ReadFile(deck)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := config.Load(deck)
			if err != nil {
				t.Skipf("not a deck: %v", err)
			}
			parsed, err := parser.New().Parse(content)
			if err != nil {
				t.Skipf("not a deck: %v", err)
			}
			for _, slide := range transformer.New(cfg).Transform(parsed).Slides {
				for _, block := range slide.CodeBlocks {
					if block.Problem != "" {
						t.Errorf("%s slide %d: %s", deck, slide.Index+1, block.Problem)
					}
				}
			}
		})
	}
}
