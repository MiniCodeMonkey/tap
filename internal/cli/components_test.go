package cli

import (
	"path/filepath"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/components"
	"github.com/MiniCodeMonkey/tap/internal/parser"
)

// componentsTestdataDir points at internal/components/testdata, reused here
// instead of duplicating fixture components.
func componentsTestdataDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(filepath.Join("..", "components", "testdata"))
	if err != nil {
		t.Fatalf("failed to resolve testdata dir: %v", err)
	}
	return dir
}

func TestBuildComponents_ResolvesAndReturnsSortedErrors(t *testing.T) {
	deckDir := componentsTestdataDir(t)
	pres := &parser.Presentation{
		Slides: []parser.Slide{
			{Directives: parser.SlideDirectives{Layout: "./syntax-error/Broken.jsx"}},
			{Directives: parser.SlideDirectives{Layout: "./tiny/Tiny.jsx"}},
		},
	}

	resolved, errs := buildComponents(pres, deckDir, false, true, "/components/")

	if len(resolved) != 2 {
		t.Fatalf("expected 2 resolved paths, got %d", len(resolved))
	}
	if resolved["./tiny/Tiny.jsx"].Bundle == nil {
		t.Error("expected ./tiny/Tiny.jsx to build successfully")
	}
	if len(errs) == 0 {
		t.Error("expected build errors for the broken component")
	}
}

func TestComponentBundleFiles_BuildsFileMap(t *testing.T) {
	resolved := map[string]components.Result{
		"./slides/RollingDeploy.jsx": {
			Bundle: &components.Bundle{
				Name:       "RollingDeploy",
				Hash:       "abc123",
				JavaScript: []byte("export default 1;"),
				CSS:        []byte("body{}"),
				SourceMap:  []byte("{}"),
			},
		},
		"./slides/Broken.jsx": {
			Errors: []components.BuildError{{File: "Broken.jsx", Line: 1, Column: 1, Message: "oops"}},
		},
	}

	files := componentBundleFiles(resolved)

	if _, ok := files["RollingDeploy-abc123.js"]; !ok {
		t.Error("expected the .js file to be present")
	}
	if _, ok := files["RollingDeploy-abc123.css"]; !ok {
		t.Error("expected the .css file to be present")
	}
	if _, ok := files["RollingDeploy-abc123.js.map"]; !ok {
		t.Error("expected the .js.map file to be present")
	}
	if len(files) != 3 {
		t.Errorf("expected exactly 3 files (nothing for the broken component), got %d: %+v", len(files), files)
	}
}

// TestComponentBundleFiles_IncludesEmittedAssets checks that a bundle's
// emitted (not inlined) assets are added to the same file map the JS, CSS,
// and source map already go into, so the existing /components/ server
// route (see internal/server/routes.go's generic ComponentBundleStore
// lookup) serves them with no server-side change needed.
func TestComponentBundleFiles_IncludesEmittedAssets(t *testing.T) {
	resolved := map[string]components.Result{
		"./slides/RollingDeploy.jsx": {
			Bundle: &components.Bundle{
				Name:       "RollingDeploy",
				Hash:       "abc123",
				JavaScript: []byte("export default 1;"),
				Assets: []components.Asset{
					{Name: "photo-xyz.png", Content: []byte("fake-png-bytes"), ContentType: "image/png"},
				},
			},
		},
	}

	files := componentBundleFiles(resolved)

	asset, ok := files["photo-xyz.png"]
	if !ok {
		t.Fatal("expected the emitted asset to be present in the file map")
	}
	if asset.ContentType != "image/png" {
		t.Errorf("ContentType = %q, want %q", asset.ContentType, "image/png")
	}
	if string(asset.Content) != "fake-png-bytes" {
		t.Errorf("Content = %q, want %q", asset.Content, "fake-png-bytes")
	}
}

func TestExternalInputDirs_OutsideDeckTreeOnly(t *testing.T) {
	deckDir := "/decks/mytalk"
	resolved := map[string]components.Result{
		"./slides/RollingDeploy.jsx": {
			Bundle: &components.Bundle{
				Name: "RollingDeploy",
				Hash: "abc123",
				Inputs: []string{
					"/decks/mytalk/slides/RollingDeploy.jsx", // inside the deck tree
					"/decks/shared/Thing.jsx",                // outside it
				},
			},
		},
		"./slides/Broken.jsx": {
			Errors: []components.BuildError{{File: "Broken.jsx", Message: "oops"}},
		},
	}

	dirs := externalInputDirs(resolved, deckDir)

	if len(dirs) != 1 {
		t.Fatalf("expected 1 external directory, got %d: %v", len(dirs), dirs)
	}
	if dirs[0] != "/decks/shared" {
		t.Errorf("expected /decks/shared, got %q", dirs[0])
	}
}

func TestExternalInputDirs_NoBundleNoResultSkipped(t *testing.T) {
	resolved := map[string]components.Result{
		"./slides/Broken.jsx": {Errors: []components.BuildError{{File: "Broken.jsx", Message: "oops"}}},
	}

	dirs := externalInputDirs(resolved, "/decks/mytalk")

	if len(dirs) != 0 {
		t.Errorf("expected no directories for a component with no successful bundle, got %v", dirs)
	}
}

func TestComponentErrorsError_NilWhenEmpty(t *testing.T) {
	if err := componentErrorsError(nil); err != nil {
		t.Errorf("expected nil for no errors, got %v", err)
	}
}

func TestComponentErrorsError_FormatsEachLine(t *testing.T) {
	errs := []components.BuildError{
		{File: "Broken.jsx", Line: 2, Column: 3, Message: "Unexpected token"},
	}
	err := componentErrorsError(errs)
	if err == nil {
		t.Fatal("expected a non-nil error")
	}
	want := "error: Broken.jsx:2:3: Unexpected token"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestComponentWarnings_FlattensSortedByPathAndFormatsEachLine(t *testing.T) {
	resolved := map[string]components.Result{
		"./slides/RollingDeploy.jsx": {
			Bundle: &components.Bundle{
				Name: "RollingDeploy",
				Warnings: []components.BuildError{
					{File: "RollingDeploy.jsx", Line: 4, Column: 9, Message: "unused variable \"x\""},
				},
			},
		},
		"./charts/LatencyDrop.jsx": {
			Bundle: &components.Bundle{Name: "LatencyDrop"},
		},
		"./slides/Broken.jsx": {
			Errors: []components.BuildError{{File: "Broken.jsx", Message: "oops"}},
		},
	}

	warnings := componentWarnings(resolved)

	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning (none from the bundle-less and warning-less results), got %d: %+v", len(warnings), warnings)
	}
	want := "RollingDeploy.jsx:4:9: unused variable \"x\""
	if warnings[0].Error() != want {
		t.Errorf("got %q, want %q", warnings[0].Error(), want)
	}
}

// TestComponentWarningLines_FormatsWithWarningPrefix checks the plain
// "warning: ..." lines fed to the TUI model's warnings display (see
// internal/tui/dev.go's viewWarnings): the same format
// printComponentWarningsToStderr prints, but as strings a caller renders
// itself instead of writing straight to the terminal.
func TestComponentWarningLines_FormatsWithWarningPrefix(t *testing.T) {
	warnings := []components.BuildError{
		{File: "RollingDeploy.jsx", Line: 4, Column: 9, Message: "unused variable \"x\""},
	}

	lines := componentWarningLines(warnings)

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %+v", len(lines), lines)
	}
	want := "warning: RollingDeploy.jsx:4:9: unused variable \"x\""
	if lines[0] != want {
		t.Errorf("got %q, want %q", lines[0], want)
	}
}
