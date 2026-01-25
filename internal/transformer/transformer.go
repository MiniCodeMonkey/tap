// Package transformer converts parsed presentations into frontend-ready format.
package transformer

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tapsh/tap/internal/config"
	"github.com/tapsh/tap/internal/parser"
)

// TransformedPresentation is the JSON-serializable output for the frontend.
type TransformedPresentation struct {
	Config config.Config      `json:"config"`
	Slides []TransformedSlide `json:"slides"`
}

// TransformedSlide represents a slide ready for frontend rendering.
type TransformedSlide struct {
	Background *BackgroundConfig      `json:"background,omitempty"`
	Layout     string                 `json:"layout"`
	HTML       string                 `json:"html"`
	Transition string                 `json:"transition,omitempty"`
	Notes      string                 `json:"notes,omitempty"`
	CodeBlocks []TransformedCodeBlock `json:"codeBlocks,omitempty"`
	Fragments  []TransformedFragment  `json:"fragments,omitempty"`
	Index      int                    `json:"index"`
}

// TransformedCodeBlock represents a code block ready for frontend rendering.
type TransformedCodeBlock struct {
	Language   string `json:"language"`
	Code       string `json:"code"`
	Driver     string `json:"driver,omitempty"`
	Connection string `json:"connection,omitempty"`
}

// TransformedFragment represents a fragment group for incremental reveals.
type TransformedFragment struct {
	Content string `json:"content"`
	Index   int    `json:"index"`
}

// BackgroundConfig holds background styling for a slide.
type BackgroundConfig struct {
	Value string `json:"value"`
	Type  string `json:"type"` // "color", "image", or "gradient"
}

// Transformer converts parser.Presentation to TransformedPresentation.
type Transformer struct {
	config  *config.Config
	baseDir string // Base directory for resolving relative paths
}

// New creates a new Transformer with the given configuration.
func New(cfg *config.Config) *Transformer {
	return &Transformer{
		config: cfg,
	}
}

// NewWithBaseDir creates a new Transformer with the given configuration and base directory.
// The base directory is used for resolving relative image paths.
func NewWithBaseDir(cfg *config.Config, baseDir string) *Transformer {
	return &Transformer{
		config:  cfg,
		baseDir: baseDir,
	}
}

// SetBaseDir sets the base directory for resolving relative paths.
func (t *Transformer) SetBaseDir(baseDir string) {
	t.baseDir = baseDir
}

// Transform converts a parsed Presentation into a TransformedPresentation
// suitable for JSON serialization and frontend consumption.
func (t *Transformer) Transform(pres *parser.Presentation) *TransformedPresentation {
	result := &TransformedPresentation{
		Config: *t.config,
		Slides: make([]TransformedSlide, 0, len(pres.Slides)),
	}

	for _, slide := range pres.Slides {
		transformed := t.transformSlide(slide)
		result.Slides = append(result.Slides, transformed)
	}

	return result
}

// transformSlide converts a single parser.Slide to TransformedSlide.
func (t *Transformer) transformSlide(slide parser.Slide) TransformedSlide {
	layout := t.resolveLayout(slide)
	html := t.resolveImagePaths(slide.HTML)

	// Process HTML for layouts that use ||| column separator
	if layout == "two-column" || layout == "split-media" || layout == "sidebar" {
		html = processTwoColumnHTML(html)
	} else if layout == "three-column" {
		html = processThreeColumnHTML(html)
	}

	transformed := TransformedSlide{
		Index:  slide.Index,
		HTML:   html,
		Layout: layout,
		Notes:  slide.Directives.Notes,
	}

	// Set transition (per-slide directive overrides global config)
	if slide.Directives.Transition != "" {
		transformed.Transition = slide.Directives.Transition
	} else {
		transformed.Transition = t.config.Transition
	}

	// Transform fragments
	if len(slide.Fragments) > 0 {
		transformed.Fragments = make([]TransformedFragment, len(slide.Fragments))
		for i, frag := range slide.Fragments {
			transformed.Fragments[i] = TransformedFragment{
				Content: frag.Content,
				Index:   frag.Index,
			}
		}
	}

	// Transform code blocks
	if len(slide.CodeBlocks) > 0 {
		transformed.CodeBlocks = make([]TransformedCodeBlock, len(slide.CodeBlocks))
		for i, block := range slide.CodeBlocks {
			transformed.CodeBlocks[i] = TransformedCodeBlock{
				Language:   block.Language,
				Code:       block.Code,
				Driver:     block.Meta.Driver,
				Connection: block.Meta.Connection,
			}
		}
	}

	// Transform background
	if slide.Directives.Background != "" {
		transformed.Background = t.parseBackground(slide.Directives.Background)
	}

	return transformed
}

