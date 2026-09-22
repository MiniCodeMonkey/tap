// Package cli provides the command-line interface for Tap.
package cli

import (
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/themes"
	"github.com/spf13/cobra"
)

// Flags for the theme commands.
var (
	themeListJSON   bool
	themeShowJSON   bool
	themeShowPrompt bool
)

// themeCmd is the parent command for theme inspection.
var themeCmd = &cobra.Command{
	Use:   "theme",
	Short: "Inspect built-in themes",
	Long: `Give people and LLMs command-line access to each built-in theme's
tokens and illustration style, so an image model can be prompted for
illustrations that fit a theme.`,
}

// themeListCmd lists every built-in theme.
var themeListCmd = &cobra.Command{
	Use:   "list",
	Short: "List every built-in theme",
	Args:  cobra.NoArgs,
	RunE:  runThemeList,
}

// themeShowCmd shows one theme's tokens and illustration style.
var themeShowCmd = &cobra.Command{
	Use:   "show [slug|deck]",
	Short: "Show one theme's tokens and illustration style",
	Long: `Show a theme's name, polarity, pitch, tokens (colors, fonts, motion,
spacing), and illustration style.

With --prompt, prints a ready-to-paste style brief for an image model
instead: palette with hex values and roles, line and shape language,
texture, mood, things to avoid, and the canvas size.

The argument is a theme slug, or a deck file or folder whose theme to
show. With no argument, tap uses the deck in the current folder.

Examples:
  tap theme show terminal
  tap theme show terminal --json
  tap theme show terminal --prompt
  tap theme show slides.md --prompt`,
	Args: cobra.MaximumNArgs(1),
	RunE: runThemeShow,
}

func init() {
	rootCmd.AddCommand(themeCmd)
	themeCmd.AddCommand(themeListCmd)
	themeCmd.AddCommand(themeShowCmd)

	themeListCmd.Flags().BoolVar(&themeListJSON, "json", false, "print the list as JSON")

	themeShowCmd.Flags().BoolVar(&themeShowJSON, "json", false, "print the theme as JSON")
	themeShowCmd.Flags().BoolVar(&themeShowPrompt, "prompt", false, "print a style brief for an image model")
}

// runThemeList implements `tap theme list`.
func runThemeList(cmd *cobra.Command, args []string) error {
	all := themes.All()
	if themeListJSON {
		return printJSONOK(cmd.OutOrStdout(), struct {
			Themes []themes.Theme `json:"themes"`
		}{Themes: all})
	}

	writer := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(writer, "SLUG\tNAME\tPOLARITY\tPITCH")
	for _, theme := range all {
		fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", theme.Slug, theme.Name, theme.Polarity, theme.Pitch)
	}
	return writer.Flush()
}

// runThemeShow implements `tap theme show`.
func runThemeShow(cmd *cobra.Command, args []string) error {
	if themeShowJSON && themeShowPrompt {
		return userError(codeUsage, errors.New("--json and --prompt cannot be used together"))
	}

	slug, err := resolveThemeShowSlug(firstArg(args))
	if err != nil {
		return err
	}
	theme, ok := findTheme(slug)
	if !ok {
		return userError(codeUnknownTheme, unknownThemeError(slug))
	}
	tokens, ok := themes.Tokens(slug)
	if !ok {
		return internalError(codeInternal, fmt.Errorf("theme %q has no tokens", slug))
	}
	illustration, _ := themes.Illustration(slug)

	switch {
	case themeShowPrompt:
		fmt.Fprintln(cmd.OutOrStdout(), buildThemePrompt(theme, tokens, illustration))
	case themeShowJSON:
		return printJSONOK(cmd.OutOrStdout(), themeShowJSONOutput{
			Slug:         theme.Slug,
			Name:         theme.Name,
			Polarity:     theme.Polarity,
			Pitch:        theme.Pitch,
			Tokens:       buildThemeTokensJSON(tokens),
			Illustration: illustration,
			Canvas:       themeCanvasJSON{Ratio: "16:9", Width: 1920, Height: 1080},
		})
	default:
		printThemeHuman(theme, tokens, illustration)
	}
	return nil
}

