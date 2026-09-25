package slidelist

import (
	"strings"
	"testing"
)

// The problem the page shows in a live block and /api/execute refuses it
// with rides along in the slide list, so the app can mark the block's
// box; a block that can run has none.
func TestBuildCarriesEachBlocksProblem(t *testing.T) {
	source := "---\ndrivers:\n  sqlite: {}\n---\n\n# Query\n\n```sql {driver: sqlite}\nSELECT 1;\n```\n\n---\n\n# Shell\n\n```bash {driver: shell}\necho six\n```\n"
	result, err := Build([]byte(source), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if got := result.Slides[0].CodeBlocks[0].Problem; got != "" {
		t.Errorf("a declared driver's block has a problem: %q", got)
	}
	want := `This deck does not declare the shell driver. Add "shell: {}" under drivers in the frontmatter.`
	if got := result.Slides[1].CodeBlocks[0].Problem; got != want {
		t.Errorf("problem = %q, want %q", got, want)
	}

	noDrivers := "# Query\n\n```sql {driver: sqlite}\nSELECT 1;\n```\n"
	result, err = Build([]byte(noDrivers), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	got := result.Slides[0].CodeBlocks[0].Problem
	if !strings.HasPrefix(got, "This deck does not declare the sqlite driver. Add this to the frontmatter:") || !strings.Contains(got, "\n  sqlite: {}") {
		t.Errorf("problem without a drivers map = %q", got)
	}
}
