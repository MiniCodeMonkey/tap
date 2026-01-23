package driver

import (
	"bytes"
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// DefaultPostgresTimeout is the default timeout for PostgreSQL query execution.
const DefaultPostgresTimeout = 30 * time.Second

// DefaultPostgresPort is the default port for PostgreSQL connections.
const DefaultPostgresPort = 5432

// PostgresDriver executes SQL queries using the psql CLI.
type PostgresDriver struct {
	// WorkingDir is the directory used for resolving relative paths.
	// If empty, the current working directory is used.
	WorkingDir string
}

// NewPostgresDriver creates a new PostgresDriver with the specified working directory.
func NewPostgresDriver(workingDir string) *PostgresDriver {
	return &PostgresDriver{
		WorkingDir: workingDir,
	}
}

// Name returns the driver identifier.
func (d *PostgresDriver) Name() string {
	return "postgres"
}

// Execute runs the provided SQL query against a PostgreSQL database.
// The config map supports the following keys:
//   - host: PostgreSQL server hostname (default: localhost)
//   - port: PostgreSQL server port (default: 5432)
//   - user: PostgreSQL username
//   - password: PostgreSQL password
//   - database: database name
//   - timeout: execution timeout in seconds (default: 30)
//   - workdir: override the working directory for this execution
func (d *PostgresDriver) Execute(ctx context.Context, code string, config map[string]string) Result {
	// Determine timeout from config or use default
	timeout := DefaultPostgresTimeout
	if timeoutStr, ok := config["timeout"]; ok {
		if seconds, err := strconv.Atoi(timeoutStr); err == nil && seconds > 0 {
			timeout = time.Duration(seconds) * time.Second
		}
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build psql command arguments
	// Use --no-psqlrc to avoid user config affecting output
	// Use -A for unaligned output mode (easier parsing)
	// Use -F for field separator (we'll use tab for easier parsing)
	// Use --pset=border=2 for table format with borders
	args := []string{
		"--no-psqlrc",
		"--pset=border=2",
		"--pset=format=aligned",
	}

	// Host
	host := "localhost"
	if h, ok := config["host"]; ok && h != "" {
		host = h
	}
	args = append(args, "-h", host)

	// Port
	port := DefaultPostgresPort
	if portStr, ok := config["port"]; ok && portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
			port = p
		}
	}
	args = append(args, "-p", strconv.Itoa(port))

	// User
	if user, ok := config["user"]; ok && user != "" {
		args = append(args, "-U", user)
	}

	// Database
	if db, ok := config["database"]; ok && db != "" {
		args = append(args, "-d", db)
	}

	// Create command
	cmd := exec.CommandContext(ctx, "psql", args...)

	// Set working directory
	workDir := d.WorkingDir
	if configWorkDir, ok := config["workdir"]; ok && configWorkDir != "" {
		workDir = configWorkDir
	}
	if workDir != "" {
		cmd.Dir = workDir
	}

	// Set PGPASSWORD environment variable for password (more secure than command line)
	if password, ok := config["password"]; ok && password != "" {
		cmd.Env = append(cmd.Environ(), "PGPASSWORD="+password)
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
		result.Data = parsePostgresTableOutput(result.Output)
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
			// Mask credentials in error messages
			result.Error = maskPostgresCredentials(stderrStr, config)
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			result.Error = "psql exited with code " + strconv.Itoa(exitErr.ExitCode())
		} else {
			result.Error = maskPostgresCredentials(err.Error(), config)
		}
	}

	return result
}

// maskPostgresCredentials replaces sensitive values in error messages with [REDACTED].
func maskPostgresCredentials(message string, config map[string]string) string {
	// Mask password
	if pw, ok := config["password"]; ok && pw != "" {
		message = strings.ReplaceAll(message, pw, "[REDACTED]")
	}
	// Mask user if it looks like it might be embedded in a connection string
	if user, ok := config["user"]; ok && user != "" && len(user) > 2 {
		// Only mask user if it appears with @ or credentials context
		if strings.Contains(message, user+"@") || strings.Contains(message, `"`+user+`"`) {
			message = strings.ReplaceAll(message, user+"@", "[REDACTED]@")
			message = strings.ReplaceAll(message, `"`+user+`"`, `"[REDACTED]"`)
		}
	}
	return message
}

