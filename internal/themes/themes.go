// Package themes holds the built-in list of theme slugs, names, polarity,
// and pitch, embedded from themes.json. It is the single source of truth
// for config validation, the tap new wizard, the tap dev theme picker, and
// (mirrored via themes.json's frontend copy) the live switcher.
package themes

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sync"
)

//go:embed themes.json
var themesJSON []byte

// Theme describes one built-in theme entry.
type Theme struct {
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	Polarity string `json:"polarity"`
	Pitch    string `json:"pitch"`
}

var (
	all      []Theme
	bySlug   map[string]bool
	loadOnce sync.Once
)

// load parses the embedded themes.json once and caches the result.
func load() {
	loadOnce.Do(func() {
		if err := json.Unmarshal(themesJSON, &all); err != nil {
			panic(fmt.Sprintf("themes: failed to parse themes.json: %v", err))
		}
		bySlug = make(map[string]bool, len(all))
		for _, t := range all {
			bySlug[t.Slug] = true
		}
	})
}

// All returns every built-in theme, in themes.json order (base first).
func All() []Theme {
	load()
	result := make([]Theme, len(all))
	copy(result, all)
	return result
}

// IsValid reports whether slug names a built-in theme.
func IsValid(slug string) bool {
	load()
	return bySlug[slug]
}

// Slugs returns every built-in theme's slug, in themes.json order.
func Slugs() []string {
	load()
	slugs := make([]string, len(all))
	for i, t := range all {
		slugs[i] = t.Slug
	}
	return slugs
}
