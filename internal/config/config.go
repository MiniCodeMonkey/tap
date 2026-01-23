// Package config handles presentation configuration from YAML frontmatter.
package config

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/joho/godotenv"
	"gopkg.in/yaml.v3"
)

// Config represents the presentation configuration from YAML frontmatter.
type Config struct {
	Drivers     map[string]DriverConfig `yaml:"drivers"`
	Title       string                  `yaml:"title"`
	Theme       string                  `yaml:"theme"`
	Author      string                  `yaml:"author"`
	Date        string                  `yaml:"date"`
	AspectRatio string                  `yaml:"aspectRatio"`
	Transition  string                  `yaml:"transition"`
	CodeTheme   string                  `yaml:"codeTheme"`
	Fragments   bool                    `yaml:"fragments"`
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
		Theme:       "minimal",
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

// Validate checks the Config for invalid values and returns an error
// with a descriptive message if validation fails.
func (c *Config) Validate() error {
	// Validate aspect ratio
	if c.AspectRatio != "" && !validAspectRatios[c.AspectRatio] {
		return fmt.Errorf("invalid aspectRatio %q: must be one of 16:9, 4:3, or 16:10", c.AspectRatio)
	}

	// Validate transition
	if c.Transition != "" && !validTransitions[c.Transition] {
		return fmt.Errorf("invalid transition %q: must be one of none, fade, slide, push, or zoom", c.Transition)
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
