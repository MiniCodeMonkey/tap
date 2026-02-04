package transformer

import (
	"encoding/json"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/parser"
)

func TestNewTransformer(t *testing.T) {
	cfg := config.DefaultConfig()
	tr := New(cfg)

	if tr == nil {
		t.Fatal("New() returned nil")
	}
	if tr.config != cfg {
		t.Error("transformer config not set correctly")
	}
}

func TestTransformEmptyPresentation(t *testing.T) {
	cfg := config.DefaultConfig()
	tr := New(cfg)

	pres := &parser.Presentation{
		Slides: []parser.Slide{},
	}

	result := tr.Transform(pres)

	if result == nil {
		t.Fatal("Transform() returned nil")
	}
	if len(result.Slides) != 0 {
		t.Errorf("expected 0 slides, got %d", len(result.Slides))
	}
}

func TestTransformSingleSlide(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Title = "Test Presentation"
	cfg.Theme = "minimal"
	tr := New(cfg)

	pres := &parser.Presentation{
		Slides: []parser.Slide{
			{
				Index:   0,
				Content: "# Hello World",
				HTML:    "<h1>Hello World</h1>",
			},
		},
	}

	result := tr.Transform(pres)

	if len(result.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(result.Slides))
	}

	slide := result.Slides[0]
	if slide.Index != 0 {
		t.Errorf("expected index 0, got %d", slide.Index)
	}
	if slide.HTML != "<h1>Hello World</h1>" {
		t.Errorf("unexpected HTML: %q", slide.HTML)
	}
	// Auto-detection now identifies single H1 as title layout
	if slide.Layout != "title" {
		t.Errorf("expected layout 'title', got %q", slide.Layout)
	}
}

func TestTransformWithDirectives(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Transition = "fade"
	tr := New(cfg)

	pres := &parser.Presentation{
		Slides: []parser.Slide{
			{
				Index: 0,
				HTML:  "<h1>Title</h1>",
				Directives: parser.SlideDirectives{
					Layout:     "title",
					Transition: "slide",
					Notes:      "Speaker notes here",
					Background: "#ff0000",
				},
			},
		},
	}

	result := tr.Transform(pres)
	slide := result.Slides[0]

	if slide.Layout != "title" {
		t.Errorf("expected layout 'title', got %q", slide.Layout)
	}
	if slide.Transition != "slide" {
		t.Errorf("expected transition 'slide', got %q", slide.Transition)
	}
	if slide.Notes != "Speaker notes here" {
		t.Errorf("expected notes 'Speaker notes here', got %q", slide.Notes)
	}
	if slide.Background == nil {
		t.Fatal("expected background to be set")
	}
	if slide.Background.Value != "#ff0000" {
		t.Errorf("expected background value '#ff0000', got %q", slide.Background.Value)
	}
	if slide.Background.Type != "color" {
		t.Errorf("expected background type 'color', got %q", slide.Background.Type)
	}
}

func TestTransformWithDefaultTransition(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Transition = "zoom"
	tr := New(cfg)

	pres := &parser.Presentation{
		Slides: []parser.Slide{
			{
				Index: 0,
				HTML:  "<p>Content</p>",
				// No directive transition specified
			},
		},
	}

	result := tr.Transform(pres)
	slide := result.Slides[0]

	// Should use global config transition
	if slide.Transition != "zoom" {
		t.Errorf("expected transition 'zoom' from config, got %q", slide.Transition)
	}
}

func TestTransformWithFragments(t *testing.T) {
	cfg := config.DefaultConfig()
	tr := New(cfg)

	pres := &parser.Presentation{
		Slides: []parser.Slide{
			{
				Index: 0,
				HTML:  "<p>Content</p>",
				Fragments: []parser.Fragment{
					{Index: 0, Content: "First point"},
					{Index: 1, Content: "Second point"},
					{Index: 2, Content: "Third point"},
				},
			},
		},
	}

	result := tr.Transform(pres)
	slide := result.Slides[0]

	if len(slide.Fragments) != 3 {
		t.Fatalf("expected 3 fragments, got %d", len(slide.Fragments))
	}

	for i, frag := range slide.Fragments {
		if frag.Index != i {
			t.Errorf("fragment %d: expected index %d, got %d", i, i, frag.Index)
		}
	}

	if slide.Fragments[0].Content != "First point" {
		t.Errorf("fragment 0: unexpected content %q", slide.Fragments[0].Content)
	}
}

