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

// countLeadingBackticks returns the number of consecutive backticks at the start of a line.
func countLeadingBackticks(line string) int {
	count := 0
	for _, ch := range line {
		if ch == '`' {
			count++
		} else {
			break
		}
	}
	return count
}

// SplitSlidesPreservingCodeBlocks splits text on "---" delimiters while preserving
// code blocks. Any "---" inside a fenced code block (``` or ````) is NOT treated
// as a slide delimiter.
func SplitSlidesPreservingCodeBlocks(text string) []string {
	lines := strings.Split(text, "\n")
	var slides []string
	var currentSlide strings.Builder
	insideCodeBlock := false
	codeBlockFenceLength := 0

	for i, line := range lines {
		// Check for code block fence (must be at least 3 backticks)
		backtickCount := countLeadingBackticks(line)
		if backtickCount >= 3 {
			if !insideCodeBlock {
				// Opening a code block
				insideCodeBlock = true
				codeBlockFenceLength = backtickCount
			} else if backtickCount >= codeBlockFenceLength {
				// Check if this is a closing fence (just backticks, possibly with trailing whitespace)
				trimmedAfterBackticks := strings.TrimSpace(line[backtickCount:])
				if trimmedAfterBackticks == "" {
					// Closing the code block
					insideCodeBlock = false
					codeBlockFenceLength = 0
				}
			}
		}

		// Check for slide delimiter only when not in a code block
		if !insideCodeBlock && slideDelimiter.MatchString(line) {
			// End current slide, start new one
			slides = append(slides, currentSlide.String())
			currentSlide.Reset()
		} else {
			// Add line to current slide
			if currentSlide.Len() > 0 || i > 0 {
				// Add newline before line (except for very first line when builder is empty)
				if currentSlide.Len() > 0 {
					currentSlide.WriteString("\n")
				}
			}
			currentSlide.WriteString(line)
		}
	}

	// Don't forget the last slide
	if currentSlide.Len() > 0 {
		slides = append(slides, currentSlide.String())
	}

	return slides
}

// Parse parses markdown content and returns a Presentation with slides.
// Slides are split on "---" delimiters. Frontmatter (if present) is skipped.
func (p *Parser) Parse(content []byte) (*Presentation, error) {
	// Convert to string for easier manipulation
	text := string(content)

	// Skip frontmatter if present
	text = skipFrontmatter(text)

	// Split content on --- delimiter, preserving code blocks
	parts := SplitSlidesPreservingCodeBlocks(text)

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

		// Parse code blocks from the slide content
		codeBlocks := parseCodeBlocks(contentAfterDirectives)

		// Parse fragments from pause markers and render to HTML
		fragments := p.parseFragments(contentAfterDirectives)

		// Auto-fragment list items when fragments: true and no explicit pause markers
		if directives.Fragments && !hasPauseMarkers(contentAfterDirectives) {
			transformedHTML, listItemCount := autoFragmentListItems(html)
			if listItemCount > 0 {
				html = transformedHTML
				// Create fragment entries for each list item
				// This tells the frontend how many fragment steps exist
				fragments = make([]Fragment, listItemCount)
				for i := 0; i < listItemCount; i++ {
					fragments[i] = Fragment{
						Content: "", // Content is inline in the HTML, not in fragment structs
						Index:   i,
					}
				}
			}
		}

		slide := Slide{
			Content:    contentAfterDirectives,
			HTML:       html,
			Index:      len(presentation.Slides),
			Directives: directives,
			Fragments:  fragments,
			CodeBlocks: codeBlocks,
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

// codeBlockPattern matches fenced code blocks with info string.
// Captures: (1) info string, (2) code content
// Example: ```sql {driver: mysql, connection: mydb}
var codeBlockPattern = regexp.MustCompile("(?m)^```([^\\n]*)\\n([\\s\\S]*?)\\n```")

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

// metaPattern matches {key: value, ...} at the end of info string.
// Example: sql {driver: mysql, connection: mydb}
var metaPattern = regexp.MustCompile(`\{([^}]*)\}\s*$`)

// pausePattern matches <!-- pause --> markers for fragment splitting.
// Supports variations: <!-- pause -->, <!--pause-->, <!-- pause-->, etc.
var pausePattern = regexp.MustCompile(`(?m)^\s*<!--\s*pause\s*-->\s*$`)

// parseCodeBlocks extracts fenced code blocks from slide content.
// It parses the info string for language and optional driver configuration.
// Example: ```sql {driver: mysql, connection: mydb}
func parseCodeBlocks(content string) []CodeBlock {
	blocks := []CodeBlock{}

	matches := codeBlockPattern.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}

		infoString := strings.TrimSpace(match[1])
		code := match[2]

		block := CodeBlock{
			Code: code,
			Meta: CodeBlockMeta{},
		}

		// Parse info string for language and metadata
		// Format: language {driver: driverName, connection: connName}
		if metaMatch := metaPattern.FindStringSubmatch(infoString); metaMatch != nil {
			// Extract language (everything before the {})
			langPart := strings.TrimSpace(infoString[:len(infoString)-len(metaMatch[0])])
			block.Language = langPart

			// Parse metadata inside {}
			metaContent := metaMatch[1]
			block.Meta = parseCodeBlockMeta(metaContent)
		} else {
			// No metadata, just language
			block.Language = infoString
		}

		blocks = append(blocks, block)
	}

	return blocks
}

