package components

import (
	"path/filepath"
	"strings"

	"github.com/MiniCodeMonkey/tap/internal/parser"
)

// componentPathExtensions lists the file extensions that make a layout or
// fence value a component path rather than an ordinary layout name.
var componentPathExtensions = map[string]bool{
	".jsx": true,
	".tsx": true,
	".js":  true,
	".ts":  true,
}

// IsComponentPath reports whether value names a component file: relative to
// the deck file (starting with "./" or "../") and ending in .jsx, .tsx,
// .js, or .ts. Used to tell a whole-slide component layout apart from a
// built-in layout name.
func IsComponentPath(value string) bool {
	if !strings.HasPrefix(value, "./") && !strings.HasPrefix(value, "../") {
		return false
	}
	return componentPathExtensions[filepath.Ext(value)]
}

// Result is what Resolve returns for one distinct component source path:
// either a successfully built Bundle, or the build errors that prevented
// one.
type Result struct {
	Bundle *Bundle
	Errors []BuildError
}

// Resolve builds every distinct component path a presentation's slides use,
// as a whole-slide layout or an inline ```component fence, exactly once. It
// returns a map from that path, as written in the deck, to its build
// result, so the transformer can look up each slide's component(s) by the
// same path it already has.
func Resolve(presentation *parser.Presentation, deckDirectory string, options Options) map[string]Result {
	options.DeckDirectory = deckDirectory

	paths := collectComponentPaths(presentation)
	results := make(map[string]Result, len(paths))
	for path := range paths {
		bundle, buildErrors := Build(path, options)
		results[path] = Result{Bundle: bundle, Errors: buildErrors}
	}
	return results
}

// collectComponentPaths gathers every distinct component path referenced by
// a presentation's slides, as a set.
func collectComponentPaths(presentation *parser.Presentation) map[string]bool {
	paths := make(map[string]bool)
	for _, slide := range presentation.Slides {
		if IsComponentPath(slide.Directives.Layout) {
			paths[slide.Directives.Layout] = true
		}
		for _, component := range slide.Components {
			paths[component.Source] = true
		}
	}
	return paths
}
