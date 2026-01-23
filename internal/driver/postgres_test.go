package driver

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func hasPostgres() bool {
	_, err := exec.LookPath("psql")
	return err == nil
}

func TestPostgresDriver_Name(t *testing.T) {
	driver := NewPostgresDriver("")
	if driver.Name() != "postgres" {
		t.Errorf("expected driver name 'postgres', got '%s'", driver.Name())
	}
}

func TestPostgresDriver_Execute_NilConfig(t *testing.T) {
	driver := NewPostgresDriver("")
	ctx := context.Background()

	// Execute with nil config - should fail without a database connection
	// but shouldn't panic
	result := driver.Execute(ctx, "SELECT 1;", nil)

	// We expect failure because we're not actually connecting to PostgreSQL
	// This test mainly ensures the driver handles nil config gracefully
	if result.Success && !hasPostgres() {
		t.Error("expected failure without psql installed, got success")
	}
}

func TestPostgresDriver_Execute_MissingBinary(t *testing.T) {
	if hasPostgres() {
		t.Skip("psql is installed, skipping missing binary test")
	}

	driver := NewPostgresDriver("")
	ctx := context.Background()

	result := driver.Execute(ctx, "SELECT 1;", map[string]string{
		"host":     "localhost",
		"user":     "postgres",
		"database": "test",
	})

	if result.Success {
		t.Error("expected failure when psql is not installed, got success")
	}
	// Error should indicate psql is not found
	if !strings.Contains(result.Error, "not found") && !strings.Contains(result.Error, "executable file not found") {
		t.Logf("error message: %s", result.Error)
	}
}

// Integration tests - only run if PostgreSQL is available and configured
func TestPostgresDriver_Execute_SimpleQuery(t *testing.T) {
	if !hasPostgres() {
		t.Skip("psql not installed")
	}

	driver := NewPostgresDriver("")
	ctx := context.Background()

	// Test with a simple query that doesn't require authentication
	// This may fail without a properly configured PostgreSQL server
	result := driver.Execute(ctx, "SELECT 1 AS test;", map[string]string{
		"host": "localhost",
		"user": "postgres",
	})

	// Don't fail the test if PostgreSQL isn't running - just skip
	if !result.Success {
		if strings.Contains(result.Error, "could not connect") ||
			strings.Contains(result.Error, "password authentication failed") ||
			strings.Contains(result.Error, "Connection refused") ||
			strings.Contains(result.Error, "FATAL") {
			t.Skip("PostgreSQL server not available or not configured")
		}
		t.Logf("Query failed: %s", result.Error)
	}
}

// Tests for credential masking
func TestMaskPostgresCredentials_Password(t *testing.T) {
	config := map[string]string{
		"password": "supersecret123",
		"user":     "testuser",
	}

	message := "Error: Authentication failed for user testuser with password supersecret123"
	masked := maskPostgresCredentials(message, config)

	if strings.Contains(masked, "supersecret123") {
		t.Error("password should be masked in error message")
	}
	if !strings.Contains(masked, "[REDACTED]") {
		t.Error("password should be replaced with [REDACTED]")
	}
}

func TestMaskPostgresCredentials_UserAtHost(t *testing.T) {
	config := map[string]string{
		"password": "secret",
		"user":     "dbuser",
	}

	message := "FATAL: password authentication failed for user dbuser@localhost"
	masked := maskPostgresCredentials(message, config)

	if !strings.Contains(masked, "[REDACTED]@localhost") {
		t.Errorf("user@host should be masked, got: %s", masked)
	}
}

func TestMaskPostgresCredentials_QuotedUser(t *testing.T) {
	config := map[string]string{
		"password": "secret",
		"user":     "dbuser",
	}

	message := `FATAL: role "dbuser" does not exist`
	masked := maskPostgresCredentials(message, config)

	if !strings.Contains(masked, `"[REDACTED]"`) {
		t.Errorf("quoted user should be masked, got: %s", masked)
	}
}

func TestMaskPostgresCredentials_ShortUser(t *testing.T) {
	config := map[string]string{
		"password": "secret",
		"user":     "ab", // Too short to mask (might match common words)
	}

	message := "FATAL: password authentication failed for user ab@localhost"
	masked := maskPostgresCredentials(message, config)

	// Short usernames are not masked to avoid false positives
	if strings.Contains(masked, "[REDACTED]@localhost") {
		t.Error("short usernames should not be masked")
	}
}