// resolveThemeShowSlug returns the theme tap theme show describes. arg is
// a built-in slug, or a deck file or folder whose theme to use. An empty
// arg means the deck in the current folder. A deck that names no theme
// uses "base", with a note on standard error.
func resolveThemeShowSlug(arg string) (string, error) {
	if arg != "" && themes.IsValid(arg) {
		return arg, nil
	}
	if arg != "" {
		if _, err := os.Stat(arg); os.IsNotExist(err) {
			return "", userError(codeUnknownTheme, unknownThemeError(arg))
		}
	}

	deck, err := resolveDeck(arg)
	if err != nil {
		return "", err
	}
	cfg, err := config.Load(deck)
	if err != nil {
		return "", userError(codeInvalidDeck, fmt.Errorf("failed to load %s: %w", deck, err))
	}
	if err := cfg.Validate(); err != nil {
		return "", userError(codeInvalidDeck, fmt.Errorf("invalid configuration in %s: %w", deck, err))
	}
	if cfg.Theme == "" {
		Warningln(fmt.Sprintf("Warning: %s names no theme, using \"base\"", deck))
		return "base", nil
	}
	// cfg.Validate() already turned an unknown theme into "base" and
	// printed a warning to standard error.
	return cfg.Theme, nil
}

// findTheme looks up a theme's summary (name, polarity, pitch) by slug.
func findTheme(slug string) (themes.Theme, bool) {
	for _, theme := range themes.All() {
		if theme.Slug == slug {
			return theme, true
		}
	}
	return themes.Theme{}, false
}

// unknownThemeError is defined in export_images.go and reused here: it reports
// an unknown theme slug, listing every valid one.

// ============================================================================
// Human-readable output
// ============================================================================

func printThemeHuman(theme themes.Theme, tokens map[string]string, illustration themes.IllustrationStyle) {
	fmt.Printf("%s (%s)\n", theme.Name, theme.Slug)
	fmt.Printf("  Polarity: %s\n", theme.Polarity)
	fmt.Printf("  Pitch: %s\n", theme.Pitch)

	fmt.Println("\n  Colors:")
	printColorRole("bg", "background", tokens)
	printColorRole("fg", "foreground", tokens)
	printColorRole("muted", "muted text", tokens)
	printColorRole("accent", "accent", tokens)
	printColorRole("accent-text", "accent (as text)", tokens)
	printColorRole("accent-2", "second accent", tokens)
	printColorRole("surface", "surface", tokens)
	printColorRole("status-ok", "status ok", tokens)
	printColorRole("status-warn", "status warn", tokens)
	printColorRole("status-error", "status error", tokens)

	fmt.Println("\n  Fonts:")
	fmt.Printf("    display: %s\n", tokens["--font-display"])
	fmt.Printf("    body: %s\n", tokens["--font-body"])
	fmt.Printf("    mono: %s\n", tokens["--font-mono"])

	fmt.Println("\n  Motion:")
	fmt.Printf("    ease: %s\n", tokens["--ease"])
	fmt.Printf("    duration: %s\n", tokens["--dur"])

	fmt.Println("\n  Spacing:")
	fmt.Printf("    space unit: %s\n", tokens["--space-unit"])
	fmt.Printf("    radius: %s\n", tokens["--radius"])
	fmt.Printf("    stroke width: %s\n", tokens["--stroke-width"])

	fmt.Println("\n  Illustration:")
	fmt.Printf("    medium: %s\n", illustration.Medium)
	fmt.Printf("    line: %s\n", illustration.Line)
	fmt.Printf("    shapes: %s\n", illustration.Shapes)
	fmt.Printf("    texture: %s\n", illustration.Texture)
	fmt.Printf("    palette use: %s\n", illustration.PaletteUse)
	fmt.Printf("    mood: %s\n", illustration.Mood)
	fmt.Printf("    avoid: %s\n", illustration.Avoid)
}

func printColorRole(cssSuffix, role string, tokens map[string]string) {
	fmt.Printf("    %s (%s): %s\n", role, cssSuffix, tokens["--"+cssSuffix])
}

// ============================================================================
// JSON output
// ============================================================================

type themeColorsJSON struct {
	BG          string            `json:"bg"`
	FG          string            `json:"fg"`
	Muted       string            `json:"muted"`
	Accent      string            `json:"accent"`
	AccentText  string            `json:"accentText"`
	Accent2     string            `json:"accent2"`
	Surface     string            `json:"surface"`
	StatusOk    string            `json:"statusOk"`
	StatusWarn  string            `json:"statusWarn"`
	StatusError string            `json:"statusError"`
	Extra       map[string]string `json:"extra,omitempty"`
}

type themeFontsJSON struct {
	Display string `json:"display"`
	Body    string `json:"body"`
	Mono    string `json:"mono"`
}

