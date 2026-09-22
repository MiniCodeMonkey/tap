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

func TestBuildAllSlidesSkippedIsAUserError(t *testing.T) {
	for name, content := range map[string]string{
		"3 slides, all skipped": "<!-- skip: true -->\n\n# One\n\n---\n\n<!-- skip: true -->\n\n# Two\n\n---\n\n<!-- skip: true -->\n\n# Three\n",
		"1 slide, skipped":      "<!-- skip: true -->\n\n# Only\n",
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			deck := filepath.Join(dir, "talk.md")
			if err := os.WriteFile(deck, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(dir, "dist")

			exitCode, stdout, stderr := runTap(t, "build", deck, "-o", output, "--json")
			if exitCode != exitUserError {
				t.Fatalf("exit code = %d, want %d; stdout %q, stderr %q", exitCode, exitUserError, stdout, stderr)
			}
			if !strings.Contains(stdout, `"code": "invalid_deck"`) {
				t.Errorf("stdout = %q, want an invalid_deck JSON error", stdout)
			}
			if !strings.Contains(stdout, "every slide has skip: true, so there is nothing to export") {
				t.Errorf("stdout = %q, want the same message the exporters use", stdout)
			}
			if _, err := os.Stat(filepath.Join(output, "index.html")); err == nil {
				t.Error("the build wrote an index.html for a deck with nothing to build")
			}
		})
	}
}
