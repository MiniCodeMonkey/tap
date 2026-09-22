// Package transformer converts parsed presentations into frontend-ready format.
package transformer

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/MiniCodeMonkey/tap/internal/components"
	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/parser"
)

// TransformedPresentation is the JSON-serializable output for the frontend.
type TransformedPresentation struct {
	Config config.Config      `json:"config"`
	Slides []TransformedSlide `json:"slides"`
}

// TransformedSlide represents a slide ready for frontend rendering.
type TransformedSlide struct {
	Background    *BackgroundConfig      `json:"background,omitempty"`
	Layout        string                 `json:"layout"`
	HTML          string                 `json:"html"`
	Slots         map[string]string      `json:"slots"`
	SlotOrder     []string               `json:"slotOrder"`
	FragmentCount int                    `json:"fragmentCount"`
	Steps         int                    `json:"steps"`
	Transition    string                 `json:"transition,omitempty"`
	Notes         string                 `json:"notes,omitempty"`
	Tag           string                 `json:"tag,omitempty"`
	Badge         string                 `json:"badge,omitempty"`
	CodeBlocks    []TransformedCodeBlock `json:"codeBlocks,omitempty"`
	Index         int                    `json:"index"`
	Scroll        bool                   `json:"scroll,omitempty"`
	ScrollSpeed   int                    `json:"scrollSpeed,omitempty"`
	// Component describes the whole-slide component when Layout is
	// "component" (the slide's layout directive names a component file).
	Component *WholeSlideComponent `json:"component,omitempty"`
	// Components describes each inline ```component fence found on the
	// slide, in document order.
	Components []InlineComponent `json:"components,omitempty"`
	// Hash identifies the slide's content (see SlideHash). The frontend
	// keeps a slide it already rendered when the slide at the same
	// position has the same hash, and tap dev lists the slides whose hash
	// changed in its "update" message.
	Hash string `json:"hash"`
	// StepsInvalid carries parser.SlideDirectives.StepsInvalid through to
	// layouts.Validate, which turns it into a slide warning; it is not
	// part of the frontend's slide JSON.
	StepsInvalid bool `json:"-"`
}

// WholeSlideComponent is the slide JSON shape for a layout directive that
// names a component file.
type WholeSlideComponent struct {
	Source string `json:"source"`
	URL    string `json:"url,omitempty"`
	CSS    string `json:"css,omitempty"`
	// Error is the formatted build error when the component failed to
	// build; the frontend shows an error card instead of rendering it.
	Error string `json:"error,omitempty"`
}

// InlineComponent is the slide JSON shape for one ```component fence.
type InlineComponent struct {
	Index  int             `json:"index"`
	Source string          `json:"source"`
	URL    string          `json:"url,omitempty"`
	CSS    string          `json:"css,omitempty"`
	Props  json.RawMessage `json:"props"`
	// Error is the formatted build error when the component failed to
	// build; the frontend shows an error card instead of rendering it.
	Error string `json:"error,omitempty"`
}

// TransformedCodeBlock represents a code block ready for frontend rendering.
type TransformedCodeBlock struct {
	Language       string `json:"language"`
	Code           string `json:"code"`
	Driver         string `json:"driver,omitempty"`
	Connection     string `json:"connection,omitempty"`
	HighlightLines string `json:"highlightLines,omitempty"`
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
	// components holds each distinct component path's build result, set by
	// SetComponents before Transform. A nil map means no component was
	// resolved for any path (every lookup misses, which transformSlide
	// treats as "not resolved").
	components map[string]components.Result
	// componentURLPrefix is prepended to a bundle's "<name>-<hash>.<ext>"
	// to build its URL. Empty means the dev server default ("/components/");
	// the static builder sets a relative "components/" so the built folder
	// works from any base path (see the builder's image path handling).
	componentURLPrefix string
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

// SetComponents provides the build result for every distinct component
// path the presentation's slides use (see internal/components.Resolve), so
// Transform can fill in component URLs, CSS, and errors.
func (t *Transformer) SetComponents(resolved map[string]components.Result) {
	t.components = resolved
}

// SetComponentURLPrefix sets the prefix used to build a component bundle's
// URL, replacing the dev server default of "/components/". The static
// builder passes a relative "components/" so the built output works when
// served from any base path.
func (t *Transformer) SetComponentURLPrefix(prefix string) {
	t.componentURLPrefix = prefix
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
		transformed.Hash = SlideHash(transformed)
		result.Slides = append(result.Slides, transformed)
	}

	return result
}