// parseCodeBlockMeta parses the content inside {} in code block info strings.
// Supports both YAML-like (key: value) and simple (key=value) formats.
// Example: "driver: mysql, connection: mydb" or "driver=mysql, connection=mydb"
func parseCodeBlockMeta(content string) CodeBlockMeta {
	meta := CodeBlockMeta{}

	// Try parsing as YAML first
	var yamlData map[string]interface{}
	// Wrap in braces for valid YAML map format
	if err := yaml.Unmarshal([]byte("{"+content+"}"), &yamlData); err == nil {
		if driver, ok := yamlData["driver"].(string); ok {
			meta.Driver = driver
		}
		if connection, ok := yamlData["connection"].(string); ok {
			meta.Connection = connection
		}
		return meta
	}

	// Fall back to simple key=value or key: value parsing
	parts := strings.Split(content, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		var key, value string

		if idx := strings.Index(part, ":"); idx != -1 {
			key = strings.TrimSpace(part[:idx])
			value = strings.TrimSpace(part[idx+1:])
		} else if idx := strings.Index(part, "="); idx != -1 {
			key = strings.TrimSpace(part[:idx])
			value = strings.TrimSpace(part[idx+1:])
		} else {
			continue
		}

		switch key {
		case "driver":
			meta.Driver = value
		case "connection":
			meta.Connection = value
		}
	}

	return meta
}

// parseFragments splits slide content on <!-- pause --> markers.
// It returns a slice of Fragment structs, each containing HTML content for incremental reveal.
// If no pause markers are found, returns a single fragment with all content as HTML.
func (p *Parser) parseFragments(content string) []Fragment {
	// Split content on pause markers
	parts := pausePattern.Split(content, -1)

	fragments := make([]Fragment, 0, len(parts))
	for i, part := range parts {
		// Trim whitespace from fragment content
		trimmedContent := strings.TrimSpace(part)

		// Skip empty fragments (can occur with consecutive pause markers)
		if trimmedContent == "" {
			continue
		}

		// Render fragment content to HTML
		html, err := p.renderHTML([]byte(trimmedContent))
		if err != nil {
			// If rendering fails, use the raw content
			html = trimmedContent
		}

		fragments = append(fragments, Fragment{
			Content: html,
			Index:   i,
		})
	}

	// Re-index fragments to be consecutive (after skipping empty ones)
	for i := range fragments {
		fragments[i].Index = i
	}

	return fragments
}

// liPattern matches <li> opening tags (with or without attributes).
var liPattern = regexp.MustCompile(`<li(\s[^>]*)?>`)

// autoFragmentListItems transforms HTML to add fragment classes to list items.
// It adds class="fragment fragment-hidden" and data-fragment-index attributes to each <li> element.
// The fragment-hidden class ensures items are hidden initially until revealed by navigation.
// Returns the transformed HTML and the number of list items found.
func autoFragmentListItems(html string) (string, int) {
	fragmentIndex := 0

	result := liPattern.ReplaceAllStringFunc(html, func(match string) string {
		index := fragmentIndex
		fragmentIndex++

		// Check if the <li> already has attributes
		if match == "<li>" {
			return `<li class="fragment fragment-hidden" data-fragment-index="` + intToString(index) + `">`
		}

		// Has existing attributes - need to merge class if present or add it
		// Check if there's already a class attribute
		if strings.Contains(match, `class="`) {
			// Insert "fragment fragment-hidden " at the start of the existing class value
			return strings.Replace(match, `class="`, `class="fragment fragment-hidden `, 1) +
				` data-fragment-index="` + intToString(index) + `"`
		}

		// No class attribute, add both class and data-fragment-index
		// Insert before the closing >
		return match[:len(match)-1] + ` class="fragment fragment-hidden" data-fragment-index="` + intToString(index) + `">`
	})

	return result, fragmentIndex
}

// intToString converts an integer to a string without importing strconv.
func intToString(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + intToString(-n)
	}
	digits := ""
	for n > 0 {
		digits = string(rune('0'+n%10)) + digits
		n /= 10
	}
	return digits
}

// hasPauseMarkers checks if the content contains any <!-- pause --> markers.
func hasPauseMarkers(content string) bool {
	return pausePattern.MatchString(content)
}
