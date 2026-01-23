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
