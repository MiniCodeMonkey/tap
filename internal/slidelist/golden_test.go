package slidelist

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite the golden files in testdata/golden")

// goldenDecks maps each golden file name to the deck it pins.
var goldenDecks = map[string]string{
	"basic":           "../../examples/basic.md",
	"code-demo":       "../../examples/code-demo.md",
	"conference-talk": "../../examples/conference-talk.md",
	"launch-day":      "../../examples/launch-day.md",
	"map-demo":        "../../examples/map-demo.md",
	"sql-demo":        "../../examples/sql-demo.md",
	"theme-tour":      "../../examples/theme-tour.md",
	"components":      "../../examples/components/deck.md",
	"fences":          "testdata/fences.md",
}

func TestGoldenSlideLists(t *testing.T) {
	for name, path := range goldenDecks {
		t.Run(name, func(t *testing.T) {
			got, err := json.MarshalIndent(buildFile(t, path), "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, '\n')

			goldenPath := filepath.Join("testdata", "golden", name+".json")
			if *update {
				if err := os.MkdirAll(filepath.Dir(goldenPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(goldenPath)
			if err != nil {
				t.Fatalf("%v: run go test ./internal/slidelist -run TestGoldenSlideLists -update", err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("slide list of %s differs from %s. If the change is intended, run with -update and review the diff.\ngot:\n%s", path, goldenPath, got)
			}
		})
	}
}

func TestEveryExampleDeckHasAGoldenFile(t *testing.T) {
	decks, err := filepath.Glob("../../examples/*.md")
	if err != nil {
		t.Fatal(err)
	}
	known := map[string]bool{}
	for _, path := range goldenDecks {
		known[path] = true
	}
	for _, deck := range decks {
		if !known[filepath.ToSlash(deck)] {
			t.Errorf("%s has no entry in goldenDecks", deck)
		}
	}
}

func TestTheFencesFixture(t *testing.T) {
	result := buildFile(t, filepath.Join("testdata", "fences.md"))

	wantRanges := [][2]int{{5, 11}, {15, 18}, {22, 26}, {30, 34}, {38, 45}}
	wantBlocks := []CodeBlock{
		{Block: 1, Language: "yaml", Line: 7},
		{Block: 1, Language: "markdown", Line: 15},
		{Block: 1, Language: "markdown", Line: 22},
		{Block: 1, Language: "yaml", Line: 32},
		{Block: 1, Language: "sql", Driver: "sqlite", Live: true, Line: 42},
	}
	if len(result.Slides) != len(wantRanges) {
		t.Fatalf("got %d slides, want %d: a --- inside a fence must not split a slide", len(result.Slides), len(wantRanges))
	}
	for index, slide := range result.Slides {
		if slide.StartLine != wantRanges[index][0] || slide.EndLine != wantRanges[index][1] {
			t.Errorf("slide %d: lines %d-%d, want %d-%d", index+1, slide.StartLine, slide.EndLine, wantRanges[index][0], wantRanges[index][1])
		}
		if len(slide.CodeBlocks) != 1 || slide.CodeBlocks[0] != wantBlocks[index] {
			t.Errorf("slide %d code blocks = %+v, want [%+v]", index+1, slide.CodeBlocks, wantBlocks[index])
		}
		if len(slide.Errors) != 0 {
			t.Errorf("slide %d errors = %v", index+1, slide.Errors)
		}
	}
	if last := result.Slides[4]; !last.Skip || last.Title != "A skipped slide" {
		t.Errorf("slide 5 = %+v, want skipped, titled", last)
	}
	for index := 0; index < 4; index++ {
		if result.Slides[index].Skip {
			t.Errorf("slide %d is marked skipped", index+1)
		}
	}
	if strings.Join(result.Errors, "") != "" {
		t.Errorf("deck errors = %v", result.Errors)
	}
}
