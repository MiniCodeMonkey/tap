package parser

import (
	"strings"
	"testing"
)

func TestParse_NotesComments(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantNotes string
	}{
		{
			name:      "single line trailing notes comment",
			input:     "# Title\n\nBody text.\n\n<!-- notes: Remember the demo. -->",
			wantNotes: "Remember the demo.",
		},
		{
			name:      "multi-line free text trailing notes comment",
			input:     "# Your App Is Fine.\n\n<!-- notes:\nHost has introduced me. One sentence of bio, then the chart.\n-->",
			wantNotes: "Host has introduced me. One sentence of bio, then the chart.",
		},
		{
			name:      "multi-line trailing notes comment keeps inner line breaks",
			input:     "# Title\n\nBody.\n\n<!-- notes:\nFirst line.\n\nSecond line after a blank.\n-->",
			wantNotes: "First line.\n\nSecond line after a blank.",
		},
		{
			name:      "directive notes and trailing notes comment are joined, directive first",
			input:     "<!--\nnotes: From the directive block.\n-->\n# Title\n\nBody.\n\n<!-- notes: From the trailing comment. -->",
			wantNotes: "From the directive block.\n\nFrom the trailing comment.",
		},
		{
			name:      "several trailing notes comments join in document order",
			input:     "# Title\n\nBody.\n\n<!-- notes: First note. -->\n\nMore body.\n\n<!-- notes: Second note. -->",
			wantNotes: "First note.\n\nSecond note.",
		},
		{
			name:      "pause comment keeps its meaning and is not treated as notes",
			input:     "# Title\n\nFirst.\n\n<!-- pause -->\n\nSecond.\n\n<!-- notes: The real notes. -->",
			wantNotes: "The real notes.",
		},
		{
			name:      "notes comment inside a named slot is removed and still counts",
			input:     "# Title\n\nDefault content.\n\n::caption\nCaption text.\n\n<!-- notes: Notes from inside a slot. -->",
			wantNotes: "Notes from inside a slot.",
		},
		{
			name:      "first directive comment as pure free-text notes, no other directives",
			input:     "<!-- notes:\nPure notes, no other directives here.\n-->\n# Title",
			wantNotes: "Pure notes, no other directives here.",
		},
		{
			name:      "first directive comment notes with a colon in the free text",
			input:     "<!-- notes:\nStart time: 10am, don't run over.\n-->\n# Title",
			wantNotes: "Start time: 10am, don't run over.",
		},
		{
			name:      "first directive comment notes starting with a quote",
			input:     "<!-- notes:\n\"Quoted\" opening line.\n-->\n# Title",
			wantNotes: "\"Quoted\" opening line.",
		},
		{
			name:      "notes comment that begins after other text on the same line",
			input:     "# Title\n\nVisible text. <!-- notes: Hidden aside. -->",
			wantNotes: "Hidden aside.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			pres, err := p.Parse([]byte(tt.input))
			if err != nil {
				t.Fatalf("Parse() returned error: %v", err)
			}
			if len(pres.Slides) != 1 {
				t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
			}
			slide := pres.Slides[0]
			if slide.Directives.Notes != tt.wantNotes {
				t.Errorf("Notes = %q, want %q", slide.Directives.Notes, tt.wantNotes)
			}
			if strings.Contains(slide.Content, "notes:") {
				t.Errorf("slide content should not contain a notes comment: %q", slide.Content)
			}
			if strings.Contains(slide.HTML, "notes:") || strings.Contains(slide.HTML, "notes-") {
				t.Errorf("rendered HTML should not contain notes text: %q", slide.HTML)
			}
			for name, html := range slide.Slots {
				if strings.Contains(html, "notes:") {
					t.Errorf("slot %q HTML should not contain notes text: %q", name, html)
				}
			}
		})
	}
}