// resolveLayout determines the layout for a slide.
// If a layout directive is specified, it takes precedence.
// Otherwise, auto-detects layout based on content.
func (t *Transformer) resolveLayout(slide parser.Slide) string {
	if slide.Directives.Layout != "" {
		return slide.Directives.Layout
	}
	return detectLayout(slide)
}

// detectLayout auto-detects the appropriate layout based on slide content.
// Detection priority:
//  1. two-column: contains ||| separator
//  2. title: only H1, optional subtitle (paragraph or small text)
//  3. section: only H2 (large section header)
//  4. code-focus: single code block taking >50% of content
//  5. quote: blockquote as primary content
//  6. default: everything else
func detectLayout(slide parser.Slide) string {
	html := slide.HTML
	content := slide.Content

	// Check for two-column layout (||| separator in content)
	if containsTwoColumnSeparator(content) {
		return "two-column"
	}

	// Check for title layout (only H1, optional subtitle)
	if isTitleLayout(html) {
		return "title"
	}

	// Check for section layout (only H2)
	if isSectionLayout(html) {
		return "section"
	}

	// Check for code-focus layout (single code block >50% content)
	if isCodeFocusLayout(slide) {
		return "code-focus"
	}

	// Check for quote layout (blockquote as primary content)
	if isQuoteLayout(html) {
		return "quote"
	}

	return "default"
}

// containsTwoColumnSeparator checks if the content has a ||| column separator.
func containsTwoColumnSeparator(content string) bool {
	// Look for ||| on its own line or as a separator
	for i := 0; i <= len(content)-3; i++ {
		if content[i:i+3] == "|||" {
			return true
		}
	}
	return false
}

// isTitleLayout checks if the HTML contains only an H1, with an optional subtitle.
// Subtitle can be a paragraph (<p>) following the H1.
func isTitleLayout(html string) bool {
	// Must have exactly one H1
	h1Count := countHTMLTag(html, "h1")
	if h1Count != 1 {
		return false
	}

	// Must not have H2-H6
	for _, tag := range []string{"h2", "h3", "h4", "h5", "h6"} {
		if countHTMLTag(html, tag) > 0 {
			return false
		}
	}

	// Count other significant content elements
	pCount := countHTMLTag(html, "p")
	ulCount := countHTMLTag(html, "ul")
	olCount := countHTMLTag(html, "ol")
	preCount := countHTMLTag(html, "pre")
	blockquoteCount := countHTMLTag(html, "blockquote")
	tableCount := countHTMLTag(html, "table")

	// Allow at most one paragraph (subtitle) and no other block content
	if pCount > 1 || ulCount > 0 || olCount > 0 || preCount > 0 || blockquoteCount > 0 || tableCount > 0 {
		return false
	}

	return true
}

// isSectionLayout checks if the HTML contains only an H2.
func isSectionLayout(html string) bool {
	// Must have exactly one H2
	h2Count := countHTMLTag(html, "h2")
	if h2Count != 1 {
		return false
	}

	// Must not have H1 or other headers
	for _, tag := range []string{"h1", "h3", "h4", "h5", "h6"} {
		if countHTMLTag(html, tag) > 0 {
			return false
		}
	}

	// Must not have significant other content
	pCount := countHTMLTag(html, "p")
	ulCount := countHTMLTag(html, "ul")
	olCount := countHTMLTag(html, "ol")
	preCount := countHTMLTag(html, "pre")
	blockquoteCount := countHTMLTag(html, "blockquote")
	tableCount := countHTMLTag(html, "table")

	if pCount > 0 || ulCount > 0 || olCount > 0 || preCount > 0 || blockquoteCount > 0 || tableCount > 0 {
		return false
	}

	return true
}

