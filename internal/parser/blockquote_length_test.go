package parser

import (
	"strings"
	"testing"
)

func TestBlockquoteLengthClass(t *testing.T) {
	tests := []struct {
		name   string
		markup string
		want   string
	}{
		{"short: at the 80-rune boundary", "> " + strings.Repeat("a", 80), "short"},
		{"short: well under the boundary", "> Stay short.", "short"},
		{"medium: just over the short boundary", "> " + strings.Repeat("a", 81), "medium"},
		{"medium: at the 180-rune boundary", "> " + strings.Repeat("a", 180), "medium"},
		{"long: just over the medium boundary", "> " + strings.Repeat("a", 181), "long"},
		{"long: well over the boundary", "> " + strings.Repeat("a", 260), "long"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pres, err := New().Parse([]byte(test.markup))
			if err != nil {
				t.Fatalf("Parse() error: %v", err)
			}
			html := slideHTML(t, pres, 0)
			if !strings.Contains(html, `data-length="`+test.want+`"`) {
				t.Errorf("HTML = %q, want it to contain data-length=%q", html, test.want)
			}
		})
	}
}

func TestBlockquoteLengthClass_InlineMarkupCountedByTextOnly(t *testing.T) {
	pres, err := New().Parse([]byte("> Keep it **short**"))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	html := slideHTML(t, pres, 0)
	if !strings.Contains(html, `data-length="short"`) {
		t.Errorf("HTML = %q, want data-length=\"short\" (bold markup should not count toward length)", html)
	}
}

func TestBlockquoteLengthClass_MultiParagraphCountsAllText(t *testing.T) {
	// Two paragraphs inside one blockquote: the length hook counts the
	// quote's whole visible text, not just its first paragraph.
	markup := "> " + strings.Repeat("a", 100) + "\n>\n> " + strings.Repeat("b", 100)
	pres, err := New().Parse([]byte(markup))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	html := slideHTML(t, pres, 0)
	if !strings.Contains(html, `data-length="long"`) {
		t.Errorf("HTML = %q, want data-length=\"long\" (200 runes across both paragraphs)", html)
	}
}

func TestBlockquoteLengthClass_RawHTMLBlockquoteUntouched(t *testing.T) {
	pres, err := New().Parse([]byte("<blockquote>Raw HTML quote</blockquote>"))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	html := slideHTML(t, pres, 0)
	if strings.Contains(html, "data-length") {
		t.Errorf("HTML = %q, want a raw HTML blockquote left untouched (no data-length)", html)
	}
}

func TestBlockquoteLengthClass_HeadingsUnaffected(t *testing.T) {
	// The blockquote transformer must not add data-length to headings, and
	// the heading transformer must not add it to blockquotes: each node
	// kind keeps its own thresholds.
	markup := "# " + strings.Repeat("a", 90) + "\n\n> " + strings.Repeat("b", 30)
	pres, err := New().Parse([]byte(markup))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	html := slideHTML(t, pres, 0)
	if !strings.Contains(html, `<h1 id="`) || !strings.Contains(html, `data-length="long">`+strings.Repeat("a", 90)+"</h1>") {
		t.Errorf("HTML = %q, want the 90-rune h1 classified long by the heading thresholds", html)
	}
	if !strings.Contains(html, `<blockquote data-length="short">`) {
		t.Errorf("HTML = %q, want the 30-rune blockquote classified short by the blockquote thresholds", html)
	}
}