func TestParse_MixedDirectivesAndFreeTextNotes(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantNotes  string
		wantLayout string
	}{
		{
			name:       "notes first, then a directive",
			input:      "<!--\nnotes: Start time: 10am, don't run over.\nlayout: big-stat\n-->\n# Title",
			wantNotes:  "Start time: 10am, don't run over.",
			wantLayout: "big-stat",
		},
		{
			name:       "a directive first, then notes",
			input:      "<!--\nlayout: big-stat\nnotes: Start time: 10am, don't run over.\n-->\n# Title",
			wantNotes:  "Start time: 10am, don't run over.",
			wantLayout: "big-stat",
		},
		{
			name:       "multi-line free text notes between two directives",
			input:      "<!--\nlayout: big-stat\nnotes:\nFirst line of notes.\nSecond line of notes.\ntransition: fade\n-->\n# Title",
			wantNotes:  "First line of notes.\nSecond line of notes.",
			wantLayout: "big-stat",
		},
		{
			name:       "free text with a colon and an apostrophe, directive after",
			input:      "<!--\nnotes: Don't skip the demo: it's the whole point.\ntag: workshop\n-->\n# Title",
			wantNotes:  "Don't skip the demo: it's the whole point.",
			wantLayout: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			pres, err := p.Parse([]byte(tt.input))
			if err != nil {
				t.Fatalf("Parse() returned error: %v", err)
			}
			if len(pres.Slides) != 1 {
				t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
			}
			slide := pres.Slides[0]
			if slide.Directives.Notes != tt.wantNotes {
				t.Errorf("Notes = %q, want %q", slide.Directives.Notes, tt.wantNotes)
			}
			if slide.Directives.Layout != tt.wantLayout {
				t.Errorf("Layout = %q, want %q", slide.Directives.Layout, tt.wantLayout)
			}
		})
	}

	// The "free text with a colon and an apostrophe" case above also sets
	// "tag: workshop"; verify it separately since the table only checks
	// Notes and Layout.
	p := New()
	pres, err := p.Parse([]byte("<!--\nnotes: Don't skip the demo: it's the whole point.\ntag: workshop\n-->\n# Title"))
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	if pres.Slides[0].Directives.Tag != "workshop" {
		t.Errorf("Tag = %q, want %q", pres.Slides[0].Directives.Tag, "workshop")
	}
}

// TestParse_NotesRunProseCollision covers the ruling for a line inside a
// notes run (after a "notes:" line, in the mixed-comment fallback) that
// happens to start with a known directive key: it only counts as a
// directive line when its value is plausible for that key. For layout,
// transition, fragments, scroll, and scroll-speed, the value must be a
// single token with no whitespace; for background, tag, and badge, any
// value counts as a directive, since those keys naturally take
// multi-word or path-like values. A line that fails the plausibility
// check stays part of the notes text instead of truncating it.
func TestParse_NotesRunProseCollision(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantNotes     string
		wantLayout    string
		wantScrollSpd int
	}{
		{
			name:          "prose line inside a notes run that merely starts with a directive key stays notes",
			input:         "<!--\nnotes: Remember the rule:\nlayout: keep it simple, that's the message.\n-->\n# Title",
			wantNotes:     "Remember the rule:\nlayout: keep it simple, that's the message.",
			wantLayout:    "",
			wantScrollSpd: 0,
		},
		{
			name:          "layout with a plausible single-token value after notes is still a directive",
			input:         "<!--\nnotes: some free text\nlayout: big-stat\n-->\n# Title",
			wantNotes:     "some free text",
			wantLayout:    "big-stat",
			wantScrollSpd: 0,
		},
		{
			name:          "scroll-speed with a plausible single-token value after notes is still a directive",
			input:         "<!--\nnotes: another note\nscroll-speed: 2000\n-->\n# Title",
			wantNotes:     "another note",
			wantLayout:    "",
			wantScrollSpd: 2000,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			pres, err := p.Parse([]byte(tt.input))
			if err != nil {
				t.Fatalf("Parse() returned error: %v", err)
			}
			if len(pres.Slides) != 1 {
				t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
			}
			slide := pres.Slides[0]
			if slide.Directives.Notes != tt.wantNotes {
				t.Errorf("Notes = %q, want %q", slide.Directives.Notes, tt.wantNotes)
			}
			if slide.Directives.Layout != tt.wantLayout {
				t.Errorf("Layout = %q, want %q", slide.Directives.Layout, tt.wantLayout)
			}
			if slide.Directives.ScrollSpeed != tt.wantScrollSpd {
				t.Errorf("ScrollSpeed = %d, want %d", slide.Directives.ScrollSpeed, tt.wantScrollSpd)
			}
		})
	}
}

func TestParse_NotesCommentInFencedCodeBlockIsCode(t *testing.T) {
	input := "# Title\n\n```markdown\n<!-- notes: This is example code, not notes. -->\n```\n"
	p := New()
	pres, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	slide := pres.Slides[0]
	if slide.Directives.Notes != "" {
		t.Errorf("expected no notes extracted from a fenced code block, got %q", slide.Directives.Notes)
	}
	if !strings.Contains(slide.Content, "<!-- notes: This is example code, not notes. -->") {
		t.Error("code block content should retain the comment verbatim")
	}
}

