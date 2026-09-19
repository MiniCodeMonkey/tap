package components

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestBuildErrorError_NamesSlidesWithoutChangingThePrefix checks that the
// leading "<file>:<line>:<column>: <message>" part - what tools parse -
// stays exactly as before, with the slide list as a trailing suffix, for
// both one slide and several.
func TestBuildErrorError_NamesSlidesWithoutChangingThePrefix(t *testing.T) {
	noSlides := BuildError{File: "slides/Broken.jsx", Line: 7, Column: 1, Message: `Unexpected "return"`}
	want := `slides/Broken.jsx:7:1: Unexpected "return"`
	if got := noSlides.Error(); got != want {
		t.Errorf("Error() with no slides = %q, want %q", got, want)
	}

	oneSlide := noSlides
	oneSlide.SlideNumbers = []int{2}
	want = `slides/Broken.jsx:7:1: Unexpected "return" (used on slide 2)`
	if got := oneSlide.Error(); got != want {
		t.Errorf("Error() with one slide = %q, want %q", got, want)
	}

	severalSlides := noSlides
	severalSlides.SlideNumbers = []int{2, 5}
	want = `slides/Broken.jsx:7:1: Unexpected "return" (used on slides 2, 5)`
	if got := severalSlides.Error(); got != want {
		t.Errorf("Error() with several slides = %q, want %q", got, want)
	}
}

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
	want := "component files must live inside the deck folder: ../outside/Outside.jsx (imports from outside are allowed, entry files are not)"
	if errs[0].Message != want {
		t.Errorf("message = %q, want %q", errs[0].Message, want)
	}
	if errs[0].Error() != want {
		t.Errorf("Error() = %q, want %q (no File prefix for a path-containment error)", errs[0].Error(), want)
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
	if !strings.Contains(errs[0].Message, "deck folder") {
		t.Errorf("message = %q, want it to explain components must live inside the deck folder", errs[0].Message)
	}
}

// writeAssetFixture writes a JSX entry file importing one image, plus the
// image itself sized to bytes, into deckDirectory. Real PNG bytes are not
// needed: the asset-size plugin (and esbuild's own dataurl/file loaders it
// delegates to) work on the raw bytes without decoding them.
func writeAssetFixture(t *testing.T, deckDirectory string, imageBytes int) {
	t.Helper()
	image := make([]byte, imageBytes)
	for i := range image {
		image[i] = byte(i)
	}
	if err := os.WriteFile(filepath.Join(deckDirectory, "photo.png"), image, 0o644); err != nil {
		t.Fatalf("write image fixture: %v", err)
	}
	source := `import photo from "./photo.png";
export default function WithImage() {
  return photo;
}
`
	if err := os.WriteFile(filepath.Join(deckDirectory, "WithImage.jsx"), []byte(source), 0o644); err != nil {
		t.Fatalf("write entry fixture: %v", err)
	}
}

