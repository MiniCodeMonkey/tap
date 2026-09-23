package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/components"
)

// resetComponentFlags clears the package-level flag variables the
// component tests mutate, so tests don't leak state into each other.
func resetComponentFlags() {
	componentInline = false
	componentTS = false
}

// withWorkingDirectory runs fn with the process working directory set to
// dir, restoring the original directory afterward.
func withWorkingDirectory(t *testing.T, dir string, fn func()) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("os.Chdir(%q): %v", dir, err)
	}
	defer func() {
		if err := os.Chdir(original); err != nil {
			t.Fatalf("os.Chdir(%q): %v", original, err)
		}
	}()
	fn()
}

// TestComponentNewDeckFolderArgument checks that a folder argument writes
// into that folder, not its parent.
func TestComponentNewDeckFolderArgument(t *testing.T) {
	parent := t.TempDir()
	deckDir := filepath.Join(parent, "my-talk")
	if err := os.Mkdir(deckDir, 0o755); err != nil {
		t.Fatalf("failed to create deck directory: %v", err)
	}
	resetComponentFlags()

	if _, err := scaffoldComponent("RollingDeploy", deckDir); err != nil {
		t.Fatalf("scaffoldComponent: %v", err)
	}

	componentPath := filepath.Join(deckDir, "slides", "RollingDeploy.jsx")
	if _, err := os.Stat(componentPath); err != nil {
		t.Fatalf("expected component to be written inside the deck directory at %s: %v", componentPath, err)
	}

	parentComponentPath := filepath.Join(parent, "slides", "RollingDeploy.jsx")
	if _, err := os.Stat(parentComponentPath); err == nil {
		t.Errorf("did not expect a component written into the parent directory at %s", parentComponentPath)
	}
}

