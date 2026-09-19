// Package components bundles a deck-supplied React component file with
// esbuild's Go API. It never runs JavaScript: static facts like the steps
// count come from a regular expression over the source text, not execution.
package components

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/evanw/esbuild/pkg/api"
)

// Bundle is the result of building one component entry file.
type Bundle struct {
	Source          string
	Name            string
	Hash            string
	JavaScript      []byte
	CSS             []byte
	SourceMap       []byte
	Steps           int
	HasStepsExport  bool
	PreviewDisabled bool
	// Inputs lists the absolute paths of every file the bundle pulled in
	// (the entry file plus every local file it imports), read from
	// esbuild's metafile so a caller can watch them for changes.
	Inputs []string
	// Warnings holds esbuild warnings that did not stop the build, in the
	// same format as BuildError. A caller decides whether and how to
	// print them; this package does not print anything itself.
	Warnings []BuildError
}

// BuildError describes one problem esbuild reported, in the exact format
// the spec requires for terminal and error-card output.
type BuildError struct {
	File    string
	Line    int
	Column  int
	Message string
}

func (e BuildError) Error() string {
	// A build error with no esbuild source location (Line and Column both
	// zero, such as the deck-folder containment check below) omits the
	// position instead of printing the misleading "0:0".
	if e.Line == 0 && e.Column == 0 {
		return fmt.Sprintf("%s: %s", e.File, e.Message)
	}
	return fmt.Sprintf("%s:%d:%d: %s", e.File, e.Line, e.Column, e.Message)
}

// Options configures a build.
type Options struct {
	// DeckDirectory is the deck's folder. A source path that resolves
	// outside it is rejected.
	DeckDirectory string
	Minify        bool
	SourceMaps    bool
}

// stepsExportPattern and previewExportPattern match against esbuild's own
// printed form of the source (see transformForExportScan), not the raw
// file text: esbuild's parser and printer strip comments, normalize
// whitespace, drop "as const", and always terminate the statement with a
// semicolon, so the captured value is whatever text sits between "=" and
// ";". A lookalike assignment inside a real string or template literal
// still matches this way; that is an accepted limitation.
var (
	stepsExportPattern   = regexp.MustCompile(`export\s+const\s+steps\s*=\s*([^;]+);`)
	previewExportPattern = regexp.MustCompile(`export\s+const\s+preview\s*=\s*([^;]+);`)
	unsafeNameCharacters = regexp.MustCompile(`[^a-zA-Z0-9_-]+`)
	unresolvedImportText = regexp.MustCompile(`Could not resolve "([^"]+)"`)
)

// fixedTsconfigRaw pins the JSX transform to the automatic React runtime,
// overriding any "jsx", "jsxImportSource", or "jsxFactory" setting from a
// tsconfig.json esbuild would otherwise discover in or above the deck
// folder. Without it, a deck-adjacent tsconfig.json can switch the build to
// the dev JSX runtime, the classic runtime, or another library's JSX
// runtime entirely, none of which match the production React the host
// embeds. A deck's own "paths" aliases are not honored this way; the spec
// does not promise those.
const fixedTsconfigRaw = `{"compilerOptions":{"jsx":"react-jsx","jsxImportSource":"react"}}`

