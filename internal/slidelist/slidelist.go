// Package slidelist describes each slide of a deck with what an editor
// needs to draw it: its line range, layout, title, reveal counts, skip
// flag, errors and code blocks. tap slide list prints it, and the source
// endpoint of tap dev --app answers with it.
package slidelist

import (
	"fmt"
	"strings"

	"github.com/MiniCodeMonkey/tap/internal/components"
	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/layouts"
	"github.com/MiniCodeMonkey/tap/internal/parser"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

// Result is the slide list of one deck.
//
//nolint:govet // fieldalignment: field order is the JSON output order
type Result struct {
	Slides []Slide `json:"slides"`
	// Errors are problems with the deck as a whole, such as frontmatter
	// that does not parse. A problem with one slide is in its own Errors.
	Errors []string `json:"errors"`
}

// Slide describes one slide of a deck.
//
// StartLine and EndLine are 1-based line numbers in the deck file, and
// both are inclusive. The range covers the slide's text, including its
// directive comment, without leading or trailing blank lines. Blank lines
// next to a "---" separator belong to no slide. The separator lines and
// the frontmatter belong to no slide either. A "---" inside a fenced code
// block is text, not a separator.
//
//nolint:govet // fieldalignment: field order is the JSON output order
type Slide struct {
	// Number is the slide's 1-based position in the deck. Skipped slides
	// count, so the number matches URLs and every other tap command.
	Number    int `json:"number"`
	StartLine int `json:"startLine"`
	EndLine   int `json:"endLine"`
	// Layout is the layout the slide renders with: its layout directive,
	// or the layout tap picks from its content. A slide that is a
	// component has the component's path.
	Layout string `json:"layout"`
	// Title is the text of the slide's first heading, or "" when it has
	// none.
	Title string `json:"title"`
	// Fragments counts the slide's fragment reveals: its pause markers, or
	// its list items when its fragments directive is on.
	Fragments int `json:"fragments"`
	// Steps is the slide's step count: its steps directive, or else the
	// largest "export const steps" among its components.
	Steps int `json:"steps"`
	// Skip is true when the slide's skip directive is true.
	Skip bool `json:"skip"`
	// Errors lists the slide's problems: a parse error, a layout or slot
	// warning, a component that failed to build.
	Errors     []string    `json:"errors"`
	CodeBlocks []CodeBlock `json:"codeBlocks"`
}

// CodeBlock describes one fenced code block of a slide. A ```component
// fence is not a code block.
//
//nolint:govet // fieldalignment: field order is the JSON output order
type CodeBlock struct {
	// Block is the block's 1-based position among the slide's code blocks.
	Block    int    `json:"block"`
	Language string `json:"language"`
	Driver   string `json:"driver"`
	// Live is true when the block names a driver, so tap dev can run it.
	Live bool `json:"live"`
	// Line is the 1-based deck file line of the block's opening fence. It
	// is 0 only for an empty fence with no info string.
	Line int `json:"line"`
}

// Build returns the slide list of a deck. source is the deck's markdown,
// and baseDir is its folder, where component files resolve. Build bundles
// the deck's components, because a component's steps export sets its
// slide's step count. A problem in the deck goes in the Result, never in
// the error, so the list stays complete while a slide is broken.
func Build(source []byte, baseDir string) (Result, error) {
	result := Result{Slides: []Slide{}, Errors: []string{}}

	cfg, err := config.FromSource(source)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("frontmatter: %v", err))
		cfg = config.DefaultConfig()
	} else if err := cfg.Validate(); err != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("frontmatter: %v", err))
	}

	presentation, slideErrors := parser.New().ParseKeepingErrors(source)
	resolved := components.Resolve(presentation, baseDir, components.Options{
		DeckDirectory:   baseDir,
		AssetPublicPath: "/components/",
	})
	deckTransformer := transformer.NewWithBaseDir(cfg, baseDir)
	deckTransformer.SetComponents(resolved)
	transformed := deckTransformer.Transform(presentation)

	warningsBySlide := map[int][]string{}
	for _, warning := range layouts.Validate(transformed) {
		warningsBySlide[warning.SlideNumber] = append(warningsBySlide[warning.SlideNumber], warning.Message)
	}

	lines := strings.Split(strings.ReplaceAll(string(source), "\r\n", "\n"), "\n")
	for index, parsed := range presentation.Slides {
		rendered := transformed.Slides[index]
		number := index + 1
		slide := Slide{
			Number:     number,
			StartLine:  parsed.StartLine,
			EndLine:    parsed.EndLine,
			Layout:     rendered.Layout,
			Title:      parser.SlideTitle(parsed.Content),
			Fragments:  rendered.FragmentCount,
			Steps:      rendered.Steps,
			Skip:       parsed.Directives.Skip,
			Errors:     []string{},
			CodeBlocks: []CodeBlock{},
		}
		if rendered.Component != nil {
			slide.Layout = rendered.Component.Source
		}

		if err, failed := slideErrors[index]; failed {
			slide.Errors = append(slide.Errors, err.Error())
		}
		slide.Errors = append(slide.Errors, warningsBySlide[number]...)
		for _, component := range rendered.Components {
			if component.Error != "" {
				slide.Errors = append(slide.Errors, fmt.Sprintf("component %q failed to build: %s", component.Source, component.Error))
			}
		}

		fenceLines := parser.FenceLines(slideText(lines, parsed.StartLine, parsed.EndLine))
		for blockIndex, block := range parsed.CodeBlocks {
			codeBlock := CodeBlock{
				Block:    blockIndex + 1,
				Language: block.Language,
				Driver:   block.Meta.Driver,
				Live:     block.Meta.Driver != "",
			}
			if blockIndex < len(fenceLines) && fenceLines[blockIndex] > 0 {
				codeBlock.Line = parsed.StartLine + fenceLines[blockIndex] - 1
			}
			slide.CodeBlocks = append(slide.CodeBlocks, codeBlock)
		}

		result.Slides = append(result.Slides, slide)
	}
	return result, nil
}

// slideText returns lines startLine to endLine, 1-based and inclusive,
// joined back into one string.
func slideText(lines []string, startLine, endLine int) string {
	if startLine < 1 || endLine > len(lines) || startLine > endLine {
		return ""
	}
	return strings.Join(lines[startLine-1:endLine], "\n")
}
