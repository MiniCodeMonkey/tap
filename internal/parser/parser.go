// Package parser provides markdown parsing functionality for tap presentations.
package parser

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// Presentation represents a parsed markdown presentation.
type Presentation struct {
	Slides []Slide
}

// Slide represents a single slide in the presentation.
type Slide struct {
	// Content is the raw markdown content of the slide.
	Content string
	// HTML is the rendered HTML of the slide content.
	HTML string
	// Directives contains per-slide configuration parsed from HTML comments.
	Directives SlideDirectives
	// Fragments contains the fragment groups for incremental reveals.
	Fragments []Fragment
	// CodeBlocks contains the code blocks found in this slide.
	CodeBlocks []CodeBlock
	// Index is the zero-based slide index.
	Index int
}

// SlideDirectives contains per-slide configuration options.
type SlideDirectives struct {
	Layout     string
	Transition string
	Background string
	Notes      string
	Fragments  bool
}

// Fragment represents a content fragment for incremental reveals.
type Fragment struct {
	Content string
	Index   int
}

// CodeBlock represents a fenced code block in a slide.
type CodeBlock struct {
	Language string
	Code     string
	Meta     CodeBlockMeta
}

// CodeBlockMeta contains metadata parsed from code block info strings.
type CodeBlockMeta struct {
	Driver     string
	Connection string
}

// Parser handles markdown parsing for presentations.
type Parser struct {
	md goldmark.Markdown
}

// New creates a new Parser with goldmark configured for presentation parsing.
// It enables the following extensions:
//   - Table: GFM tables
//   - Strikethrough: ~~strikethrough~~ text
//   - TaskList: - [x] checkboxes
//   - Linkify: auto-link URLs
func New() *Parser {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.Table,
			extension.Strikethrough,
			extension.TaskList,
			extension.Linkify,
		),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(), // Allow raw HTML in markdown
		),
	)

	return &Parser{
		md: md,
	}
}

// Markdown returns the underlying goldmark.Markdown instance.
func (p *Parser) Markdown() goldmark.Markdown {
	return p.md
}

// slideDelimiter is the pattern used to split slides.
// It matches "---" on its own line (with optional surrounding whitespace).
var slideDelimiter = regexp.MustCompile(`(?m)^---\s*$`)

// Parse parses markdown content and returns a Presentation with slides.
// Slides are split on "---" delimiters. Frontmatter (if present) is skipped.
func (p *Parser) Parse(content []byte) (*Presentation, error) {
	// Convert to string for easier manipulation
	text := string(content)

	// Skip frontmatter if present
	text = skipFrontmatter(text)

	// Split content on --- delimiter
	parts := slideDelimiter.Split(text, -1)

	presentation := &Presentation{
		Slides: make([]Slide, 0, len(parts)),
	}

	for i, part := range parts {
		// Trim whitespace from slide content
		slideContent := strings.TrimSpace(part)

		// Skip empty slides
		if slideContent == "" {
			continue
		}

		// Render markdown to HTML
		html, err := p.renderHTML([]byte(slideContent))
		if err != nil {
			return nil, err
		}

		slide := Slide{
			Content:    slideContent,
			HTML:       html,
			Index:      len(presentation.Slides),
			Directives: SlideDirectives{},
			Fragments:  []Fragment{},
			CodeBlocks: []CodeBlock{},
		}

		// Preserve original index for debugging
		_ = i

		presentation.Slides = append(presentation.Slides, slide)
	}

	return presentation, nil
}

// skipFrontmatter removes YAML frontmatter from the beginning of the content.
// Frontmatter is delimited by "---" at the start and end.
func skipFrontmatter(text string) string {
	// Check if content starts with frontmatter delimiter
	if !strings.HasPrefix(strings.TrimSpace(text), "---") {
		return text
	}

	// Find the first ---
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "---") {
		return text
	}

	// Find the closing ---
	rest := text[3:] // Skip the first "---"
	idx := strings.Index(rest, "\n---")
	if idx == -1 {
		// No closing delimiter, return original
		return text
	}

	// Skip past the closing delimiter and any trailing newline
	afterFrontmatter := rest[idx+4:] // +4 for "\n---"
	return strings.TrimPrefix(afterFrontmatter, "\n")
}

// renderHTML converts markdown content to HTML.
func (p *Parser) renderHTML(content []byte) (string, error) {
	var buf bytes.Buffer
	if err := p.md.Convert(content, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