// Build bundles the component at sourcePath. sourcePath may be absolute or
// relative to options.DeckDirectory. It always returns a non-nil error
// slice on failure (bundle is nil); on success the error slice is nil.
func Build(sourcePath string, options Options) (*Bundle, []BuildError) {
	absSource, resolveErr := resolveSourcePath(sourcePath, options.DeckDirectory)
	if resolveErr != nil {
		return nil, []BuildError{{
			File:    sourcePath,
			Message: resolveErr.Error(),
		}}
	}

	sourceText, readErr := readFileText(absSource)
	if readErr != nil {
		return nil, []BuildError{{
			File:    cleanErrorFile(absSource, options.DeckDirectory),
			Message: readErr.Error(),
		}}
	}

	sourcemap := api.SourceMapNone
	if options.SourceMaps {
		sourcemap = api.SourceMapLinked
	}

	result := api.Build(api.BuildOptions{
		EntryPoints:       []string{absSource},
		Bundle:            true,
		Write:             false,
		Outdir:            "out",
		Format:            api.FormatESModule,
		Platform:          api.PlatformBrowser,
		JSX:               api.JSXAutomatic,
		TsconfigRaw:       fixedTsconfigRaw,
		Target:            api.ES2022,
		MinifyWhitespace:  options.Minify,
		MinifyIdentifiers: options.Minify,
		MinifySyntax:      options.Minify,
		Sourcemap:         sourcemap,
		Metafile:          true,
		Loader: map[string]api.Loader{
			// LLM-authored decks often put JSX in a plain .js file; treat
			// it as JSX rather than the default JS-only loader. .ts is
			// left alone since JSX syntax is not valid TypeScript there.
			".js": api.LoaderJSX,
			// Assets are inlined as data URLs regardless of size. The
			// spec's rule to copy assets over 100 KB instead is out of
			// scope for this package.
			".png":   api.LoaderDataURL,
			".jpg":   api.LoaderDataURL,
			".jpeg":  api.LoaderDataURL,
			".gif":   api.LoaderDataURL,
			".svg":   api.LoaderDataURL,
			".webp":  api.LoaderDataURL,
			".woff2": api.LoaderDataURL,
		},
		Plugins: []api.Plugin{hostShimPlugin()},
	})

	if len(result.Errors) > 0 {
		return nil, convertMessages(result.Errors, absSource, options.DeckDirectory)
	}

	if violation := checkInputContainment(result.Metafile, absSource, options.DeckDirectory); violation != nil {
		return nil, []BuildError{*violation}
	}

	bundle := &Bundle{
		Source: absSource,
		Name:   safeName(absSource),
	}

	if len(result.Warnings) > 0 {
		bundle.Warnings = convertMessages(result.Warnings, absSource, options.DeckDirectory)
	}

	for _, output := range result.OutputFiles {
		switch {
		case strings.HasSuffix(output.Path, ".css"):
			bundle.CSS = output.Contents
		case strings.HasSuffix(output.Path, ".js.map"):
			bundle.SourceMap = output.Contents
		case strings.HasSuffix(output.Path, ".js"):
			bundle.JavaScript = output.Contents
		}
	}

	bundle.Hash = contentHash(bundle.JavaScript, bundle.CSS)
	bundle.Inputs = metafileInputs(result.Metafile)

	scanCode := transformForExportScan(absSource, sourceText)

	if match := stepsExportPattern.FindStringSubmatch(scanCode); match != nil {
		// A negative value is not a step count; treat it like a value
		// that failed to parse.
		if steps, err := strconv.Atoi(strings.TrimSpace(match[1])); err == nil && steps >= 0 {
			bundle.HasStepsExport = true
			bundle.Steps = steps
		}
	}

	if match := previewExportPattern.FindStringSubmatch(scanCode); match != nil && strings.TrimSpace(match[1]) == "false" {
		bundle.PreviewDisabled = true
	}

	return bundle, nil
}

// transformForExportScan runs the entry file's source through esbuild's
// own parser and printer, so the steps and preview patterns match against
// a normalized, comment-free form instead of raw source text (which a
// hand-written scanner can misread, for example an unmatched quote in JSX
// text, a regular expression literal containing "//", or URL text inside
// JSX). If the source fails to parse, this returns the original source
// unchanged: the bundle step below reports the real parse error, so steps
// and preview are simply treated as absent here rather than reported a
// second time.
func transformForExportScan(absSource, sourceText string) string {
	result := api.Transform(sourceText, api.TransformOptions{
		Loader:      transformLoaderForFile(absSource),
		JSX:         api.JSXAutomatic,
		TsconfigRaw: fixedTsconfigRaw,
	})
	if len(result.Errors) > 0 {
		return sourceText
	}
	return string(result.Code)
}