// TestBuildInlinesAssetsUnderTheSizeThreshold checks that an imported image
// under assetInlineThreshold is inlined as a data URL, in the bundle's
// JavaScript, exactly as it always was - and lists no separate Asset.
func TestBuildInlinesAssetsUnderTheSizeThreshold(t *testing.T) {
	deckDirectory := t.TempDir()
	writeAssetFixture(t, deckDirectory, assetInlineThreshold-1)

	bundle, errs := Build("WithImage.jsx", Options{DeckDirectory: deckDirectory, AssetPublicPath: "/components/"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(bundle.Assets) != 0 {
		t.Errorf("expected no emitted assets for a small image, got %+v", bundle.Assets)
	}
	if !strings.Contains(string(bundle.JavaScript), "data:image/png;base64,") {
		t.Error("expected the small image to be inlined as a data URL in the bundle's JavaScript")
	}
}

// TestBuildEmitsAssetsAtOrAboveTheSizeThreshold checks that an imported
// image at or above assetInlineThreshold is emitted as its own file
// (Bundle.Assets) instead, referenced from the bundle's JavaScript by a URL
// under Options.AssetPublicPath rather than inlined.
func TestBuildEmitsAssetsAtOrAboveTheSizeThreshold(t *testing.T) {
	deckDirectory := t.TempDir()
	writeAssetFixture(t, deckDirectory, assetInlineThreshold)

	bundle, errs := Build("WithImage.jsx", Options{DeckDirectory: deckDirectory, AssetPublicPath: "/components/"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(bundle.Assets) != 1 {
		t.Fatalf("expected 1 emitted asset for a large image, got %d: %+v", len(bundle.Assets), bundle.Assets)
	}
	asset := bundle.Assets[0]
	if asset.ContentType != "image/png" {
		t.Errorf("ContentType = %q, want %q", asset.ContentType, "image/png")
	}
	if len(asset.Content) != assetInlineThreshold {
		t.Errorf("Content length = %d, want %d", len(asset.Content), assetInlineThreshold)
	}
	if strings.Contains(string(bundle.JavaScript), "data:image/png;base64,") {
		t.Error("did not expect the large image to be inlined as a data URL")
	}
	wantURL := "/components/" + asset.Name
	if !strings.Contains(string(bundle.JavaScript), wantURL) {
		t.Errorf("expected the bundle's JavaScript to reference %q", wantURL)
	}
}

// TestBuildEmittedAssetNameIsSafeForAHostileFileName reproduces the bundle
// syntax break a hostile asset file name causes: esbuild bakes an emitted
// file's name into the bundle's JavaScript as a plain string literal, so a
// name built from the original file's base name ("we ird'na"me<x>.png")
// can break the string it lands in. The emitted name must only ever
// contain safe characters.
func TestBuildEmittedAssetNameIsSafeForAHostileFileName(t *testing.T) {
	deckDirectory := t.TempDir()
	image := make([]byte, assetInlineThreshold)
	for i := range image {
		image[i] = byte(i)
	}
	hostileName := `we ird'na"me<x>.png`
	if err := os.WriteFile(filepath.Join(deckDirectory, hostileName), image, 0o644); err != nil {
		t.Skipf("filesystem does not support this file name: %v", err)
	}
	// Escaped as a JS string literal: the import specifier still has to
	// parse, whatever character the file name itself carries.
	escapedName := strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(hostileName)
	source := "import photo from \"./" + escapedName + "\";\n" +
		"export default function WithImage() {\n  return photo;\n}\n"
	if err := os.WriteFile(filepath.Join(deckDirectory, "WithImage.jsx"), []byte(source), 0o644); err != nil {
		t.Fatalf("write entry fixture: %v", err)
	}

	bundle, errs := Build("WithImage.jsx", Options{DeckDirectory: deckDirectory, AssetPublicPath: "/components/"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(bundle.Assets) != 1 {
		t.Fatalf("expected 1 emitted asset, got %d: %+v", len(bundle.Assets), bundle.Assets)
	}

	asset := bundle.Assets[0]
	if unsafeNameCharacters.MatchString(strings.TrimSuffix(asset.Name, filepath.Ext(asset.Name))) {
		t.Errorf("emitted asset name %q contains unsafe characters", asset.Name)
	}
	if strings.ContainsAny(asset.Name, `'"<> `) {
		t.Errorf("emitted asset name %q must not carry the original file name's unsafe characters", asset.Name)
	}
	if !strings.Contains(string(bundle.JavaScript), "/components/"+asset.Name) {
		t.Errorf("expected the bundle's JavaScript to reference %q", asset.Name)
	}
}

// writeCSSAssetFixture writes a JSX entry file that imports a stylesheet
// with a url() token pointing at a large image, plus the image itself
// sized to bytes, into deckDirectory.
func writeCSSAssetFixture(t *testing.T, deckDirectory string, imageBytes int) {
	t.Helper()
	image := make([]byte, imageBytes)
	for i := range image {
		image[i] = byte(i)
	}
	if err := os.WriteFile(filepath.Join(deckDirectory, "big.png"), image, 0o644); err != nil {
		t.Fatalf("write image fixture: %v", err)
	}
	css := `.banner { background-image: url("./big.png"); }` + "\n"
	if err := os.WriteFile(filepath.Join(deckDirectory, "Banner.css"), []byte(css), 0o644); err != nil {
		t.Fatalf("write css fixture: %v", err)
	}
	source := `import "./Banner.css";
export default function Banner() {
  return <div className="banner" />;
}
`
	if err := os.WriteFile(filepath.Join(deckDirectory, "Banner.jsx"), []byte(source), 0o644); err != nil {
		t.Fatalf("write entry fixture: %v", err)
	}
}

// cssURLFuncPattern finds a url(...) token in emitted CSS.
var cssURLFuncPattern = regexp.MustCompile(`url\(([^)]+)\)`)

// TestBuildCSSAssetURLResolvesUnderAStaticSubPath reproduces the static
// build bug where a CSS url() token referencing a large (non-inlined)
// asset used the bundle's public path exactly like a JavaScript import,
// producing a URL that a browser resolves relative to the CSS file
// itself - which already lives inside the public path directory - doubling
// the prefix. It builds a component whose CSS references an image at the
// inline threshold, serves the emitted files (as they would sit in
// dist/components/ under a sub path), and resolves the CSS url() token
// against the CSS file's own URL exactly as a browser would.
func TestBuildCSSAssetURLResolvesUnderAStaticSubPath(t *testing.T) {
	deckDirectory := t.TempDir()
	writeCSSAssetFixture(t, deckDirectory, assetInlineThreshold)

	bundle, errs := Build("Banner.jsx", Options{DeckDirectory: deckDirectory, AssetPublicPath: "components/"})
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(bundle.Assets) != 1 {
		t.Fatalf("expected 1 emitted asset, got %d: %+v", len(bundle.Assets), bundle.Assets)
	}
	if len(bundle.CSS) == 0 {
		t.Fatal("expected the bundle to have CSS")
	}

	match := cssURLFuncPattern.FindStringSubmatch(string(bundle.CSS))
	if match == nil {
		t.Fatalf("expected a url() token in the emitted CSS, got %q", bundle.CSS)
	}
	token := strings.Trim(match[1], `"'`)

	mux := http.NewServeMux()
	mux.HandleFunc("/sub2/components/"+bundle.Name+"-"+bundle.Hash+".css", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/css")
		w.Write(bundle.CSS)
	})
	for _, asset := range bundle.Assets {
		content := asset.Content
		mux.HandleFunc("/sub2/components/"+asset.Name, func(w http.ResponseWriter, r *http.Request) {
			w.Write(content)
		})
	}
	server := httptest.NewServer(mux)
	defer server.Close()

	cssURL, err := url.Parse(server.URL + "/sub2/components/" + bundle.Name + "-" + bundle.Hash + ".css")
	if err != nil {
		t.Fatalf("parse CSS URL: %v", err)
	}
	assetRef, err := url.Parse(token)
	if err != nil {
		t.Fatalf("parse url() token %q: %v", token, err)
	}
	resolved := cssURL.ResolveReference(assetRef)

	response, err := http.Get(resolved.String())
	if err != nil {
		t.Fatalf("GET %s: %v", resolved, err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Errorf("GET %s status = %d, want 200 (url() token was %q)", resolved, response.StatusCode, token)
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
