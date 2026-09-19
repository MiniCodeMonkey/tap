// Package cli provides the command-line interface for Tap.
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/MiniCodeMonkey/tap/internal/components"
	"github.com/MiniCodeMonkey/tap/internal/parser"
	"github.com/MiniCodeMonkey/tap/internal/server"
)

// buildComponents resolves and bundles every component a presentation's
// slides use (see internal/components.Resolve), and returns the resolved
// map for the transformer plus a flattened, deterministically ordered list
// of build errors for terminal output.
func buildComponents(parsed *parser.Presentation, deckDirectory string, minify bool, sourceMaps bool) (map[string]components.Result, []components.BuildError) {
	resolved := components.Resolve(parsed, deckDirectory, components.Options{
		DeckDirectory: deckDirectory,
		Minify:        minify,
		SourceMaps:    sourceMaps,
	})

	paths := make([]string, 0, len(resolved))
	for path := range resolved {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	var buildErrors []components.BuildError
	for _, path := range paths {
		buildErrors = append(buildErrors, resolved[path].Errors...)
	}
	return resolved, buildErrors
}

// componentBundleFiles builds the dev server's file map (bundle file name
// to content and content type) from a resolved component map, for
// Server.SetComponentBundles.
func componentBundleFiles(resolved map[string]components.Result) map[string]server.ComponentBundleFile {
	files := make(map[string]server.ComponentBundleFile)
	for _, result := range resolved {
		bundle := result.Bundle
		if bundle == nil {
			continue
		}
		base := bundle.Name + "-" + bundle.Hash
		files[base+".js"] = server.ComponentBundleFile{
			ContentType: "application/javascript; charset=utf-8",
			Content:     bundle.JavaScript,
		}
		if len(bundle.CSS) > 0 {
			files[base+".css"] = server.ComponentBundleFile{
				ContentType: "text/css; charset=utf-8",
				Content:     bundle.CSS,
			}
		}
		if len(bundle.SourceMap) > 0 {
			files[base+".js.map"] = server.ComponentBundleFile{
				ContentType: "application/json; charset=utf-8",
				Content:     bundle.SourceMap,
			}
		}
	}
	return files
}

// externalInputDirs collects the directories of every file a resolved
// component pulled in (Bundle.Inputs, from esbuild's metafile) that lies
// outside the deck directory tree, deduplicated. The recursive watch of the
// deck directory (see server.Watcher.Start) already covers everything
// inside it; this is only for an import that reaches outside the deck
// folder, such as "../shared/Thing.jsx", which that walk never visits. The
// result is meant for server.Watcher.AddExtraDirs, which itself skips
// anything under node_modules.
func externalInputDirs(resolved map[string]components.Result, deckDirectory string) []string {
	absDeckDirectory, err := filepath.Abs(deckDirectory)
	if err != nil {
		absDeckDirectory = deckDirectory
	}

	seen := make(map[string]bool)
	var directories []string
	for _, result := range resolved {
		if result.Bundle == nil {
			continue
		}
		for _, input := range result.Bundle.Inputs {
			dir := filepath.Dir(input)
			if seen[dir] {
				continue
			}
			if relativePath, relativePathErr := filepath.Rel(absDeckDirectory, dir); relativePathErr == nil && relativePath != ".." && !strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
				// Inside the deck tree; the recursive watch already covers it.
				continue
			}
			seen[dir] = true
			directories = append(directories, dir)
		}
	}
	return directories
}

// printComponentErrorsToStderr prints one line per component build error in
// the spec's exact terminal format ("error: <file>:<line>:<column>:
// <message>"). Only safe to call when nothing else owns the terminal,
// mirroring printLayoutWarningsToStderr.
func printComponentErrorsToStderr(buildErrors []components.BuildError) {
	for _, buildError := range buildErrors {
		fmt.Fprintf(os.Stderr, "error: %s\n", buildError.Error())
	}
}

// componentWarnings flattens every bundle's esbuild warnings out of a
// resolved component map, in the same deterministic path order
// buildComponents uses for errors, for terminal output.
func componentWarnings(resolved map[string]components.Result) []components.BuildError {
	paths := make([]string, 0, len(resolved))
	for path := range resolved {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	var warnings []components.BuildError
	for _, path := range paths {
		if bundle := resolved[path].Bundle; bundle != nil {
			warnings = append(warnings, bundle.Warnings...)
		}
	}
	return warnings
}

// printComponentWarningsToStderr prints one line per component build
// warning, in the same "warning: <file>:<line>:<column>: <message>" format
// tap uses for other warnings. A warning never fails the build; only safe
// to call when nothing else owns the terminal, mirroring
// printComponentErrorsToStderr.
func printComponentWarningsToStderr(warnings []components.BuildError) {
	for _, warning := range warnings {
		Warning("warning: %s\n", warning.Error())
	}
}

// componentErrorsError joins component build errors into a single error for
// display through the TUI model's SetError, or nil when there are none.
func componentErrorsError(buildErrors []components.BuildError) error {
	if len(buildErrors) == 0 {
		return nil
	}
	lines := make([]string, len(buildErrors))
	for i, buildError := range buildErrors {
		lines[i] = "error: " + buildError.Error()
	}
	return errors.New(strings.Join(lines, "\n"))
}
