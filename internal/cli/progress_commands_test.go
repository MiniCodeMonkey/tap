package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const progressDeck = "---\ntitle: Progress\n---\n\n# One\n\n---\n\n# Two\n"

func writeProgressDeck(t *testing.T) string {
	t.Helper()
	deckPath := filepath.Join(t.TempDir(), "progress.md")
	if err := os.WriteFile(deckPath, []byte(progressDeck), 0o644); err != nil {
		t.Fatal(err)
	}
	return deckPath
}

func phasesOf(lines []map[string]any) string {
	var phases []string
	for _, line := range lines {
		phases = append(phases, line["phase"].(string))
	}
	return strings.Join(phases, ",")
}

func TestProgressFlagOnTheLongRunningCommands(t *testing.T) {
	for _, path := range [][]string{{"export", "pdf"}, {"export", "images"}, {"build"}} {
		command, _, err := rootCmd.Find(path)
		if err != nil {
			t.Fatalf("%v not found: %v", path, err)
		}
		if command.Flags().Lookup("progress") == nil {
			t.Errorf("tap %s has no --progress", strings.Join(path, " "))
		}
	}
}

func TestBuildProgressJSON(t *testing.T) {
	deckPath := writeProgressDeck(t)
	outputDir := filepath.Join(t.TempDir(), "dist")

	exitCode, _, stderr := runTap(t, "build", deckPath, "--output", outputDir, "--progress", "json")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}

	lines := checkProgressOutput(t, stderr)
	if phasesOf(lines) != "load,parse,bundle,write,done" {
		t.Errorf("phases = %s, want load,parse,bundle,write,done", phasesOf(lines))
	}
	done := lines[len(lines)-1]
	if done["ok"] != true || done["output"] != outputDir {
		t.Errorf("done line = %v, want ok and output %q", done, outputDir)
	}
	if files, _ := done["files"].(float64); files < 1 {
		t.Errorf("done line files = %v, want at least 1", done["files"])
	}
}

func TestBuildProgressJSONFailure(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.md")

	exitCode, stdout, stderr := runTap(t, "build", missing, "--progress", "json")
	if exitCode != exitUserError {
		t.Errorf("exit code = %d, want %d", exitCode, exitUserError)
	}
	lines := checkProgressOutput(t, stderr)
	done := lines[len(lines)-1]
	errorObject, _ := done["error"].(map[string]any)
	if done["ok"] != false || errorObject["code"] != "deck_not_found" {
		t.Errorf("done line = %v, want a deck_not_found failure", done)
	}
	if strings.Contains(stderr, "Error:") {
		t.Errorf("stderr has the human error line next to the done line: %q", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want nothing", stdout)
	}
}

func TestProgressRejectsAnUnknownFormat(t *testing.T) {
	deckPath := writeProgressDeck(t)
	exitCode, _, stderr := runTap(t, "build", deckPath, "--progress", "xml")
	if exitCode != exitUserError || !strings.Contains(stderr, "the only format is json") {
		t.Errorf("exit %d, stderr %q", exitCode, stderr)
	}
}

func TestExportPDFProgressJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}
	deckPath := writeProgressDeck(t)
	outputPath := filepath.Join(t.TempDir(), "progress.pdf")

	exitCode, _, stderr := runTap(t, "export", "pdf", deckPath, "--output", outputPath, "--progress", "json")
	if exitCode == exitInternal && os.Getenv("CI") == "" {
		t.Skipf("skipping: no browser: %s", stderr)
	}
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}

	lines := checkProgressOutput(t, stderr)
	var renders []string
	for _, line := range lines {
		if line["phase"] == "render" {
			// fmt prints a whole float64 such as 1 as "1".
			renders = append(renders, fmt.Sprintf("%v/%v", line["done"], line["total"]))
		}
	}
	if strings.Join(renders, " ") != "1/2 2/2" {
		t.Errorf("render lines = %v, want 1/2 2/2", renders)
	}
	done := lines[len(lines)-1]
	if done["ok"] != true || done["pages"] != float64(2) || done["output"] != outputPath {
		t.Errorf("done line = %v, want ok, 2 pages and output %q", done, outputPath)
	}
}

func TestExportImagesProgressJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}
	deckPath := writeProgressDeck(t)
	outputDir := filepath.Join(t.TempDir(), "images")

	exitCode, _, stderr := runTap(t, "export", "images", deckPath, "--all", "--output", outputDir, "--progress", "json")
	if exitCode == exitInternal && os.Getenv("CI") == "" {
		t.Skipf("skipping: no browser: %s", stderr)
	}
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}

	lines := checkProgressOutput(t, stderr)
	if phasesOf(lines) != "render,render,done" {
		t.Errorf("phases = %s, want render,render,done", phasesOf(lines))
	}
	files, _ := lines[len(lines)-1]["files"].([]any)
	if len(files) != 2 {
		t.Errorf("done line files = %v, want 2 files", lines[len(lines)-1]["files"])
	}
}
