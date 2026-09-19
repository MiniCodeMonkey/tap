package components

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHostModuleNames(t *testing.T) {
	want := []string{"react", "react/jsx-runtime", "react-dom", "react-dom/client", "motion", "motion/react", "tap"}
	got := HostModuleNames()
	if len(got) != len(want) {
		t.Fatalf("HostModuleNames() = %v, want %v", got, want)
	}
	for i, name := range want {
		if got[i] != name {
			t.Errorf("HostModuleNames()[%d] = %q, want %q", i, got[i], name)
		}
	}
}

func TestBuildTinyComponent(t *testing.T) {
	bundle, errs := Build("Tiny.jsx", Options{DeckDirectory: "testdata/tiny"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if bundle.Name != "Tiny" {
		t.Errorf("Name = %q, want %q", bundle.Name, "Tiny")
	}
	if len(bundle.Hash) != 12 {
		t.Errorf("Hash = %q, want 12 hex characters", bundle.Hash)
	}
	if !strings.Contains(string(bundle.JavaScript), "Tiny") {
		t.Errorf("JavaScript does not contain the component body: %s", bundle.JavaScript)
	}
	t.Logf("tiny bundle size: %d bytes", len(bundle.JavaScript))
}

func TestBuildTsxComponent(t *testing.T) {
	bundle, errs := Build("Tsx.tsx", Options{DeckDirectory: "testdata/tsx"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if bundle.Name != "Tsx" {
		t.Errorf("Name = %q, want %q", bundle.Name, "Tsx")
	}
	// TypeScript type annotations must be stripped, never bundled as-is.
	if strings.Contains(string(bundle.JavaScript), ": number") {
		t.Errorf("JavaScript still contains a TypeScript type annotation: %s", bundle.JavaScript)
	}
}

func TestBuildShimsHostModulesOutOfTheBundle(t *testing.T) {
	bundle, errs := Build("UsesHost.jsx", Options{DeckDirectory: "testdata/host-modules"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	javascript := string(bundle.JavaScript)
	if !strings.Contains(javascript, `__TAP_HOST__`) {
		t.Errorf("JavaScript does not reference window.__TAP_HOST__: %s", javascript)
	}
	// If react, motion, or tap were bundled instead of shimmed, esbuild
	// would have failed to resolve them (none are installed in this
	// fixture's node_modules), so a successful build with no errors is
	// itself evidence they were externalized. Also check their own
	// source never leaked in as literal text.
	for _, marker := range []string{"ReactCurrentDispatcher", "useStepInternal"} {
		if strings.Contains(javascript, marker) {
			t.Errorf("JavaScript unexpectedly contains host library internals: %q", marker)
		}
	}
}

func TestBuildResolvesFixtureNodeModules(t *testing.T) {
	bundle, errs := Build("UsesLib.jsx", Options{DeckDirectory: "testdata/npm-import"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	javascript := string(bundle.JavaScript)
	if !strings.Contains(javascript, "toUpperCase") {
		t.Errorf("JavaScript does not contain the resolved package's body: %s", javascript)
	}
}

func TestBuildMissingPackageError(t *testing.T) {
	_, errs := Build("Missing.jsx", Options{DeckDirectory: "testdata/missing-package"})
	if len(errs) == 0 {
		t.Fatal("expected an error for a missing package")
	}
	message := errs[0].Message
	if !strings.Contains(message, "left-pad-not-installed") {
		t.Errorf("message = %q, want it to name the missing package", message)
	}
	if !strings.Contains(message, "npm install") {
		t.Errorf("message = %q, want it to suggest npm install", message)
	}
}

func TestBuildUnresolvedHostSubpathPointsAtTapEntryPoints(t *testing.T) {
	_, errs := Build("UsesDevRuntime.jsx", Options{DeckDirectory: "testdata/host-module-subpath"})
	if len(errs) == 0 {
		t.Fatal("expected an error for an unresolved host module subpath")
	}
	message := errs[0].Message
	if strings.Contains(message, "npm install") {
		t.Errorf("message = %q, want no npm install suggestion for a tap-provided module", message)
	}
	if !strings.Contains(message, "react/jsx-dev-runtime") {
		t.Errorf("message = %q, want it to name the unresolved subpath", message)
	}
	if !strings.Contains(message, "react/jsx-runtime") {
		t.Errorf("message = %q, want it to list tap's actual entry points", message)
	}
}

func TestBuildCSSImport(t *testing.T) {
	bundle, errs := Build("WithCss.jsx", Options{DeckDirectory: "testdata/css-import"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if !strings.Contains(string(bundle.CSS), "hotpink") {
		t.Errorf("CSS = %q, want it to contain the imported rule", bundle.CSS)
	}
}

func TestBuildSyntaxError(t *testing.T) {
	_, errs := Build("Broken.jsx", Options{DeckDirectory: "testdata/syntax-error"})
	if len(errs) == 0 {
		t.Fatal("expected a syntax error")
	}
	err := errs[0]
	if err.Line == 0 {
		t.Errorf("Line = %d, want a nonzero line", err.Line)
	}
	if !strings.HasSuffix(err.File, "Broken.jsx") {
		t.Errorf("File = %q, want it to name the broken file", err.File)
	}
	if err.File != "Broken.jsx" {
		t.Errorf("File = %q, want a clean path relative to the deck directory (\"Broken.jsx\")", err.File)
	}
	if strings.Contains(err.File, "..") {
		t.Errorf("File = %q, want no \"..\" segments", err.File)
	}
	if err.Message == "" {
		t.Errorf("Message is empty")
	}
	formatted := err.Error()
	if !strings.Contains(formatted, "Broken.jsx:") {
		t.Errorf("Error() = %q, want it to start with the file", formatted)
	}
}

func TestBuildStepsExport(t *testing.T) {
	bundle, errs := Build("WithSteps.jsx", Options{DeckDirectory: "testdata/steps-export"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if !bundle.HasStepsExport {
		t.Error("HasStepsExport = false, want true")
	}
	if bundle.Steps != 5 {
		t.Errorf("Steps = %d, want 5", bundle.Steps)
	}
}

func TestBuildStepsExportWithTypeAnnotation(t *testing.T) {
	bundle, errs := Build("WithTypedSteps.tsx", Options{DeckDirectory: "testdata/steps-export-typed"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if !bundle.HasStepsExport {
		t.Error("HasStepsExport = false, want true")
	}
	if bundle.Steps != 3 {
		t.Errorf("Steps = %d, want 3", bundle.Steps)
	}
}

func TestBuildNoStepsExport(t *testing.T) {
	bundle, errs := Build("Tiny.jsx", Options{DeckDirectory: "testdata/tiny"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if bundle.HasStepsExport {
		t.Error("HasStepsExport = true, want false")
	}
	if bundle.Steps != 0 {
		t.Errorf("Steps = %d, want 0", bundle.Steps)
	}
}

func TestBuildPreviewDisabled(t *testing.T) {
	bundle, errs := Build("NoPreview.jsx", Options{DeckDirectory: "testdata/preview-disabled"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if !bundle.PreviewDisabled {
		t.Error("PreviewDisabled = false, want true")
	}
}

func TestBuildPreviewEnabledByDefault(t *testing.T) {
	bundle, errs := Build("Tiny.jsx", Options{DeckDirectory: "testdata/tiny"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if bundle.PreviewDisabled {
		t.Error("PreviewDisabled = true, want false")
	}
}

func TestBuildRejectsPathOutsideDeckDirectory(t *testing.T) {
	_, errs := Build("../outside/Outside.jsx", Options{DeckDirectory: "testdata/tiny"})
	if len(errs) == 0 {
		t.Fatal("expected an error for a path outside the deck directory")
	}
	if !strings.Contains(errs[0].Message, "deck's folder") {
		t.Errorf("message = %q, want it to explain components must live inside the deck's folder", errs[0].Message)
	}
	if strings.Contains(errs[0].Error(), ":0:0:") {
		t.Errorf("Error() = %q, want no \":0:0:\" position for an error with no source location", errs[0].Error())
	}
}

func TestBuildTracksInputFiles(t *testing.T) {
	bundle, errs := Build("WithCss.jsx", Options{DeckDirectory: "testdata/css-import"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(bundle.Inputs) < 2 {
		t.Fatalf("Inputs = %v, want at least the entry file and the imported CSS", bundle.Inputs)
	}
	foundEntry := false
	foundCSS := false
	for _, input := range bundle.Inputs {
		if strings.HasSuffix(input, "WithCss.jsx") {
			foundEntry = true
		}
		if strings.HasSuffix(input, "deploy.css") {
			foundCSS = true
		}
	}
	if !foundEntry {
		t.Errorf("Inputs %v does not include the entry file", bundle.Inputs)
	}
	if !foundCSS {
		t.Errorf("Inputs %v does not include the imported CSS file", bundle.Inputs)
	}
}

func TestBuildIgnoresCommentedOutStepsExport(t *testing.T) {
	bundle, errs := Build("OnlyCommented.jsx", Options{DeckDirectory: "testdata/steps-comments"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if bundle.HasStepsExport {
		t.Error("HasStepsExport = true, want false: the only steps export is commented out")
	}
	if bundle.Steps != 0 {
		t.Errorf("Steps = %d, want 0", bundle.Steps)
	}
}

func TestBuildFindsStepsExportAfterJSXApostropheAndLineComment(t *testing.T) {
	// The JSX text "Don't panic" has an unmatched quote; a naive
	// hand-written scanner treating it as a string opener would scan to
	// end of file and miss the following line comment and real export.
	// esbuild's own parser has no such problem.
	bundle, errs := Build("Commented.jsx", Options{DeckDirectory: "testdata/steps-comments"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if !bundle.HasStepsExport {
		t.Fatal("HasStepsExport = false, want true: the real export follows the JSX text and the comment")
	}
	if bundle.Steps != 3 {
		t.Errorf("Steps = %d, want 3 (not 99 from the commented-out line)", bundle.Steps)
	}
}

func TestBuildFindsStepsExportAfterJSXApostropheAndBlockComment(t *testing.T) {
	bundle, errs := Build("BlockCommented.jsx", Options{DeckDirectory: "testdata/steps-comments"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if !bundle.HasStepsExport {
		t.Fatal("HasStepsExport = false, want true: the real export follows the JSX text and the block comment")
	}
	if bundle.Steps != 3 {
		t.Errorf("Steps = %d, want 3 (not 98 from the commented-out line)", bundle.Steps)
	}
}

func TestBuildFindsStepsExportAfterRegexAndURLLookalikes(t *testing.T) {
	// A regular expression literal containing "//" and JSX text
	// containing "https://" both look like a line comment to a naive
	// scanner but not to esbuild's parser.
	bundle, errs := Build("RegexAndUrl.jsx", Options{DeckDirectory: "testdata/steps-jsx-text-lookalikes"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if !bundle.HasStepsExport {
		t.Fatal("HasStepsExport = false, want true")
	}
	if bundle.Steps != 3 {
		t.Errorf("Steps = %d, want 3", bundle.Steps)
	}
}

func TestBuildStepsExportAsConst(t *testing.T) {
	bundle, errs := Build("AsConst.tsx", Options{DeckDirectory: "testdata/steps-export-as-const"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if !bundle.HasStepsExport {
		t.Fatal("HasStepsExport = false, want true for \"export const steps = 5 as const\"")
	}
	if bundle.Steps != 5 {
		t.Errorf("Steps = %d, want 5", bundle.Steps)
	}
}

func TestBuildStepsExportRequiresConstNotLet(t *testing.T) {
	bundle, errs := Build("NotConst.jsx", Options{DeckDirectory: "testdata/steps-export-let"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if bundle.HasStepsExport {
		t.Error("HasStepsExport = true, want false: \"export let steps\" is not a static const export")
	}
	if bundle.Steps != 0 {
		t.Errorf("Steps = %d, want 0", bundle.Steps)
	}
}

func TestBuildStepsExportOverflowIsIgnored(t *testing.T) {
	bundle, errs := Build("Overflow.jsx", Options{DeckDirectory: "testdata/steps-export-overflow"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if bundle.HasStepsExport {
		t.Error("HasStepsExport = true, want false: a value too large for an int must not count as a steps export")
	}
	if bundle.Steps != 0 {
		t.Errorf("Steps = %d, want 0", bundle.Steps)
	}
}

func TestBuildWarnings(t *testing.T) {
	bundle, errs := Build("Warning.jsx", Options{DeckDirectory: "testdata/warning"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(bundle.Warnings) != 1 {
		t.Fatalf("Warnings = %v, want exactly one warning", bundle.Warnings)
	}
	warning := bundle.Warnings[0]
	if !strings.Contains(warning.Message, "Duplicate key") {
		t.Errorf("Warnings[0].Message = %q, want it to mention the duplicate object key", warning.Message)
	}
	if warning.Line == 0 {
		t.Errorf("Warnings[0].Line = 0, want a nonzero line")
	}
	if !strings.HasSuffix(warning.File, "Warning.jsx") {
		t.Errorf("Warnings[0].File = %q, want it to name the file", warning.File)
	}
}

func TestBuildIsDeterministic(t *testing.T) {
	first, errs := Build("Tiny.jsx", Options{DeckDirectory: "testdata/tiny"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	second, errs := Build("Tiny.jsx", Options{DeckDirectory: "testdata/tiny"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if first.Hash != second.Hash {
		t.Errorf("Hash changed between builds of the same fixture: %q vs %q", first.Hash, second.Hash)
	}
	if !bytes.Equal(first.JavaScript, second.JavaScript) {
		t.Error("JavaScript bytes changed between builds of the same fixture")
	}
}

func TestBuildJavaScriptDoesNotLeakAbsoluteMachinePath(t *testing.T) {
	repoRoot, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	// repoRoot is .../internal/components; walk up to the repo root so we
	// check for the machine-specific prefix, not just this package's path.
	repoRoot = filepath.Dir(filepath.Dir(repoRoot))

	bundle, errs := Build("Tiny.jsx", Options{DeckDirectory: "testdata/tiny"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if strings.Contains(string(bundle.JavaScript), repoRoot) {
		t.Errorf("JavaScript contains the machine-specific repo root path %q", repoRoot)
	}
}

func TestBuildRejectsSymlinkEscapingDeckDirectory(t *testing.T) {
	deckDirectory := t.TempDir()
	outsideDirectory := t.TempDir()

	outsideFile := filepath.Join(outsideDirectory, "Outside.jsx")
	outsideSource := "export default function Outside() { return null; }\n"
	if err := os.WriteFile(outsideFile, []byte(outsideSource), 0o644); err != nil {
		t.Fatalf("write outside fixture: %v", err)
	}

	linkPath := filepath.Join(deckDirectory, "Linked.jsx")
	if err := os.Symlink(outsideFile, linkPath); err != nil {
		t.Skipf("symlinks not supported on this platform: %v", err)
	}

	_, errs := Build("Linked.jsx", Options{DeckDirectory: deckDirectory})
	if len(errs) == 0 {
		t.Fatal("expected an error for a symlink that escapes the deck directory")
	}
	if !strings.Contains(errs[0].Message, "deck's folder") {
		t.Errorf("message = %q, want it to explain components must live inside the deck's folder", errs[0].Message)
	}
}

func TestBuildStepsExportRejectsNegativeValue(t *testing.T) {
	bundle, errs := Build("Negative.jsx", Options{DeckDirectory: "testdata/steps-export-negative"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if bundle.HasStepsExport {
		t.Error("HasStepsExport = true, want false: a negative value is not a step count")
	}
	if bundle.Steps != 0 {
		t.Errorf("Steps = %d, want 0", bundle.Steps)
	}
}

func TestBuildAllowsJSXInPlainJSFile(t *testing.T) {
	bundle, errs := Build("PlainJs.js", Options{DeckDirectory: "testdata/js-with-jsx"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if !strings.Contains(string(bundle.JavaScript), "plain-js") {
		t.Errorf("JavaScript does not contain the component body: %s", bundle.JavaScript)
	}
}

func TestBuildRejectsJSONImportFromOutsideDeckDirectory(t *testing.T) {
	root := t.TempDir()
	deckDirectory := filepath.Join(root, "deck")
	if err := os.Mkdir(deckDirectory, 0o755); err != nil {
		t.Fatalf("mkdir deck directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "secrets.json"), []byte(`{"key":"value"}`), 0o644); err != nil {
		t.Fatalf("write outside fixture: %v", err)
	}
	entrySource := "import data from '../secrets.json';\nexport default function Entry() { return data; }\n"
	if err := os.WriteFile(filepath.Join(deckDirectory, "Entry.jsx"), []byte(entrySource), 0o644); err != nil {
		t.Fatalf("write entry fixture: %v", err)
	}

	bundle, errs := Build("Entry.jsx", Options{DeckDirectory: deckDirectory})
	if len(errs) == 0 {
		t.Fatal("expected an error for a data file imported from outside the deck folder")
	}
	if bundle != nil {
		t.Error("bundle bytes returned despite the containment violation")
	}
	if !strings.Contains(errs[0].Message, "imports a data file from outside the deck folder") {
		t.Errorf("message = %q, want it to name the outside-deck data file violation", errs[0].Message)
	}
	if !strings.HasSuffix(errs[0].File, "Entry.jsx") {
		t.Errorf("File = %q, want it to name the importing component file", errs[0].File)
	}
}

func TestBuildRejectsSymlinkToDataFileOutsideDeckDirectory(t *testing.T) {
	deckDirectory := t.TempDir()
	outsideDirectory := t.TempDir()

	outsideFile := filepath.Join(outsideDirectory, "secret.txt")
	if err := os.WriteFile(outsideFile, []byte("shh"), 0o644); err != nil {
		t.Fatalf("write outside fixture: %v", err)
	}

	linkPath := filepath.Join(deckDirectory, "key.txt")
	if err := os.Symlink(outsideFile, linkPath); err != nil {
		t.Skipf("symlinks not supported on this platform: %v", err)
	}

	entrySource := "import key from './key.txt';\nexport default function Entry() { return key; }\n"
	if err := os.WriteFile(filepath.Join(deckDirectory, "Entry.jsx"), []byte(entrySource), 0o644); err != nil {
		t.Fatalf("write entry fixture: %v", err)
	}

	bundle, errs := Build("Entry.jsx", Options{DeckDirectory: deckDirectory})
	if len(errs) == 0 {
		t.Fatal("expected an error for a symlink pointing to a file outside the deck folder")
	}
	if bundle != nil {
		t.Error("bundle bytes returned despite the containment violation")
	}
	if !strings.Contains(errs[0].Message, "imports a data file from outside the deck folder") {
		t.Errorf("message = %q, want it to name the outside-deck data file violation", errs[0].Message)
	}
}

func TestBuildIgnoresAncestorTsconfigJSXSetting(t *testing.T) {
	deckDirectory := "testdata/tsconfig-jsxdev"
	bundle, errs := Build("Component.jsx", Options{DeckDirectory: deckDirectory})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	javascript := string(bundle.JavaScript)
	if strings.Contains(javascript, "jsx-dev-runtime") {
		t.Errorf("JavaScript references jsx-dev-runtime despite the fixed tsconfig: %s", javascript)
	}
	if strings.Contains(javascript, "createElement(") {
		t.Errorf("JavaScript uses the classic createElement runtime despite the fixed tsconfig: %s", javascript)
	}
}