// SlideHash returns a short hash of everything the frontend renders for
// slide: its JSON with Index and Hash left out, so a slide that only moved
// keeps its hash. json.Marshal sorts map keys, so equal slides always give
// equal hashes. Returns "" if marshalling fails, which cannot realistically
// happen for this struct; callers that compare two hashes (see ChangedSlides
// in internal/server/revision.go) must never treat two empty hashes as
// equal, or a slide whose hash could not be computed would be reported
// unchanged and never re-rendered.
func SlideHash(slide TransformedSlide) string {
	slide.Index = 0
	slide.Hash = ""
	data, err := json.Marshal(slide)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:6])
}

// transformSlide converts a single parser.Slide to TransformedSlide.
func (t *Transformer) transformSlide(slide parser.Slide) TransformedSlide {
	html := t.resolveImagePaths(slide.HTML)
	html = t.resolveAsciinemaPaths(html)

	slots := make(map[string]string, len(slide.Slots))
	for name, slotHTML := range slide.Slots {
		resolved := t.resolveImagePaths(slotHTML)
		resolved = t.resolveAsciinemaPaths(resolved)
		slots[name] = resolved
	}

	transformed := TransformedSlide{
		Index:         slide.Index,
		HTML:          html,
		Slots:         slots,
		SlotOrder:     slide.SlotOrder,
		FragmentCount: slide.FragmentCount,
		Notes:         slide.Directives.Notes,
		Tag:           slide.Directives.Tag,
		Badge:         slide.Directives.Badge,
		StepsInvalid:  slide.Directives.StepsInvalid,
	}

	if components.IsComponentPath(slide.Directives.Layout) {
		transformed.Layout = "component"
		transformed.Component = t.buildWholeSlideComponent(slide.Directives.Layout)
	} else {
		transformed.Layout = t.resolveLayout(slide)
	}

	if len(slide.Components) > 0 {
		transformed.Components = make([]InlineComponent, len(slide.Components))
		for i, component := range slide.Components {
			transformed.Components[i] = t.buildInlineComponent(component)
		}
	}

	transformed.Steps = t.countSteps(slide)

	// Set transition (per-slide directive overrides global config)
	if slide.Directives.Transition != "" {
		transformed.Transition = slide.Directives.Transition
	} else {
		transformed.Transition = t.config.Transition
	}

	// Transform code blocks
	if len(slide.CodeBlocks) > 0 {
		transformed.CodeBlocks = make([]TransformedCodeBlock, len(slide.CodeBlocks))
		for i, block := range slide.CodeBlocks {
			transformed.CodeBlocks[i] = TransformedCodeBlock{
				Language:       block.Language,
				Code:           block.Code,
				Driver:         block.Meta.Driver,
				Connection:     block.Meta.Connection,
				HighlightLines: block.Meta.HighlightLines,
			}
		}
	}

	// Transform background
	if slide.Directives.Background != "" {
		transformed.Background = t.parseBackground(slide.Directives.Background)
	}

	// Transform scroll settings
	if slide.Directives.Scroll {
		transformed.Scroll = true
		if slide.Directives.ScrollSpeed > 0 {
			transformed.ScrollSpeed = slide.Directives.ScrollSpeed
		}
	}

	return transformed
}

// countSteps returns the number of clicker presses that step-driven content
// on the slide uses. The slide's own "steps" directive always wins.
// Otherwise: a "map" fence gives a floor of 1 (the pre-existing rule); a
// whole-slide component's static "export const steps" export and, for
// inline components, the maximum across all of them, raise it further.
func (t *Transformer) countSteps(slide parser.Slide) int {
	if slide.Directives.HasSteps {
		return slide.Directives.Steps
	}

	steps := 0
	for _, block := range slide.CodeBlocks {
		if block.Language == "map" {
			steps = 1
		}
	}

	if components.IsComponentPath(slide.Directives.Layout) {
		if bundle := t.resolvedBundle(slide.Directives.Layout); bundle != nil && bundle.HasStepsExport && bundle.Steps > steps {
			steps = bundle.Steps
		}
	}

	for _, component := range slide.Components {
		if bundle := t.resolvedBundle(component.Source); bundle != nil && bundle.HasStepsExport && bundle.Steps > steps {
			steps = bundle.Steps
		}
	}

	return steps
}

// resolvedBundle looks up a component source path's build result, and
// returns its Bundle, or nil when the path wasn't resolved or failed to
// build.
func (t *Transformer) resolvedBundle(source string) *components.Bundle {
	result, ok := t.components[source]
	if !ok {
		return nil
	}
	return result.Bundle
}

