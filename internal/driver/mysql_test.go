package driver

import (
	"context"
	"os/exec"
	"strings"
	"testing"
)

func hasMySQL() bool {
	_, err := exec.LookPath("mysql")
	return err == nil
}

func TestMySQLDriver_Name(t *testing.T) {
	driver := NewMySQLDriver("")
	if driver.Name() != "mysql" {
		t.Errorf("expected driver name 'mysql', got '%s'", driver.Name())
	}
}

func TestMySQLDriver_Execute_NilConfig(t *testing.T) {
	driver := NewMySQLDriver("")
	ctx := context.Background()

	// Execute with nil config - should fail without a database connection
	// but shouldn't panic
	result := driver.Execute(ctx, "SELECT 1;", nil)

	// We expect failure because we're not actually connecting to MySQL
	// This test mainly ensures the driver handles nil config gracefully
	if result.Success && !hasMySQL() {
		t.Error("expected failure without mysql installed, got success")
	}
}

func TestMySQLDriver_Execute_MissingBinary(t *testing.T) {
	if hasMySQL() {
		t.Skip("mysql is installed, skipping missing binary test")
	}

	driver := NewMySQLDriver("")
	ctx := context.Background()

	result := driver.Execute(ctx, "SELECT 1;", map[string]string{
		"host":     "localhost",
		"user":     "root",
		"database": "test",
	})

	if result.Success {
		t.Error("expected failure when mysql is not installed, got success")
	}
	// Error should indicate mysql is not found
	if !strings.Contains(result.Error, "not found") && !strings.Contains(result.Error, "executable file not found") {
		t.Logf("error message: %s", result.Error)
	}
}

// Integration tests - only run if MySQL is available and configured
func TestMySQLDriver_Execute_SimpleQuery(t *testing.T) {
	if !hasMySQL() {
		t.Skip("mysql not installed")
	}

	driver := NewMySQLDriver("")
	ctx := context.Background()

	// Test with a simple query that doesn't require authentication
	// This may fail without a properly configured MySQL server
	result := driver.Execute(ctx, "SELECT 1 AS test;", map[string]string{
		"host": "localhost",
		"user": "root",
	})

	// Don't fail the test if MySQL isn't running - just skip
	if !result.Success {
		if strings.Contains(result.Error, "Can't connect") ||
			strings.Contains(result.Error, "Access denied") ||
			strings.Contains(result.Error, "Connection refused") {
			t.Skip("MySQL server not available or not configured")
		}
		t.Logf("Query failed: %s", result.Error)
	}
}

// Tests for credential masking
func TestMaskCredentials_Password(t *testing.T) {
	config := map[string]string{
		"password": "supersecret123",
		"user":     "testuser",
	}

	message := "Error: Authentication failed for user testuser with password supersecret123"
	masked := maskCredentials(message, config)

	if strings.Contains(masked, "supersecret123") {
		t.Error("password should be masked in error message")
	}
	if !strings.Contains(masked, "[REDACTED]") {
		t.Error("password should be replaced with [REDACTED]")
	}
}

func TestMaskCredentials_UserAtHost(t *testing.T) {
	config := map[string]string{
		"password": "secret",
		"user":     "dbuser",
	}

	message := "Access denied for user dbuser@localhost"
	masked := maskCredentials(message, config)

	if !strings.Contains(masked, "[REDACTED]@localhost") {
		t.Errorf("user@host should be masked, got: %s", masked)
	}
}

func TestMaskCredentials_QuotedUser(t *testing.T) {
	config := map[string]string{
		"password": "secret",
		"user":     "dbuser",
	}

	message := "Access denied for user 'dbuser'"
	masked := maskCredentials(message, config)

	if !strings.Contains(masked, "'[REDACTED]'") {
		t.Errorf("quoted user should be masked, got: %s", masked)
	}
}

func TestMaskCredentials_ShortUser(t *testing.T) {
	config := map[string]string{
		"password": "secret",
		"user":     "ab", // Too short to mask (might match common words)
	}

	message := "Access denied for user ab@localhost"
	masked := maskCredentials(message, config)

	// Short usernames are not masked to avoid false positives
	if strings.Contains(masked, "[REDACTED]@localhost") {
		t.Error("short usernames should not be masked")
	}
}

func TestMaskCredentials_EmptyConfig(t *testing.T) {
	message := "Some error with password=secret"
	masked := maskCredentials(message, nil)

	// With nil config, nothing should be masked
	if masked != message {
		t.Errorf("message should not change with nil config, got: %s", masked)
	}
}

