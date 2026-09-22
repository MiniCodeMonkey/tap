// Package config handles presentation configuration from YAML frontmatter.
package config

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"

	"github.com/MiniCodeMonkey/tap/internal/themes"
)

// Config represents the presentation configuration from YAML frontmatter.
type Config struct {
	Drivers     map[string]DriverConfig `yaml:"drivers" json:"drivers,omitempty"`
	ThemeColors map[string]string       `yaml:"themeColors" json:"themeColors,omitempty"`
	Title       string                  `yaml:"title" json:"title,omitempty"`
	Theme       string                  `yaml:"theme" json:"theme,omitempty"`
	CustomTheme string                  `yaml:"customTheme" json:"customTheme,omitempty"`
	Author      string                  `yaml:"author" json:"author,omitempty"`
	Date        string                  `yaml:"date" json:"date,omitempty"`
	AspectRatio string                  `yaml:"aspectRatio" json:"aspectRatio,omitempty"`
	Transition  string                  `yaml:"transition" json:"transition,omitempty"`
	// SlideNumbers turns off the slide number the theme draws on every
	// slide when set to false. Nil (the key left out) keeps the numbers.
	SlideNumbers *bool `yaml:"slideNumbers" json:"slideNumbers,omitempty"`
	// PresenterLayout names the layout the presenter view opens in. It is a
	// suggestion: a device that has chosen a layout for itself keeps that
	// choice. An empty value means the deck expresses no preference.
	PresenterLayout string    `yaml:"presenterLayout" json:"presenterLayout,omitempty"`
	Recording       Recording `yaml:"recording" json:"recording,omitempty"`
}

// DefaultDriverTimeoutSeconds is how long a live code run may take when
// the deck's driver settings give no timeout.
const DefaultDriverTimeoutSeconds = 30

// DefaultRecordingOutput is where recordings go, relative to the deck,
// when recording.output is not set.
const DefaultRecordingOutput = "recordings"

// Recording configures the screen recording the dev TUI can start. Every
// key is optional; an omitted block leaves the defaults in place.
type Recording struct {
	// Output is the directory recordings are written to, relative to the
	// deck. Empty means "recordings" next to the deck.
	Output string `yaml:"output" json:"output,omitempty"`
	// Audio selects the microphone: "default" or empty for the system
	// default input, "none" for a silent recording, or a CoreAudio device
	// UID. Names and indexes are not accepted by the recorder.
	Audio string `yaml:"audio" json:"audio,omitempty"`
	// WarnAfter is how long a recording runs before the TUI warns about it.
	WarnAfter string `yaml:"warnAfter" json:"warnAfter,omitempty"`
	// StopAfter is how long a recording runs before it stops itself. "off"
	// disables the cap.
	StopAfter string `yaml:"stopAfter" json:"stopAfter,omitempty"`
	// Display preselects an entry in the record picker. Zero means the
	// picker opens on the main display.
	Display int `yaml:"display" json:"display,omitempty"`
	// ShowClicks draws mouse clicks in the recording.
	ShowClicks bool `yaml:"showClicks" json:"showClicks,omitempty"`
	// Chapters turns off the sidecar chapter list when set to false. Nil
	// (the key left out) writes the list.
	Chapters *bool `yaml:"chapters" json:"chapters,omitempty"`
}

// defaultWarnAfter is when a running recording starts nagging: long enough
// that no ordinary talk reaches it, short enough to catch one left running
// through the hallway conversation afterwards.
const defaultWarnAfter = 90 * time.Minute

// defaultStopAfter bounds a forgotten recording before it fills a disk.
const defaultStopAfter = 3 * time.Hour

// WarnAfterDuration is how long a recording runs before the TUI warns.
func (r *Recording) WarnAfterDuration() time.Duration {
	return parseRecordingDuration(r.WarnAfter, defaultWarnAfter)
}

// StopAfterDuration is how long a recording runs before it stops itself.
// Zero means the cap is off.
func (r *Recording) StopAfterDuration() time.Duration {
	if strings.EqualFold(strings.TrimSpace(r.StopAfter), "off") {
		return 0
	}
	return parseRecordingDuration(r.StopAfter, defaultStopAfter)
}

// ChaptersEnabled reports whether the sidecar chapter list is written.
func (r *Recording) ChaptersEnabled() bool {
	return r.Chapters == nil || *r.Chapters
}