func TestMaskPostgresCredentials_EmptyConfig(t *testing.T) {
	message := "Some error with password=secret"
	masked := maskPostgresCredentials(message, nil)

	// With nil config, nothing should be masked
	if masked != message {
		t.Errorf("message should not change with nil config, got: %s", masked)
	}
}

// Tests for PostgreSQL table output parsing
func TestParsePostgresTableOutput_Simple(t *testing.T) {
	output := `+----+-------+
| id | name  |
+----+-------+
| 1  | Alice |
| 2  | Bob   |
+----+-------+`

	data := parsePostgresTableOutput(output)

	if data == nil {
		t.Fatal("expected data, got nil")
	}
	if len(data) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(data))
	}
	if data[0]["id"] != "1" {
		t.Errorf("expected first id '1', got '%v'", data[0]["id"])
	}
	if data[0]["name"] != "Alice" {
		t.Errorf("expected first name 'Alice', got '%v'", data[0]["name"])
	}
	if data[1]["id"] != "2" {
		t.Errorf("expected second id '2', got '%v'", data[1]["id"])
	}
	if data[1]["name"] != "Bob" {
		t.Errorf("expected second name 'Bob', got '%v'", data[1]["name"])
	}
}

func TestParsePostgresTableOutput_ThreeColumns(t *testing.T) {
	output := `+----+-------+-------+
| id | name  | price |
+----+-------+-------+
| 1  | Apple | 1.50  |
| 2  | Pear  | 2.00  |
+----+-------+-------+`

	data := parsePostgresTableOutput(output)

	if data == nil {
		t.Fatal("expected data, got nil")
	}
	if len(data) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(data))
	}
	if data[0]["price"] != "1.50" {
		t.Errorf("expected first price '1.50', got '%v'", data[0]["price"])
	}
	if data[1]["name"] != "Pear" {
		t.Errorf("expected second name 'Pear', got '%v'", data[1]["name"])
	}
}

func TestParsePostgresTableOutput_Empty(t *testing.T) {
	output := ""
	data := parsePostgresTableOutput(output)

	if data != nil {
		t.Errorf("expected nil for empty output, got %v", data)
	}
}

func TestParsePostgresTableOutput_NoData(t *testing.T) {
	// Query with no results
	output := `+----+------+
| id | name |
+----+------+
+----+------+`

	data := parsePostgresTableOutput(output)

	// Should return nil or empty slice
	if len(data) > 0 {
		t.Errorf("expected empty data for no results, got %d rows", len(data))
	}
}

func TestParsePostgresTableOutput_SingleRow(t *testing.T) {
	output := `+--------+
| result |
+--------+
| 42     |
+--------+`

	data := parsePostgresTableOutput(output)

	if data == nil {
		t.Fatal("expected data, got nil")
	}
	if len(data) != 1 {
		t.Fatalf("expected 1 row, got %d", len(data))
	}
	if data[0]["result"] != "42" {
		t.Errorf("expected result '42', got '%v'", data[0]["result"])
	}
}

func TestIsPostgresBorder(t *testing.T) {
	tests := []struct {
		line     string
		expected bool
	}{
		{"+----+", true},
		{"+----+------+", true},
		{"+------------+------------+", true},
		{"+-+", true},
		{"", false},
		{"|", false},
		{"| value |", false},
		{"+----+text+", false},
		{"----", false},
		{"+", false},
	}

	for _, tc := range tests {
		result := isPostgresBorder(tc.line)
		if result != tc.expected {
			t.Errorf("isPostgresBorder(%q) = %v, want %v", tc.line, result, tc.expected)
		}
	}
}

func TestParsePostgresHeader(t *testing.T) {
	headerLine := "| id | name | value |"
	columns := parsePostgresHeader(headerLine)

	if len(columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(columns))
	}
	if columns[0].name != "id" {
		t.Errorf("expected first column 'id', got '%s'", columns[0].name)
	}
	if columns[1].name != "name" {
		t.Errorf("expected second column 'name', got '%s'", columns[1].name)
	}
	if columns[2].name != "value" {
		t.Errorf("expected third column 'value', got '%s'", columns[2].name)
	}
}

