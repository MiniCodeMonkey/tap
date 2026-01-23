package driver

import (
	"bytes"
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// DefaultSQLiteTimeout is the default timeout for SQLite query execution.
const DefaultSQLiteTimeout = 30 * time.Second

// SQLiteDriver executes SQL queries using the sqlite3 CLI.
type SQLiteDriver struct {
	// WorkingDir is the directory used for resolving relative database paths.
	// If empty, the current working directory is used.
	WorkingDir string
}

// NewSQLiteDriver creates a new SQLiteDriver with the specified working directory.
func NewSQLiteDriver(workingDir string) *SQLiteDriver {
	return &SQLiteDriver{
		WorkingDir: workingDir,
	}
}

// Name returns the driver identifier.
func (d *SQLiteDriver) Name() string {
	return "sqlite"
}

// Execute runs the provided SQL query against a SQLite database.
// The config map supports the following keys:
//   - database: path to the SQLite database file, or ":memory:" for in-memory (default: ":memory:")
//   - timeout: execution timeout in seconds (default: 30)
//   - workdir: override the working directory for resolving relative database paths
func (d *SQLiteDriver) Execute(ctx context.Context, code string, config map[string]string) Result {
	// Determine timeout from config or use default
	timeout := DefaultSQLiteTimeout
	if timeoutStr, ok := config["timeout"]; ok {
		if seconds, err := strconv.Atoi(timeoutStr); err == nil && seconds > 0 {
			timeout = time.Duration(seconds) * time.Second
		}
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Get database path (default to in-memory)
	database := ":memory:"
	if dbPath, ok := config["database"]; ok && dbPath != "" {
		database = dbPath
	}

	// Build sqlite3 command with table-formatted output
	// -header shows column names, -column formats as aligned columns
	cmd := exec.CommandContext(ctx, "sqlite3", "-header", "-column", database)

	// Set working directory
	workDir := d.WorkingDir
	if configWorkDir, ok := config["workdir"]; ok && configWorkDir != "" {
		workDir = configWorkDir
	}
	if workDir != "" {
		cmd.Dir = workDir
	}

	// Pass SQL code via stdin
	cmd.Stdin = strings.NewReader(code)

	// Capture stdout and stderr
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run the command
	err := cmd.Run()

	// Build result
	result := Result{
		Output:  strings.TrimSuffix(stdout.String(), "\n"),
		Success: true,
	}

	// Parse output into structured data for table rendering
	if result.Success && result.Output != "" {
		result.Data = parseColumnOutput(result.Output)
	}

	// Handle errors
	if err != nil {
		result.Success = false

		// Check for context deadline exceeded
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = "query execution timed out"
			return result
		}

		// Check for context canceled
		if ctx.Err() == context.Canceled {
			result.Error = "query execution canceled"
			return result
		}

		// Get error from stderr or command failure
		stderrStr := strings.TrimSuffix(stderr.String(), "\n")
		if stderrStr != "" {
			result.Error = stderrStr
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			result.Error = "sqlite3 exited with code " + strconv.Itoa(exitErr.ExitCode())
		} else {
			result.Error = err.Error()
		}
	}

	return result
}

// parseColumnOutput parses sqlite3 -header -column output into structured data.
// Returns nil if the output doesn't contain valid tabular data.
func parseColumnOutput(output string) []map[string]interface{} {
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		// Need at least header and one data row
		return nil
	}

	// Parse header line to get column names and positions
	headerLine := lines[0]
	columns := parseColumnHeaders(headerLine)
	if len(columns) == 0 {
		return nil
	}

	// Skip the separator line (dashes) if present
	dataStartIdx := 1
	if len(lines) > 1 && isHeaderSeparator(lines[1]) {
		dataStartIdx = 2
	}

	// Parse data rows
	var data []map[string]interface{}
	for i := dataStartIdx; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			continue
		}

		row := parseColumnRow(line, columns)
		if row != nil {
			data = append(data, row)
		}
	}

	return data
}

// columnInfo holds column name and position information.
type columnInfo struct {
	name  string
	start int
	end   int
}

// parseColumnHeaders extracts column names and their positions from the header line.
func parseColumnHeaders(headerLine string) []columnInfo {
	var columns []columnInfo

	// Find column boundaries by looking for sequences of non-space characters
	inColumn := false
	startPos := 0
	var currentName strings.Builder

	for i, ch := range headerLine {
		if ch == ' ' {
			if inColumn {
				// End of column name
				columns = append(columns, columnInfo{
					name:  currentName.String(),
					start: startPos,
					end:   i,
				})
				currentName.Reset()
				inColumn = false
			}
		} else {
			if !inColumn {
				// Start of column name
				startPos = i
				inColumn = true
			}
			currentName.WriteRune(ch)
		}
	}

	// Handle last column
	if inColumn && currentName.Len() > 0 {
		columns = append(columns, columnInfo{
			name:  currentName.String(),
			start: startPos,
			end:   len(headerLine),
		})
	}

	// Set the end of the last column to -1 to indicate "rest of line"
	if len(columns) > 0 {
		columns[len(columns)-1].end = -1
	}

	return columns
}

// isHeaderSeparator checks if a line consists of dashes and spaces (separator line).
func isHeaderSeparator(line string) bool {
	if line == "" {
		return false
	}
	for _, ch := range line {
		if ch != '-' && ch != ' ' {
			return false
		}
	}
	return true
}

// parseColumnRow extracts values from a data row based on column positions.
func parseColumnRow(line string, columns []columnInfo) map[string]interface{} {
	if len(columns) == 0 {
		return nil
	}

	row := make(map[string]interface{})

	for i, col := range columns {
		var value string

		// Handle last column (extends to end of line)
		if col.end == -1 || i == len(columns)-1 {
			if col.start < len(line) {
				value = strings.TrimSpace(line[col.start:])
			}
		} else if col.start < len(line) {
			endPos := col.end
			if endPos > len(line) {
				endPos = len(line)
			}
			value = strings.TrimSpace(line[col.start:endPos])
		}

		row[col.name] = value
	}

	return row
}
