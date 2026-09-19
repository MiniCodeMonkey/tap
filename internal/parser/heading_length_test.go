package parser

import (
	"strings"
	"testing"
)

func TestHeadingLengthClass(t *testing.T) {
	tests := []struct {
		name    string
		markup  string
		want    string
	}{
		{"short: at the 24-rune boundary", "# " + strings.Repeat("a", 24), "short"},
		{"short: well under the boundary", "# Ship it", "short"},
		{"medium: just over the short boundary", "# " + strings.Repeat("a", 25), "medium"},
		{"medium: at the 60-rune boundary", "# " + strings.Repeat("a", 60), "medium"},
		{"long: just over the medium boundary", "# " + strings.Repeat("a", 61), "long"},
		{"long: well over the boundary", "# " + strings.Repeat("a", 95), "long"},
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

func TestHeadingLengthClass_InlineMarkupCountedByText(t *testing.T) {
	// "Ship it **now**" renders 12 visible characters ("Ship it now"
	// minus the space collapsing is not a thing here: "Ship it now" is 11
	// runes) once the ** markers are stripped, well inside "short".
	pres, err := New().Parse([]byte("# Ship it **now**"))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	html := slideHTML(t, pres, 0)
	if !strings.Contains(html, `data-length="short"`) {
		t.Errorf("HTML = %q, want data-length=\"short\" (bold markup should not count toward length)", html)
	}

	// A heading whose *markup* pushes it over 24 characters but whose
	// *text* does not must still be "short".
	longMarkupShortText := "# `" + strings.Repeat("x", 20) + "`"
	pres2, err := New().Parse([]byte(longMarkupShortText))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	html2 := slideHTML(t, pres2, 0)
	if !strings.Contains(html2, `data-length="short"`) {
		t.Errorf("HTML = %q, want data-length=\"short\" (backtick fence characters should not count)", html2)
	}
}

func TestHeadingLengthClass_LinkTextOnly(t *testing.T) {
	// The link target shouldn't count, only the visible link text (4 runes: "docs").
	pres, err := New().Parse([]byte("# [docs](https://example.com/a/very/long/path/that/would/tip/the/count)"))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	html := slideHTML(t, pres, 0)
	if !strings.Contains(html, `data-length="short"`) {
		t.Errorf("HTML = %q, want data-length=\"short\" (link URL should not count toward length)", html)
	}
}

func TestHeadingLengthClass_EmojiAndMultibyteCountedByRunes(t *testing.T) {
	// "🎉" is a single rune (one codepoint) but 4 bytes in UTF-8; "日本語"
	// is three runes but 9 bytes. Both must be counted as runes, not
	// bytes, or a short heading with wide characters would wrongly read
	// as medium/long.
	pres, err := New().Parse([]byte("# 🎉 日本語"))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	html := slideHTML(t, pres, 0)
	// "🎉 日本語" is 5 runes (emoji, space, three CJK characters).
	if !strings.Contains(html, `data-length="short"`) {
		t.Errorf("HTML = %q, want data-length=\"short\"", html)
	}
}

func TestHeadingLengthClass_H4NotClassified(t *testing.T) {
	pres, err := New().Parse([]byte("#### A Heading"))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	html := slideHTML(t, pres, 0)
	if strings.Contains(html, "data-length") {
		t.Errorf("HTML = %q, want no data-length on an h4", html)
	}
}

func TestHeadingLengthClass_KeepsAutoHeadingID(t *testing.T) {
	pres, err := New().Parse([]byte("# Ship It"))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	html := slideHTML(t, pres, 0)
	if !strings.Contains(html, `id="ship-it"`) {
		t.Errorf("HTML = %q, want the auto heading id to survive alongside data-length", html)
	}
	if !strings.Contains(html, `data-length="short"`) {
		t.Errorf("HTML = %q, want data-length=\"short\"", html)
	}
}

func TestHeadingLengthClass_RawHTMLHeadingUntouched(t *testing.T) {
	pres, err := New().Parse([]byte("<h1>Raw HTML Heading</h1>"))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	html := slideHTML(t, pres, 0)
	if strings.Contains(html, "data-length") {
		t.Errorf("HTML = %q, want a raw HTML heading left untouched (no data-length)", html)
	}
	if !strings.Contains(html, "<h1>Raw HTML Heading</h1>") {
		t.Errorf("HTML = %q, want the raw HTML heading rendered verbatim", html)
	}
}

func TestHeadingLengthClass_InSlotsAndFragments(t *testing.T) {
	input := "# Title\n\n<!-- pause -->\n\n## " + strings.Repeat("a", 30) + "\n\n::caption\n### " + strings.Repeat("b", 90)
	pres, err := New().Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	if len(pres.Slides) == 0 {
		t.Fatal("expected at least one slide")
	}
	slide := pres.Slides[0]

	var all strings.Builder
	all.WriteString(slide.HTML)
	for _, name := range slide.SlotOrder {
		all.WriteString(slide.Slots[name])
	}
	combined := all.String()

	if !strings.Contains(combined, `data-length="short"`) {
		t.Errorf("combined HTML = %q, want a short heading (the h1 title)", combined)
	}
	if !strings.Contains(combined, `data-length="medium"`) {
		t.Errorf("combined HTML = %q, want a medium heading (the fragmented h2)", combined)
	}
	if !strings.Contains(combined, `data-length="long"`) {
		t.Errorf("combined HTML = %q, want a long heading (the h3 in the caption slot)", combined)
	}
}

// slideHTML returns the full rendered HTML for a slide: its default HTML
// plus every named slot, concatenated, since a heading may land in either
// depending on whether the input used slot markers.
func slideHTML(t *testing.T, pres *Presentation, index int) string {
	t.Helper()
	if index >= len(pres.Slides) {
		t.Fatalf("slide index %d out of range (only %d slides)", index, len(pres.Slides))
	}
	slide := pres.Slides[index]
	var b strings.Builder
	b.WriteString(slide.HTML)
	for _, name := range slide.SlotOrder {
		b.WriteString(slide.Slots[name])
	}
	return b.String()
}
