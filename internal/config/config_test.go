package config

import (
	"os"
	"strings"
	"testing"
)

func TestValidate_ValidAspectRatios(t *testing.T) {
	validRatios := []string{"16:9", "4:3", "16:10"}

	for _, ratio := range validRatios {
		cfg := DefaultConfig()
		cfg.AspectRatio = ratio

		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() returned error for valid aspectRatio %q: %v", ratio, err)
		}
	}
}

func TestValidate_InvalidAspectRatio(t *testing.T) {
	invalidRatios := []string{"16:10:1", "1:1", "21:9", "invalid", "16-9"}

	for _, ratio := range invalidRatios {
		cfg := DefaultConfig()
		cfg.AspectRatio = ratio

		err := cfg.Validate()
		if err == nil {
			t.Errorf("Validate() should return error for invalid aspectRatio %q", ratio)
			continue
		}

		if !strings.Contains(err.Error(), "aspectRatio") {
			t.Errorf("error message should mention aspectRatio, got: %v", err)
		}
		if !strings.Contains(err.Error(), ratio) {
			t.Errorf("error message should include the invalid value %q, got: %v", ratio, err)
		}
	}
}

func TestValidate_ValidTransitions(t *testing.T) {
	validTransitions := []string{"none", "fade", "slide", "push", "zoom"}

	for _, transition := range validTransitions {
		cfg := DefaultConfig()
		cfg.Transition = transition

		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() returned error for valid transition %q: %v", transition, err)
		}
	}
}

func TestValidate_InvalidTransition(t *testing.T) {
	invalidTransitions := []string{"dissolve", "wipe", "flip", "invalid", "FADE"}

	for _, transition := range invalidTransitions {
		cfg := DefaultConfig()
		cfg.Transition = transition

		err := cfg.Validate()
		if err == nil {
			t.Errorf("Validate() should return error for invalid transition %q", transition)
			continue
		}

		if !strings.Contains(err.Error(), "transition") {
			t.Errorf("error message should mention transition, got: %v", err)
		}
		if !strings.Contains(err.Error(), transition) {
			t.Errorf("error message should include the invalid value %q, got: %v", transition, err)
		}
	}
}

func TestValidate_EmptyValues(t *testing.T) {
	cfg := &Config{
		AspectRatio: "",
		Transition:  "",
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() should allow empty values (use defaults), got error: %v", err)
	}
}

func TestValidate_DefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if err := cfg.Validate(); err != nil {
		t.Errorf("DefaultConfig() should pass validation, got error: %v", err)
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	// When both aspectRatio and transition are invalid, the first error should be returned
	cfg := &Config{
		AspectRatio: "invalid-ratio",
		Transition:  "invalid-transition",
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() should return error for invalid config")
	}

	// First validation check is aspectRatio
	if !strings.Contains(err.Error(), "aspectRatio") {
		t.Errorf("error should mention aspectRatio first, got: %v", err)
	}
}

func TestResolveEnvVars_SimpleVariable(t *testing.T) {
	// Set up test environment variable
	os.Setenv("TEST_DB_PASSWORD", "secret123")
	defer os.Unsetenv("TEST_DB_PASSWORD")

	result := resolveEnvVars("$TEST_DB_PASSWORD")
	if result != "secret123" {
		t.Errorf("resolveEnvVars($TEST_DB_PASSWORD) = %q, want %q", result, "secret123")
	}
}

func TestResolveEnvVars_BracesSyntax(t *testing.T) {
	// Set up test environment variable
	os.Setenv("TEST_DB_USER", "admin")
	defer os.Unsetenv("TEST_DB_USER")

	result := resolveEnvVars("${TEST_DB_USER}")
	if result != "admin" {
		t.Errorf("resolveEnvVars(${TEST_DB_USER}) = %q, want %q", result, "admin")
	}
}

func TestResolveEnvVars_MixedContent(t *testing.T) {
	os.Setenv("TEST_HOST", "localhost")
	os.Setenv("TEST_PORT", "5432")
	defer os.Unsetenv("TEST_HOST")
	defer os.Unsetenv("TEST_PORT")

	result := resolveEnvVars("postgres://$TEST_HOST:${TEST_PORT}/mydb")
	expected := "postgres://localhost:5432/mydb"
	if result != expected {
		t.Errorf("resolveEnvVars() = %q, want %q", result, expected)
	}
}

func TestResolveEnvVars_UndefinedVariable(t *testing.T) {
	// Ensure variable is not set
	os.Unsetenv("UNDEFINED_VAR")

	result := resolveEnvVars("$UNDEFINED_VAR")
	if result != "$UNDEFINED_VAR" {
		t.Errorf("resolveEnvVars($UNDEFINED_VAR) = %q, want %q (unchanged)", result, "$UNDEFINED_VAR")
	}
}

func TestResolveEnvVars_NoVariables(t *testing.T) {
	input := "plain text without variables"
	result := resolveEnvVars(input)
	if result != input {
		t.Errorf("resolveEnvVars() = %q, want %q", result, input)
	}
}

