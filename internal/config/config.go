// Package config handles presentation configuration from YAML frontmatter.
package config

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
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
	CodeTheme   string                  `yaml:"codeTheme" json:"codeTheme,omitempty"`
	Fragments   bool                    `yaml:"fragments" json:"fragments,omitempty"`
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

// Load reads a markdown file and parses its YAML frontmatter into a Config.
// The frontmatter is expected to be enclosed between "---" delimiters at the
// start of the file.
func Load(path string) (*Config, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)

	// Check for frontmatter start delimiter
	if !scanner.Scan() {
		return nil, fmt.Errorf("empty file")
	}

	firstLine := strings.TrimSpace(scanner.Text())
	if firstLine != "---" {
		// No frontmatter, return default config
		return DefaultConfig(), nil
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
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	if !foundEnd {
		return nil, fmt.Errorf("frontmatter not closed: missing closing ---")
	}

	// Parse YAML frontmatter
	cfg := DefaultConfig()
	if err := yaml.Unmarshal([]byte(frontmatter.String()), cfg); err != nil {
		return nil, fmt.Errorf("failed to parse frontmatter: %w", err)
	}

	// Load .env file from presentation directory
	dir := filepath.Dir(path)
	if err := LoadEnv(dir); err != nil {
		return nil, fmt.Errorf("failed to load .env file: %w", err)
	}

	// Resolve environment variables in sensitive fields
	cfg.ResolveEnvVars()

	return cfg, nil
}

// DefaultConfig returns a Config with sensible default values.
func DefaultConfig() *Config {
	return &Config{
		Theme:       "paper",
		AspectRatio: "16:9",
		Transition:  "fade",
		CodeTheme:   "github-dark",
		Fragments:   true,
		Drivers:     make(map[string]DriverConfig),
	}
}

// validAspectRatios contains the allowed aspect ratio values.
var validAspectRatios = map[string]bool{
	"16:9":  true,
	"4:3":   true,
	"16:10": true,
}

// validTransitions contains the allowed transition values.
var validTransitions = map[string]bool{
	"none":  true,
	"fade":  true,
	"slide": true,
	"push":  true,
	"zoom":  true,
}

// validThemes contains the allowed theme values.
var validThemes = map[string]bool{
	"paper":    true,
	"noir":     true,
	"aurora":   true,
	"phosphor": true,
	"poster":   true,
}

// legacyThemeMapping maps old theme names to new theme names for backwards compatibility.
var legacyThemeMapping = map[string]string{
	"minimal":   "paper",
	"keynote":   "noir",
	"gradient":  "aurora",
	"terminal":  "phosphor",
	"brutalist": "poster",
}

// validThemeColorKeys contains the allowed themeColors keys.
var validThemeColorKeys = map[string]bool{
	"background": true, // maps to --color-bg
	"text":       true, // maps to --color-text
	"muted":      true, // maps to --color-muted
	"accent":     true, // maps to --color-accent
	"codeBg":     true, // maps to --color-code-bg
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
// It also normalizes legacy theme names to their new equivalents.
func (c *Config) Validate() error {
	// Validate and normalize theme
	if c.Theme != "" {
		normalized := NormalizeTheme(c.Theme)
		if normalized == "" {
			return fmt.Errorf("invalid theme %q: must be one of paper, noir, aurora, phosphor, or poster", c.Theme)
		}
		c.Theme = normalized
	}

	// Validate aspect ratio
	if c.AspectRatio != "" && !validAspectRatios[c.AspectRatio] {
		return fmt.Errorf("invalid aspectRatio %q: must be one of 16:9, 4:3, or 16:10", c.AspectRatio)
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

// NormalizeTheme converts legacy theme names to new theme names and returns the normalized theme.
// If the theme is a legacy name, it logs a deprecation warning and returns the new name.
// If the theme is invalid, it returns an empty string.
func NormalizeTheme(theme string) string {
	// Check if it's already a valid new theme
	if validThemes[theme] {
		return theme
	}

	// Check if it's a legacy theme name
	if newName, ok := legacyThemeMapping[theme]; ok {
		log.Printf("Warning: theme %q is deprecated, please use %q instead", theme, newName)
		return newName
	}

	// Invalid theme
	return ""
}

// ValidThemeNames returns the list of valid theme names.
func ValidThemeNames() []string {
	return []string{"paper", "noir", "aurora", "phosphor", "poster"}
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
