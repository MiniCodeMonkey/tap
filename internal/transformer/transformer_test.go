package transformer

import (
	"encoding/json"
	"testing"

	"github.com/tapsh/tap/internal/config"
	"github.com/tapsh/tap/internal/parser"
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
	if slide.Layout != "default" {
		t.Errorf("expected layout 'default', got %q", slide.Layout)
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

	// Marshal to JSON
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}

	// Check that fragments key is omitted (due to omitempty)
	jsonStr := string(data)
	if containsField(jsonStr, "fragments") {
		t.Error("expected 'fragments' field to be omitted when empty")
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