// parseRecordingDuration falls back to the default for an empty or
// unparseable value. Validate is what reports an unparseable one; the
// accessors stay total so a bad value never stops a recording mid-talk.
func parseRecordingDuration(value string, fallback time.Duration) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

// DriverConfig represents the configuration for a code execution driver.
type DriverConfig struct {
	Connections map[string]ConnectionConfig `yaml:"connections"`
	Command     string                      `yaml:"command"`
	Args        []string                    `yaml:"args"`
	Timeout     int                         `yaml:"timeout"`
}

// ConnectionConfig represents connection details for a driver.
type ConnectionConfig struct {
	Host     string `yaml:"host"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Database string `yaml:"database"`
	Path     string `yaml:"path"`
	Port     int    `yaml:"port"`
}

// Load reads a markdown file and parses its YAML frontmatter into a
// Config (see FromSource). When the deck has frontmatter, it also loads
// the .env file in the deck's folder and resolves environment variables
// in the driver settings.
func Load(path string) (*Config, error) {
	source, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	cfg, found, err := parseFrontmatter(source)
	if err != nil || !found {
		return cfg, err
	}

	// Load .env file from presentation directory
	if err := LoadEnv(filepath.Dir(path)); err != nil {
		return nil, fmt.Errorf("failed to load .env file: %w", err)
	}

	// Resolve environment variables in sensitive fields
	cfg.ResolveEnvVars()

	return cfg, nil
}

// FromSource parses the YAML frontmatter at the start of a deck's markdown
// into a Config, with Load's rules: the first line must be "---", and a
// deck without frontmatter gets DefaultConfig. Unlike Load, it reads no
// .env file and resolves no environment variables, so it has no side
// effects.
func FromSource(source []byte) (*Config, error) {
	cfg, _, err := parseFrontmatter(source)
	return cfg, err
}

// parseFrontmatter parses the frontmatter of source over DefaultConfig.
// found is false when source has no frontmatter.
func parseFrontmatter(source []byte) (cfg *Config, found bool, err error) {
	scanner := bufio.NewScanner(bytes.NewReader(source))

	// Check for frontmatter start delimiter
	if !scanner.Scan() {
		return nil, false, fmt.Errorf("empty file")
	}

	firstLine := strings.TrimSpace(scanner.Text())
	if firstLine != "---" {
		// No frontmatter, return default config
		return DefaultConfig(), false, nil
	}

	// Read frontmatter content until closing delimiter
	var frontmatter strings.Builder
	foundEnd := false

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			foundEnd = true
			break
		}
		frontmatter.WriteString(line)
		frontmatter.WriteString("\n")
	}

	if err := scanner.Err(); err != nil {
		return nil, false, fmt.Errorf("error reading file: %w", err)
	}

	if !foundEnd {
		return nil, false, fmt.Errorf("frontmatter not closed: missing closing ---")
	}

	// Parse YAML frontmatter
	cfg = DefaultConfig()
	if err := yaml.Unmarshal([]byte(frontmatter.String()), cfg); err != nil {
		return nil, false, fmt.Errorf("failed to parse frontmatter: %w", err)
	}
	return cfg, true, nil
}

// DefaultConfig returns a Config with sensible default values.
func DefaultConfig() *Config {
	return &Config{
		Theme:       "base",
		AspectRatio: "16:9",
		Transition:  "fade",
		Drivers:     make(map[string]DriverConfig),
	}
}

// aspectRatioValues lists the allowed aspectRatio values, in the order
// the schema shows them.
var aspectRatioValues = []string{"16:9", "4:3", "16:10"}

// validAspectRatios contains the allowed aspect ratio values.
var validAspectRatios = valueSet(aspectRatioValues)

// presenterLayoutValues lists the allowed presenterLayout values.
var presenterLayoutValues = []string{"standard", "notes-first", "duo", "slide-only", "notes-only"}

// validPresenterLayouts contains the allowed presenterLayout values.
var validPresenterLayouts = valueSet(presenterLayoutValues)

// transitionValues lists the allowed transition values.
var transitionValues = []string{"none", "fade", "slide", "push", "zoom"}

// validTransitions contains the allowed transition values.
var validTransitions = valueSet(transitionValues)

// themeColorKeys lists the allowed themeColors keys and the CSS custom
// property each one sets.
var themeColorKeys = []struct {
	name     string
	property string
}{
	{"background", "--color-bg"},
	{"text", "--color-text"},
	{"muted", "--color-muted"},
	{"accent", "--color-accent"},
	{"codeBg", "--color-code-bg"},
}

// validThemeColorKeys contains the allowed themeColors keys.
var validThemeColorKeys = func() map[string]bool {
	keys := make(map[string]bool, len(themeColorKeys))
	for _, key := range themeColorKeys {
		keys[key.name] = true
	}
	return keys
}()

// valueSet turns a list of allowed values into a set for lookups.
func valueSet(values []string) map[string]bool {
	set := make(map[string]bool, len(values))
	for _, value := range values {
		set[value] = true
	}
	return set
}

// hexColorPattern matches valid CSS hex colors (#RGB, #RRGGBB, #RGBA, #RRGGBBAA).
var hexColorPattern = regexp.MustCompile(`^#([0-9a-fA-F]{3}|[0-9a-fA-F]{4}|[0-9a-fA-F]{6}|[0-9a-fA-F]{8})$`)

// isValidColor checks if a string is a valid CSS color value.
// Supports hex colors (#RGB, #RRGGBB, #RGBA, #RRGGBBAA) and CSS color functions.
func isValidColor(value string) bool {
	// Check hex color
	if hexColorPattern.MatchString(value) {
		return true
	}

	// Check CSS color functions (rgb, rgba, hsl, hsla, oklch, etc.)
	colorFunctions := []string{"rgb(", "rgba(", "hsl(", "hsla(", "oklch(", "oklab(", "lch(", "lab("}
	for _, prefix := range colorFunctions {
		if len(value) > len(prefix) && value[:len(prefix)] == prefix {
			return true
		}
	}

	// Check named colors (basic set - not exhaustive, but covers common cases)
	namedColors := map[string]bool{
		"black": true, "white": true, "red": true, "green": true, "blue": true,
		"yellow": true, "orange": true, "purple": true, "pink": true, "gray": true,
		"grey": true, "transparent": true, "currentColor": true, "inherit": true,
	}
	return namedColors[value]
}

// Validate checks the Config for invalid values and returns an error
// with a descriptive message if validation fails.
// It also normalizes an unknown theme name to "base".
func (c *Config) Validate() error {
	// Normalize the theme. An unknown theme falls back to "base" with a
	// warning rather than failing validation.
	if c.Theme != "" {
		c.Theme = NormalizeTheme(c.Theme)
	}

	// Validate aspect ratio
	if c.AspectRatio != "" && !validAspectRatios[c.AspectRatio] {
		return fmt.Errorf("invalid aspectRatio %q: must be one of 16:9, 4:3, or 16:10", c.AspectRatio)
	}

	// Validate presenter layout
	if c.PresenterLayout != "" && !validPresenterLayouts[c.PresenterLayout] {
		return fmt.Errorf("invalid presenterLayout %q: must be one of standard, notes-first, duo, slide-only, notes-only", c.PresenterLayout)
	}

	// Validate transition
	if c.Transition != "" && !validTransitions[c.Transition] {
		return fmt.Errorf("invalid transition %q: must be one of none, fade, slide, push, or zoom", c.Transition)
	}

	// Validate themeColors keys (invalid colors are logged as warnings but not errors)
	for key := range c.ThemeColors {
		if !validThemeColorKeys[key] {
			return fmt.Errorf("invalid themeColors key %q: must be one of background, text, muted, accent, or codeBg", key)
		}
	}

	// Validate recording durations
	for _, field := range []struct {
		name  string
		value string
	}{
		{"warnAfter", c.Recording.WarnAfter},
		{"stopAfter", c.Recording.StopAfter},
	} {
		value := strings.TrimSpace(field.value)
		if value == "" || (field.name == "stopAfter" && strings.EqualFold(value, "off")) {
			continue
		}
		parsed, err := time.ParseDuration(value)
		if err != nil || parsed <= 0 {
			return fmt.Errorf("invalid recording.%s %q: must be a positive duration such as 90m or 3h", field.name, field.value)
		}
	}

	// Validate recording display
	if c.Recording.Display < 0 {
		return fmt.Errorf("invalid recording.display %d: must be 0 or greater", c.Recording.Display)
	}

	return nil
}

// envVarPattern matches environment variable references like $VAR_NAME or ${VAR_NAME}.
var envVarPattern = regexp.MustCompile(`\$\{?([A-Za-z_][A-Za-z0-9_]*)\}?`)

// LoadEnv loads environment variables from a .env file in the specified directory.
// If the .env file doesn't exist, it returns nil (no error).
func LoadEnv(dir string) error {
	envPath := filepath.Join(dir, ".env")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		return nil
	}
	return godotenv.Load(envPath)
}

