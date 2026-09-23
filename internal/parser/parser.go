// Package parser provides markdown parsing functionality for tap presentations.
package parser

import (
	"encoding/json"
	"fmt"
	stdhtml "html"
	"os"
	"regexp"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/util"
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
	// Slots maps a slot name to its rendered HTML.
	Slots map[string]string
	// SlotOrder lists the slot names in document order.
	SlotOrder []string
	// FragmentCount is the number of fragment reveals on the slide.
	FragmentCount int
	// CodeBlocks contains the code blocks found in this slide.
	CodeBlocks []CodeBlock
	// Components contains the inline ```component fences found in this
	// slide, in document order, with the running index used for their
	// data-component-index placeholder.
	Components []Component
	// Index is the zero-based slide index.
	Index int
	// StartLine and EndLine are the 1-based, inclusive lines of the deck
	// file the slide covers: its text, including its directive comment,
	// without leading or trailing blank lines. Blank lines next to a "---"
	// separator belong to no slide, and neither do the separator lines
	// and the frontmatter.
	StartLine int
	EndLine   int
}

// SlideDirectives contains per-slide configuration options.
type SlideDirectives struct {
	Layout      string
	Transition  string
	Background  string
	Notes       string
	Tag         string // Decorative metadata label (e.g., "// workshop")
	Badge       string // Decorative metadata badge (e.g., "v2.0")
	Fragments   bool
	Scroll      bool // Enable scroll reveal for long content
	ScrollSpeed int  // Animation duration in milliseconds (default: 2000)
	Steps       int  // Explicit clicker-step count, overriding auto-detection
	HasSteps    bool // Whether the "steps" directive was present
	// StepsInvalid is true when a "steps:" directive was present but its
	// value was negative, or did not parse as an int at all (for example a
	// number too large for one); the directive is ignored either way, and
	// a caller that surfaces slide warnings should tell the deck author.
	StepsInvalid bool
	// Skip is true when the slide's "skip" directive is true. Presenting,
	// slide counts, tap build and tap export leave the slide out. tap dev
	// still shows it when someone goes to it directly.
	Skip bool
	// SkipInvalid is true when a "skip:" directive was present but its
	// value did not parse as a YAML boolean, for example "yes" (which YAML
	// resolves as a string, not true) or an empty value. The directive is
	// ignored either way, and a caller that surfaces slide warnings should
	// tell the deck author, the same way StepsInvalid does.
	SkipInvalid bool
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
	// HighlightLines is a line-highlight spec such as "3" or "1,3-5",
	// parsed from a fence info string like "```php {1,3-5}".
	HighlightLines string
}