func TestTransformWithCodeBlocks(t *testing.T) {
	cfg := config.DefaultConfig()
	tr := New(cfg)

	pres := &parser.Presentation{
		Slides: []parser.Slide{
			{
				Index: 0,
				HTML:  "<pre><code>SELECT * FROM users</code></pre>",
				CodeBlocks: []parser.CodeBlock{
					{
						Language: "sql",
						Code:     "SELECT * FROM users",
						Meta: parser.CodeBlockMeta{
							Driver:     "mysql",
							Connection: "prod",
						},
					},
				},
			},
		},
	}

	result := tr.Transform(pres)
	slide := result.Slides[0]

	if len(slide.CodeBlocks) != 1 {
		t.Fatalf("expected 1 code block, got %d", len(slide.CodeBlocks))
	}

	block := slide.CodeBlocks[0]
	if block.Language != "sql" {
		t.Errorf("expected language 'sql', got %q", block.Language)
	}
	if block.Code != "SELECT * FROM users" {
		t.Errorf("unexpected code content: %q", block.Code)
	}
	if block.Driver != "mysql" {
		t.Errorf("expected driver 'mysql', got %q", block.Driver)
	}
	if block.Connection != "prod" {
		t.Errorf("expected connection 'prod', got %q", block.Connection)
	}
}

func TestTransformMultipleSlides(t *testing.T) {
	cfg := config.DefaultConfig()
	tr := New(cfg)

	pres := &parser.Presentation{
		Slides: []parser.Slide{
			{Index: 0, HTML: "<h1>Slide 1</h1>"},
			{Index: 1, HTML: "<h2>Slide 2</h2>"},
			{Index: 2, HTML: "<p>Slide 3</p>"},
		},
	}

	result := tr.Transform(pres)

	if len(result.Slides) != 3 {
		t.Fatalf("expected 3 slides, got %d", len(result.Slides))
	}

	for i, slide := range result.Slides {
		if slide.Index != i {
			t.Errorf("slide %d: expected index %d, got %d", i, i, slide.Index)
		}
	}
}

func TestTransformConfigIncluded(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Title = "My Presentation"
	cfg.Theme = "gradient"
	cfg.AspectRatio = "4:3"
	tr := New(cfg)

	pres := &parser.Presentation{
		Slides: []parser.Slide{
			{Index: 0, HTML: "<p>Content</p>"},
		},
	}

	result := tr.Transform(pres)

	if result.Config.Title != "My Presentation" {
		t.Errorf("expected title 'My Presentation', got %q", result.Config.Title)
	}
	if result.Config.Theme != "gradient" {
		t.Errorf("expected theme 'gradient', got %q", result.Config.Theme)
	}
	if result.Config.AspectRatio != "4:3" {
		t.Errorf("expected aspectRatio '4:3', got %q", result.Config.AspectRatio)
	}
}

func TestBackgroundTypeDetection(t *testing.T) {
	cfg := config.DefaultConfig()
	tr := New(cfg)

	testCases := []struct {
		value    string
		expected string
	}{
		{"#ff0000", "color"},
		{"red", "color"},
		{"rgb(255, 0, 0)", "color"},
		{"background.png", "image"},
		{"./images/bg.jpg", "image"},
		{"/assets/hero.jpeg", "image"},
		{"image.gif", "image"},
		{"icon.svg", "image"},
		{"photo.webp", "image"},
		{"https://example.com/image.png", "image"},
		{"http://example.com/bg.jpg", "image"},
		{"linear-gradient(to right, red, blue)", "gradient"},
		{"radial-gradient(circle, red, blue)", "gradient"},
		{"conic-gradient(red, blue)", "gradient"},
	}

	for _, tc := range testCases {
		pres := &parser.Presentation{
			Slides: []parser.Slide{
				{
					Index: 0,
					HTML:  "<p>Test</p>",
					Directives: parser.SlideDirectives{
						Background: tc.value,
					},
				},
			},
		}

		result := tr.Transform(pres)
		slide := result.Slides[0]

		if slide.Background == nil {
			t.Errorf("value %q: expected background to be set", tc.value)
			continue
		}
		if slide.Background.Type != tc.expected {
			t.Errorf("value %q: expected type %q, got %q", tc.value, tc.expected, slide.Background.Type)
		}
	}
}

