package driver

import (
	"bytes"
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// DefaultCustomTimeout is the default timeout for custom driver execution.
const DefaultCustomTimeout = 30 * time.Second

// CustomDriver executes code using a user-defined command.
// It allows users to define custom drivers in their presentation frontmatter
// for any language or tool.
//
//nolint:govet // fieldalignment: struct layout is optimized for readability
type CustomDriver struct {
	// Args are additional arguments passed before the code.
	// The code is passed as the last argument via stdin or as a temp file.
	Args []string
	// name is the driver identifier.
	name string
	// Command is the executable to run (e.g., "python", "node", "ruby").
	Command string
	// WorkingDir is the directory in which commands are executed.
	WorkingDir string
	// Timeout is the execution timeout. If 0, DefaultCustomTimeout is used.
	Timeout time.Duration
}

// CustomDriverConfig holds configuration for creating a CustomDriver.
//
//nolint:govet // fieldalignment: struct layout is optimized for readability
type CustomDriverConfig struct {
	Args       []string
	Name       string
	Command    string
	WorkingDir string
	Timeout    int // seconds
}

// NewCustomDriver creates a new CustomDriver with the specified configuration.
func NewCustomDriver(cfg CustomDriverConfig) *CustomDriver {
	timeout := DefaultCustomTimeout
	if cfg.Timeout > 0 {
		timeout = time.Duration(cfg.Timeout) * time.Second
	}

	return &CustomDriver{
		name:       cfg.Name,
		Command:    cfg.Command,
		Args:       cfg.Args,
		Timeout:    timeout,
		WorkingDir: cfg.WorkingDir,
	}
}

// Name returns the driver identifier.
func (d *CustomDriver) Name() string {
	return d.name
}

// Execute runs the provided code using the custom command and returns the result.
// The code is passed to the command via stdin.
// The config map supports the following keys:
//   - timeout: override execution timeout in seconds
//   - workdir: override the working directory for this execution
func (d *CustomDriver) Execute(ctx context.Context, code string, config map[string]string) Result {
	// Determine timeout from config or use driver default
	timeout := d.Timeout
	if timeoutStr, ok := config["timeout"]; ok {
		if seconds, err := strconv.Atoi(timeoutStr); err == nil && seconds > 0 {
			timeout = time.Duration(seconds) * time.Second
		}
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build command arguments
	args := make([]string, len(d.Args))
	copy(args, d.Args)

	// Create command
	cmd := exec.CommandContext(ctx, d.Command, args...)

	// Pass code via stdin
	cmd.Stdin = strings.NewReader(code)

	// Set working directory
	workDir := d.WorkingDir
	if configWorkDir, ok := config["workdir"]; ok && configWorkDir != "" {
		workDir = configWorkDir
	}
	if workDir != "" {
		cmd.Dir = workDir
	}

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

	// Handle errors
	if err != nil {
		result.Success = false

		// Check for context deadline exceeded
		if ctx.Err() == context.DeadlineExceeded {
			result.Error = "execution timed out"
			return result
		}

		// Check for context canceled
		if ctx.Err() == context.Canceled {
			result.Error = "execution canceled"
			return result
		}

		// Get exit code if available
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode := exitErr.ExitCode()
			stderrStr := strings.TrimSuffix(stderr.String(), "\n")
			if stderrStr != "" {
				result.Error = stderrStr
			} else {
				result.Error = "command exited with code " + strconv.Itoa(exitCode)
			}
		} else {
			result.Error = err.Error()
		}
	}

	// Include stderr in output even if command succeeded but wrote to stderr
	if result.Success && stderr.Len() > 0 {
		stderrStr := strings.TrimSuffix(stderr.String(), "\n")
		if result.Output != "" {
			result.Output = result.Output + "\n" + stderrStr
		} else {
			result.Output = stderrStr
		}
	}

	return result
}

// RegisterCustomDrivers registers custom drivers from a driver configuration map.
// It creates a CustomDriver for each entry that has a Command defined and registers
// it with the provided registry. Built-in drivers (shell, sqlite, mysql, postgres)
// are skipped as they should be registered separately.
func RegisterCustomDrivers(registry *Registry, drivers map[string]DriverConfigInput) {
	builtinDrivers := map[string]bool{
		"shell":    true,
		"sqlite":   true,
		"mysql":    true,
		"postgres": true,
	}

	for name, cfg := range drivers {
		// Skip built-in drivers
		if builtinDrivers[name] {
			continue
		}

		// Skip if no command defined
		if cfg.Command == "" {
			continue
		}

		// Create and register custom driver
		driver := NewCustomDriver(CustomDriverConfig{
			Name:    name,
			Command: cfg.Command,
			Args:    cfg.Args,
			Timeout: cfg.Timeout,
		})
		registry.Register(driver)
	}
}

// DriverConfigInput represents the driver configuration from frontmatter.
// This is used by RegisterCustomDrivers to accept driver definitions.
//
//nolint:govet // fieldalignment: struct layout is optimized for readability
type DriverConfigInput struct {
	Args    []string
	Command string
	Timeout int
}