type themeMotionJSON struct {
	Ease     string `json:"ease"`
	Duration string `json:"duration"`
}

type themeSpacingJSON struct {
	SpaceUnit   string `json:"spaceUnit"`
	Radius      string `json:"radius"`
	StrokeWidth string `json:"strokeWidth"`
}

type themeTokensJSON struct {
	Colors  themeColorsJSON   `json:"colors"`
	Fonts   themeFontsJSON    `json:"fonts"`
	Motion  themeMotionJSON   `json:"motion"`
	Spacing themeSpacingJSON  `json:"spacing"`
	Other   map[string]string `json:"other"`
}

type themeCanvasJSON struct {
	Ratio  string `json:"ratio"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type themeShowJSONOutput struct {
	Slug         string                   `json:"slug"`
	Name         string                   `json:"name"`
	Polarity     string                   `json:"polarity"`
	Pitch        string                   `json:"pitch"`
	Tokens       themeTokensJSON          `json:"tokens"`
	Illustration themes.IllustrationStyle `json:"illustration"`
	Canvas       themeCanvasJSON          `json:"canvas"`
}

// knownTokenCSSNames are the tokens.go RequiredTokens, used to split a
// theme's full token map into its named fields and the catch-all "other".
var knownTokenCSSNames = func() map[string]bool {
	known := make(map[string]bool, len(themes.RequiredTokens))
	for _, name := range themes.RequiredTokens {
		known[name] = true
	}
	return known
}()

func buildThemeTokensJSON(tokens map[string]string) themeTokensJSON {
	other := make(map[string]string)
	for name, value := range tokens {
		if !knownTokenCSSNames[name] {
			other[name] = value
		}
	}

	bgHex := toHexOrAsWritten(tokens["--bg"], "")
	extraColors := extraPaletteColors(tokens, bgHex)
	var extra map[string]string
	if len(extraColors) > 0 {
		extra = make(map[string]string, len(extraColors))
		for _, color := range extraColors {
			extra[color.Name] = color.Hex
		}
	}

	return themeTokensJSON{
		Colors: themeColorsJSON{
			BG:          tokens["--bg"],
			FG:          tokens["--fg"],
			Muted:       tokens["--muted"],
			Accent:      tokens["--accent"],
			AccentText:  tokens["--accent-text"],
			Accent2:     tokens["--accent-2"],
			Surface:     tokens["--surface"],
			StatusOk:    tokens["--status-ok"],
			StatusWarn:  tokens["--status-warn"],
			StatusError: tokens["--status-error"],
			Extra:       extra,
		},
		Fonts: themeFontsJSON{
			Display: tokens["--font-display"],
			Body:    tokens["--font-body"],
			Mono:    tokens["--font-mono"],
		},
		Motion: themeMotionJSON{
			Ease:     tokens["--ease"],
			Duration: tokens["--dur"],
		},
		Spacing: themeSpacingJSON{
			SpaceUnit:   tokens["--space-unit"],
			Radius:      tokens["--radius"],
			StrokeWidth: tokens["--stroke-width"],
		},
		Other: other,
	}
}

// ============================================================================
// --prompt output
// ============================================================================

// buildThemePrompt builds a plain-text style brief for an image model: a
// one-line style summary, the palette with hex values and roles (plus any
// other root color tokens the theme defines), how the palette is used,
// line and shape language, texture, mood, things to avoid, a font-feel
// hint, the no-text instruction, and the canvas. Plain sentences, no
// markdown markup, no em dashes, kept under 200 words.
func buildThemePrompt(theme themes.Theme, tokens map[string]string, illustration themes.IllustrationStyle) string {
	bg := toHexOrAsWritten(tokens["--bg"], "")
	fg := toHexOrAsWritten(tokens["--fg"], bg)
	accent := toHexOrAsWritten(tokens["--accent"], bg)
	muted := toHexOrAsWritten(tokens["--muted"], bg)
	surface := toHexOrAsWritten(tokens["--surface"], bg)
	accentText := toHexOrAsWritten(tokens["--accent-text"], bg)

	otherRoleHexes := map[string]bool{bg: true, fg: true, muted: true, surface: true}

	var b strings.Builder
	fmt.Fprintf(&b, "Illustration style for the %q slide theme: %s. ", theme.Name, illustration.Medium)
	fmt.Fprintf(&b, "Use only this palette: background %s, foreground %s, accent %s, muted %s, surface %s", bg, fg, accent, muted, surface)
	if accentText != "" && accentText != accent && !otherRoleHexes[accentText] {
		fmt.Fprintf(&b, ", second accent %s", accentText)
	}
	fmt.Fprint(&b, ". ")

	extra := extraPaletteColors(tokens, bg)
	if len(extra) > 0 {
		parts := make([]string, len(extra))
		for i, color := range extra {
			parts[i] = fmt.Sprintf("%s %s", color.Name, color.Hex)
		}
		fmt.Fprintf(&b, "Extra colors: %s. ", strings.Join(parts, ", "))
	}

	statusOk := toHexOrAsWritten(tokens["--status-ok"], bg)
	statusWarn := toHexOrAsWritten(tokens["--status-warn"], bg)
	statusError := toHexOrAsWritten(tokens["--status-error"], bg)
	fmt.Fprintf(&b, "Status colors, only where meaning requires them: ok %s, warn %s, error %s. ", statusOk, statusWarn, statusError)

	fmt.Fprintf(&b, "Palette use: %s. ", illustration.PaletteUse)
	fmt.Fprintf(&b, "Line: %s. ", illustration.Line)
	fmt.Fprintf(&b, "Shapes: %s. ", illustration.Shapes)
	fmt.Fprintf(&b, "Texture: %s. ", illustration.Texture)
	fmt.Fprintf(&b, "Mood: %s. ", illustration.Mood)
	fmt.Fprintf(&b, "Avoid: %s. ", illustration.Avoid)
	fmt.Fprintf(&b, "%s ", fontFeelSentence(tokens["--font-display"], tokens["--font-body"]))
	fmt.Fprint(&b, "No text or lettering in the image unless asked. ")
	fmt.Fprintf(&b, "Canvas 16:9, sitting on a %s background so it blends into the slide.", bg)

	return strings.TrimSpace(b.String())
}

// fontFeelSentence builds the prompt's font-feel sentence from just the
// first family of the display and body font stacks, unquoted. When both
// stacks start with the same family, it is named once.
func fontFeelSentence(fontDisplay, fontBody string) string {
	display := firstFontFamily(fontDisplay)
	body := firstFontFamily(fontBody)

	if display == body {
		return fmt.Sprintf("Type feel, as a style hint only: %s.", display)
	}
	return fmt.Sprintf("Type feel, as a style hint only: %s for display, %s for body.", display, body)
}

// firstFontFamily returns the first family in a CSS font stack (e.g.
// "'JetBrains Mono', ui-monospace, monospace" -> "JetBrains Mono"), with
// surrounding quotes stripped.
func firstFontFamily(fontStack string) string {
	first := strings.TrimSpace(strings.SplitN(fontStack, ",", 2)[0])
	first = strings.Trim(first, `'"`)
	return first
}

