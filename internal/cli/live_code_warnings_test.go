package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/config"
)

func loadDeckForTest(t *testing.T, content string) string {
	t.Helper()
	deckPath := filepath.Join(t.TempDir(), "deck.md")
	if err := os.WriteFile(deckPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return deckPath
}

func TestUndeclaredDriverWarningsNameTheFileAndLine(t *testing.T) {
	deckPath := loadDeckForTest(t, "---\ntitle: Query\n---\n\n# Query\n\n```sql {driver: sqlite}\nSELECT 1;\n```\n")
	cfg, err := config.Load(deckPath)
	if err != nil {
		t.Fatal(err)
	}
	pres, _, _, _, _, err := loadPresentation(deckPath, cfg, filepath.Dir(deckPath))
	if err != nil {
		t.Fatal(err)
	}

	got := undeclaredDriverWarnings(deckPath, pres)
	want := fmt.Sprintf("warning: %s:7: This deck does not declare the sqlite driver. Add this to the frontmatter:\n\ndrivers:\n  sqlite: {}", deckPath)
	if len(got) != 1 || got[0] != want {
		t.Errorf("warnings = %q, want [%q]", got, want)
	}
}

func TestUndeclaredDriverWarningsAreEmptyWhenEveryDriverIsDeclared(t *testing.T) {
	deckPath := loadDeckForTest(t, "---\ndrivers:\n  sqlite: {}\n---\n\n# Query\n\n```sql {driver: sqlite}\nSELECT 1;\n```\n")
	cfg, _ := config.Load(deckPath)
	pres, _, _, _, _, err := loadPresentation(deckPath, cfg, filepath.Dir(deckPath))
	if err != nil {
		t.Fatal(err)
	}
	if got := undeclaredDriverWarnings(deckPath, pres); len(got) != 0 {
		t.Errorf("warnings = %q, want none", got)
	}
}