func TestParse_NotesCommentEndsAtFirstArrow(t *testing.T) {
	// HTML (and goldmark) end a comment at the first "-->"; tap must agree.
	// Text in the notes body after that point is not notes, it is content.
	input := "# Title\n\n<!-- notes: Keep it short --> Rest of the line stays.\n\nAnother paragraph."
	p := New()
	pres, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	slide := pres.Slides[0]
	if slide.Directives.Notes != "Keep it short" {
		t.Errorf("Notes = %q, want %q", slide.Directives.Notes, "Keep it short")
	}
	if !strings.Contains(slide.Content, "Rest of the line stays.") {
		t.Errorf("content after the closing --> should be kept: %q", slide.Content)
	}
	if !strings.Contains(slide.Content, "Another paragraph.") {
		t.Errorf("following lines should not be dropped: %q", slide.Content)
	}
	if strings.Contains(slide.HTML, "Keep it short") {
		t.Errorf("notes text should not leak into HTML: %q", slide.HTML)
	}
}

func TestParse_NotesCommentThatDoesNotStartALine(t *testing.T) {
	// A notes comment does not have to start a line: text before it on the
	// same line is content, not part of the comment. Before this fix, the
	// whole line was left untouched because extractNotesComments only
	// looked for "<!--" at the start of a (trimmed) line, so the comment
	// stayed in the HTML verbatim and its text never reached notes.
	input := "# Title\n\nVisible text. <!-- notes: Hidden aside. -->\n\nMore body."
	p := New()
	pres, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	slide := pres.Slides[0]

	if slide.Directives.Notes != "Hidden aside." {
		t.Errorf("Notes = %q, want %q", slide.Directives.Notes, "Hidden aside.")
	}
	if !strings.Contains(slide.Content, "Visible text.") {
		t.Errorf("text before the comment should stay as content: %q", slide.Content)
	}
	if strings.Contains(slide.Content, "<!--") {
		t.Errorf("the comment itself should be removed from content: %q", slide.Content)
	}
	if !strings.Contains(slide.HTML, "Visible text.") {
		t.Errorf("text before the comment should be rendered: %q", slide.HTML)
	}
	if strings.Contains(slide.HTML, "Hidden aside.") || strings.Contains(slide.HTML, "<!--") {
		t.Errorf("the comment and its notes text should not leak into HTML: %q", slide.HTML)
	}
	if !strings.Contains(slide.Content, "More body.") {
		t.Errorf("following lines should not be dropped: %q", slide.Content)
	}
}

func TestParse_NotesCommentContainingUnclosedPauseMarker(t *testing.T) {
	// The literal text "<!-- pause" inside a notes comment, without its own
	// closing "-->", is swallowed as part of the notes comment (which does
	// not end until ITS first "-->"). It must not be read as a real pause
	// marker and must not create a fragment.
	input := "# Title\n\nFirst part.\n\n<!-- notes:\nSome intro text.\n<!-- pause\nMore notes after the embedded pause-like text.\n-->\n\nRest of slide."
	p := New()
	pres, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	slide := pres.Slides[0]
	wantNotes := "Some intro text.\n<!-- pause\nMore notes after the embedded pause-like text."
	if slide.Directives.Notes != wantNotes {
		t.Errorf("Notes = %q, want %q", slide.Directives.Notes, wantNotes)
	}
	if slide.FragmentCount != 0 {
		t.Errorf("FragmentCount = %d, want 0 (the embedded \"<!-- pause\" text is inside the notes comment, not a real marker)", slide.FragmentCount)
	}
	if !strings.Contains(slide.Content, "Rest of slide.") {
		t.Errorf("content after the notes comment should be kept: %q", slide.Content)
	}
}

func TestParse_PauseCommentStillSplitsFragments(t *testing.T) {
	input := "# Title\n\nFirst.\n\n<!-- pause -->\n\nSecond.\n\n<!-- notes: Notes text. -->"
	p := New()
	pres, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	slide := pres.Slides[0]
	if slide.FragmentCount != 1 {
		t.Errorf("FragmentCount = %d, want 1 (pause comment should still split fragments)", slide.FragmentCount)
	}
}

