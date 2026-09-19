package themes

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
)

//go:embed tokens.json
var tokensJSON []byte

// Illustration describes a theme's illustration style: what an image model
// should be told to draw so the result looks like it belongs on this
// theme's slides.
type IllustrationStyle struct {
	Medium     string `json:"medium"`
	Line       string `json:"line"`
	Shapes     string `json:"shapes"`
	Texture    string `json:"texture"`
	PaletteUse string `json:"palette_use"`
	Mood       string `json:"mood"`
	Avoid      string `json:"avoid"`
}

// themeData is one theme's entry in tokens.json: its CSS custom properties
// (by CSS name, e.g. "--bg") and its illustration style.
type themeData struct {
	Tokens       map[string]string `json:"tokens"`
	Illustration IllustrationStyle `json:"illustration"`
}

// RequiredTokens lists the CSS custom property names every theme must
// declare on its root token block. frontend/scripts/extract-theme-tokens.mjs
// extracts every custom property a theme declares, not just these, but
// these are the ones the rest of tap (useTheme(), tap theme show) depends
// on existing for every theme.
var RequiredTokens = []string{
	"--bg", "--fg", "--muted", "--accent", "--accent-text", "--accent-2", "--surface",
	"--status-ok", "--status-warn", "--status-error",
	"--font-display", "--font-body", "--font-mono", "--ease", "--dur",
	"--space-unit", "--radius", "--stroke-width",
}

var (
	tokenData     map[string]themeData
	tokenLoadOnce sync.Once
)

// loadTokens parses the embedded tokens.json once and caches the result.
func loadTokens() {
	tokenLoadOnce.Do(func() {
		if err := json.Unmarshal(tokensJSON, &tokenData); err != nil {
			panic(fmt.Sprintf("themes: failed to parse tokens.json: %v", err))
		}
	})
}

// Tokens returns every CSS custom property declared on slug's root token
// block, keyed by CSS name (e.g. "--bg"), and whether slug has a tokens.json
// entry at all.
func Tokens(slug string) (map[string]string, bool) {
	loadTokens()
	data, ok := tokenData[slug]
	if !ok {
		return nil, false
	}
	result := make(map[string]string, len(data.Tokens))
	for k, v := range data.Tokens {
		result[k] = v
	}
	return result, true
}

// Illustration returns slug's illustration style, and whether slug has a
// tokens.json entry at all.
func Illustration(slug string) (IllustrationStyle, bool) {
	loadTokens()
	data, ok := tokenData[slug]
	if !ok {
		return illustrationZero, false
	}
	return data.Illustration, true
}

var illustrationZero IllustrationStyle

// TokenSlugs returns every slug with a tokens.json entry, sorted.
func TokenSlugs() []string {
	loadTokens()
	slugs := make([]string, 0, len(tokenData))
	for slug := range tokenData {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	return slugs
}