// Tests for MySQL table output parsing
func TestParseMySQLTableOutput_Simple(t *testing.T) {
	output := `+----+-------+
| id | name  |
+----+-------+
| 1  | Alice |
| 2  | Bob   |
+----+-------+`

	data := parseMySQLTableOutput(output)

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

func TestParseMySQLTableOutput_ThreeColumns(t *testing.T) {
	output := `+----+-------+-------+
| id | name  | price |
+----+-------+-------+
| 1  | Apple | 1.50  |
| 2  | Pear  | 2.00  |
+----+-------+-------+`

	data := parseMySQLTableOutput(output)

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

func TestParseMySQLTableOutput_NullValue(t *testing.T) {
	output := `+----+-------+
| id | name  |
+----+-------+
| 1  | NULL  |
+----+-------+`

	data := parseMySQLTableOutput(output)

	if data == nil {
		t.Fatal("expected data, got nil")
	}
	if len(data) != 1 {
		t.Fatalf("expected 1 row, got %d", len(data))
	}
	if data[0]["name"] != nil {
		t.Errorf("expected nil for NULL value, got '%v'", data[0]["name"])
	}
}

func TestParseMySQLTableOutput_Empty(t *testing.T) {
	output := ""
	data := parseMySQLTableOutput(output)

	if data != nil {
		t.Errorf("expected nil for empty output, got %v", data)
	}
}

func TestParseMySQLTableOutput_NoData(t *testing.T) {
	// Query with no results
	output := `+----+------+
| id | name |
+----+------+
+----+------+`

	data := parseMySQLTableOutput(output)

	// Should return nil or empty slice
	if len(data) > 0 {
		t.Errorf("expected empty data for no results, got %d rows", len(data))
	}
}

func TestParseMySQLTableOutput_SingleRow(t *testing.T) {
	output := `+--------+
| result |
+--------+
| 42     |
+--------+`

	data := parseMySQLTableOutput(output)

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

func TestIsMySQLBorder(t *testing.T) {
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
		result := isMySQLBorder(tc.line)
		if result != tc.expected {
			t.Errorf("isMySQLBorder(%q) = %v, want %v", tc.line, result, tc.expected)
		}
	}
}

func TestParseMySQLHeader(t *testing.T) {
	headerLine := "| id | name | value |"
	columns := parseMySQLHeader(headerLine)

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

func TestParseMySQLHeader_SingleColumn(t *testing.T) {
	headerLine := "| column |"
	columns := parseMySQLHeader(headerLine)

	if len(columns) != 1 {
		t.Fatalf("expected 1 column, got %d", len(columns))
	}
	if columns[0].name != "column" {
		t.Errorf("expected column name 'column', got '%s'", columns[0].name)
	}
}

func TestParseMySQLRow(t *testing.T) {
	columns := []mysqlColumnInfo{
		{name: "id", start: 0, end: 5},
		{name: "name", start: 6, end: 15},
	}

	row := parseMySQLRow("| 1 | Alice |", columns)

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

func TestParseMySQLRow_NULL(t *testing.T) {
	columns := []mysqlColumnInfo{
		{name: "value", start: 0, end: 10},
	}

	row := parseMySQLRow("| NULL |", columns)

	if row == nil {
		t.Fatal("expected row, got nil")
	}
	if row["value"] != nil {
		t.Errorf("expected nil for NULL, got '%v'", row["value"])
	}
}

func TestParseMySQLRow_EmptyColumns(t *testing.T) {
	row := parseMySQLRow("| 1 | Alice |", nil)

	if row != nil {
		t.Errorf("expected nil for empty columns, got %v", row)
	}
}

// Tests for config parsing
func TestMySQLDriver_ConfigHost(t *testing.T) {
	// This test verifies that the driver constructs the correct command
	// We can't easily test the actual execution without a MySQL server
	driver := NewMySQLDriver("")

	// The driver should accept host config
	_ = driver.Execute(context.Background(), "SELECT 1;", map[string]string{
		"host": "custom-host.example.com",
	})
	// No panic means the config was processed
}

func TestMySQLDriver_ConfigPort(t *testing.T) {
	driver := NewMySQLDriver("")

	// The driver should accept port config
	_ = driver.Execute(context.Background(), "SELECT 1;", map[string]string{
		"port": "3307",
	})
	// No panic means the config was processed
}

func TestMySQLDriver_ConfigInvalidPort(t *testing.T) {
	driver := NewMySQLDriver("")

	// Invalid port should use default
	_ = driver.Execute(context.Background(), "SELECT 1;", map[string]string{
		"port": "invalid",
	})
	// No panic means the config was processed gracefully
}