// isCodeFocusLayout checks if the slide has a single code block taking >50% of content.
func isCodeFocusLayout(slide parser.Slide) bool {
	// Must have exactly one code block
	if len(slide.CodeBlocks) != 1 {
		return false
	}

	// Check if code block is >50% of the total content
	codeLen := len(slide.CodeBlocks[0].Code)
	totalLen := len(slide.Content)

	// Avoid division by zero
	if totalLen == 0 {
		return false
	}

	// Code must be more than 50% of content
	return float64(codeLen)/float64(totalLen) > 0.5
}

// isQuoteLayout checks if the HTML has a blockquote as the primary content.
func isQuoteLayout(html string) bool {
	// Must have at least one blockquote
	blockquoteCount := countHTMLTag(html, "blockquote")
	if blockquoteCount == 0 {
		return false
	}

	// Must not have headers (quotes shouldn't have headers as main content)
	for _, tag := range []string{"h1", "h2", "h3", "h4", "h5", "h6"} {
		if countHTMLTag(html, tag) > 0 {
			return false
		}
	}

	// Must not have code blocks or tables
	preCount := countHTMLTag(html, "pre")
	tableCount := countHTMLTag(html, "table")

	if preCount > 0 || tableCount > 0 {
		return false
	}

	// Allow paragraphs (often for attribution) and lists
	return true
}

// countHTMLTag counts occurrences of an HTML tag (opening tags only).
func countHTMLTag(html, tag string) int {
	count := 0
	openTag := "<" + tag
	openTagLen := len(openTag)

	for i := 0; i <= len(html)-openTagLen; i++ {
		if html[i:i+openTagLen] == openTag {
			// Check that it's followed by > or space (not a different tag like <h10>)
			if i+openTagLen < len(html) {
				nextChar := html[i+openTagLen]
				if nextChar == '>' || nextChar == ' ' || nextChar == '\t' || nextChar == '\n' {
					count++
				}
			} else if i+openTagLen == len(html) {
				// Tag at end of string (malformed but count it)
				count++
			}
		}
	}
	return count
}

// columnSeparatorPattern matches ||| in HTML, possibly wrapped in <p> tags.
var columnSeparatorPattern = regexp.MustCompile(`(?s)<p>\s*\|\|\|\s*</p>|\|\|\|`)

// processTwoColumnHTML transforms HTML content for two-column layout.
// It finds the ||| separator and wraps content before and after in column divs.
func processTwoColumnHTML(html string) string {
	// Find the separator
	loc := columnSeparatorPattern.FindStringIndex(html)
	if loc == nil {
		// No separator found, return as-is
		return html
	}

	// Split at the first separator
	leftContent := strings.TrimSpace(html[:loc[0]])
	rightContent := strings.TrimSpace(html[loc[1]:])

	// Check for a second separator (for content before the columns)
	loc2 := columnSeparatorPattern.FindStringIndex(rightContent)
	if loc2 != nil {
		// Three parts: header, left column, right column
		// The first part (leftContent) is the header
		// The middle part is the actual left column
		// The last part is the right column
		headerContent := leftContent
		leftContent = strings.TrimSpace(rightContent[:loc2[0]])
		rightContent = strings.TrimSpace(rightContent[loc2[1]:])

		return headerContent + "\n" +
			`<div class="column column-left">` + leftContent + `</div>` +
			`<div class="column column-right">` + rightContent + `</div>`
	}

	// Two parts: left column, right column
	return `<div class="column column-left">` + leftContent + `</div>` +
		`<div class="column column-right">` + rightContent + `</div>`
}

// processThreeColumnHTML transforms HTML content for three-column layout.
// It finds the ||| separators and wraps content in column divs.
func processThreeColumnHTML(html string) string {
	// Find the first separator
	loc1 := columnSeparatorPattern.FindStringIndex(html)
	if loc1 == nil {
		// No separator found, return as-is
		return html
	}

	// Split at the first separator
	leftContent := strings.TrimSpace(html[:loc1[0]])
	remaining := strings.TrimSpace(html[loc1[1]:])

	// Find the second separator
	loc2 := columnSeparatorPattern.FindStringIndex(remaining)
	if loc2 == nil {
		// Only two parts, treat as two-column
		return `<div class="column">` + leftContent + `</div>` +
			`<div class="column">` + remaining + `</div>`
	}

	// Three parts
	middleContent := strings.TrimSpace(remaining[:loc2[0]])
	rightContent := strings.TrimSpace(remaining[loc2[1]:])

	return `<div class="column">` + leftContent + `</div>` +
		`<div class="column">` + middleContent + `</div>` +
		`<div class="column">` + rightContent + `</div>`
}