// resolveEnvVars replaces $VAR_NAME and ${VAR_NAME} syntax with actual
// environment variable values. If a variable is not set, the reference
// is left unchanged.
func resolveEnvVars(s string) string {
	return envVarPattern.ReplaceAllStringFunc(s, func(match string) string {
		// Extract variable name from match
		varName := envVarPattern.FindStringSubmatch(match)[1]
		if value, exists := os.LookupEnv(varName); exists {
			return value
		}
		return match
	})
}

// ResolveEnvVars resolves environment variable references in sensitive config fields.
// This includes passwords and other credential-related fields in driver connections.
func (c *Config) ResolveEnvVars() {
	for driverName, driver := range c.Drivers {
		for connName, conn := range driver.Connections {
			conn.Password = resolveEnvVars(conn.Password)
			conn.User = resolveEnvVars(conn.User)
			conn.Host = resolveEnvVars(conn.Host)
			conn.Database = resolveEnvVars(conn.Database)
			conn.Path = resolveEnvVars(conn.Path)
			driver.Connections[connName] = conn
		}
		c.Drivers[driverName] = driver
	}
}

// NormalizeTheme returns the theme unchanged if it is valid. Any other theme
// name logs a warning and falls back to "base".
func NormalizeTheme(theme string) string {
	if themes.IsValid(theme) {
		return theme
	}

	log.Printf("Warning: unknown theme %q, using \"base\"", theme)
	return "base"
}