// TestParse_CRLFDeckYieldsNotesWithoutCR reproduces a deck saved with
// Windows line endings. Notes text (and slide content) must not contain
// stray "\r" characters.
func TestParse_CRLFDeckYieldsNotesWithoutCR(t *testing.T) {
	input := "# Title\r\n\r\nBody text.\r\n\r\n<!-- notes:\r\nFirst line.\r\nSecond line.\r\n-->\r\n"
	p := New()
	pres, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}
	slide := pres.Slides[0]
	wantNotes := "First line.\nSecond line."
	if slide.Directives.Notes != wantNotes {
		t.Errorf("Notes = %q, want %q", slide.Directives.Notes, wantNotes)
	}
	if strings.Contains(slide.Directives.Notes, "\r") {
		t.Errorf("Notes should not contain a carriage return: %q", slide.Directives.Notes)
	}
	if strings.Contains(slide.Content, "\r") {
		t.Errorf("Content should not contain a carriage return: %q", slide.Content)
	}
}

// TestParse_ThreeSlideNotesDeck reproduces the real-world 91-slide deck's
// pattern: every slide writes its speaker notes as a trailing HTML comment
// after the slide content. Each slide's notes must be parsed and must never
// leak into the rendered HTML or slot content.
func TestParse_ThreeSlideNotesDeck(t *testing.T) {
	input := `# Your App Is Fine.

<!-- notes:
Host has introduced me. One sentence of bio, then the chart.
-->

---

# The Traffic Graph

Steady growth, nothing alarming.

<!-- notes:
Point at the graph. Let it sit for a beat before moving on.
-->

---

<!--
layout: big-stat
-->

# 10x

Traffic multiplier during the incident.

<!-- notes:
This is the number that matters. Say it slowly.
-->
`

	p := New()
	pres, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	if len(pres.Slides) != 3 {
		t.Fatalf("expected 3 slides, got %d", len(pres.Slides))
	}

	wantNotes := []string{
		"Host has introduced me. One sentence of bio, then the chart.",
		"Point at the graph. Let it sit for a beat before moving on.",
		"This is the number that matters. Say it slowly.",
	}

	for i, slide := range pres.Slides {
		if slide.Directives.Notes != wantNotes[i] {
			t.Errorf("slide %d Notes = %q, want %q", i, slide.Directives.Notes, wantNotes[i])
		}
		for name, html := range slide.Slots {
			if strings.Contains(html, "notes:") || strings.Contains(html, wantNotes[i]) {
				t.Errorf("slide %d slot %q HTML leaked notes text: %q", i, name, html)
			}
		}
	}

	if pres.Slides[2].Directives.Layout != "big-stat" {
		t.Errorf("slide 2 layout = %q, want %q (directives and trailing notes must coexist)", pres.Slides[2].Directives.Layout, "big-stat")
	}
}

// TestDirectiveKeyNamesMatchesAppliedFields is a drift guard: every key in
// directiveKeyNames (used by splitMixedDirectiveComment to recognize a
// directive line) must actually be applied by applyDirectiveFields. It
// parses a one-line directive comment for each key with a value plausible
// for that key and asserts the resulting SlideDirectives differs from its
// zero value, so a key added to one list but not the other is caught here
// instead of silently doing nothing.
func TestDirectiveKeyNamesMatchesAppliedFields(t *testing.T) {
	values := map[string]string{
		"layout":       "big-stat",
		"transition":   "fade",
		"background":   "#111111",
		"tag":          "workshop",
		"badge":        "v2.0",
		"fragments":    "true",
		"scroll":       "true",
		"scroll-speed": "2000",
		"steps":        "5",
	}

	if len(values) != len(directiveKeyNames) {
		t.Fatalf("this test's values map covers %d keys, but directiveKeyNames has %d; keep them in sync", len(values), len(directiveKeyNames))
	}

	for _, key := range directiveKeyNames {
		t.Run(key, func(t *testing.T) {
			value, known := values[key]
			if !known {
				t.Fatalf("directiveKeyNames contains %q, which this test does not know a plausible value for", key)
			}
			input := "<!--\n" + key + ": " + value + "\n-->\n# Title"
			p := New()
			pres, err := p.Parse([]byte(input))
			if err != nil {
				t.Fatalf("Parse() returned error: %v", err)
			}
			if pres.Slides[0].Directives == (SlideDirectives{}) {
				t.Errorf("directive key %q with value %q did not change SlideDirectives away from its zero value; applyDirectiveFields may be missing a case for it", key, value)
			}
		})
	}
}
