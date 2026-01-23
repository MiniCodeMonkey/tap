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
	"gopkg.in/yaml.v3"
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

	for _, part := range parts {
		// Trim whitespace from slide content
		slideContent := strings.TrimSpace(part)

		// Skip empty slides
		if slideContent == "" {
			continue
		}

		// Parse directives from HTML comments at slide start
		directives, contentAfterDirectives := parseDirectives(slideContent)

		// Render markdown to HTML (use content after directives removed)
		html, err := p.renderHTML([]byte(contentAfterDirectives))
		if err != nil {
			return nil, err
		}

		slide := Slide{
			Content:    contentAfterDirectives,
			HTML:       html,
			Index:      len(presentation.Slides),
			Directives: directives,
			Fragments:  []Fragment{},
			CodeBlocks: []CodeBlock{},
		}

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

// directivePattern matches HTML comments containing YAML directives at the start of slides.
// Example: <!-- layout: title \n transition: fade -->
var directivePattern = regexp.MustCompile(`(?s)^\s*<!--\s*(.*?)\s*-->`)

// parseDirectives extracts YAML directives from an HTML comment at the start of slide content.
// It returns the parsed directives and the content with the directive comment removed.
func parseDirectives(content string) (SlideDirectives, string) {
	directives := SlideDirectives{}

	match := directivePattern.FindStringSubmatch(content)
	if match == nil {
		return directives, content
	}

	// Extract the YAML content from the comment
	yamlContent := match[1]

	// Parse the YAML into the directives struct
	// We use a map first to handle the yaml parsing, then extract fields
	var yamlData map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlContent), &yamlData); err != nil {
		// If YAML parsing fails, return unchanged content
		// This allows non-directive HTML comments to pass through
		return directives, content
	}

	// Extract known directive fields
	if layout, ok := yamlData["layout"].(string); ok {
		directives.Layout = layout
	}
	if transition, ok := yamlData["transition"].(string); ok {
		directives.Transition = transition
	}
	if background, ok := yamlData["background"].(string); ok {
		directives.Background = background
	}
	if notes, ok := yamlData["notes"].(string); ok {
		directives.Notes = notes
	}
	if fragments, ok := yamlData["fragments"].(bool); ok {
		directives.Fragments = fragments
	}

	// Remove the directive comment from content
	remainingContent := strings.TrimPrefix(content, match[0])
	remainingContent = strings.TrimLeft(remainingContent, "\n")

	return directives, remainingContent
}
