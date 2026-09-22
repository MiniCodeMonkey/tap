package layouts

import (
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/parser"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

func TestEveryLayoutHasATemplate(t *testing.T) {
	templates := Templates()
	if len(templates) != len(Names()) {
		t.Fatalf("%d templates for %d layouts", len(templates), len(Names()))
	}
	seen := map[string]bool{}
	for _, template := range templates {
		seen[template.Name] = true
		if template.Description == "" || len(template.Fields) == 0 {
			t.Errorf("template %q needs a description and at least one field", template.Name)
		}
	}
	for _, name := range Names() {
		if !seen[name] {
			t.Errorf("layout %q has no template", name)
		}
	}
}

func TestRenderSlideDefaults(t *testing.T) {
	want := map[string]string{
		"title":        "# Title\n",
		"section":      "## Section\n",
		"default":      "## Header\n\n- Point one\n- Point two\n",
		"two-column":   "::left\n\nLeft content\n\n::right\n\nRight content\n",
		"code-focus":   "<!--\nlayout: code-focus\n-->\n\n```\n// Your code here\n```\n",
		"quote":        "<!--\nlayout: quote\n-->\n\n> \"Your quote here\"\n",
		"big-stat":     "<!--\nlayout: big-stat\n-->\n\n# 100%\n\nDescription\n",
		"three-column": "::left\n\nLeft content\n\n::center\n\nCenter content\n\n::right\n\nRight content\n",
		"sidebar":      "<!--\nlayout: sidebar\n-->\n\n## Header\n\nMain content\n\n::sidebar\n\n- Note one\n- Note two\n",
		"split-media":  "<!--\nlayout: split-media\n-->\n\n## Header\n\nDescribe the image\n\n::media\n\nImage or video\n",
		"cover":        "<!--\nlayout: cover\n-->\n\n# Title\n",
		"blank":        "<!--\nlayout: blank\n-->\n\nContent\n",
	}
	for name, expected := range want {
		got, err := RenderSlide(name, nil)
		if err != nil || got != expected {
			t.Errorf("RenderSlide(%q, nil) = (%q, %v), want %q", name, got, err, expected)
		}
	}
}

func TestRenderSlideUsesTheValues(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{"title", []string{"Hello", "World"}, "# Hello\n\nWorld\n"},
		{"two-column", []string{"Compare", "A", "B"}, "## Compare\n\n::left\n\nA\n\n::right\n\nB\n"},
		{"quote", []string{"Less is more", "Mies"}, "<!--\nlayout: quote\n-->\n\n> \"Less is more\"\n>\n> -- Mies\n"},
		{"cover", []string{"Launch", "images/hero.jpg"}, "<!--\nlayout: cover\nbackground: images/hero.jpg\n-->\n\n# Launch\n"},
		{"split-media", []string{"Dashboard", "New look", "images/shot.png"}, "<!--\nlayout: split-media\n-->\n\n## Dashboard\n\nNew look\n\n::media\n\n![](images/shot.png)\n"},
	}
	for _, tt := range tests {
		got, err := RenderSlide(tt.name, tt.values)
		if err != nil || got != tt.want {
			t.Errorf("RenderSlide(%q, %q) = (%q, %v), want %q", tt.name, tt.values, got, err, tt.want)
		}
	}
}

func TestEveryTemplateRendersAsItsOwnLayout(t *testing.T) {
	for _, template := range Templates() {
		t.Run(template.Name, func(t *testing.T) {
			body, err := RenderSlide(template.Name, nil)
			if err != nil {
				t.Fatal(err)
			}
			presentation, err := parser.New().Parse([]byte("---\ntitle: Templates\n---\n\n" + body))
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			transformed := transformer.New(config.DefaultConfig()).Transform(presentation)
			if len(transformed.Slides) != 1 {
				t.Fatalf("%d slides, want 1", len(transformed.Slides))
			}
			if got := transformed.Slides[0].Layout; got != template.Name {
				t.Errorf("layout = %q, want %q", got, template.Name)
			}
			if warnings := Validate(transformed); len(warnings) > 0 {
				t.Errorf("warnings: %+v", warnings)
			}
		})
	}
}

func TestRenderSlideUnknownLayoutListsTheLayouts(t *testing.T) {
	_, err := RenderSlide("bogus", nil)
	if err == nil || !strings.Contains(err.Error(), "big-stat") || !strings.Contains(err.Error(), "split-media") {
		t.Errorf("RenderSlide(bogus) error = %v, want it to list the layouts", err)
	}
}
