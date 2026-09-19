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
// same path it already has. A build error's SlideNumbers is set to every
// one-based slide number that uses the path, so one broken bundle used by
// several slides can be reported once, naming all of them.
func Resolve(presentation *parser.Presentation, deckDirectory string, options Options) map[string]Result {
	options.DeckDirectory = deckDirectory

	pathSlideNumbers := collectComponentPathSlideNumbers(presentation)
	results := make(map[string]Result, len(pathSlideNumbers))
	for path, slideNumbers := range pathSlideNumbers {
		bundle, buildErrors := Build(path, options)
		for i := range buildErrors {
			buildErrors[i].SlideNumbers = slideNumbers
		}
		results[path] = Result{Bundle: bundle, Errors: buildErrors}
	}
	return results
}

// collectComponentPathSlideNumbers gathers every distinct component path
// referenced by a presentation's slides, mapped to the one-based slide
// numbers that reference it, in ascending order.
func collectComponentPathSlideNumbers(presentation *parser.Presentation) map[string][]int {
	paths := make(map[string][]int)
	addUse := func(path string, slideNumber int) {
		numbers := paths[path]
		if len(numbers) > 0 && numbers[len(numbers)-1] == slideNumber {
			// The same slide can reach the same path twice (a whole-slide
			// layout that also happens to match an inline fence's source,
			// or duplicate fences), and slides are visited in order, so a
			// repeat always lands right after the last entry.
			return
		}
		paths[path] = append(numbers, slideNumber)
	}

	for _, slide := range presentation.Slides {
		slideNumber := slide.Index + 1
		if IsComponentPath(slide.Directives.Layout) {
			addUse(slide.Directives.Layout, slideNumber)
		}
		for _, component := range slide.Components {
			addUse(component.Source, slideNumber)
		}
	}
	return paths
}