// ============================================================================
// Extra palette colors
// ============================================================================

// paletteColor is one extra color to mention in the --prompt brief: a
// plain-words token name (e.g. "accent 2") and its hex (or as-written)
// value.
type paletteColor struct {
	Name string
	Hex  string
}

// paletteRoleTokens are the five token names already named as roles in the
// prompt's palette sentence (and printed elsewhere as the theme's known
// colors), excluded here so they are never repeated.
var paletteRoleTokens = []string{
	"--bg", "--fg", "--muted", "--accent", "--accent-text", "--accent-2", "--surface",
	"--status-ok", "--status-warn", "--status-error",
}

var colorValuePattern = regexp.MustCompile(`(?i)^(#[0-9a-f]{3,8}|rgba?\([^)]*\)|hsla?\([^)]*\))$`)

// isColorValue reports whether a CSS custom property's value is a color:
// hex, rgb()/rgba(), or hsl()/hsla().
func isColorValue(value string) bool {
	return colorValuePattern.MatchString(strings.TrimSpace(value))
}

// humanizeTokenName turns a CSS custom property name into plain words:
// "--accent-2" becomes "accent 2", "--green" becomes "green". A leading
// "color " is stripped too, since every such token already lives under a
// "--color-*" bridge to the shared chrome (see docs/reference/theme-
// porting.md) and the prefix reads as noise once it's a plain phrase:
// "--color-border" becomes "border", not "color border".
func humanizeTokenName(name string) string {
	name = strings.TrimPrefix(name, "--")
	words := strings.ReplaceAll(name, "-", " ")
	return strings.TrimPrefix(words, "color ")
}

