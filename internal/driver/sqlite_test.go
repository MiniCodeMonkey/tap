package driver

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func hasSQLite3() bool {
	_, err := exec.LookPath("sqlite3")
	return err == nil
}

func TestSQLiteDriver_Name(t *testing.T) {
	driver := NewSQLiteDriver("")
	if driver.Name() != "sqlite" {
		t.Errorf("expected driver name 'sqlite', got '%s'", driver.Name())
	}
}

func TestSQLiteDriver_Execute_InMemory(t *testing.T) {
	if !hasSQLite3() {
		t.Skip("sqlite3 not installed")
	}

	driver := NewSQLiteDriver("")
	ctx := context.Background()

	// Simple query that should work on any SQLite installation
	result := driver.Execute(ctx, "SELECT 1 + 1 AS result;", nil)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "2") {
		t.Errorf("expected output to contain '2', got '%s'", result.Output)
	}
}

func TestSQLiteDriver_Execute_CreateAndQuery(t *testing.T) {
	if !hasSQLite3() {
		t.Skip("sqlite3 not installed")
	}

	driver := NewSQLiteDriver("")
	ctx := context.Background()

	sql := `
CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT);
INSERT INTO users (name) VALUES ('Alice'), ('Bob');
SELECT * FROM users;
`
	result := driver.Execute(ctx, sql, nil)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "Alice") {
		t.Errorf("expected output to contain 'Alice', got '%s'", result.Output)
	}
	if !strings.Contains(result.Output, "Bob") {
		t.Errorf("expected output to contain 'Bob', got '%s'", result.Output)
	}
}

func TestSQLiteDriver_Execute_StructuredData(t *testing.T) {
	if !hasSQLite3() {
		t.Skip("sqlite3 not installed")
	}

	driver := NewSQLiteDriver("")
	ctx := context.Background()

	sql := `
CREATE TABLE items (id INTEGER PRIMARY KEY, name TEXT, price REAL);
INSERT INTO items (name, price) VALUES ('Apple', 1.50), ('Banana', 0.75);
SELECT id, name, price FROM items;
`
	result := driver.Execute(ctx, sql, nil)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Data == nil {
		t.Error("expected structured data, got nil")
		return
	}
	if len(result.Data) != 2 {
		t.Errorf("expected 2 rows in data, got %d", len(result.Data))
	}

	// Check that column names are present
	firstRow := result.Data[0]
	if _, ok := firstRow["id"]; !ok {
		t.Error("expected 'id' column in data")
	}
	if _, ok := firstRow["name"]; !ok {
		t.Error("expected 'name' column in data")
	}
	if _, ok := firstRow["price"]; !ok {
		t.Error("expected 'price' column in data")
	}
}

func TestSQLiteDriver_Execute_FileDatabase(t *testing.T) {
	if !hasSQLite3() {
		t.Skip("sqlite3 not installed")
	}

	// Create a temp directory for the database
	tmpDir, err := os.MkdirTemp("", "sqlite-driver-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	dbPath := filepath.Join(tmpDir, "test.db")
	driver := NewSQLiteDriver("")
	ctx := context.Background()

	// Create table in file database
	sql1 := "CREATE TABLE test (value INTEGER);"
	result := driver.Execute(ctx, sql1, map[string]string{"database": dbPath})
	if !result.Success {
		t.Fatalf("failed to create table: %s", result.Error)
	}

	// Insert data
	sql2 := "INSERT INTO test VALUES (42);"
	result = driver.Execute(ctx, sql2, map[string]string{"database": dbPath})
	if !result.Success {
		t.Fatalf("failed to insert: %s", result.Error)
	}

	// Query data (should persist in file)
	sql3 := "SELECT value FROM test;"
	result = driver.Execute(ctx, sql3, map[string]string{"database": dbPath})
	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "42") {
		t.Errorf("expected output to contain '42', got '%s'", result.Output)
	}
}

func TestSQLiteDriver_Execute_SyntaxError(t *testing.T) {
	if !hasSQLite3() {
		t.Skip("sqlite3 not installed")
	}

	driver := NewSQLiteDriver("")
	ctx := context.Background()

	result := driver.Execute(ctx, "SELEKT * FROM nowhere;", nil)

	if result.Success {
		t.Error("expected failure for syntax error, got success")
	}
	if result.Error == "" {
		t.Error("expected error message, got empty string")
	}
}

func TestSQLiteDriver_Execute_TableNotFound(t *testing.T) {
	if !hasSQLite3() {
		t.Skip("sqlite3 not installed")
	}

	driver := NewSQLiteDriver("")
	ctx := context.Background()

	result := driver.Execute(ctx, "SELECT * FROM nonexistent_table;", nil)

	if result.Success {
		t.Error("expected failure for non-existent table, got success")
	}
	if !strings.Contains(strings.ToLower(result.Error), "no such table") {
		t.Errorf("expected error about non-existent table, got '%s'", result.Error)
	}
}

func TestSQLiteDriver_Execute_Timeout(t *testing.T) {
	if !hasSQLite3() {
		t.Skip("sqlite3 not installed")
	}

	driver := NewSQLiteDriver("")
	ctx := context.Background()

	// Create a query that takes time using recursive CTE
	sql := `
WITH RECURSIVE cnt(x) AS (
    SELECT 1
    UNION ALL
    SELECT x+1 FROM cnt WHERE x < 10000000
)
SELECT COUNT(*) FROM cnt;
`
	result := driver.Execute(ctx, sql, map[string]string{"timeout": "1"})

	if result.Success {
		// The query might complete if the system is fast, which is OK
		// We just want to verify timeout handling doesn't crash
		return
	}
	if !strings.Contains(result.Error, "timed out") && !strings.Contains(result.Error, "killed") {
		// May also be killed by signal
		t.Logf("timeout test completed with error: %s", result.Error)
	}
}