// parseBackground parses a background directive value and determines its type.
func (t *Transformer) parseBackground(value string) *BackgroundConfig {
	// Detect background type based on value format
	bgType := "color"
	resolvedValue := value

	// Check for image (URL or file path)
	if isImageURL(value) {
		bgType = "image"
		// Resolve relative image paths to /local/ URLs
		resolvedValue = t.resolveImagePath(value)
	} else if isGradient(value) {
		bgType = "gradient"
	}

	return &BackgroundConfig{
		Value: resolvedValue,
		Type:  bgType,
	}
}

// isImageURL checks if the value looks like an image URL or file path.
func isImageURL(value string) bool {
	// Check for common image extensions
	imageExtensions := []string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp"}
	for _, ext := range imageExtensions {
		if len(value) > len(ext) && value[len(value)-len(ext):] == ext {
			return true
		}
	}

	// Check for URL protocols
	if len(value) > 8 && (value[:7] == "http://" || value[:8] == "https://") {
		return true
	}

	return false
}

// isGradient checks if the value looks like a CSS gradient.
func isGradient(value string) bool {
	gradientPrefixes := []string{"linear-gradient(", "radial-gradient(", "conic-gradient("}
	for _, prefix := range gradientPrefixes {
		if len(value) >= len(prefix) && value[:len(prefix)] == prefix {
			return true
		}
	}
	return false
}

// imgSrcPattern matches img src attributes in HTML.
// Captures the entire img tag and the src attribute value.
var imgSrcPattern = regexp.MustCompile(`(<img\s[^>]*src=["'])([^"']+)(["'][^>]*>)`)

// supportedImageExtensions lists all supported image formats.
var supportedImageExtensions = []string{".png", ".jpg", ".jpeg", ".gif", ".svg", ".webp"}

// resolveImagePaths processes HTML and resolves all image src paths.
func (t *Transformer) resolveImagePaths(html string) string {
	// If no base directory is set, return HTML unchanged
	if t.baseDir == "" {
		return html
	}

	return imgSrcPattern.ReplaceAllStringFunc(html, func(match string) string {
		submatches := imgSrcPattern.FindStringSubmatch(match)
		if len(submatches) != 4 {
			return match
		}

		prefix := submatches[1]  // <img ... src="
		src := submatches[2]     // the path
		suffix := submatches[3]  // " ...>

		resolved := t.resolveImagePath(src)
		return prefix + resolved + suffix
	})
}

// resolveImagePath resolves a single image path to a URL for the dev server.
// It handles:
// - Absolute URLs (https://, http://) - returned unchanged
// - Absolute file paths (starting with /) - returned unchanged
// - Relative paths - converted to /local/... URL for dev server
func (t *Transformer) resolveImagePath(path string) string {
	// Return unchanged if path is empty
	if path == "" {
		return path
	}

	// Check if it's an absolute URL
	if isAbsoluteURL(path) {
		return path
	}

	// Check if it's an absolute file path
	if filepath.IsAbs(path) {
		return path
	}

	// Check if it's a supported image format
	if !isSupportedImageFormat(path) {
		return path
	}

	// If no base directory set, return unchanged
	if t.baseDir == "" {
		return path
	}

	// Convert to /local/ URL for the dev server
	// Clean the path to remove . and .. components
	cleanPath := filepath.Clean(path)
	// Convert Windows backslashes to forward slashes for URL
	cleanPath = strings.ReplaceAll(cleanPath, "\\", "/")
	// Remove leading ./ if present
	cleanPath = strings.TrimPrefix(cleanPath, "./")

	return "/local/" + cleanPath
}

// isAbsoluteURL checks if the path is an absolute URL (http:// or https://).
func isAbsoluteURL(path string) bool {
	lowerPath := strings.ToLower(path)
	return strings.HasPrefix(lowerPath, "http://") || strings.HasPrefix(lowerPath, "https://")
}

// isSupportedImageFormat checks if the path has a supported image extension.
func isSupportedImageFormat(path string) bool {
	lowerPath := strings.ToLower(path)
	for _, ext := range supportedImageExtensions {
		if strings.HasSuffix(lowerPath, ext) {
			return true
		}
	}
	return false
}