func TestResolveEnvVars_EmptyString(t *testing.T) {
	result := resolveEnvVars("")
	if result != "" {
		t.Errorf("resolveEnvVars(\"\") = %q, want empty string", result)
	}
}

func TestConfig_ResolveEnvVars(t *testing.T) {
	// Set up test environment variables
	os.Setenv("TEST_MYSQL_PASSWORD", "mysql_secret")
	os.Setenv("TEST_MYSQL_USER", "mysql_admin")
	defer os.Unsetenv("TEST_MYSQL_PASSWORD")
	defer os.Unsetenv("TEST_MYSQL_USER")

	cfg := &Config{
		Drivers: map[string]DriverConfig{
			"mysql": {
				Connections: map[string]ConnectionConfig{
					"default": {
						Host:     "localhost",
						User:     "$TEST_MYSQL_USER",
						Password: "$TEST_MYSQL_PASSWORD",
						Database: "testdb",
						Port:     3306,
					},
				},
			},
		},
	}

	cfg.ResolveEnvVars()

	conn := cfg.Drivers["mysql"].Connections["default"]
	if conn.User != "mysql_admin" {
		t.Errorf("User = %q, want %q", conn.User, "mysql_admin")
	}
	if conn.Password != "mysql_secret" {
		t.Errorf("Password = %q, want %q", conn.Password, "mysql_secret")
	}
	// Non-variable fields should remain unchanged
	if conn.Host != "localhost" {
		t.Errorf("Host = %q, want %q", conn.Host, "localhost")
	}
}

func TestConfig_ResolveEnvVars_MultipleDrivers(t *testing.T) {
	os.Setenv("TEST_PG_PASSWORD", "pg_secret")
	os.Setenv("TEST_SQLITE_PATH", "/data/test.db")
	defer os.Unsetenv("TEST_PG_PASSWORD")
	defer os.Unsetenv("TEST_SQLITE_PATH")

	cfg := &Config{
		Drivers: map[string]DriverConfig{
			"postgres": {
				Connections: map[string]ConnectionConfig{
					"prod": {
						Password: "${TEST_PG_PASSWORD}",
					},
				},
			},
			"sqlite": {
				Connections: map[string]ConnectionConfig{
					"local": {
						Path: "$TEST_SQLITE_PATH",
					},
				},
			},
		},
	}

	cfg.ResolveEnvVars()

	pgConn := cfg.Drivers["postgres"].Connections["prod"]
	if pgConn.Password != "pg_secret" {
		t.Errorf("postgres password = %q, want %q", pgConn.Password, "pg_secret")
	}

	sqliteConn := cfg.Drivers["sqlite"].Connections["local"]
	if sqliteConn.Path != "/data/test.db" {
		t.Errorf("sqlite path = %q, want %q", sqliteConn.Path, "/data/test.db")
	}
}

func TestLoadEnv_NonexistentFile(t *testing.T) {
	// LoadEnv should return nil for non-existent .env file
	err := LoadEnv("/nonexistent/directory")
	if err != nil {
		t.Errorf("LoadEnv() should return nil for non-existent .env file, got: %v", err)
	}
}

func TestValidate_ValidThemeColorsKeys(t *testing.T) {
	validKeys := []string{"background", "text", "muted", "accent", "codeBg"}

	for _, key := range validKeys {
		cfg := DefaultConfig()
		cfg.ThemeColors = map[string]string{key: "#ff0000"}

		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() returned error for valid themeColors key %q: %v", key, err)
		}
	}
}

func TestValidate_InvalidThemeColorsKey(t *testing.T) {
	invalidKeys := []string{"color", "bg", "primary", "secondary", "invalid"}

	for _, key := range invalidKeys {
		cfg := DefaultConfig()
		cfg.ThemeColors = map[string]string{key: "#ff0000"}

		err := cfg.Validate()
		if err == nil {
			t.Errorf("Validate() should return error for invalid themeColors key %q", key)
			continue
		}

		if !strings.Contains(err.Error(), "themeColors") {
			t.Errorf("error message should mention themeColors, got: %v", err)
		}
		if !strings.Contains(err.Error(), key) {
			t.Errorf("error message should include the invalid key %q, got: %v", key, err)
		}
	}
}

func TestValidate_PartialThemeColors(t *testing.T) {
	// Partial overrides should work (only specify some colors)
	cfg := DefaultConfig()
	cfg.ThemeColors = map[string]string{
		"accent": "#ef4444",
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() should allow partial themeColors, got error: %v", err)
	}
}

func TestValidate_AllThemeColors(t *testing.T) {
	// All color keys together should work
	cfg := DefaultConfig()
	cfg.ThemeColors = map[string]string{
		"background": "#ffffff",
		"text":       "#0a0a0a",
		"muted":      "#71717a",
		"accent":     "#78716c",
		"codeBg":     "#1e1e1e",
	}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() should allow all themeColors keys, got error: %v", err)
	}
}

func TestValidate_EmptyThemeColors(t *testing.T) {
	// Empty themeColors map should be valid
	cfg := DefaultConfig()
	cfg.ThemeColors = map[string]string{}

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() should allow empty themeColors, got error: %v", err)
	}
}

