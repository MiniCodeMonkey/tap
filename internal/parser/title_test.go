package parser

import "testing"

func TestSlideTitle(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"first heading", "Intro text\n\n## What We Knew\n\n# Later", "What We Knew"},
		{"bold and code markers are removed", "## The **naive** `lookup`", "The naive lookup"},
		{"a heading in a backtick fence is code", "```bash\n# not a title\n```\n\n# Real", "Real"},
		{"a heading in a tilde fence is code", "~~~\n# not a title\n~~~\n\n# Real", "Real"},
		{"no heading", "![](diagram.png)", ""},
		{"closing hashes are dropped", "# Title #", "Title"},
		{"a shorter run inside a longer fence does not close it", "````\n```\n# Title\n````", ""},
		{"a different fence character inside does not close it", "```\n~~~\n# Title\n```", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SlideTitle(tt.content); got != tt.want {
				t.Errorf("SlideTitle() = %q, want %q", got, tt.want)
			}
		})
	}
}
