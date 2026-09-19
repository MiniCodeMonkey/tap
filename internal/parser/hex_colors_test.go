package parser

import "testing"

func TestParse_UnquotedHexColorBackground(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "unquoted 3 digit hex color",
			input: "<!--\nbackground: #111\n-->\n# Title",
			want:  "#111",
		},
		{
			name:  "unquoted 6 digit hex color",
			input: "<!--\nbackground: #111111\n-->\n# Title",
			want:  "#111111",
		},
		{
			name:  "unquoted 8 digit hex color",
			input: "<!--\nbackground: #111111ff\n-->\n# Title",
			want:  "#111111ff",
		},
		{
			name:  "already quoted color still works",
			input: "<!--\nbackground: \"#111111\"\n-->\n# Title",
			want:  "#111111",
		},
		{
			name:  "quoted color with trailing YAML comment still works",
			input: "<!--\nbackground: \"#111111\" # dark\n-->\n# Title",
			want:  "#111111",
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
			got := pres.Slides[0].Directives.Background
			if got != tt.want {
				t.Errorf("Background = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParse_NotesBlockWithHexLikeLineIsNotAltered(t *testing.T) {
	input := "<!--\nbackground: #111111\nnotes: |\n  color: #fff is the accent\n  Keep talking about the palette.\n-->\n# Title"
	p := New()
	pres, err := p.Parse([]byte(input))
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	slide := pres.Slides[0]
	if slide.Directives.Background != "#111111" {
		t.Errorf("Background = %q, want %q", slide.Directives.Background, "#111111")
	}
	want := "color: #fff is the accent\nKeep talking about the palette."
	if slide.Directives.Notes != want {
		t.Errorf("Notes = %q, want %q", slide.Directives.Notes, want)
	}
}

func TestQuoteHexColorValues(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "3 digit hex",
			input: "background: #abc",
			want:  `background: "#abc"`,
		},
		{
			name:  "4 digit hex",
			input: "background: #abcd",
			want:  `background: "#abcd"`,
		},
		{
			name:  "6 digit hex",
			input: "background: #aabbcc",
			want:  `background: "#aabbcc"`,
		},
		{
			name:  "8 digit hex",
			input: "background: #aabbccdd",
			want:  `background: "#aabbccdd"`,
		},
		{
			name:  "already quoted is untouched",
			input: `background: "#aabbcc"`,
			want:  `background: "#aabbcc"`,
		},
		{
			name:  "any directive key, not just background",
			input: "tag: #aabbcc",
			want:  `tag: "#aabbcc"`,
		},
		{
			name:  "line inside a notes block scalar is untouched",
			input: "notes: |\n  background: #aabbcc\n  more text",
			want:  "notes: |\n  background: #aabbcc\n  more text",
		},
		{
			name:  "line after a block scalar ends is processed normally",
			input: "notes: |\n  some notes\nbackground: #aabbcc",
			want:  "notes: |\n  some notes\nbackground: \"#aabbcc\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := quoteHexColorValues(tt.input)
			if got != tt.want {
				t.Errorf("quoteHexColorValues(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