func TestTransformJSONSerializable(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Title = "JSON Test"
	tr := New(cfg)

	pres := &parser.Presentation{
		Slides: []parser.Slide{
			{
				Index: 0,
				HTML:  "<h1>Title</h1>",
				Directives: parser.SlideDirectives{
					Layout:     "title",
					Background: "#000000",
					Notes:      "Test notes",
				},
				Fragments: []parser.Fragment{
					{Index: 0, Content: "Fragment 1"},
				},
				CodeBlocks: []parser.CodeBlock{
					{Language: "go", Code: "fmt.Println(\"hello\")"},
				},
			},
		},
	}

	result := tr.Transform(pres)

	// Attempt to marshal to JSON
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal to JSON: %v", err)
	}

	// Verify we can unmarshal it back
	var unmarshaled TransformedPresentation
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal from JSON: %v", err)
	}

	// Verify some fields survived the round-trip
	if unmarshaled.Config.Title != "JSON Test" {
		t.Errorf("config title not preserved: got %q", unmarshaled.Config.Title)
	}
	if len(unmarshaled.Slides) != 1 {
		t.Fatalf("expected 1 slide after round-trip, got %d", len(unmarshaled.Slides))
	}
	if unmarshaled.Slides[0].Layout != "title" {
		t.Errorf("slide layout not preserved: got %q", unmarshaled.Slides[0].Layout)
	}
}

func TestTransformNoBackgroundWhenEmpty(t *testing.T) {
	cfg := config.DefaultConfig()
	tr := New(cfg)

	pres := &parser.Presentation{
		Slides: []parser.Slide{
			{
				Index: 0,
				HTML:  "<p>No background</p>",
				// No background directive
			},
		},
	}

	result := tr.Transform(pres)
	slide := result.Slides[0]

	if slide.Background != nil {
		t.Error("expected background to be nil when not specified")
	}
}

func TestTransformNoFragmentsOmittedInJSON(t *testing.T) {
	cfg := config.DefaultConfig()
	tr := New(cfg)

	pres := &parser.Presentation{
		Slides: []parser.Slide{
			{Index: 0, HTML: "<p>No fragments</p>"},
		},
	}

	result := tr.Transform(pres)

	// Check that the slide's Fragments field is nil (will be omitted due to omitempty)
	if result.Slides[0].Fragments != nil {
		t.Error("expected slide Fragments to be nil when no pause markers present")
	}

	// Marshal just the slide to verify omitempty behavior
	slideData, err := json.Marshal(result.Slides[0])
	if err != nil {
		t.Fatalf("failed to marshal slide: %v", err)
	}

	// Check that fragments key is omitted from the slide JSON (due to omitempty)
	slideJSON := string(slideData)
	if containsField(slideJSON, "fragments") {
		t.Error("expected 'fragments' field to be omitted from slide when empty")
	}
}

func TestTransformNoCodeBlocksOmittedInJSON(t *testing.T) {
	cfg := config.DefaultConfig()
	tr := New(cfg)

	pres := &parser.Presentation{
		Slides: []parser.Slide{
			{Index: 0, HTML: "<p>No code</p>"},
		},
	}

	result := tr.Transform(pres)

	// Marshal to JSON
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Check that codeBlocks key is omitted (due to omitempty)
	jsonStr := string(data)
	if containsField(jsonStr, "codeBlocks") {
		t.Error("expected 'codeBlocks' field to be omitted when empty")
	}
}

// containsField checks if a JSON string contains a specific field name
func containsField(jsonStr, fieldName string) bool {
	// Simple check for "fieldName": pattern
	return len(jsonStr) > 0 && (jsonStr[0] == '{' || jsonStr[0] == '[') &&
		(stringContains(jsonStr, "\""+fieldName+"\":") || stringContains(jsonStr, "\""+fieldName+"\" :"))
}

func stringContains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr) >= 0
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// --- Layout Auto-Detection Tests (US-021) ---