func TestComponentNewJSONPrintsTheSnippet(t *testing.T) {
	deckDir := t.TempDir()
	exitCode, stdout, stderr := runTap(t, "component", "new", "Counter", deckDir, "--json")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}
	var output struct {
		OK      bool     `json:"ok"`
		Files   []string `json:"files"`
		Snippet string   `json:"snippet"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if !output.OK || len(output.Files) != 1 {
		t.Errorf("output = %+v, want ok and one file", output)
	}
	if output.Snippet != componentSnippet("Counter", "jsx", false) {
		t.Errorf("snippet = %q, want %q", output.Snippet, componentSnippet("Counter", "jsx", false))
	}
}

func TestComponentNewCommandShape(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"component", "new"})
	if err != nil || command.Name() != "new" || command.Parent().Name() != "component" {
		t.Fatalf("tap component new not found: %v", err)
	}
	if command.Use != "new <Name> [deck]" {
		t.Errorf("Use = %q, want %q", command.Use, "new <Name> [deck]")
	}
	if command.Flags().Lookup("deck") != nil {
		t.Error("--deck should be removed: the deck is a positional argument")
	}
}

func TestScaffoldComponentReturnsFilesAndSnippet(t *testing.T) {
	resetComponentFlags()
	deckDir := t.TempDir()
	result, err := scaffoldComponent("RollingDeploy", deckDir)
	if err != nil {
		t.Fatalf("scaffoldComponent() error = %v", err)
	}
	wantFile := filepath.Join(deckDir, "slides", "RollingDeploy.jsx")
	if len(result.Files) != 1 || result.Files[0] != wantFile {
		t.Errorf("Files = %v, want [%s]", result.Files, wantFile)
	}
	if result.Snippet != componentSnippet("RollingDeploy", "jsx", false) {
		t.Errorf("Snippet = %q", result.Snippet)
	}
}

func TestComponentNewWholeSlideJSX(t *testing.T) {
	dir := t.TempDir()
	resetComponentFlags()

	withWorkingDirectory(t, dir, func() {
		if _, err := scaffoldComponent("RollingDeploy", ""); err != nil {
			t.Fatalf("scaffoldComponent: %v", err)
		}
	})

	componentPath := filepath.Join(dir, "slides", "RollingDeploy.jsx")
	assertBuildsCleanly(t, componentPath, dir)

	bundle, errs := components.Build(componentPath, components.Options{DeckDirectory: dir})
	if len(errs) > 0 {
		t.Fatalf("unexpected build errors: %v", errs)
	}
	if bundle.Steps != 3 {
		t.Errorf("Steps = %d, want 3", bundle.Steps)
	}
	if !bundle.HasStepsExport {
		t.Errorf("HasStepsExport = false, want true")
	}
}

func TestComponentNewWholeSlideTSX(t *testing.T) {
	dir := t.TempDir()
	resetComponentFlags()
	componentTS = true

	withWorkingDirectory(t, dir, func() {
		if _, err := scaffoldComponent("RollingDeploy", ""); err != nil {
			t.Fatalf("scaffoldComponent: %v", err)
		}
	})

	componentPath := filepath.Join(dir, "slides", "RollingDeploy.tsx")
	assertBuildsCleanly(t, componentPath, dir)

	envPath := filepath.Join(dir, "tap-env.d.ts")
	if _, err := os.Stat(envPath); err != nil {
		t.Fatalf("tap-env.d.ts was not written: %v", err)
	}
	shimsPath := filepath.Join(dir, "tap-shims.d.ts")
	if _, err := os.Stat(shimsPath); err != nil {
		t.Fatalf("tap-shims.d.ts was not written: %v", err)
	}

	bundle, errs := components.Build(componentPath, components.Options{DeckDirectory: dir})
	if len(errs) > 0 {
		t.Fatalf("unexpected build errors: %v", errs)
	}
	if bundle.Steps != 3 || !bundle.HasStepsExport {
		t.Errorf("Steps = %d, HasStepsExport = %v, want 3, true", bundle.Steps, bundle.HasStepsExport)
	}
}

func TestComponentNewInlineJSX(t *testing.T) {
	dir := t.TempDir()
	resetComponentFlags()
	componentInline = true

	withWorkingDirectory(t, dir, func() {
		if _, err := scaffoldComponent("LatencyDrop", ""); err != nil {
			t.Fatalf("scaffoldComponent: %v", err)
		}
	})

	componentPath := filepath.Join(dir, "components", "LatencyDrop.jsx")
	assertBuildsCleanly(t, componentPath, dir)
}

func TestComponentNewInlineTSX(t *testing.T) {
	dir := t.TempDir()
	resetComponentFlags()
	componentInline = true
	componentTS = true

	withWorkingDirectory(t, dir, func() {
		if _, err := scaffoldComponent("LatencyDrop", ""); err != nil {
			t.Fatalf("scaffoldComponent: %v", err)
		}
	})

	componentPath := filepath.Join(dir, "components", "LatencyDrop.tsx")
	assertBuildsCleanly(t, componentPath, dir)

	envPath := filepath.Join(dir, "tap-env.d.ts")
	if _, err := os.Stat(envPath); err != nil {
		t.Fatalf("tap-env.d.ts was not written: %v", err)
	}
	shimsPath := filepath.Join(dir, "tap-shims.d.ts")
	if _, err := os.Stat(shimsPath); err != nil {
		t.Fatalf("tap-shims.d.ts was not written: %v", err)
	}
}

func TestComponentNewInvalidName(t *testing.T) {
	dir := t.TempDir()
	resetComponentFlags()

	withWorkingDirectory(t, dir, func() {
		if _, err := scaffoldComponent("rollingDeploy", ""); err == nil {
			t.Fatalf("expected an error for a non-PascalCase name")
		}
	})
}

func TestComponentNewRefusesOverwrite(t *testing.T) {
	dir := t.TempDir()
	resetComponentFlags()

	withWorkingDirectory(t, dir, func() {
		if _, err := scaffoldComponent("RollingDeploy", ""); err != nil {
			t.Fatalf("first scaffoldComponent: %v", err)
		}
		_, err := scaffoldComponent("RollingDeploy", "")
		if err == nil {
			t.Fatalf("expected an error on the second write")
		}
		if !strings.Contains(err.Error(), filepath.Join("slides", "RollingDeploy.jsx")) {
			t.Errorf("error %q does not name the existing file", err.Error())
		}
	})
}

func TestComponentNewLeavesExistingTapEnv(t *testing.T) {
	dir := t.TempDir()
	resetComponentFlags()
	componentTS = true

	envPath := filepath.Join(dir, "tap-env.d.ts")
	custom := []byte("// custom tap-env.d.ts, left alone\n")
	if err := os.WriteFile(envPath, custom, 0o644); err != nil {
		t.Fatalf("failed to seed tap-env.d.ts: %v", err)
	}
	shimsPath := filepath.Join(dir, "tap-shims.d.ts")
	customShims := []byte("// custom tap-shims.d.ts, left alone\n")
	if err := os.WriteFile(shimsPath, customShims, 0o644); err != nil {
		t.Fatalf("failed to seed tap-shims.d.ts: %v", err)
	}

	withWorkingDirectory(t, dir, func() {
		if _, err := scaffoldComponent("RollingDeploy", ""); err != nil {
			t.Fatalf("scaffoldComponent: %v", err)
		}
	})

	got, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("failed to read tap-env.d.ts: %v", err)
	}
	if !bytes.Equal(got, custom) {
		t.Errorf("tap-env.d.ts was overwritten; got %q, want unchanged %q", got, custom)
	}

	gotShims, err := os.ReadFile(shimsPath)
	if err != nil {
		t.Fatalf("failed to read tap-shims.d.ts: %v", err)
	}
	if !bytes.Equal(gotShims, customShims) {
		t.Errorf("tap-shims.d.ts was overwritten; got %q, want unchanged %q", gotShims, customShims)
	}
}

func TestComponentSnippetWholeSlide(t *testing.T) {
	got := componentSnippet("RollingDeploy", "jsx", false)
	want := "<!--\nlayout: ./slides/RollingDeploy.jsx\n-->\n\n# Title\n"
	if got != want {
		t.Errorf("componentSnippet() = %q, want %q", got, want)
	}
}

func TestComponentSnippetInline(t *testing.T) {
	got := componentSnippet("LatencyDrop", "tsx", true)
	want := "```component ./components/LatencyDrop.tsx\n{ \"label\": \"Requests per second\", \"value\": 1200 }\n```\n"
	if got != want {
		t.Errorf("componentSnippet() = %q, want %q", got, want)
	}
}

// assertBuildsCleanly bundles componentPath with internal/components.Build
// and fails the test on any build error or warning: the generated template
// must be a working component, not just syntactically present.
func assertBuildsCleanly(t *testing.T, componentPath, deckDirectory string) {
	t.Helper()

	bundle, errs := components.Build(componentPath, components.Options{DeckDirectory: deckDirectory})
	if len(errs) > 0 {
		t.Fatalf("unexpected build errors for %s: %v", componentPath, errs)
	}
	if len(bundle.Warnings) > 0 {
		t.Fatalf("unexpected build warnings for %s: %v", componentPath, bundle.Warnings)
	}
}
