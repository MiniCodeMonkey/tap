package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/themes"
)

func TestBuildThemePrompt_UnderWordLimitAndPlainText(t *testing.T) {
	for _, slug := range []string{"terminal", "product", "base"} {
		t.Run(slug, func(t *testing.T) {
			theme, ok := findTheme(slug)
			if !ok {
				t.Fatalf("theme %q not found", slug)
			}
			tokens, ok := themes.Tokens(slug)
			if !ok {
				t.Fatalf("theme %q has no tokens", slug)
			}
			illustration, _ := themes.Illustration(slug)

			prompt := buildThemePrompt(theme, tokens, illustration)

			wordCount := len(strings.Fields(prompt))
			if wordCount >= 200 {
				t.Errorf("prompt for %q has %d words, want under 200", slug, wordCount)
			}
			if !strings.Contains(prompt, tokens["--bg"]) {
				t.Errorf("prompt for %q does not contain its --bg hex value %q", slug, tokens["--bg"])
			}
			if !strings.Contains(prompt, "Palette use:") {
				t.Errorf("prompt for %q is missing the Palette use sentence", slug)
			}
			if strings.Contains(prompt, "—") {
				t.Errorf("prompt for %q contains an em dash", slug)
			}
			for _, marker := range []string{"```", "**", "##", "* ", "- "} {
				if strings.Contains(prompt, marker) {
					t.Errorf("prompt for %q contains markdown marker %q", slug, marker)
				}
			}
			// Code block tokens (e.g. --color-code-bg) mean nothing to an
			// image model, and base's own dark code background would
			// otherwise contradict its plain white brief.
			if slug == "base" && strings.Contains(prompt, "code") {
				t.Errorf("prompt for base mentions \"code\", want code block tokens excluded:\n%s", prompt)
			}
		})
	}

	// Every theme's brief, not just terminal and product's, must stay
	// under the 200-word budget.
	for _, theme := range themes.All() {
		tokens, ok := themes.Tokens(theme.Slug)
		if !ok {
			t.Fatalf("theme %q has no tokens", theme.Slug)
		}
		illustration, _ := themes.Illustration(theme.Slug)

		prompt := buildThemePrompt(theme, tokens, illustration)

		wordCount := len(strings.Fields(prompt))
		if wordCount >= 200 {
			t.Errorf("prompt for %q has %d words, want under 200", theme.Slug, wordCount)
		}
	}
}

func TestBuildThemePrompt_TransitContainsAccentText(t *testing.T) {
	theme, ok := findTheme("transit")
	if !ok {
		t.Fatal("theme \"transit\" not found")
	}
	tokens, ok := themes.Tokens("transit")
	if !ok {
		t.Fatal("transit has no tokens")
	}
	illustration, _ := themes.Illustration("transit")

	prompt := buildThemePrompt(theme, tokens, illustration)

	accentText := toHexOrAsWritten(tokens["--accent-text"], toHexOrAsWritten(tokens["--bg"], ""))
	if !strings.Contains(prompt, accentText) {
		t.Errorf("prompt for transit does not contain its --accent-text hex value %q:\n%s", accentText, prompt)
	}
	if !strings.Contains(prompt, "second accent") {
		t.Errorf("prompt for transit does not mention a second accent:\n%s", prompt)
	}
}