// ValidThemeNames returns the list of valid theme names.
func ValidThemeNames() []string {
	return themes.Slugs()
}

// UpdateThemeInFile updates the theme field in a markdown file's frontmatter.
// If the file has no frontmatter, it adds one with just the theme.
// If the frontmatter has no theme field, it adds one.
func UpdateThemeInFile(path string, newTheme string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	if len(lines) == 0 {
		return fmt.Errorf("empty file")
	}

	// Check if file has frontmatter
	if strings.TrimSpace(lines[0]) != "---" {
		// No frontmatter - add one with just the theme
		newContent := fmt.Sprintf("---\ntheme: %s\n---\n%s", newTheme, string(content))
		return os.WriteFile(path, []byte(newContent), 0644)
	}

	// Find the end of frontmatter
	endIndex := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIndex = i
			break
		}
	}

	if endIndex == -1 {
		return fmt.Errorf("frontmatter not closed")
	}

	// Look for existing theme line in frontmatter
	themeLineIndex := -1
	for i := 1; i < endIndex; i++ {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "theme:") {
			themeLineIndex = i
			break
		}
	}

	if themeLineIndex != -1 {
		// Replace existing theme line
		lines[themeLineIndex] = fmt.Sprintf("theme: %s", newTheme)
	} else {
		// Add theme line after opening ---
		newLines := make([]string, 0, len(lines)+1)
		newLines = append(newLines, lines[0])
		newLines = append(newLines, fmt.Sprintf("theme: %s", newTheme))
		newLines = append(newLines, lines[1:]...)
		lines = newLines
	}

	newContent := strings.Join(lines, "\n")
	return os.WriteFile(path, []byte(newContent), 0644)
}

// ResolveCustomThemePath resolves the customTheme path relative to the given base directory.
// If the customTheme is already an absolute path or empty, it returns it unchanged.
// Returns the resolved path and any error encountered while checking the file.
func (c *Config) ResolveCustomThemePath(baseDir string) (string, error) {
	if c.CustomTheme == "" {
		return "", nil
	}

	// If it's already an absolute path, use it directly
	if filepath.IsAbs(c.CustomTheme) {
		if _, err := os.Stat(c.CustomTheme); os.IsNotExist(err) {
			return "", fmt.Errorf("custom theme file not found: %s", c.CustomTheme)
		}
		return c.CustomTheme, nil
	}

	// Resolve relative to base directory
	resolved := filepath.Join(baseDir, c.CustomTheme)
	resolved = filepath.Clean(resolved)

	// Check if file exists
	if _, err := os.Stat(resolved); os.IsNotExist(err) {
		return "", fmt.Errorf("custom theme file not found: %s (resolved from %s)", resolved, c.CustomTheme)
	}

	return resolved, nil
}