// transformLoaderForFile picks esbuild's loader from the entry file's
// extension, matching the set of component file extensions the spec
// allows. .js is treated as JSX, like the main build's Loader map, since
// an LLM-authored deck often puts JSX in a plain .js file; .ts stays
// TypeScript-only, since JSX syntax is not valid there.
func transformLoaderForFile(path string) api.Loader {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".tsx":
		return api.LoaderTSX
	case ".ts":
		return api.LoaderTS
	default:
		return api.LoaderJSX
	}
}

// resolveSourcePath resolves sourcePath against deckDirectory and rejects
// anything that lands outside it, symlinks included.
func resolveSourcePath(sourcePath, deckDirectory string) (string, error) {
	path := sourcePath
	if !filepath.IsAbs(path) {
		path = filepath.Join(deckDirectory, path)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve component path: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(absPath); err == nil {
		absPath = resolved
	}

	absDeckDirectory, err := filepath.Abs(deckDirectory)
	if err != nil {
		return "", fmt.Errorf("resolve deck directory: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(absDeckDirectory); err == nil {
		absDeckDirectory = resolved
	}

	relativePath, err := filepath.Rel(absDeckDirectory, absPath)
	if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("component files must live inside the deck's folder")
	}

	return absPath, nil
}

// safeName derives a URL-safe component name from the entry file's base
// name, stripping its extension.
func safeName(absSource string) string {
	base := filepath.Base(absSource)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	return unsafeNameCharacters.ReplaceAllString(base, "-")
}

// contentHash returns the first 12 hex characters of the SHA-256 of the
// JavaScript bytes followed by the CSS bytes.
func contentHash(javascript, css []byte) string {
	hash := sha256.New()
	hash.Write(javascript)
	hash.Write(css)
	sum := hex.EncodeToString(hash.Sum(nil))
	return sum[:12]
}

// allowedOutsideExtensions lists the file extensions a bundle input may
// have when it lies outside the deck folder and outside any node_modules
// folder: code and stylesheets, never data or asset files.
var allowedOutsideExtensions = map[string]bool{
	".js":  true,
	".jsx": true,
	".mjs": true,
	".cjs": true,
	".ts":  true,
	".tsx": true,
	".css": true,
}

// metafileInputEntry is the subset of esbuild's per-input metafile shape
// this package reads: the list of paths that input itself imports, used to
// find which file pulled in an offending path.
type metafileInputEntry struct {
	Imports []struct {
		Path string `json:"path"`
	} `json:"imports"`
}

// metafileDocument is the subset of esbuild's metafile JSON this package
// decodes.
type metafileDocument struct {
	Inputs map[string]metafileInputEntry `json:"inputs"`
}

// checkInputContainment resolves symlinks on every path esbuild's metafile
// lists as a bundle input and rejects the build when one lies outside the
// deck folder unless it is under a node_modules folder or is code or a
// stylesheet. It is the only defense against a component pulling a data or
// asset file (JSON, text, an image, a font) from outside the deck folder
// into the bundle, whether by a relative import or by an in-deck symlink
// that resolves outside; resolveSourcePath above only ever checks the
// entry file itself.
func checkInputContainment(metafileJSON, absSource, deckDirectory string) *BuildError {
	var metafile metafileDocument
	if err := json.Unmarshal([]byte(metafileJSON), &metafile); err != nil {
		return nil
	}

	absDeckDirectory, err := filepath.Abs(deckDirectory)
	if err != nil {
		return nil
	}
	if resolved, err := filepath.EvalSymlinks(absDeckDirectory); err == nil {
		absDeckDirectory = resolved
	}

	importedBy := make(map[string]string, len(metafile.Inputs))
	for path, entry := range metafile.Inputs {
		for _, dependencyImport := range entry.Imports {
			if _, exists := importedBy[dependencyImport.Path]; !exists {
				importedBy[dependencyImport.Path] = path
			}
		}
	}

	for path := range metafile.Inputs {
		if strings.HasPrefix(path, hostNamespace+":") {
			continue
		}

		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		resolvedPath := absPath
		if resolved, err := filepath.EvalSymlinks(absPath); err == nil {
			resolvedPath = resolved
		}

		relativePath, err := filepath.Rel(absDeckDirectory, resolvedPath)
		insideDeck := err == nil && relativePath != ".." && !strings.HasPrefix(relativePath, ".."+string(filepath.Separator))
		if insideDeck {
			continue
		}
		if isUnderNodeModules(resolvedPath) {
			continue
		}
		if allowedOutsideExtensions[strings.ToLower(filepath.Ext(resolvedPath))] {
			continue
		}

		importer, ok := importedBy[path]
		if !ok {
			importer = absSource
		}

		return &BuildError{
			File:    cleanErrorFile(importer, deckDirectory),
			Message: fmt.Sprintf("imports a data file from outside the deck folder: %s (copy it into the deck folder)", cleanErrorFile(resolvedPath, deckDirectory)),
		}
	}

	return nil
}

// isUnderNodeModules reports whether any path component of an absolute path
// is literally "node_modules".
func isUnderNodeModules(absPath string) bool {
	for _, part := range strings.Split(filepath.ToSlash(absPath), "/") {
		if part == "node_modules" {
			return true
		}
	}
	return false
}

// metafileInputs extracts the absolute paths esbuild pulled into the
// bundle from its metafile JSON, so a caller can watch them for changes.
func metafileInputs(metafileJSON string) []string {
	paths := parseMetafileInputPaths(metafileJSON)
	inputs := make([]string, 0, len(paths))
	for _, path := range paths {
		if strings.HasPrefix(path, hostNamespace+":") {
			continue
		}
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		inputs = append(inputs, absPath)
	}
	return inputs
}

// readFileText reads a source file as text for the regular expression
// scan; the bundler never executes it.
func readFileText(path string) (string, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read component file: %w", err)
	}
	return string(contents), nil
}