// extraPaletteColors lists every root color token beyond the five palette
// roles, for the --prompt brief's "Extra colors:" sentence (and --json's
// tokens.colors.extra). Shiki syntax-highlighting tokens and any token
// whose name mentions a code block ("--color-code-bg", "--code-line", and
// so on) are excluded entirely: neither means anything to an image model,
// and a theme's dark code-block background in particular has no bearing
// on its illustration palette (base's near-black code background would
// otherwise contradict its plain white brief). Those tokens are still
// available under --json's tokens.other. Any remaining color whose hex
// value already matches one already listed (a role, or an extra already
// added) is skipped so the same color is never named twice. backgroundHex
// is passed through to toHexOrAsWritten, so a translucent extra color
// composites over the theme's own background the same way the palette
// roles do. The result is sorted by token name for a stable order, and
// capped at eight colors.
func extraPaletteColors(tokens map[string]string, backgroundHex string) []paletteColor {
	isRole := make(map[string]bool, len(paletteRoleTokens))
	seenHex := make(map[string]bool, len(paletteRoleTokens))
	for _, name := range paletteRoleTokens {
		isRole[name] = true
		if value, ok := tokens[name]; ok {
			seenHex[toHexOrAsWritten(value, backgroundHex)] = true
		}
	}

	names := make([]string, 0, len(tokens))
	for name := range tokens {
		names = append(names, name)
	}
	sort.Strings(names)

	var result []paletteColor
	for _, name := range names {
		if isRole[name] {
			continue
		}
		if strings.HasPrefix(name, "--shiki-") || strings.Contains(name, "code") {
			continue
		}
		value := tokens[name]
		if !isColorValue(value) {
			continue
		}
		hex := toHexOrAsWritten(value, backgroundHex)
		if seenHex[hex] {
			continue
		}
		seenHex[hex] = true
		result = append(result, paletteColor{Name: humanizeTokenName(name), Hex: hex})
		if len(result) == 8 {
			break
		}
	}

	return result
}

// ============================================================================
// Color conversion
// ============================================================================

var (
	rgbFuncPattern = regexp.MustCompile(`^rgba?\(\s*([\d.]+)\s*,\s*([\d.]+)\s*,\s*([\d.]+)\s*(?:,\s*([\d.]+)\s*)?\)$`)
	hslFuncPattern = regexp.MustCompile(`^hsla?\(\s*([\d.]+)\s*,\s*([\d.]+)%\s*,\s*([\d.]+)%\s*(?:,\s*([\d.]+)\s*)?\)$`)
	hexPattern     = regexp.MustCompile(`^#([0-9a-fA-F]{6}|[0-9a-fA-F]{3})$`)
)

// toHexOrAsWritten converts an rgb()/rgba()/hsl()/hsla() color value to a
// hex string. When the value has alpha below 1, its color is composited
// over backgroundHex first, since a translucent color's own components
// alone would misrepresent how it actually reads against a solid canvas
// (rgba(0, 0, 0, 0.02), a near-invisible wash, would otherwise become
// black). backgroundHex is only used in that case; pass "" when no
// backdrop is known, or for a value that is converting the theme's own
// --bg (always opaque). A value that is already hex, or that this cannot
// convert (a named color, var(), color-mix()), is returned exactly as
// written.
func toHexOrAsWritten(value string, backgroundHex string) string {
	value = strings.TrimSpace(value)
	if hexPattern.MatchString(value) {
		return value
	}

	if match := rgbFuncPattern.FindStringSubmatch(value); match != nil {
		r, g, b, a, ok := parseRGBA(match)
		if !ok {
			return value
		}
		return compositeToHex(r, g, b, a, backgroundHex)
	}

	if match := hslFuncPattern.FindStringSubmatch(value); match != nil {
		r, g, b, a, ok := parseHSLA(match)
		if !ok {
			return value
		}
		return compositeToHex(r, g, b, a, backgroundHex)
	}

	return value
}

// parseRGBA reads the four rgb()/rgba() regex groups (r, g, b, optional
// alpha) into 0-255 components and a 0-1 alpha (defaulting to 1 when the
// value had no alpha).
func parseRGBA(match []string) (r, g, b int, a float64, ok bool) {
	var errR, errG, errB error
	r, errR = parseColorComponent(match[1])
	g, errG = parseColorComponent(match[2])
	b, errB = parseColorComponent(match[3])
	if errR != nil || errG != nil || errB != nil {
		return 0, 0, 0, 0, false
	}
	a = 1
	if match[4] != "" {
		alpha, err := strconv.ParseFloat(match[4], 64)
		if err != nil {
			return 0, 0, 0, 0, false
		}
		a = clamp01(alpha)
	}
	return r, g, b, a, true
}