// Component represents one inline ```component fence found in a slide.
type Component struct {
	// Index is the per-slide running index used for the fence's
	// placeholder (data-component-index), threaded across slots and
	// fragments exactly like a code block's index.
	Index int
	// Source is the component file path as written after "component" in
	// the fence info string, relative to the deck file.
	Source string
	// Props is the fence body, parsed and validated as a JSON object
	// ("{}" when the body is empty).
	Props json.RawMessage
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
			// Runs once per parse, after block/inline parsing produces the
			// AST and before rendering; assigns each h1-h3 its data-length
			// attribute. The priority only matters relative to other
			// registered transformers, of which there are none yet.
			parser.WithASTTransformers(
				util.Prioritized(NewHeadingLengthTransformer(), 100),
				util.Prioritized(NewBlockquoteLengthTransformer(), 100),
			),
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(), // Allow raw HTML in markdown
			// Priority 100 is lower than the default html.Renderer's 1000
			// (see goldmark.NewMarkdown), which wins it registration for
			// KindFencedCodeBlock so it can carry a line-highlight spec
			// onto the rendered <code> tag.
			renderer.WithNodeRenderers(
				util.Prioritized(NewFencedCodeBlockRenderer(), 100),
			),
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

// SplitSlidesPreservingCodeBlocks splits text on "---" delimiters while
// preserving code blocks. A "---" inside a fenced code block (backticks or
// tildes, see fenceTracker) is not a slide delimiter.
func SplitSlidesPreservingCodeBlocks(text string) []string {
	lines := strings.Split(text, "\n")
	var slides []string
	var currentSlide strings.Builder
	var fences fenceTracker

	for i, line := range lines {
		insideFence := fences.advance(line)

		// A "---" line is a slide delimiter only outside a fenced code block
		if !insideFence && slideDelimiter.MatchString(line) {
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

// SplitSlidesRaw splits text on "---" delimiter lines the same way
// SplitSlidesPreservingCodeBlocks does (a delimiter inside a fenced code
// block, see fenceTracker, is not a boundary), but keeps every line's exact
// text, including blank lines and a separator's own trailing whitespace.
// separators[i] is the literal delimiter line between parts[i] and
// parts[i+1], so len(separators) == len(parts)-1.
//
// A caller rebuilding text from these parts reproduces it exactly with:
//
//	result := parts[0]
//	for i, sep := range separators {
//		result += "\n" + sep + "\n" + parts[i+1]
//	}
//
// That lets a caller change one part and rejoin the rest byte for byte,
// unlike SplitSlidesPreservingCodeBlocks, which drops the blank line right
// after a delimiter and so cannot be rejoined losslessly.
func SplitSlidesRaw(text string) (parts []string, separators []string) {
	lines := strings.Split(text, "\n")
	var fences fenceTracker
	start := 0

	for i, line := range lines {
		insideFence := fences.advance(line)
		if !insideFence && slideDelimiter.MatchString(line) {
			parts = append(parts, strings.Join(lines[start:i], "\n"))
			separators = append(separators, line)
			start = i + 1
		}
	}
	parts = append(parts, strings.Join(lines[start:], "\n"))

	return parts, separators
}

// slideChunk is one slide's raw markdown, as SplitSlidesPreservingCodeBlocks
// would return it, plus the 1-based line (within the text passed to
// splitSlidesPreservingCodeBlocksWithLines) that Content's first line sits
// on.
type slideChunk struct {
	Content   string
	StartLine int
}

// splitSlidesPreservingCodeBlocksWithLines splits text the same way
// SplitSlidesPreservingCodeBlocks does, but also records each slide's
// starting line, so Parse can turn a line number inside a slide into a
// real line number in the deck file.
func splitSlidesPreservingCodeBlocksWithLines(text string) []slideChunk {
	lines := strings.Split(text, "\n")
	var slides []slideChunk
	var currentLines []string
	var fences fenceTracker
	slideStartLine := 1

	flush := func() {
		slides = append(slides, slideChunk{Content: strings.Join(currentLines, "\n"), StartLine: slideStartLine})
		currentLines = nil
	}

	for i, line := range lines {
		insideFence := fences.advance(line)

		if !insideFence && slideDelimiter.MatchString(line) {
			flush()
			slideStartLine = i + 2
		} else {
			currentLines = append(currentLines, line)
		}
	}

	if len(currentLines) > 0 {
		flush()
	}

	return slides
}

// Parse parses markdown content and returns a Presentation with slides.
// Slides are split on "---" delimiters. Frontmatter (if present) is
// skipped. A slide that fails to parse fails the whole parse, naming the
// first such slide.
func (p *Parser) Parse(content []byte) (*Presentation, error) {
	presentation, slideErrors := p.ParseKeepingErrors(content)
	for index := range presentation.Slides {
		if err, failed := slideErrors[index]; failed {
			return nil, fmt.Errorf("slide %d: %w", index+1, err)
		}
	}
	return presentation, nil
}

// ParseKeepingErrors parses content the way Parse does, but a slide that
// fails to parse stays in the presentation with its Index, line range,
// Directives and Content, and no HTML. slideErrors maps the 0-based index
// of each such slide to its error. An editor uses it to show every slide
// of a deck while one of them is broken.
func (p *Parser) ParseKeepingErrors(content []byte) (presentation *Presentation, slideErrors map[int]error) {
	// Normalize CRLF line endings to LF so a Windows-saved deck parses the
	// same way as one saved with Unix line endings, and no stray "\r"
	// characters end up in slide content, HTML, or notes. This never
	// changes a line's number: each "\r\n" becomes exactly one "\n".
	text := strings.ReplaceAll(string(content), "\r\n", "\n")

	// Skip frontmatter if present, and remember how many file lines it
	// took up, so a slide's line numbers can be translated back to real
	// file lines.
	text, frontmatterLineOffset := skipFrontmatterWithLineOffset(text)

	// Split content on --- delimiter, preserving code blocks, and keep
	// each slide's starting line for the same reason.
	chunks := splitSlidesPreservingCodeBlocksWithLines(text)

	presentation = &Presentation{Slides: make([]Slide, 0, len(chunks))}
	slideErrors = map[int]error{}
	for _, chunk := range chunks {
		// Skip empty slides
		if strings.TrimSpace(chunk.Content) == "" {
			continue
		}
		index := len(presentation.Slides)
		slide, err := p.parseSlide(chunk, frontmatterLineOffset, index)
		if err != nil {
			slideErrors[index] = err
		}
		presentation.Slides = append(presentation.Slides, slide)
	}
	return presentation, slideErrors
}

// parseSlide parses one non-empty chunk into the slide with the given
// 0-based index. frontmatterLineOffset is the number of file lines before
// the text the chunk came from. When the slide fails to parse, the
// returned slide still has its Index, line range, Directives and Content.
func (p *Parser) parseSlide(chunk slideChunk, frontmatterLineOffset int, index int) (Slide, error) {
	// Trim whitespace from slide content
	slideContent := strings.TrimSpace(chunk.Content)
	startLine := frontmatterLineOffset + chunk.StartLine + leadingWhitespaceLines(chunk.Content)
	slide := Slide{
		Index:     index,
		StartLine: startLine,
		EndLine:   startLine + strings.Count(slideContent, "\n"),
	}

	// Parse directives from HTML comments at slide start
	directives, contentAfterDirectives := parseDirectives(slideContent)
	// parseDirectives only ever removes a prefix, so contentAfterDirectives
	// is always a suffix of slideContent; the newlines in what was
	// removed are exactly the lines the directive comment took up.
	slideFileLine := startLine + strings.Count(slideContent[:len(slideContent)-len(contentAfterDirectives)], "\n")

	// Remove any further notes comments from the rest of the slide
	// (e.g. a trailing "<!-- notes: ... -->" after the content), and
	// join their text onto directive notes, if any, in document order.
	// This can remove lines from the middle of the slide, which this
	// package does not track, so a slide with a notes comment loses
	// exact component fence line numbers.
	var extraNotes []string
	beforeNotes := contentAfterDirectives
	contentAfterDirectives, extraNotes = extractNotesComments(contentAfterDirectives)
	lineNumbersExact := len(extraNotes) == 0 && contentAfterDirectives == beforeNotes
	if len(extraNotes) > 0 {
		joined := strings.Join(extraNotes, "\n\n")
		if directives.Notes != "" {
			directives.Notes = directives.Notes + "\n\n" + joined
		} else {
			directives.Notes = joined
		}
	}
	slide.Directives = directives

	// Pre-process images with attributes (e.g., {width=50%}) to HTML.
	// Every replacement stays on the single line the image markdown
	// was on, so this never shifts line numbers.
	contentAfterDirectives = transformImageAttributes(contentAfterDirectives, index+1)

	// Pre-process asciinema code blocks to move info string meta into
	// body. A block with metadata pairs turns one line into several,
	// which does shift the lines after it.
	if asciinemaInfoPattern.MatchString(contentAfterDirectives) {
		lineNumbersExact = false
	}
	contentAfterDirectives = transformAsciinemaBlocks(contentAfterDirectives)
	slide.Content = contentAfterDirectives

	sections, err := splitSlots(contentAfterDirectives)
	if err != nil {
		return slide, err
	}

	slots := make(map[string]string, len(sections))
	slotOrder := make([]string, 0, len(sections))
	fragmentCount := 0
	codeBlockCount := 0
	componentCount := 0
	autoFragment := directives.Fragments && !hasPauseMarkers(contentAfterDirectives)
	var fullHTML strings.Builder
	var codeBlocks []CodeBlock
	var components []Component
	for _, section := range sections {
		sectionFileLine := slideFileLine + (section.StartLine - 1)
		slotHTML, nextFragmentIndex, nextCodeBlockIndex, nextComponentIndex, blocks, sectionComponents, err := p.renderSlot(section.Content, fragmentCount, codeBlockCount, componentCount, sectionFileLine, lineNumbersExact)
		if err != nil {
			return slide, err
		}
		fragmentCount = nextFragmentIndex
		codeBlockCount = nextCodeBlockIndex
		componentCount = nextComponentIndex
		codeBlocks = append(codeBlocks, blocks...)
		components = append(components, sectionComponents...)
		if autoFragment {
			slotHTML, fragmentCount = autoFragmentListItems(slotHTML, fragmentCount)
		}
		slots[section.Name] = slotHTML
		slotOrder = append(slotOrder, section.Name)
		fullHTML.WriteString(slotHTML)
	}

	slide.HTML = fullHTML.String()
	slide.Slots = slots
	slide.SlotOrder = slotOrder
	slide.FragmentCount = fragmentCount
	slide.CodeBlocks = codeBlocks
	slide.Components = components
	return slide, nil
}

// skipFrontmatterWithLineOffset removes YAML frontmatter from the
// beginning of text the same way skipFrontmatter does, and also returns
// how many lines of text were removed from the front to get there (any
// leading blank lines plus the frontmatter block itself), so a caller can
// turn a line number within the returned string into a real line number
// in text.
func skipFrontmatterWithLineOffset(text string) (string, int) {
	leadingTrimmed := strings.TrimLeft(text, " \t\r\n")
	if !strings.HasPrefix(leadingTrimmed, "---") {
		return text, 0
	}

	rest := leadingTrimmed[3:]
	closingIndex := strings.Index(rest, "\n---")
	if closingIndex == -1 {
		return text, 0
	}

	frontmatterBlock := leadingTrimmed[:3+closingIndex+4]
	afterFrontmatter := rest[closingIndex+4:]
	remaining := strings.TrimPrefix(afterFrontmatter, "\n")

	removedLength := len(text) - len(leadingTrimmed) + len(frontmatterBlock) + (len(afterFrontmatter) - len(remaining))
	return remaining, strings.Count(text[:removedLength], "\n")
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

	// Extract the YAML content from the comment, quoting any bare hex
	// color values first so they survive YAML parsing (an unquoted "#"
	// starts a YAML comment, which would otherwise silently empty the value).
	yamlContent := quoteHexColorValues(match[1])

	// Parse the YAML into the directives struct
	// We use a map first to handle the yaml parsing, then extract fields
	var yamlData map[string]interface{}
	err := yaml.Unmarshal([]byte(yamlContent), &yamlData)
	if err != nil {
		// The whole comment isn't valid YAML. This happens when it mixes
		// real directives with free-text notes that aren't valid YAML
		// themselves (a colon or a leading quote in the notes body, for
		// example), in either order. Split the comment line by line: a
		// line starting with a known directive key becomes its own small
		// YAML document, and the "notes:" line plus every line after it
		// up to the next directive line become the free-text notes.
		hasNotes, notesText, mixedData := splitMixedDirectiveComment(yamlContent)
		if !hasNotes {
			// No notes line either: leave the content unchanged so
			// non-directive HTML comments keep passing through.
			return directives, content
		}
		applyDirectiveFields(mixedData, &directives)
		directives.Notes = notesText
		remainingContent := strings.TrimPrefix(content, match[0])
		remainingContent = strings.TrimLeft(remainingContent, "\n")
		return directives, remainingContent
	}

	applyDirectiveFields(yamlData, &directives)

	// The comment may be a pure notes comment written as free text where
	// YAML happened to still parse, but not into a clean "notes" string
	// (such as an unindented second line read as another mapping key).
	// Fall back to free text in that case.
	if directives.Notes == "" && isNotesComment(yamlContent) {
		directives.Notes = notesTextFromComment(yamlContent)
	}

	// Remove the directive comment from content
	remainingContent := strings.TrimPrefix(content, match[0])
	remainingContent = strings.TrimLeft(remainingContent, "\n")

	return directives, remainingContent
}

// directiveField describes one directive's YAML key and how its value is
// copied out of a parsed YAML map into a SlideDirectives. This is the
// single source of truth for which directive keys parseDirectives
// recognizes, other than "notes" (handled separately, since its value can
// span multiple lines and does not have to be valid YAML): applyDirectiveFields
// iterates it to fill in known fields, and directiveKeyNames (in notes.go,
// used to tell a directive line apart from notes prose) is derived from
// its keys, so the two cannot drift apart.
type directiveField struct {
	key   string
	apply func(yamlData map[string]interface{}, directives *SlideDirectives)
}

var directiveFields = []directiveField{
	{"layout", func(y map[string]interface{}, d *SlideDirectives) {
		if v, ok := y["layout"].(string); ok {
			d.Layout = v
		}
	}},
	{"transition", func(y map[string]interface{}, d *SlideDirectives) {
		if v, ok := y["transition"].(string); ok {
			d.Transition = v
		}
	}},
	{"background", func(y map[string]interface{}, d *SlideDirectives) {
		if v, ok := y["background"].(string); ok {
			d.Background = v
		}
	}},
	{"tag", func(y map[string]interface{}, d *SlideDirectives) {
		if v, ok := y["tag"].(string); ok {
			d.Tag = v
		}
	}},
	{"badge", func(y map[string]interface{}, d *SlideDirectives) {
		if v, ok := y["badge"].(string); ok {
			d.Badge = v
		}
	}},
	{"fragments", func(y map[string]interface{}, d *SlideDirectives) {
		if v, ok := y["fragments"].(bool); ok {
			d.Fragments = v
		}
	}},
	{"scroll", func(y map[string]interface{}, d *SlideDirectives) {
		if v, ok := y["scroll"].(bool); ok {
			d.Scroll = v
		}
	}},
	{"scroll-speed", func(y map[string]interface{}, d *SlideDirectives) {
		if v, ok := y["scroll-speed"].(int); ok {
			d.ScrollSpeed = v
		}
	}},
	{"steps", func(y map[string]interface{}, d *SlideDirectives) {
		raw, present := y["steps"]
		if !present {
			return
		}
		if v, ok := raw.(int); ok && v >= 0 {
			d.Steps = v
			d.HasSteps = true
			return
		}
		d.StepsInvalid = true
	}},
	{"skip", func(y map[string]interface{}, d *SlideDirectives) {
		raw, present := y["skip"]
		if !present {
			return
		}
		if v, ok := raw.(bool); ok {
			d.Skip = v
			return
		}
		d.SkipInvalid = true
	}},
}

// applyDirectiveFields copies known directive fields, including "notes"
// when it is a clean YAML string, out of a parsed YAML map into directives,
// using directiveFields for every key besides "notes". It is used both for
// a directive comment that parses as a whole and for the directive lines
// splitMixedDirectiveComment recovers from a comment that does not; each
// caller still falls back to free-text notes extraction itself when this
// does not produce a usable "notes" value.
func applyDirectiveFields(yamlData map[string]interface{}, directives *SlideDirectives) {
	if notes, ok := yamlData["notes"].(string); ok {
		directives.Notes = notes
	}
	for _, field := range directiveFields {
		field.apply(yamlData, directives)
	}
}

// metaPattern matches one {key: value, ...} group at the end of an info
// string. splitCodeFenceInfo applies it repeatedly, so an info string can
// carry several groups, such as sql {driver: mysql} {2-3}.
var metaPattern = regexp.MustCompile(`\{([^}]*)\}\s*$`)

// pausePattern matches <!-- pause --> markers for fragment splitting.
// Supports variations: <!-- pause -->, <!--pause-->, <!-- pause-->, etc.
var pausePattern = regexp.MustCompile(`(?m)^\s*<!--\s*pause\s*-->\s*$`)

// parseCodeBlockMeta parses the content inside {} in code block info strings.
// Supports both YAML-like (key: value) and simple (key=value) formats.
// Example: "driver: mysql, connection: mydb" or "driver=mysql, connection=mydb"
func parseCodeBlockMeta(content string) CodeBlockMeta {
	meta := CodeBlockMeta{}

	// A meta of digits, commas, dashes and spaces only (e.g. "3-4" or
	// "1,3-5") is a line-highlight spec, not a driver/connection map: it
	// isn't valid YAML flow-map syntax, so it must be checked before the
	// YAML attempt below, which would otherwise just fail silently on it.
	if isHighlightLinesSpec(content) {
		meta.HighlightLines = normalizeHighlightLinesSpec(content)
		return meta
	}

	// Try parsing as YAML first. A "key=value" pair has no colon, so YAML's
	// flow-map syntax reads it as a single key mapped to null (its shorthand
	// for "key: null") rather than failing to parse; when that happens,
	// fall through to the key=value parser below instead of returning an
	// empty meta.
	var yamlData map[string]interface{}
	// Wrap in braces for valid YAML map format
	if err := yaml.Unmarshal([]byte("{"+content+"}"), &yamlData); err == nil {
		if driver, ok := yamlData["driver"].(string); ok {
			meta.Driver = driver
		}
		if connection, ok := yamlData["connection"].(string); ok {
			meta.Connection = connection
		}
		if meta.Driver != "" || meta.Connection != "" || !strings.Contains(content, "=") {
			return meta
		}
		meta = CodeBlockMeta{}
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

// liPattern matches <li> opening tags (with or without attributes).
var liPattern = regexp.MustCompile(`<li(\s[^>]*)?>`)

// autoFragmentListItems transforms HTML to add fragment classes to list items.
// It adds class="fragment fragment-hidden" and data-fragment-index attributes to each <li> element.
// The fragment-hidden class ensures items are hidden initially until revealed by navigation.
// Numbering starts at startIndex. Returns the transformed HTML and the next free index.
func autoFragmentListItems(html string, startIndex int) (string, int) {
	fragmentIndex := startIndex

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

// cssLengthPattern matches a bare CSS length or percentage value: an
// optional sign, digits with an optional decimal part, and a known unit
// (or no unit at all, for "0"). The unit is matched case-insensitively -
// CSS itself treats "300PX" the same as "300px" - everything else about
// the pattern stays case-sensitive. Anything that doesn't match this is
// rejected outright rather than interpolated into a style attribute, since
// an unvalidated value could otherwise close the attribute early or add a
// second CSS declaration; calc(...) is always rejected this way, since it
// is never a bare number and unit.
var cssLengthPattern = regexp.MustCompile(`(?i)^-?\d+(\.\d+)?(px|%|em|rem|vw|vh|vmin|vmax|pt|pc|cm|mm|in|ch|ex)?$`)

// htmlEntityReferencePattern matches the part of an HTML entity reference
// that follows its leading "&": a named reference ("amp;", "lt;", ...), a
// decimal numeric reference ("#39;"), or a hexadecimal one ("#x27;").
var htmlEntityReferencePattern = regexp.MustCompile(`^(#[0-9]+;|#[xX][0-9a-fA-F]+;|[a-zA-Z][a-zA-Z0-9]*;)`)

// escapeAltTextOnce escapes text for use inside an HTML attribute the same
// way html.EscapeString does, except an "&" that already begins a valid
// HTML entity reference is left alone instead of being escaped into
// "&amp;...". Alt text reaches this function as raw markdown source, not
// goldmark output, so an entity reference already present ("AT&amp;T")
// came from the author typing it that way, not from a previous escaping
// pass - escaping its "&" again would turn it into "&amp;amp;T", visibly
// wrong once rendered. A "&" that doesn't start a valid reference (plain
// "AT&T") is raw text and is escaped normally.
func escapeAltTextOnce(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '&':
			if htmlEntityReferencePattern.MatchString(s[i+1:]) {
				b.WriteByte(c)
				continue
			}
			b.WriteString("&amp;")
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '"':
			b.WriteString("&#34;")
		case '\'':
			b.WriteString("&#39;")
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// transformImageAttributes converts markdown images with attributes to HTML.
// It transforms ![alt](src){width=50%} to <img src="src" alt="alt" style="width: 50%">.
// This pre-processing is needed because goldmark doesn't handle the {attr} syntax.
// slideNumber is used only to name the slide in a warning printed to stderr
// when an attribute value is rejected as unsafe.
func transformImageAttributes(content string, slideNumber int) string {
	images := ParseImages(content)

	for _, img := range images {
		// Only transform if there are attributes to apply
		if img.Attributes.Width == "" && img.Attributes.Position == "" && img.Attributes.Border == "" {
			continue
		}

		// Build style attribute
		var styles []string
		if img.Attributes.Width != "" {
			if cssLengthPattern.MatchString(img.Attributes.Width) {
				styles = append(styles, "width: "+img.Attributes.Width)
			} else {
				fmt.Fprintf(os.Stderr, "warning: slide %d: dropped invalid width value %q on image %q\n", slideNumber, img.Attributes.Width, img.AltText)
			}
		}
		if img.Attributes.Position != "" {
			switch img.Attributes.Position {
			case "left":
				styles = append(styles, "float: left", "margin-right: 1em")
			case "right":
				styles = append(styles, "float: right", "margin-left: 1em")
			case "center":
				styles = append(styles, "display: block", "margin-left: auto", "margin-right: auto")
			}
		}

		if img.Attributes.Border == "none" {
			styles = append(styles, "border: none", "box-shadow: none")
		}

		styleAttr := ""
		if len(styles) > 0 {
			styleAttr = ` style="` + strings.Join(styles, "; ") + `"`
		}

		// Build the HTML img tag. Alt text and the URL are raw markdown
		// source at this point (transformImageAttributes runs before
		// goldmark sees the content), so they are escaped here, the only
		// place this hand-built tag's attributes are assembled, to guard
		// against a quote, "&", "<", or ">" in either one closing the
		// attribute early or breaking the tag out of raw-HTML mode.
		htmlImg := `<img src="` + stdhtml.EscapeString(img.URL) + `" alt="` + escapeAltTextOnce(img.AltText) + `"` + styleAttr + `>`

		// Replace the markdown image with HTML
		content = strings.Replace(content, img.Raw, htmlImg, 1)
	}

	return content
}

// asciinemaInfoPattern matches the info string of an asciinema fenced code block.
// Captures: (1) metadata content inside braces
// Example: ```asciinema {src: "./demo.cast", autoPlay: true}
var asciinemaInfoPattern = regexp.MustCompile("(?m)^```asciinema\\s*\\{([^}]*)\\}")

// transformAsciinemaBlocks moves asciinema info string metadata into the code block body.
// This is needed because goldmark discards everything after the language name in the info string.
// Transforms: ```asciinema {src: "./demo.cast", autoPlay: true}
//
//	```
//
// Into:       ```asciinema
//
//	src: ./demo.cast
//	autoPlay: true
//	```
func transformAsciinemaBlocks(content string) string {
	return asciinemaInfoPattern.ReplaceAllStringFunc(content, func(match string) string {
		submatches := asciinemaInfoPattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}

		metaContent := strings.TrimSpace(submatches[1])

		// Parse the YAML-like key: value pairs from the meta
		var lines []string
		pairs := strings.Split(metaContent, ",")
		for _, pair := range pairs {
			pair = strings.TrimSpace(pair)
			if pair != "" {
				lines = append(lines, pair)
			}
		}

		return "```asciinema\n" + strings.Join(lines, "\n")
	})
}