// componentURL builds a bundle's URL for the given extension ("js" or
// "css"), using componentURLPrefix (or the dev server default).
func (t *Transformer) componentURL(bundle *components.Bundle, extension string) string {
	prefix := t.componentURLPrefix
	if prefix == "" {
		prefix = "/components/"
	}
	return prefix + bundle.Name + "-" + bundle.Hash + "." + extension
}

// buildWholeSlideComponent builds the slide JSON's "component" field for a
// layout directive that names a component file.
func (t *Transformer) buildWholeSlideComponent(source string) *WholeSlideComponent {
	component := &WholeSlideComponent{Source: source}

	result, ok := t.components[source]
	if !ok {
		component.Error = "component not resolved"
		return component
	}
	if result.Bundle == nil {
		component.Error = joinBuildErrors(result.Errors)
		return component
	}

	component.URL = t.componentURL(result.Bundle, "js")
	if len(result.Bundle.CSS) > 0 {
		component.CSS = t.componentURL(result.Bundle, "css")
	}
	return component
}

// buildInlineComponent builds one entry of the slide JSON's "components"
// array for a ```component fence.
func (t *Transformer) buildInlineComponent(component parser.Component) InlineComponent {
	inline := InlineComponent{
		Index:  component.Index,
		Source: component.Source,
		Props:  component.Props,
	}

	result, ok := t.components[component.Source]
	if !ok {
		inline.Error = "component not resolved"
		return inline
	}
	if result.Bundle == nil {
		inline.Error = joinBuildErrors(result.Errors)
		return inline
	}

	inline.URL = t.componentURL(result.Bundle, "js")
	if len(result.Bundle.CSS) > 0 {
		inline.CSS = t.componentURL(result.Bundle, "css")
	}
	return inline
}

// joinBuildErrors joins a component's build errors into the single message
// a slide's component.error or components[i].error carries, one per line
// in the spec's "<file>:<line>:<column>: <message>" format.
func joinBuildErrors(buildErrors []components.BuildError) string {
	lines := make([]string, len(buildErrors))
	for i, buildError := range buildErrors {
		lines[i] = buildError.Error()
	}
	return strings.Join(lines, "\n")
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
//  1. three-column: slots named left, center, and right
//  2. two-column: slots named left and right
//  3. title: only H1, optional subtitle (paragraph or small text)
//  4. section: only H2 (large section header)
//  5. code-focus: single code block taking >50% of content
//  6. quote: blockquote as primary content
//  7. default: everything else
func detectLayout(slide parser.Slide) string {
	html := slide.HTML

	// Check for column layouts based on slot names.
	_, hasLeft := slide.Slots["left"]
	_, hasRight := slide.Slots["right"]
	_, hasCenter := slide.Slots["center"]
	if hasLeft && hasCenter && hasRight {
		return "three-column"
	}
	if hasLeft && hasRight {
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

		prefix := submatches[1] // <img ... src="
		src := submatches[2]    // the path
		suffix := submatches[3] // " ...>

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

// asciinemaBlockPattern matches asciinema code blocks and captures the content.
var asciinemaBlockPattern = regexp.MustCompile(`<code class="language-asciinema">([\s\S]*?)</code>`)

// asciinemaSrcPattern matches "src: path" lines in asciinema block content.
var asciinemaSrcPattern = regexp.MustCompile(`(?m)^src:\s*(?:&quot;|"|')?([^"'&\n]+)(?:&quot;|"|')?$`)

// resolveAsciinemaPaths processes HTML and resolves .cast file paths in asciinema blocks.
func (t *Transformer) resolveAsciinemaPaths(html string) string {
	if t.baseDir == "" {
		return html
	}

	return asciinemaBlockPattern.ReplaceAllStringFunc(html, func(match string) string {
		submatches := asciinemaBlockPattern.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		content := submatches[1]

		newContent := asciinemaSrcPattern.ReplaceAllStringFunc(content, func(srcLine string) string {
			srcMatches := asciinemaSrcPattern.FindStringSubmatch(srcLine)
			if len(srcMatches) < 2 {
				return srcLine
			}
			path := strings.TrimSpace(srcMatches[1])

			if isAbsoluteURL(path) || filepath.IsAbs(path) {
				return srcLine
			}

			cleanPath := filepath.Clean(path)
			cleanPath = strings.ReplaceAll(cleanPath, "\\", "/")
			cleanPath = strings.TrimPrefix(cleanPath, "./")
			return "src: /local/" + cleanPath
		})

		return `<code class="language-asciinema">` + newContent + `</code>`
	})
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
