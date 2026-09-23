package parser

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParse_FenceAttributeForms(t *testing.T) {
	tests := []struct {
		name         string
		info         string
		wantLanguage string
		wantMeta     CodeBlockMeta
	}{
		{"driver, then highlight", "sql {driver: sqlite, connection: incident} {2-3}", "sql", CodeBlockMeta{Driver: "sqlite", Connection: "incident", HighlightLines: "2-3"}},
		{"highlight, then driver", "sql {2-3} {driver: sqlite}", "sql", CodeBlockMeta{Driver: "sqlite", HighlightLines: "2-3"}},
		{"groups with no space between", "sql {driver: sqlite}{2}", "sql", CodeBlockMeta{Driver: "sqlite", HighlightLines: "2"}},
		{"driver only", "sql {driver: sqlite, connection: demo}", "sql", CodeBlockMeta{Driver: "sqlite", Connection: "demo"}},
		{"key=value form", "bash {driver=shell}", "bash", CodeBlockMeta{Driver: "shell"}},
		{"highlight only", "js {1, 3-5}", "js", CodeBlockMeta{HighlightLines: "1,3-5"}},
		{"no attributes", "go", "go", CodeBlockMeta{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pres, err := New().Parse([]byte("```" + tt.info + "\nSELECT 1;\n```"))
			if err != nil {
				t.Fatalf("Parse() returned error: %v", err)
			}
			block := pres.Slides[0].CodeBlocks[0]
			if block.Language != tt.wantLanguage {
				t.Errorf("Language = %q, want %q", block.Language, tt.wantLanguage)
			}
			if block.Meta != tt.wantMeta {
				t.Errorf("Meta = %+v, want %+v", block.Meta, tt.wantMeta)
			}
		})
	}
}

func TestParse_FenceWithDriverKeepsItsHighlightAttribute(t *testing.T) {
	pres, err := New().Parse([]byte("```sql {driver: sqlite, connection: incident} {2-3}\nSELECT 1;\nSELECT 2;\n```"))
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	if !strings.Contains(pres.Slides[0].HTML, `data-highlight-lines="2-3"`) {
		t.Errorf("HTML = %s, want data-highlight-lines=\"2-3\"", pres.Slides[0].HTML)
	}
}

func TestParse_ConferenceTalkQueryIsLive(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "examples", "conference-talk.md"))
	if err != nil {
		t.Fatal(err)
	}
	pres, err := New().Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	block := pres.Slides[3].CodeBlocks[0]
	if block.Meta.Driver != "sqlite" || block.Meta.Connection != "incident" || block.Meta.HighlightLines != "9-10" {
		t.Errorf("slide 4 block meta = %+v, want sqlite, incident, 9-10", block.Meta)
	}
}

func TestFenceLinesMatchTheSlideCodeBlocks(t *testing.T) {
	markdown := "<!--\nlayout: code-focus\n-->\n\n" +
		"```sql {driver: sqlite}\nSELECT 1;\n```\n\n" +
		"```component ./Chart.jsx\n{}\n```\n\n" +
		"- item\n\n  ```bash\n  ls\n  ```\n\n" +
		"```\nplain\n```"

	got := FenceLines(markdown)
	want := []int{5, 15, 19}
	if len(got) != len(want) {
		t.Fatalf("FenceLines() = %v, want %v", got, want)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Errorf("FenceLines()[%d] = %d, want %d", index, got[index], want[index])
		}
	}

	pres, err := New().Parse([]byte(markdown))
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	if len(pres.Slides[0].CodeBlocks) != len(want) {
		t.Errorf("the slide has %d code blocks, FenceLines found %d", len(pres.Slides[0].CodeBlocks), len(want))
	}
}