func TestValidate_NilThemeColors(t *testing.T) {
	// Nil themeColors should be valid (default)
	cfg := DefaultConfig()

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() should allow nil themeColors, got error: %v", err)
	}
}

func TestIsValidColor_HexColors(t *testing.T) {
	validHexColors := []string{
		"#fff",     // 3-digit
		"#FFF",     // 3-digit uppercase
		"#ffffff",  // 6-digit
		"#FFFFFF",  // 6-digit uppercase
		"#fffa",    // 4-digit (with alpha)
		"#ffffffaa", // 8-digit (with alpha)
	}

	for _, color := range validHexColors {
		if !isValidColor(color) {
			t.Errorf("isValidColor(%q) = false, want true", color)
		}
	}
}

func TestIsValidColor_CSSFunctions(t *testing.T) {
	validFunctions := []string{
		"rgb(255, 0, 0)",
		"rgba(255, 0, 0, 0.5)",
		"hsl(0, 100%, 50%)",
		"hsla(0, 100%, 50%, 0.5)",
		"oklch(0.7 0.15 60)",
		"oklab(0.7 -0.1 0.1)",
	}

	for _, color := range validFunctions {
		if !isValidColor(color) {
			t.Errorf("isValidColor(%q) = false, want true", color)
		}
	}
}

func TestIsValidColor_NamedColors(t *testing.T) {
	namedColors := []string{
		"black", "white", "red", "green", "blue",
		"transparent", "currentColor", "inherit",
	}

	for _, color := range namedColors {
		if !isValidColor(color) {
			t.Errorf("isValidColor(%q) = false, want true", color)
		}
	}
}

func TestIsValidColor_InvalidColors(t *testing.T) {
	invalidColors := []string{
		"notacolor",
		"#gg0000",   // invalid hex chars
		"rgb",       // function without parens
		"",          // empty string
		"#f",        // 1 char - too short
		"#ff",       // 2 chars - too short
		"#fffff",    // 5 chars - invalid (only 3, 4, 6, 8 allowed)
		"#fffffff",  // 7 chars - invalid
		"#fffffffff", // 9 chars - too long
	}

	for _, color := range invalidColors {
		if isValidColor(color) {
			t.Errorf("isValidColor(%q) = true, want false", color)
		}
	}
}

func TestValidate_ValidThemes(t *testing.T) {
	validThemes := []string{"paper", "noir", "aurora", "phosphor", "poster"}

	for _, theme := range validThemes {
		cfg := DefaultConfig()
		cfg.Theme = theme

		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() returned error for valid theme %q: %v", theme, err)
		}
	}
}

func TestValidate_InvalidTheme(t *testing.T) {
	invalidThemes := []string{"invalid", "dark", "light", "custom", "PAPER"}

	for _, theme := range invalidThemes {
		cfg := DefaultConfig()
		cfg.Theme = theme

		err := cfg.Validate()
		if err == nil {
			t.Errorf("Validate() should return error for invalid theme %q", theme)
			continue
		}

		if !strings.Contains(err.Error(), "theme") {
			t.Errorf("error message should mention theme, got: %v", err)
		}
	}
}

func TestValidate_LegacyThemeNames(t *testing.T) {
	// Legacy themes should be normalized to new names (no error, just warning logged)
	legacyToNew := map[string]string{
		"minimal":   "paper",
		"keynote":   "noir",
		"gradient":  "aurora",
		"terminal":  "phosphor",
		"brutalist": "poster",
	}

	for legacy, expected := range legacyToNew {
		cfg := DefaultConfig()
		cfg.Theme = legacy

		if err := cfg.Validate(); err != nil {
			t.Errorf("Validate() should not return error for legacy theme %q: %v", legacy, err)
			continue
		}

		if cfg.Theme != expected {
			t.Errorf("Theme should be normalized from %q to %q, got %q", legacy, expected, cfg.Theme)
		}
	}
}

func TestNormalizeTheme(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// New themes
		{"paper", "paper"},
		{"noir", "noir"},
		{"aurora", "aurora"},
		{"phosphor", "phosphor"},
		{"poster", "poster"},
		// Legacy themes
		{"minimal", "paper"},
		{"keynote", "noir"},
		{"gradient", "aurora"},
		{"terminal", "phosphor"},
		{"brutalist", "poster"},
		// Invalid themes
		{"invalid", ""},
		{"dark", ""},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizeTheme(tt.input)
			if got != tt.expected {
				t.Errorf("NormalizeTheme(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestValidThemeNames(t *testing.T) {
	expected := []string{"paper", "noir", "aurora", "phosphor", "poster"}
	got := ValidThemeNames()

	if len(got) != len(expected) {
		t.Errorf("ValidThemeNames() returned %d themes, want %d", len(got), len(expected))
	}

	for i, name := range expected {
		if got[i] != name {
			t.Errorf("ValidThemeNames()[%d] = %q, want %q", i, got[i], name)
		}
	}
}
