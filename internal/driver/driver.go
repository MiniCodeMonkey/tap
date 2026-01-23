// Package driver provides the interface and registry for code execution drivers.
// Drivers enable live code execution in presentations for various languages and databases.
package driver

import "context"

// Driver defines the interface for code execution drivers.
// Each driver knows how to execute code for a specific language or database.
type Driver interface {
	// Name returns the unique identifier for this driver (e.g., "shell", "sqlite", "mysql").
	Name() string

	// Execute runs the provided code and returns the result.
	// The ctx can be used to cancel long-running executions.
	// The config map contains connection details from the presentation frontmatter.
	Execute(ctx context.Context, code string, config map[string]string) Result
}

// Result represents the outcome of code execution.
type Result struct {
	// Output contains the standard output from execution.
	Output string `json:"output,omitempty"`

	// Error contains error message if execution failed.
	Error string `json:"error,omitempty"`

	// Data contains structured data for tabular results (e.g., SQL query results).
	// Can be used to render results as HTML tables in the frontend.
	Data []map[string]interface{} `json:"data,omitempty"`

	// Success indicates whether the execution completed without errors.
	Success bool `json:"success"`
}