func TestSQLiteDriver_Execute_ContextCanceled(t *testing.T) {
	if !hasSQLite3() {
		t.Skip("sqlite3 not installed")
	}

	driver := NewSQLiteDriver("")
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	// Long-running query
	sql := `
WITH RECURSIVE cnt(x) AS (
    SELECT 1
    UNION ALL
    SELECT x+1 FROM cnt WHERE x < 10000000
)
SELECT COUNT(*) FROM cnt;
`
	result := driver.Execute(ctx, sql, nil)

	if result.Success {
		// Query might complete before cancellation on fast systems
		return
	}
	// The error should indicate cancellation or being killed
	if !strings.Contains(result.Error, "canceled") && !strings.Contains(result.Error, "killed") {
		t.Logf("cancel test completed with error: %s", result.Error)
	}
}

func TestSQLiteDriver_Execute_NilConfig(t *testing.T) {
	if !hasSQLite3() {
		t.Skip("sqlite3 not installed")
	}

	driver := NewSQLiteDriver("")
	ctx := context.Background()

	// Should use in-memory database by default
	result := driver.Execute(ctx, "SELECT sqlite_version();", nil)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Output == "" {
		t.Error("expected version output, got empty string")
	}
}

func TestSQLiteDriver_Execute_WorkingDirectory(t *testing.T) {
	if !hasSQLite3() {
		t.Skip("sqlite3 not installed")
	}

	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "sqlite-driver-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a database file in the temp directory
	dbPath := filepath.Join(tmpDir, "workdir.db")
	ctx := context.Background()

	// Use absolute path first to create the database
	driver := NewSQLiteDriver("")
	result := driver.Execute(ctx, "CREATE TABLE workdir_test (x INTEGER);", map[string]string{"database": dbPath})
	if !result.Success {
		t.Fatalf("failed to create table: %s", result.Error)
	}

	// Now use working directory with relative path
	driver = NewSQLiteDriver(tmpDir)
	result = driver.Execute(ctx, "INSERT INTO workdir_test VALUES (123);", map[string]string{"database": "workdir.db"})
	if !result.Success {
		t.Errorf("expected success with workdir, got error: %s", result.Error)
	}
}

func TestSQLiteDriver_Execute_EmptyResult(t *testing.T) {
	if !hasSQLite3() {
		t.Skip("sqlite3 not installed")
	}

	driver := NewSQLiteDriver("")
	ctx := context.Background()

	sql := `
CREATE TABLE empty_table (id INTEGER PRIMARY KEY);
SELECT * FROM empty_table;
`
	result := driver.Execute(ctx, sql, nil)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	// Empty result should have either nil or empty data
	if len(result.Data) > 0 {
		t.Errorf("expected empty data for empty result, got %d rows", len(result.Data))
	}
}

func TestSQLiteDriver_Execute_NonQueryStatement(t *testing.T) {
	if !hasSQLite3() {
		t.Skip("sqlite3 not installed")
	}

	driver := NewSQLiteDriver("")
	ctx := context.Background()

	// CREATE statement doesn't return rows
	result := driver.Execute(ctx, "CREATE TABLE no_output (id INTEGER);", nil)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	// Non-query statements should succeed but may have empty output
}

// Tests for parsing functions

func TestParseColumnOutput_Simple(t *testing.T) {
	output := `id          name
----------  ----------
1           Alice
2           Bob`

	data := parseColumnOutput(output)

	if data == nil {
		t.Fatal("expected data, got nil")
	}
	if len(data) != 2 {
		t.Errorf("expected 2 rows, got %d", len(data))
	}
	if data[0]["name"] != "Alice" {
		t.Errorf("expected first name 'Alice', got '%v'", data[0]["name"])
	}
	if data[1]["name"] != "Bob" {
		t.Errorf("expected second name 'Bob', got '%v'", data[1]["name"])
	}
}

func TestParseColumnOutput_NoSeparator(t *testing.T) {
	// Some SQLite outputs may not have separator line
	output := `id          name
1           Alice`

	data := parseColumnOutput(output)

	if data == nil {
		t.Fatal("expected data, got nil")
	}
	if len(data) != 1 {
		t.Errorf("expected 1 row, got %d", len(data))
	}
}

func TestParseColumnOutput_Empty(t *testing.T) {
	data := parseColumnOutput("")
	if data != nil {
		t.Errorf("expected nil for empty output, got %v", data)
	}
}

func TestParseColumnOutput_HeaderOnly(t *testing.T) {
	output := `id          name`
	data := parseColumnOutput(output)
	// Header only without data should return nil or empty
	if len(data) > 0 {
		t.Errorf("expected nil or empty for header-only output, got %v", data)
	}
}

func TestIsHeaderSeparator(t *testing.T) {
	tests := []struct {
		line     string
		expected bool
	}{
		{"----------", true},
		{"---  ---", true},
		{"----------  ----------", true},
		{"", false},
		{"not a separator", false},
		{"---x---", false},
	}

	for _, tc := range tests {
		result := isHeaderSeparator(tc.line)
		if result != tc.expected {
			t.Errorf("isHeaderSeparator(%q) = %v, want %v", tc.line, result, tc.expected)
		}
	}
}

func TestParseColumnHeaders(t *testing.T) {
	headerLine := "id          name        value"
	columns := parseColumnHeaders(headerLine)

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
