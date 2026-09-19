package components

import (
	"encoding/json"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/parser"
)

func TestIsComponentPath(t *testing.T) {
	cases := map[string]bool{
		"./slides/RollingDeploy.jsx": true,
		"../shared/Chart.tsx":        true,
		"./lib/util.js":              true,
		"./lib/util.ts":              true,
		"title":                      false,
		"big-stat":                   false,
		"./notes.md":                 false,
		"slides/RollingDeploy.jsx":   false, // must start with ./ or ../
	}
	for value, want := range cases {
		if got := IsComponentPath(value); got != want {
			t.Errorf("IsComponentPath(%q) = %v, want %v", value, got, want)
		}
	}
}

func TestResolve_BuildsEachDistinctPathOnce(t *testing.T) {
	presentation := &parser.Presentation{
		Slides: []parser.Slide{
			{Directives: parser.SlideDirectives{Layout: "./tiny/Tiny.jsx"}},
			{
				Components: []parser.Component{
					{Index: 0, Source: "./tiny/Tiny.jsx", Props: json.RawMessage("{}")},
					{Index: 1, Source: "./tsx/Tsx.tsx", Props: json.RawMessage("{}")},
				},
			},
		},
	}

	results := Resolve(presentation, "testdata", Options{})

	if len(results) != 2 {
		t.Fatalf("expected 2 distinct paths resolved, got %d", len(results))
	}
	tiny, ok := results["./tiny/Tiny.jsx"]
	if !ok || tiny.Bundle == nil {
		t.Fatalf("expected ./tiny/Tiny.jsx to resolve to a bundle, got %+v", tiny)
	}
	tsx, ok := results["./tsx/Tsx.tsx"]
	if !ok || tsx.Bundle == nil {
		t.Fatalf("expected ./tsx/Tsx.tsx to resolve to a bundle, got %+v", tsx)
	}
}

func TestResolve_BuildErrorsSurfacePerPath(t *testing.T) {
	presentation := &parser.Presentation{
		Slides: []parser.Slide{
			{Directives: parser.SlideDirectives{Layout: "./syntax-error/Broken.jsx"}},
		},
	}

	results := Resolve(presentation, "testdata", Options{})

	result, ok := results["./syntax-error/Broken.jsx"]
	if !ok {
		t.Fatal("expected ./syntax-error/Broken.jsx to be present in the results")
	}
	if result.Bundle != nil {
		t.Errorf("expected a nil bundle for a broken component, got %+v", result.Bundle)
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected build errors for a broken component")
	}
}