func TestHumanizeTokenName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"--accent-2", "accent 2"},
		{"--green", "green"},
		{"--color-border", "border"},
	}
	for _, tt := range tests {
		if got := humanizeTokenName(tt.input); got != tt.want {
			t.Errorf("humanizeTokenName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFontFeelSentence(t *testing.T) {
	tests := []struct {
		name    string
		display string
		body    string
		want    string
	}{
		{
			name:    "same family, named once",
			display: "'JetBrains Mono', ui-monospace, 'SF Mono', Menlo, monospace",
			body:    "'JetBrains Mono', ui-monospace, 'SF Mono', Menlo, monospace",
			want:    "Type feel, as a style hint only: JetBrains Mono.",
		},
		{
			name:    "different families",
			display: "'Permanent Marker', cursive",
			body:    "'Special Elite', monospace",
			want:    "Type feel, as a style hint only: Permanent Marker for display, Special Elite for body.",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fontFeelSentence(tt.display, tt.body); got != tt.want {
				t.Errorf("fontFeelSentence() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtraPaletteColors_TerminalHasAccent2AndNoShikiOrCodeTokens(t *testing.T) {
	tokens, ok := themes.Tokens("terminal")
	if !ok {
		t.Fatal("terminal has no tokens")
	}

	bg := toHexOrAsWritten(tokens["--bg"], "")
	extra := extraPaletteColors(tokens, bg)

	found := false
	for _, color := range extra {
		if strings.HasPrefix(color.Name, "shiki") {
			t.Errorf("extraPaletteColors(terminal) includes a shiki token %q, want it dropped", color.Name)
		}
		if strings.Contains(color.Name, "code") {
			t.Errorf("extraPaletteColors(terminal) includes a code block token %q, want it dropped", color.Name)
		}
		if color.Name == "accent 2" {
			found = true
			if color.Hex != tokens["--accent-2"] {
				t.Errorf("accent 2 = %q, want %q", color.Hex, tokens["--accent-2"])
			}
		}
	}
	if !found {
		t.Error("extraPaletteColors(terminal) does not include accent 2")
	}
	if len(extra) > 8 {
		t.Errorf("extraPaletteColors(terminal) returned %d colors, want at most 8", len(extra))
	}
}

func TestExtraPaletteColors_SkipsColorsAlreadyListedAsRoles(t *testing.T) {
	tokens := map[string]string{
		"--bg":          "#000000",
		"--fg":          "#ffffff",
		"--muted":       "#888888",
		"--accent":      "#ff0000",
		"--accent-text": "#ff0000",
		"--surface":     "#111111",
		"--color-bg":    "#000000", // duplicate of --bg, must be skipped
		"--brand":       "#00ff00", // new, must be kept
	}

	extra := extraPaletteColors(tokens, "#000000")
	if len(extra) != 1 || extra[0].Name != "brand" || extra[0].Hex != "#00ff00" {
		t.Errorf("extraPaletteColors() = %+v, want exactly [{brand #00ff00}]", extra)
	}
}

func TestToHexOrAsWritten(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		background string
		want       string
	}{
		{"already hex, background irrelevant", "#ffb84d", "", "#ffb84d"},
		{"opaque rgb", "rgb(255, 184, 77)", "", "#ffb84d"},
		{"opaque hsl, pure red", "hsl(0, 100%, 50%)", "", "#ff0000"},
		{
			name:       "near-invisible alpha composites toward a light background, not black",
			input:      "rgba(0, 0, 0, 0.02)",
			background: "#ffffff",
			want:       "#fafafa",
		},
		{
			name:       "half-alpha hsla composites toward a dark background",
			input:      "hsla(240, 100%, 50%, 0.5)",
			background: "#000000",
			want:       "#000080",
		},
		{"not convertible, kept as written", "var(--other)", "", "var(--other)"},
		{"color-mix, kept as written", "color-mix(in srgb, red, blue)", "", "color-mix(in srgb, red, blue)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toHexOrAsWritten(tt.input, tt.background); got != tt.want {
				t.Errorf("toHexOrAsWritten(%q, %q) = %q, want %q", tt.input, tt.background, got, tt.want)
			}
		})
	}
}

func TestThemeShowCommand_JSONHoldsExpectedKeys(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}
	binary := buildTapBinaryForTest(t)

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(binary, "theme", "show", "terminal", "--json")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("tap theme show terminal --json failed: %v\nstderr: %s", err, stderr.String())
	}

	var output map[string]any
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout.String())
	}

	for _, key := range []string{"slug", "name", "polarity", "pitch", "tokens", "illustration", "canvas"} {
		if _, ok := output[key]; !ok {
			t.Errorf("JSON output is missing top-level key %q", key)
		}
	}

	tokens, ok := output["tokens"].(map[string]any)
	if !ok {
		t.Fatalf("tokens is not an object: %v", output["tokens"])
	}
	for _, key := range []string{"colors", "fonts", "motion", "spacing", "other"} {
		if _, ok := tokens[key]; !ok {
			t.Errorf("tokens is missing key %q", key)
		}
	}

	colors, ok := tokens["colors"].(map[string]any)
	if !ok {
		t.Fatalf("tokens.colors is not an object: %v", tokens["colors"])
	}
	extra, ok := colors["extra"].(map[string]any)
	if !ok {
		t.Fatalf("tokens.colors.extra is not an object: %v", colors["extra"])
	}
	if extra["accent 2"] != "#6fe3a0" {
		t.Errorf(`tokens.colors.extra["accent 2"] = %v, want "#6fe3a0"`, extra["accent 2"])
	}

	illustration, ok := output["illustration"].(map[string]any)
	if !ok {
		t.Fatalf("illustration is not an object: %v", output["illustration"])
	}
	for _, key := range []string{"medium", "line", "shapes", "texture", "palette_use", "mood", "avoid"} {
		if _, ok := illustration[key]; !ok {
			t.Errorf("illustration is missing key %q", key)
		}
	}

	if output["slug"] != "terminal" {
		t.Errorf("slug = %v, want terminal", output["slug"])
	}
}

func TestThemeShowCommand_UnknownSlugListsValidThemes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}
	binary := buildTapBinaryForTest(t)

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(binary, "theme", "show", "does-not-exist")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected the command to exit non-zero for an unknown theme")
	}
	if !strings.Contains(stderr.String(), "terminal") {
		t.Errorf("expected standard error to list valid themes, got %q", stderr.String())
	}
}

// TestResolveThemeShowSlug_DeckPicksDeckTheme verifies, in-process, that
// --deck reads the slug from a deck's own frontmatter theme.
// resolveThemeShowSlug reads the package-level --deck flag variable
// directly (as cobra's Run functions do), so the test sets and restores it
// itself rather than going through a cobra command.
func TestResolveThemeShowSlug_DeckPicksDeckTheme(t *testing.T) {
	deckPath := filepath.Join(t.TempDir(), "deck.md")
	deckContent := "---\ntheme: swiss\ntitle: Demo\n---\n\n# Slide\n"
	if err := os.WriteFile(deckPath, []byte(deckContent), 0o644); err != nil {
		t.Fatalf("failed to write test deck: %v", err)
	}

	previous := themeShowDeck
	themeShowDeck = deckPath
	defer func() { themeShowDeck = previous }()

	slug, err := resolveThemeShowSlug(nil)
	if err != nil {
		t.Fatalf("resolveThemeShowSlug() error = %v", err)
	}
	if slug != "swiss" {
		t.Errorf("slug = %q, want %q (the deck's own theme)", slug, "swiss")
	}
}
