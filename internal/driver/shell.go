package driver

import (
	"bytes"
	"context"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// DefaultShellTimeout is the default timeout for shell command execution.
const DefaultShellTimeout = 30 * time.Second

// ShellDriver executes shell commands via sh -c.
type ShellDriver struct {
	// WorkingDir is the directory in which commands are executed.
	// If empty, commands are executed in the current working directory.
	WorkingDir string
}

// NewShellDriver creates a new ShellDriver with the specified working directory.
func NewShellDriver(workingDir string) *ShellDriver {
	return &ShellDriver{
		WorkingDir: workingDir,
	}
}

// Name returns the driver identifier.
func (d *ShellDriver) Name() string {
	return "shell"
}

// Execute runs the provided shell command and returns the result.
// The config map supports the following keys:
//   - timeout: execution timeout in seconds (default: 30)
//   - workdir: override the working directory for this execution
func (d *ShellDriver) Execute(ctx context.Context, code string, config map[string]string) Result {
	// Determine timeout from config or use default
	timeout := DefaultShellTimeout
	if timeoutStr, ok := config["timeout"]; ok {
		if seconds, err := strconv.Atoi(timeoutStr); err == nil && seconds > 0 {
			timeout = time.Duration(seconds) * time.Second
		}
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create command
	cmd := exec.CommandContext(ctx, "sh", "-c", code)

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

	// Include stderr in error even if command succeeded but wrote to stderr
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