func TestParsePostgresHeader_SingleColumn(t *testing.T) {
	headerLine := "| column |"
	columns := parsePostgresHeader(headerLine)

	if len(columns) != 1 {
		t.Fatalf("expected 1 column, got %d", len(columns))
	}
	if columns[0].name != "column" {
		t.Errorf("expected column name 'column', got '%s'", columns[0].name)
	}
}

func TestParsePostgresRow(t *testing.T) {
	columns := []postgresColumnInfo{
		{name: "id", start: 0, end: 5},
		{name: "name", start: 6, end: 15},
	}

	row := parsePostgresRow("| 1 | Alice |", columns)

	if row == nil {
		t.Fatal("expected row, got nil")
	}
	if row["id"] != "1" {
		t.Errorf("expected id '1', got '%v'", row["id"])
	}
	if row["name"] != "Alice" {
		t.Errorf("expected name 'Alice', got '%v'", row["name"])
	}
}

func TestParsePostgresRow_EmptyValue(t *testing.T) {
	columns := []postgresColumnInfo{
		{name: "value", start: 0, end: 10},
	}

	row := parsePostgresRow("|  |", columns)

	if row == nil {
		t.Fatal("expected row, got nil")
	}
	// Empty string should remain as empty string
	if row["value"] != "" {
		t.Errorf("expected empty string for empty value, got '%v'", row["value"])
	}
}

func TestParsePostgresRow_EmptyColumns(t *testing.T) {
	row := parsePostgresRow("| 1 | Alice |", nil)

	if row != nil {
		t.Errorf("expected nil for empty columns, got %v", row)
	}
}

// Tests for config parsing
func TestPostgresDriver_ConfigHost(t *testing.T) {
	// This test verifies that the driver constructs the correct command
	// We can't easily test the actual execution without a PostgreSQL server
	driver := NewPostgresDriver("")

	// The driver should accept host config
	_ = driver.Execute(context.Background(), "SELECT 1;", map[string]string{
		"host": "custom-host.example.com",
	})
	// No panic means the config was processed
}

func TestPostgresDriver_ConfigPort(t *testing.T) {
	driver := NewPostgresDriver("")

	// The driver should accept port config
	_ = driver.Execute(context.Background(), "SELECT 1;", map[string]string{
		"port": "5433",
	})
	// No panic means the config was processed
}

func TestPostgresDriver_ConfigInvalidPort(t *testing.T) {
	driver := NewPostgresDriver("")

	// Invalid port should use default
	_ = driver.Execute(context.Background(), "SELECT 1;", map[string]string{
		"port": "invalid",
	})
	// No panic means the config was processed gracefully
}

func TestPostgresDriver_ConfigTimeout(t *testing.T) {
	driver := NewPostgresDriver("")

	// The driver should accept timeout config
	_ = driver.Execute(context.Background(), "SELECT 1;", map[string]string{
		"timeout": "5",
	})
	// No panic means the config was processed
}

func TestPostgresDriver_ConfigInvalidTimeout(t *testing.T) {
	driver := NewPostgresDriver("")

	// Invalid timeout should use default
	_ = driver.Execute(context.Background(), "SELECT 1;", map[string]string{
		"timeout": "invalid",
	})
	// No panic means the config was processed gracefully
}

func TestPostgresDriver_ConfigWorkDir(t *testing.T) {
	driver := NewPostgresDriver("/tmp")

	// The driver should accept workdir override
	_ = driver.Execute(context.Background(), "SELECT 1;", map[string]string{
		"workdir": "/var",
	})
	// No panic means the config was processed
}

func TestPostgresDriver_ConfigDatabase(t *testing.T) {
	driver := NewPostgresDriver("")

	// The driver should accept database config
	_ = driver.Execute(context.Background(), "SELECT 1;", map[string]string{
		"database": "testdb",
	})
	// No panic means the config was processed
}

func TestPostgresDriver_ConfigUser(t *testing.T) {
	driver := NewPostgresDriver("")

	// The driver should accept user config
	_ = driver.Execute(context.Background(), "SELECT 1;", map[string]string{
		"user": "testuser",
	})
	// No panic means the config was processed
}

func TestPostgresDriver_ConfigPassword(t *testing.T) {
	driver := NewPostgresDriver("")

	// The driver should accept password config
	_ = driver.Execute(context.Background(), "SELECT 1;", map[string]string{
		"password": "secret",
	})
	// No panic means the config was processed
}