// parseMetafileInputPaths returns the raw input paths (as esbuild wrote
// them, relative to the working directory or namespaced for a plugin)
// found in the metafile's "inputs" object.
func parseMetafileInputPaths(metafileJSON string) []string {
	var raw struct {
		Inputs map[string]json.RawMessage `json:"inputs"`
	}
	if err := json.Unmarshal([]byte(metafileJSON), &raw); err != nil {
		return nil
	}
	paths := make([]string, 0, len(raw.Inputs))
	for path := range raw.Inputs {
		paths = append(paths, path)
	}
	return paths
}

// convertMessages maps esbuild messages (errors or warnings) to BuildError,
// rewriting an unresolved bare import into the spec's npm install hint and
// cleaning up the file path.
func convertMessages(messages []api.Message, fallbackFile, deckDirectory string) []BuildError {
	errors := make([]BuildError, 0, len(messages))
	for _, message := range messages {
		file := fallbackFile
		line := 0
		column := 0
		if message.Location != nil {
			if message.Location.File != "" {
				file = message.Location.File
			}
			line = message.Location.Line
			column = message.Location.Column
		}

		text := message.Text
		if match := unresolvedImportText.FindStringSubmatch(text); match != nil {
			packageName := match[1]
			if isHostModuleOrSubpath(packageName) {
				text = fmt.Sprintf("tap provides %q directly; it does not need to be installed. Only these entry points exist: %s", packageName, strings.Join(HostModuleNames(), ", "))
			} else {
				text = fmt.Sprintf("package %q not found; run `npm install %s` next to the deck", packageName, packageName)
			}
		}

		errors = append(errors, BuildError{
			File:    cleanErrorFile(file, deckDirectory),
			Line:    line,
			Column:  column,
			Message: text,
		})
	}
	return errors
}

// cleanErrorFile returns file as a path relative to deckDirectory when
// file lives inside it, and as an absolute path otherwise, so a
// BuildError never carries a relative path with several ".." segments.
func cleanErrorFile(file, deckDirectory string) string {
	if file == "" {
		return file
	}

	absFile, err := filepath.Abs(file)
	if err != nil {
		return file
	}

	absDeckDirectory, err := filepath.Abs(deckDirectory)
	if err != nil {
		return absFile
	}

	relativePath, err := filepath.Rel(absDeckDirectory, absFile)
	if err != nil || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return absFile
	}

	return relativePath
}
