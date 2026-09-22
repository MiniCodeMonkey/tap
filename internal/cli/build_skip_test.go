package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildLeavesOutSkippedSlides(t *testing.T) {
	dir := t.TempDir()
	deck := filepath.Join(dir, "talk.md")
	content := "# Kept\n\n---\n\n<!-- skip: true -->\n\n# Left out of the build\n\n---\n\n# Also kept\n"
	if err := os.WriteFile(deck, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(dir, "dist")

	exitCode, stdout, stderr := runTap(t, "build", deck, "-o", output, "--json")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stdout %q, stderr %q", exitCode, stdout, stderr)
	}
	built, err := os.ReadFile(filepath.Join(output, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(built), "Left out of the build") {
		t.Error("the built deck holds the skipped slide")
	}
	if !strings.Contains(string(built), "Also kept") {
		t.Error("the built deck lost a slide that is not skipped")
	}
}