func TestDetectLayoutTitle(t *testing.T) {
	testCases := []struct {
		name     string
		html     string
		content  string
		expected string
	}{
		{
			name:     "H1 only",
			html:     "<h1>Welcome to My Presentation</h1>",
			content:  "# Welcome to My Presentation",
			expected: "title",
		},
		{
			name:     "H1 with subtitle paragraph",
			html:     "<h1>My Title</h1>\n<p>A subtitle here</p>",
			content:  "# My Title\n\nA subtitle here",
			expected: "title",
		},
		{
			name:     "H1 with multiple paragraphs - not title",
			html:     "<h1>Title</h1>\n<p>Para 1</p>\n<p>Para 2</p>",
			content:  "# Title\n\nPara 1\n\nPara 2",
			expected: "default",
		},
		{
			name:     "H1 with list - not title",
			html:     "<h1>Title</h1>\n<ul><li>Item</li></ul>",
			content:  "# Title\n\n- Item",
			expected: "default",
		},
		{
			name:     "H1 with H2 - not title",
			html:     "<h1>Title</h1>\n<h2>Section</h2>",
			content:  "# Title\n\n## Section",
			expected: "default",
		},
	}

	cfg := config.DefaultConfig()
	tr := New(cfg)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pres := &parser.Presentation{
				Slides: []parser.Slide{
					{Index: 0, HTML: tc.html, Content: tc.content},
				},
			}
			result := tr.Transform(pres)
			if result.Slides[0].Layout != tc.expected {
				t.Errorf("expected layout %q, got %q", tc.expected, result.Slides[0].Layout)
			}
		})
	}
}

func TestDetectLayoutSection(t *testing.T) {
	testCases := []struct {
		name     string
		html     string
		content  string
		expected string
	}{
		{
			name:     "H2 only",
			html:     "<h2>Section Title</h2>",
			content:  "## Section Title",
			expected: "section",
		},
		{
			name:     "H2 with paragraph - not section",
			html:     "<h2>Section</h2>\n<p>Some content</p>",
			content:  "## Section\n\nSome content",
			expected: "default",
		},
		{
			name:     "H2 with H1 - not section",
			html:     "<h1>Title</h1>\n<h2>Section</h2>",
			content:  "# Title\n\n## Section",
			expected: "default",
		},
		{
			name:     "H2 with code - not section",
			html:     "<h2>Code Section</h2>\n<pre><code>code</code></pre>",
			content:  "## Code Section\n\n```\ncode\n```",
			expected: "default",
		},
	}

	cfg := config.DefaultConfig()
	tr := New(cfg)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pres := &parser.Presentation{
				Slides: []parser.Slide{
					{Index: 0, HTML: tc.html, Content: tc.content},
				},
			}
			result := tr.Transform(pres)
			if result.Slides[0].Layout != tc.expected {
				t.Errorf("expected layout %q, got %q", tc.expected, result.Slides[0].Layout)
			}
		})
	}
}

func TestDetectLayoutCodeFocus(t *testing.T) {
	testCases := []struct {
		codeBlocks []parser.CodeBlock
		name       string
		html       string
		content    string
		expected   string
	}{
		{
			name:    "Single large code block",
			html:    "<pre><code>func main() {\n\tfmt.Println(\"hello\")\n}</code></pre>",
			content: "```go\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n```",
			codeBlocks: []parser.CodeBlock{
				{Language: "go", Code: "func main() {\n\tfmt.Println(\"hello\")\n}"},
			},
			expected: "code-focus",
		},
		{
			name:    "Code block less than 50% - not code-focus",
			html:    "<p>Here is some explanation about the code that follows.</p>\n<pre><code>x := 1</code></pre>\n<p>And more text after.</p>",
			content: "Here is some explanation about the code that follows.\n\n```go\nx := 1\n```\n\nAnd more text after.",
			codeBlocks: []parser.CodeBlock{
				{Language: "go", Code: "x := 1"},
			},
			expected: "default",
		},
		{
			name:    "Multiple code blocks - not code-focus",
			html:    "<pre><code>code1</code></pre>\n<pre><code>code2</code></pre>",
			content: "```\ncode1\n```\n\n```\ncode2\n```",
			codeBlocks: []parser.CodeBlock{
				{Language: "", Code: "code1"},
				{Language: "", Code: "code2"},
			},
			expected: "default",
		},
		{
			name:       "No code blocks - not code-focus",
			html:       "<p>Just text</p>",
			content:    "Just text",
			codeBlocks: nil,
			expected:   "default",
		},
	}

	cfg := config.DefaultConfig()
	tr := New(cfg)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pres := &parser.Presentation{
				Slides: []parser.Slide{
					{Index: 0, HTML: tc.html, Content: tc.content, CodeBlocks: tc.codeBlocks},
				},
			}
			result := tr.Transform(pres)
			if result.Slides[0].Layout != tc.expected {
				t.Errorf("expected layout %q, got %q", tc.expected, result.Slides[0].Layout)
			}
		})
	}
}

