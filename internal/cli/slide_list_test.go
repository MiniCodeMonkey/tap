package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/slidelist"
)

func TestSlideListCommandShape(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"slide", "list"})
	if err != nil || command.Name() != "list" || command.Parent().Name() != "slide" {
		t.Fatalf("tap slide list not found: %v", err)
	}
	if command.Use != "list [deck]" {
		t.Errorf("Use = %q, want %q", command.Use, "list [deck]")
	}
	if command.Flags().Lookup("json") == nil {
		t.Error("missing --json")
	}
}

func TestSlideListJSON(t *testing.T) {
	deck := filepath.Join("..", "..", "examples", "conference-talk.md")
	exitCode, stdout, stderr := runTap(t, "slide", "list", deck, "--json")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}
	if !strings.HasPrefix(stdout, "{\n  \"ok\": true,\n  \"slides\": [") {
		t.Errorf("stdout starts %q, want ok first, then slides", stdout[:min(len(stdout), 40)])
	}

	var output struct {
		OK     bool              `json:"ok"`
		Slides []slidelist.Slide `json:"slides"`
		Errors []string          `json:"errors"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if !output.OK || len(output.Slides) != 9 || output.Errors == nil {
		t.Fatalf("output = %+v", output)
	}
	block := output.Slides[3].CodeBlocks[0]
	if block.Driver != "sqlite" || !block.Live || block.Line != 40 {
		t.Errorf("slide 4 block = %+v", block)
	}
}

func TestSlideListTable(t *testing.T) {
	dir := t.TempDir()
	deck := filepath.Join(dir, "talk.md")
	if err := os.WriteFile(deck, []byte("# One\n\n---\n\n<!-- skip: true -->\n\n# Two\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	exitCode, stdout, stderr := runTap(t, "slide", "list", deck)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 3 || !strings.HasPrefix(lines[0], "#") {
		t.Fatalf("stdout =\n%s\nwant a header and two rows", stdout)
	}
	if !strings.Contains(lines[2], "5-7") || !strings.Contains(lines[2], "Two") || !strings.Contains(lines[2], "skipped") {
		t.Errorf("row 2 = %q, want lines 5-7, the title and \"skipped\"", lines[2])
	}
}

func TestSlideListMissingDeck(t *testing.T) {
	exitCode, stdout, _ := runTap(t, "slide", "list", filepath.Join(t.TempDir(), "missing.md"), "--json")
	if exitCode != exitUserError || !strings.Contains(stdout, `"code": "deck_not_found"`) {
		t.Errorf("exit %d, stdout %q", exitCode, stdout)
	}
}
