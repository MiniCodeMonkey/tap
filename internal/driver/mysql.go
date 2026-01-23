package driver

import (
	"bytes"
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// DefaultMySQLTimeout is the default timeout for MySQL query execution.
const DefaultMySQLTimeout = 30 * time.Second

// DefaultMySQLPort is the default port for MySQL connections.
const DefaultMySQLPort = 3306

// MySQLDriver executes SQL queries using the mysql CLI.
type MySQLDriver struct {
	// WorkingDir is the directory used for resolving relative paths.
	// If empty, the current working directory is used.
	WorkingDir string
}

// NewMySQLDriver creates a new MySQLDriver with the specified working directory.
func NewMySQLDriver(workingDir string) *MySQLDriver {
	return &MySQLDriver{
		WorkingDir: workingDir,
	}
}

// Name returns the driver identifier.
func (d *MySQLDriver) Name() string {
	return "mysql"
}

// Execute runs the provided SQL query against a MySQL database.
// The config map supports the following keys:
//   - host: MySQL server hostname (default: localhost)
//   - port: MySQL server port (default: 3306)
//   - user: MySQL username
//   - password: MySQL password
//   - database: database name
//   - timeout: execution timeout in seconds (default: 30)
//   - workdir: override the working directory for this execution
func (d *MySQLDriver) Execute(ctx context.Context, code string, config map[string]string) Result {
	// Determine timeout from config or use default
	timeout := DefaultMySQLTimeout
	if timeoutStr, ok := config["timeout"]; ok {
		if seconds, err := strconv.Atoi(timeoutStr); err == nil && seconds > 0 {
			timeout = time.Duration(seconds) * time.Second
		}
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build mysql command arguments
	args := []string{"--table"}

	// Host
	host := "localhost"
	if h, ok := config["host"]; ok && h != "" {
		host = h
	}
	args = append(args, "-h", host)

	// Port
	port := DefaultMySQLPort
	if portStr, ok := config["port"]; ok && portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
			port = p
		}
	}
	args = append(args, "-P", strconv.Itoa(port))

	// User
	if user, ok := config["user"]; ok && user != "" {
		args = append(args, "-u", user)
	}

	// Password - use -p flag format (mysql will read from stdin or env)
	password := ""
	if pw, ok := config["password"]; ok && pw != "" {
		password = pw
	}

	// Database
	if db, ok := config["database"]; ok && db != "" {
		args = append(args, db)
	}

	// Create command
	cmd := exec.CommandContext(ctx, "mysql", args...)

	// Set working directory
	workDir := d.WorkingDir
	if configWorkDir, ok := config["workdir"]; ok && configWorkDir != "" {
		workDir = configWorkDir
	}
	if workDir != "" {
		cmd.Dir = workDir
	}

	// Set MYSQL_PWD environment variable for password (more secure than command line)
	if password != "" {
		cmd.Env = append(cmd.Environ(), "MYSQL_PWD="+password)
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
		result.Data = parseMySQLTableOutput(result.Output)
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
			result.Error = maskCredentials(stderrStr, config)
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			result.Error = "mysql exited with code " + strconv.Itoa(exitErr.ExitCode())
		} else {
			result.Error = maskCredentials(err.Error(), config)
		}
	}

	return result
}

// maskCredentials replaces sensitive values in error messages with [REDACTED].
func maskCredentials(message string, config map[string]string) string {
	// Mask password
	if pw, ok := config["password"]; ok && pw != "" {
		message = strings.ReplaceAll(message, pw, "[REDACTED]")
	}
	// Mask user if it looks like it might be embedded in a connection string
	if user, ok := config["user"]; ok && user != "" && len(user) > 2 {
		// Only mask user if it appears with @ or credentials context
		if strings.Contains(message, user+"@") || strings.Contains(message, "'"+user+"'") {
			message = strings.ReplaceAll(message, user+"@", "[REDACTED]@")
			message = strings.ReplaceAll(message, "'"+user+"'", "'[REDACTED]'")
		}
	}
	return message
}

// parseMySQLTableOutput parses mysql --table output into structured data.
// Returns nil if the output doesn't contain valid tabular data.
func parseMySQLTableOutput(output string) []map[string]interface{} {
	lines := strings.Split(output, "\n")
	if len(lines) < 3 {
		// Need at least border, header, border, and data
		return nil
	}

	// Find the header line (between first two border lines)
	var headerLine string
	var dataLines []string
	borderCount := 0

	for i, line := range lines {
		if isMySQLBorder(line) {
			borderCount++
			continue
		}

		if borderCount == 1 && headerLine == "" {
			// This is the header line
			headerLine = line
		} else if borderCount >= 2 {
			// These are data lines
			if line != "" && !isMySQLBorder(line) {
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
	columns := parseMySQLHeader(headerLine)
	if len(columns) == 0 {
		return nil
	}

	// Parse data rows
	var data []map[string]interface{}
	for _, line := range dataLines {
		row := parseMySQLRow(line, columns)
		if row != nil {
			data = append(data, row)
		}
	}

	return data
}

// isMySQLBorder checks if a line is a MySQL table border (e.g., +----+----+).
func isMySQLBorder(line string) bool {
	if len(line) < 3 {
		return false
	}
	// MySQL borders start and end with + and contain - and +
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

// mysqlColumnInfo holds column name and position information.
type mysqlColumnInfo struct {
	name  string
	start int
	end   int
}

// parseMySQLHeader extracts column names and positions from a header line.
// MySQL header format: | col1 | col2 | col3 |
func parseMySQLHeader(headerLine string) []mysqlColumnInfo {
	var columns []mysqlColumnInfo

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

			columns = append(columns, mysqlColumnInfo{
				name:  name,
				start: start,
				end:   end,
			})
		}
		currentPos += len(part) + 1 // +1 for the |
	}

	return columns
}

// parseMySQLRow extracts values from a data row.
// MySQL row format: | val1 | val2 | val3 |
func parseMySQLRow(line string, columns []mysqlColumnInfo) map[string]interface{} {
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
			// Handle NULL values
			if value == "NULL" {
				row[col.name] = nil
			} else {
				row[col.name] = value
			}
		} else {
			row[col.name] = ""
		}
	}

	return row
}