func TestDetectLayoutQuote(t *testing.T) {
	testCases := []struct {
		name     string
		html     string
		content  string
		expected string
	}{
		{
			name:     "Blockquote only",
			html:     "<blockquote>\n<p>To be or not to be</p>\n</blockquote>",
			content:  "> To be or not to be",
			expected: "quote",
		},
		{
			name:     "Blockquote with attribution",
			html:     "<blockquote>\n<p>Quote text</p>\n</blockquote>\n<p>— Author</p>",
			content:  "> Quote text\n\n— Author",
			expected: "quote",
		},
		{
			name:     "Blockquote with header - not quote",
			html:     "<h2>Famous Quotes</h2>\n<blockquote>\n<p>Quote</p>\n</blockquote>",
			content:  "## Famous Quotes\n\n> Quote",
			expected: "default",
		},
		{
			name:     "Blockquote with code - not quote",
			html:     "<blockquote>\n<p>Quote</p>\n</blockquote>\n<pre><code>code</code></pre>",
			content:  "> Quote\n\n```\ncode\n```",
			expected: "default",
		},
	}

	cfg := config.DefaultConfig()
	tr := New(cfg)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pres := &parser.Presentation{
				Slides: []parser.Slide{
					{Index: 0, HTML: tc.html, Content: tc.content},
				},
			}
			result := tr.Transform(pres)
			if result.Slides[0].Layout != tc.expected {
				t.Errorf("expected layout %q, got %q", tc.expected, result.Slides[0].Layout)
			}
		})
	}
}

func TestDetectLayoutTwoColumn(t *testing.T) {
	testCases := []struct {
		name     string
		html     string
		content  string
		expected string
	}{
		{
			name:     "Two column with separator",
			html:     "<p>Left content</p>\n<p>|||</p>\n<p>Right content</p>",
			content:  "Left content\n\n|||\n\nRight content",
			expected: "two-column",
		},
		{
			name:     "Two column inline separator",
			html:     "<p>Left ||| Right</p>",
			content:  "Left ||| Right",
			expected: "two-column",
		},
		{
			name:     "No separator - not two-column",
			html:     "<p>Just regular content</p>",
			content:  "Just regular content",
			expected: "default",
		},
	}

	cfg := config.DefaultConfig()
	tr := New(cfg)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pres := &parser.Presentation{
				Slides: []parser.Slide{
					{Index: 0, HTML: tc.html, Content: tc.content},
				},
			}
			result := tr.Transform(pres)
			if result.Slides[0].Layout != tc.expected {
				t.Errorf("expected layout %q, got %q", tc.expected, result.Slides[0].Layout)
			}
		})
	}
}

func TestDetectLayoutDefault(t *testing.T) {
	testCases := []struct {
		name    string
		html    string
		content string
	}{
		{
			name:    "Regular content with paragraphs",
			html:    "<p>Some text</p>\n<p>More text</p>",
			content: "Some text\n\nMore text",
		},
		{
			name:    "H3 header with content",
			html:    "<h3>Subtitle</h3>\n<p>Content</p>",
			content: "### Subtitle\n\nContent",
		},
		{
			name:    "List content",
			html:    "<ul>\n<li>Item 1</li>\n<li>Item 2</li>\n</ul>",
			content: "- Item 1\n- Item 2",
		},
		{
			name:    "Table content",
			html:    "<table><tr><td>Cell</td></tr></table>",
			content: "| Cell |",
		},
		{
			name:    "Mixed content",
			html:    "<h1>Title</h1>\n<p>Text</p>\n<ul><li>List</li></ul>",
			content: "# Title\n\nText\n\n- List",
		},
	}

	cfg := config.DefaultConfig()
	tr := New(cfg)

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			pres := &parser.Presentation{
				Slides: []parser.Slide{
					{Index: 0, HTML: tc.html, Content: tc.content},
				},
			}
			result := tr.Transform(pres)
			if result.Slides[0].Layout != "default" {
				t.Errorf("expected layout 'default', got %q", result.Slides[0].Layout)
			}
		})
	}
}

