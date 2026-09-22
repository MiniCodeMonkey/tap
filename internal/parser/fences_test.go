package parser

import "testing"

func TestFenceTracker(t *testing.T) {
	lines := []string{
		"text",       // plain text
		"```go",      // opens a backtick fence
		"---",        // content
		"~~~",        // a tilde run does not close a backtick fence
		"```",        // closes
		"after",      // plain text
		"~~~~",       // opens a tilde fence of four
		"```",        // content
		"~~~",        // too short to close
		"~~~~~",      // closes
		"``` a`b",    // not a fence: a backtick info string cannot hold a backtick
		"   ```",     // opens, indented by three spaces
		"x",          // content
		"    ```",    // indented by four, so content
		"```",        // closes
	}
	want := []bool{false, true, true, true, true, false, true, true, true, true, false, true, true, true, true}

	var tracker fenceTracker
	for index, line := range lines {
		if got := tracker.advance(line); got != want[index] {
			t.Errorf("line %d %q: advance() = %v, want %v", index+1, line, got, want[index])
		}
	}
}

func TestSplitSlidesKeepsSeparatorsInsideFences(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{"backtick fence", "# A\n\n```yaml\n---\nkey: value\n```\n\n---\n\n# B", 2},
		{"tilde fence", "# A\n\n~~~yaml\n---\n~~~\n\n---\n\n# B", 2},
		{"four backticks around three", "# A\n\n````markdown\n```\n---\n```\n````\n\n---\n\n# B", 2},
		{"indented fence", "# A\n\n  ```\n---\n  ```\n\n---\n\n# B", 2},
		{"a separator right after a fence still splits", "```\ncode\n```\n---\n# B", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := len(SplitSlidesPreservingCodeBlocks(tt.text)); got != tt.want {
				t.Errorf("SplitSlidesPreservingCodeBlocks() gave %d slides, want %d", got, tt.want)
			}
			if got := len(splitSlidesPreservingCodeBlocksWithLines(tt.text)); got != tt.want {
				t.Errorf("splitSlidesPreservingCodeBlocksWithLines() gave %d slides, want %d", got, tt.want)
			}
		})
	}
}