// parsePostgresTableOutput parses psql border=2 output into structured data.
// Returns nil if the output doesn't contain valid tabular data.
// PostgreSQL border=2 format:
// +----+-------+
// | id | name  |
// +----+-------+
// |  1 | Alice |
// |  2 | Bob   |
// +----+-------+
func parsePostgresTableOutput(output string) []map[string]interface{} {
	lines := strings.Split(output, "\n")
	if len(lines) < 4 {
		// Need at least border, header, border, data, and border
		return nil
	}

	// Find the header line (between first two border lines)
	var headerLine string
	var dataLines []string
	borderCount := 0

	for i, line := range lines {
		if isPostgresBorder(line) {
			borderCount++
			continue
		}

		if borderCount == 1 && headerLine == "" {
			// This is the header line
			headerLine = line
		} else if borderCount >= 2 {
			// These are data lines
			if line != "" && !isPostgresBorder(line) {
				dataLines = append(dataLines, line)
			}
		}

		// Stop after the closing border
		if borderCount >= 3 && i > 0 {
			break
		}
	}

	if headerLine == "" {
		return nil
	}

	// Parse column names from header
	columns := parsePostgresHeader(headerLine)
	if len(columns) == 0 {
		return nil
	}

	// Parse data rows
	var data []map[string]interface{}
	for _, line := range dataLines {
		row := parsePostgresRow(line, columns)
		if row != nil {
			data = append(data, row)
		}
	}

	return data
}

// isPostgresBorder checks if a line is a PostgreSQL table border (e.g., +----+----+).
func isPostgresBorder(line string) bool {
	if len(line) < 3 {
		return false
	}
	// PostgreSQL borders start and end with + and contain - and +
	line = strings.TrimSpace(line)
	if line == "" {
		return false
	}
	if line[0] != '+' || line[len(line)-1] != '+' {
		return false
	}
	for _, ch := range line {
		if ch != '+' && ch != '-' {
			return false
		}
	}
	return true
}

// postgresColumnInfo holds column name and position information.
type postgresColumnInfo struct {
	name  string
	start int
	end   int
}

// parsePostgresHeader extracts column names and positions from a header line.
// PostgreSQL header format: | col1 | col2 | col3 |
func parsePostgresHeader(headerLine string) []postgresColumnInfo {
	var columns []postgresColumnInfo

	// Split by | and extract column names and positions
	parts := strings.Split(headerLine, "|")
	currentPos := 0

	for _, part := range parts {
		if part == "" {
			currentPos++ // Account for leading/trailing |
			continue
		}

		name := strings.TrimSpace(part)
		if name != "" {
			// Calculate start position (position after the |)
			start := currentPos
			end := currentPos + len(part)

			columns = append(columns, postgresColumnInfo{
				name:  name,
				start: start,
				end:   end,
			})
		}
		currentPos += len(part) + 1 // +1 for the |
	}

	return columns
}

// parsePostgresRow extracts values from a data row.
// PostgreSQL row format: | val1 | val2 | val3 |
func parsePostgresRow(line string, columns []postgresColumnInfo) map[string]interface{} {
	if len(columns) == 0 {
		return nil
	}

	parts := strings.Split(line, "|")
	row := make(map[string]interface{})

	// Filter out empty parts from leading/trailing |
	var values []string
	for _, part := range parts {
		if part != "" {
			values = append(values, strings.TrimSpace(part))
		}
	}

	// Match values to columns
	for i, col := range columns {
		if i < len(values) {
			value := values[i]
			// Handle NULL values (PostgreSQL shows empty for NULL in this format)
			// We'll keep empty strings as empty strings, not nil
			row[col.name] = value
		} else {
			row[col.name] = ""
		}
	}

	return row
}