func TestDetectLayoutDirectiveOverride(t *testing.T) {
	// Directive should always override auto-detection
	cfg := config.DefaultConfig()
	tr := New(cfg)

	pres := &parser.Presentation{
		Slides: []parser.Slide{
			{
				Index:   0,
				HTML:    "<h1>This looks like a title</h1>",
				Content: "# This looks like a title",
				Directives: parser.SlideDirectives{
					Layout: "big-stat", // Override to different layout
				},
			},
		},
	}

	result := tr.Transform(pres)
	if result.Slides[0].Layout != "big-stat" {
		t.Errorf("directive should override auto-detection: expected 'big-stat', got %q", result.Slides[0].Layout)
	}
}

func TestCountHTMLTag(t *testing.T) {
	testCases := []struct {
		html     string
		tag      string
		expected int
	}{
		{"<h1>Title</h1>", "h1", 1},
		{"<h1>One</h1><h1>Two</h1>", "h1", 2},
		{"<h1>Title</h1><h2>Sub</h2>", "h1", 1},
		{"<h1>Title</h1><h2>Sub</h2>", "h2", 1},
		{"<h1 class='title'>Title</h1>", "h1", 1},
		{"<h10>Not h1</h10>", "h1", 0},
		{"<pre><code>code</code></pre>", "pre", 1},
		{"<p>No headers</p>", "h1", 0},
		{"", "h1", 0},
	}

	for _, tc := range testCases {
		t.Run(tc.html, func(t *testing.T) {
			result := countHTMLTag(tc.html, tc.tag)
			if result != tc.expected {
				t.Errorf("countHTMLTag(%q, %q) = %d, expected %d", tc.html, tc.tag, result, tc.expected)
			}
		})
	}
}

// --- Image Path Resolution Tests (US-022) ---

func TestNewWithBaseDir(t *testing.T) {
	cfg := config.DefaultConfig()
	tr := NewWithBaseDir(cfg, "/path/to/presentation")

	if tr == nil {
		t.Fatal("NewWithBaseDir() returned nil")
	}
	if tr.config != cfg {
		t.Error("transformer config not set correctly")
	}
	if tr.baseDir != "/path/to/presentation" {
		t.Errorf("baseDir not set correctly: got %q", tr.baseDir)
	}
}

func TestSetBaseDir(t *testing.T) {
	cfg := config.DefaultConfig()
	tr := New(cfg)

	if tr.baseDir != "" {
		t.Errorf("baseDir should be empty initially: got %q", tr.baseDir)
	}

	tr.SetBaseDir("/new/path")
	if tr.baseDir != "/new/path" {
		t.Errorf("baseDir not updated: got %q", tr.baseDir)
	}
}