// parseHSLA reads the four hsl()/hsla() regex groups (hue, saturation
// percent, lightness percent, optional alpha) and converts to 0-255 RGB
// components plus a 0-1 alpha (defaulting to 1 when the value had no
// alpha).
func parseHSLA(match []string) (r, g, b int, a float64, ok bool) {
	h, hueErr := strconv.ParseFloat(match[1], 64)
	s, saturationErr := strconv.ParseFloat(match[2], 64)
	l, lightnessErr := strconv.ParseFloat(match[3], 64)
	if hueErr != nil || saturationErr != nil || lightnessErr != nil {
		return 0, 0, 0, 0, false
	}
	r, g, b = hslToRGB(h, s/100, l/100)

	a = 1
	if match[4] != "" {
		alpha, err := strconv.ParseFloat(match[4], 64)
		if err != nil {
			return 0, 0, 0, 0, false
		}
		a = clamp01(alpha)
	}
	return r, g, b, a, true
}

// hslToRGB converts HSL (hue in degrees, saturation and lightness as
// 0-1 fractions) to 0-255 RGB components, using the standard CSS Color
// Module conversion.
func hslToRGB(h, s, l float64) (r, g, b int) {
	h = math.Mod(h, 360)
	if h < 0 {
		h += 360
	}

	c := (1 - math.Abs(2*l-1)) * s
	x := c * (1 - math.Abs(math.Mod(h/60, 2)-1))
	m := l - c/2

	var rf, gf, bf float64
	switch {
	case h < 60:
		rf, gf, bf = c, x, 0
	case h < 120:
		rf, gf, bf = x, c, 0
	case h < 180:
		rf, gf, bf = 0, c, x
	case h < 240:
		rf, gf, bf = 0, x, c
	case h < 300:
		rf, gf, bf = x, 0, c
	default:
		rf, gf, bf = c, 0, x
	}

	return clampChannel((rf + m) * 255), clampChannel((gf + m) * 255), clampChannel((bf + m) * 255)
}

// compositeToHex formats an opaque color as hex directly. A color with
// alpha below 1 is first composited over backgroundHex (its own r, g, b
// blended toward the background's by (1 - alpha)); when backgroundHex
// isn't a convertible hex color, the color's own components are used as
// written, alpha aside, since there is no backdrop to composite against.
func compositeToHex(r, g, b int, a float64, backgroundHex string) string {
	if a >= 1 {
		return fmt.Sprintf("#%02x%02x%02x", r, g, b)
	}

	bgR, bgG, bgB, ok := parseHexChannels(backgroundHex)
	if !ok {
		return fmt.Sprintf("#%02x%02x%02x", r, g, b)
	}

	composite := func(fg, bg int) int {
		return clampChannel(float64(fg)*a + float64(bg)*(1-a))
	}
	return fmt.Sprintf("#%02x%02x%02x", composite(r, bgR), composite(g, bgG), composite(b, bgB))
}

// parseHexChannels reads a #rrggbb (or #rgb) hex color's three channels.
func parseHexChannels(hex string) (r, g, b int, ok bool) {
	if !hexPattern.MatchString(hex) {
		return 0, 0, 0, false
	}
	digits := hex[1:]
	if len(digits) == 3 {
		digits = string([]byte{digits[0], digits[0], digits[1], digits[1], digits[2], digits[2]})
	}
	rv, errR := strconv.ParseUint(digits[0:2], 16, 8)
	gv, errG := strconv.ParseUint(digits[2:4], 16, 8)
	bv, errB := strconv.ParseUint(digits[4:6], 16, 8)
	if errR != nil || errG != nil || errB != nil {
		return 0, 0, 0, false
	}
	return int(rv), int(gv), int(bv), true
}

func clamp01(f float64) float64 {
	if f < 0 {
		return 0
	}
	if f > 1 {
		return 1
	}
	return f
}

func clampChannel(f float64) int {
	n := int(math.Round(f))
	if n < 0 {
		return 0
	}
	if n > 255 {
		return 255
	}
	return n
}

func parseColorComponent(s string) (int, error) {
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, err
	}
	return clampChannel(f), nil
}