func TestResolveImagePathAbsoluteURL(t *testing.T) {
	cfg := config.DefaultConfig()
	tr := NewWithBaseDir(cfg, "/base/dir")

	testCases := []struct {
		input    string
		expected string
	}{
		{"https://example.com/image.png", "https://example.com/image.png"},
		{"http://example.com/image.jpg", "http://example.com/image.jpg"},
		{"HTTPS://EXAMPLE.COM/image.png", "HTTPS://EXAMPLE.COM/image.png"},
		{"HTTP://EXAMPLE.COM/image.jpg", "HTTP://EXAMPLE.COM/image.jpg"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := tr.resolveImagePath(tc.input)
			if result != tc.expected {
				t.Errorf("resolveImagePath(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestResolveImagePathAbsoluteFilePath(t *testing.T) {
	cfg := config.DefaultConfig()
	tr := NewWithBaseDir(cfg, "/base/dir")

	testCases := []struct {
		input    string
		expected string
	}{
		{"/absolute/path/image.png", "/absolute/path/image.png"},
		{"/var/www/assets/photo.jpg", "/var/www/assets/photo.jpg"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := tr.resolveImagePath(tc.input)
			if result != tc.expected {
				t.Errorf("resolveImagePath(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestResolveImagePathRelative(t *testing.T) {
	cfg := config.DefaultConfig()
	tr := NewWithBaseDir(cfg, "/presentations/demo")

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple relative", "image.png", "/presentations/demo/image.png"},
		{"relative with subdir", "images/photo.jpg", "/presentations/demo/images/photo.jpg"},
		{"relative with dot", "./diagram.svg", "/presentations/demo/diagram.svg"},
		{"relative with parent", "../shared/logo.png", "/presentations/shared/logo.png"},
		{"deep relative", "assets/images/bg.webp", "/presentations/demo/assets/images/bg.webp"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tr.resolveImagePath(tc.input)
			if result != tc.expected {
				t.Errorf("resolveImagePath(%q) = %q, expected %q", tc.input, result, tc.expected)
			}
		})
	}
}

func TestResolveImagePathSupportedFormats(t *testing.T) {
	cfg := config.DefaultConfig()
	tr := NewWithBaseDir(cfg, "/base")

	// All supported formats should be resolved
	supportedFormats := []string{
		"image.png",
		"photo.jpg",
		"picture.jpeg",
		"animation.gif",
		"vector.svg",
		"modern.webp",
		"IMAGE.PNG",  // uppercase
		"Photo.JPG",  // mixed case
		"file.JPEG",
	}

	for _, path := range supportedFormats {
		t.Run(path, func(t *testing.T) {
			result := tr.resolveImagePath(path)
			// Should be resolved (not returned unchanged)
			if result == path {
				t.Errorf("resolveImagePath(%q) should resolve relative path, got %q", path, result)
			}
		})
	}
}

func TestResolveImagePathUnsupportedFormats(t *testing.T) {
	cfg := config.DefaultConfig()
	tr := NewWithBaseDir(cfg, "/base")

	// Unsupported formats should NOT be resolved
	unsupportedFormats := []string{
		"document.pdf",
		"video.mp4",
		"audio.mp3",
		"data.json",
		"script.js",
		"style.css",
	}

	for _, path := range unsupportedFormats {
		t.Run(path, func(t *testing.T) {
			result := tr.resolveImagePath(path)
			// Should be returned unchanged
			if result != path {
				t.Errorf("resolveImagePath(%q) should not resolve unsupported format, got %q", path, result)
			}
		})
	}
}

func TestResolveImagePathNoBaseDir(t *testing.T) {
	cfg := config.DefaultConfig()
	tr := New(cfg) // No base dir

	// Without base dir, paths should be returned unchanged
	testCases := []string{
		"image.png",
		"images/photo.jpg",
		"./diagram.svg",
	}

	for _, path := range testCases {
		t.Run(path, func(t *testing.T) {
			result := tr.resolveImagePath(path)
			if result != path {
				t.Errorf("without baseDir, resolveImagePath(%q) should return unchanged, got %q", path, result)
			}
		})
	}
}

func TestResolveImagePathEmptyPath(t *testing.T) {
	cfg := config.DefaultConfig()
	tr := NewWithBaseDir(cfg, "/base")

	result := tr.resolveImagePath("")
	if result != "" {
		t.Errorf("resolveImagePath(\"\") should return empty string, got %q", result)
	}
}

func TestResolveImagePathsInHTML(t *testing.T) {
	cfg := config.DefaultConfig()
	tr := NewWithBaseDir(cfg, "/presentations/demo")

	testCases := []struct {
		name     string
		html     string
		expected string
	}{
		{
			name:     "single image",
			html:     `<p><img src="photo.png" alt="A photo"></p>`,
			expected: `<p><img src="/presentations/demo/photo.png" alt="A photo"></p>`,
		},
		{
			name:     "multiple images",
			html:     `<img src="a.png"><img src="b.jpg">`,
			expected: `<img src="/presentations/demo/a.png"><img src="/presentations/demo/b.jpg">`,
		},
		{
			name:     "absolute URL unchanged",
			html:     `<img src="https://example.com/logo.png" alt="Logo">`,
			expected: `<img src="https://example.com/logo.png" alt="Logo">`,
		},
		{
			name:     "mixed relative and absolute",
			html:     `<img src="local.png"><img src="https://example.com/remote.jpg">`,
			expected: `<img src="/presentations/demo/local.png"><img src="https://example.com/remote.jpg">`,
		},
		{
			name:     "image with subdirectory",
			html:     `<img src="images/hero.png" alt="Hero">`,
			expected: `<img src="/presentations/demo/images/hero.png" alt="Hero">`,
		},
		{
			name:     "image with attributes",
			html:     `<img src="photo.jpg" alt="Photo" width="100" class="responsive">`,
			expected: `<img src="/presentations/demo/photo.jpg" alt="Photo" width="100" class="responsive">`,
		},
		{
			name:     "single quoted src",
			html:     `<img src='image.png' alt='Test'>`,
			expected: `<img src='/presentations/demo/image.png' alt='Test'>`,
		},
		{
			name:     "no images",
			html:     `<p>Just text, no images</p>`,
			expected: `<p>Just text, no images</p>`,
		},
		{
			name:     "unsupported format unchanged",
			html:     `<img src="document.pdf" alt="PDF">`,
			expected: `<img src="document.pdf" alt="PDF">`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := tr.resolveImagePaths(tc.html)
			if result != tc.expected {
				t.Errorf("resolveImagePaths failed:\n  got:      %q\n  expected: %q", result, tc.expected)
			}
		})
	}
}

func TestResolveImagePathsNoBaseDir(t *testing.T) {
	cfg := config.DefaultConfig()
	tr := New(cfg) // No base dir

	html := `<img src="photo.png" alt="Photo">`

	result := tr.resolveImagePaths(html)
	if result != html {
		t.Errorf("without baseDir, resolveImagePaths should return unchanged HTML")
	}
}

func TestTransformWithImagePathResolution(t *testing.T) {
	cfg := config.DefaultConfig()
	tr := NewWithBaseDir(cfg, "/presentations/demo")

	pres := &parser.Presentation{
		Slides: []parser.Slide{
			{
				Index:   0,
				Content: "# Slide with image\n\n![Photo](images/photo.png)",
				HTML:    `<h1>Slide with image</h1><p><img src="images/photo.png" alt="Photo"></p>`,
			},
		},
	}

	result := tr.Transform(pres)

	expectedHTML := `<h1>Slide with image</h1><p><img src="/presentations/demo/images/photo.png" alt="Photo"></p>`
	if result.Slides[0].HTML != expectedHTML {
		t.Errorf("HTML with resolved paths:\n  got:      %q\n  expected: %q", result.Slides[0].HTML, expectedHTML)
	}
}

func TestIsAbsoluteURL(t *testing.T) {
	testCases := []struct {
		url      string
		expected bool
	}{
		{"https://example.com/image.png", true},
		{"http://example.com/image.png", true},
		{"HTTPS://EXAMPLE.COM/IMAGE.PNG", true},
		{"HTTP://EXAMPLE.COM/IMAGE.PNG", true},
		{"//example.com/image.png", false},
		{"/absolute/path.png", false},
		{"relative/path.png", false},
		{"./path.png", false},
		{"", false},
	}

	for _, tc := range testCases {
		t.Run(tc.url, func(t *testing.T) {
			result := isAbsoluteURL(tc.url)
			if result != tc.expected {
				t.Errorf("isAbsoluteURL(%q) = %v, expected %v", tc.url, result, tc.expected)
			}
		})
	}
}

func TestIsSupportedImageFormat(t *testing.T) {
	testCases := []struct {
		path     string
		expected bool
	}{
		{"image.png", true},
		{"photo.jpg", true},
		{"picture.jpeg", true},
		{"animation.gif", true},
		{"vector.svg", true},
		{"modern.webp", true},
		{"IMAGE.PNG", true},
		{"Photo.JPEG", true},
		{"document.pdf", false},
		{"video.mp4", false},
		{"script.js", false},
		{"style.css", false},
		{"data.json", false},
		{"", false},
		{"noextension", false},
	}

	for _, tc := range testCases {
		t.Run(tc.path, func(t *testing.T) {
			result := isSupportedImageFormat(tc.path)
			if result != tc.expected {
				t.Errorf("isSupportedImageFormat(%q) = %v, expected %v", tc.path, result, tc.expected)
			}
		})
	}
}
