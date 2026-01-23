package driver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestShellDriver_Name(t *testing.T) {
	driver := NewShellDriver("")
	if driver.Name() != "shell" {
		t.Errorf("expected driver name 'shell', got '%s'", driver.Name())
	}
}

func TestShellDriver_Execute_SimpleCommand(t *testing.T) {
	driver := NewShellDriver("")
	ctx := context.Background()

	result := driver.Execute(ctx, "echo hello", nil)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Output != "hello" {
		t.Errorf("expected output 'hello', got '%s'", result.Output)
	}
}

func TestShellDriver_Execute_MultipleLines(t *testing.T) {
	driver := NewShellDriver("")
	ctx := context.Background()

	result := driver.Execute(ctx, "echo 'line1'; echo 'line2'", nil)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Output != "line1\nline2" {
		t.Errorf("expected output 'line1\\nline2', got '%s'", result.Output)
	}
}

func TestShellDriver_Execute_ExitCode(t *testing.T) {
	driver := NewShellDriver("")
	ctx := context.Background()

	result := driver.Execute(ctx, "exit 42", nil)

	if result.Success {
		t.Error("expected failure, got success")
	}
	if result.Error == "" {
		t.Error("expected error message, got empty string")
	}
	if !strings.Contains(result.Error, "42") {
		t.Errorf("expected error to contain exit code 42, got '%s'", result.Error)
	}
}

func TestShellDriver_Execute_Stderr(t *testing.T) {
	driver := NewShellDriver("")
	ctx := context.Background()

	result := driver.Execute(ctx, "echo 'error message' >&2; exit 1", nil)

	if result.Success {
		t.Error("expected failure, got success")
	}
	if result.Error != "error message" {
		t.Errorf("expected error 'error message', got '%s'", result.Error)
	}
}

func TestShellDriver_Execute_StderrWithSuccess(t *testing.T) {
	driver := NewShellDriver("")
	ctx := context.Background()

	// Some commands write to stderr but still succeed (like warnings)
	result := driver.Execute(ctx, "echo 'output'; echo 'warning' >&2", nil)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	// Stderr should be included in output when command succeeds
	if !strings.Contains(result.Output, "output") {
		t.Errorf("expected output to contain 'output', got '%s'", result.Output)
	}
	if !strings.Contains(result.Output, "warning") {
		t.Errorf("expected output to contain 'warning', got '%s'", result.Output)
	}
}

func TestShellDriver_Execute_WorkingDirectory(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "shell-driver-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	driver := NewShellDriver(tmpDir)
	ctx := context.Background()

	result := driver.Execute(ctx, "pwd", nil)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	// Resolve symlinks for comparison (macOS /tmp -> /private/tmp)
	expectedDir, _ := filepath.EvalSymlinks(tmpDir)
	actualDir, _ := filepath.EvalSymlinks(result.Output)
	if actualDir != expectedDir {
		t.Errorf("expected working dir '%s', got '%s'", expectedDir, actualDir)
	}
}

func TestShellDriver_Execute_ConfigWorkDir(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "shell-driver-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Driver has no default workdir, but config overrides it
	driver := NewShellDriver("")
	ctx := context.Background()

	result := driver.Execute(ctx, "pwd", map[string]string{"workdir": tmpDir})

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	expectedDir, _ := filepath.EvalSymlinks(tmpDir)
	actualDir, _ := filepath.EvalSymlinks(result.Output)
	if actualDir != expectedDir {
		t.Errorf("expected working dir '%s', got '%s'", expectedDir, actualDir)
	}
}

func TestShellDriver_Execute_Timeout(t *testing.T) {
	driver := NewShellDriver("")
	ctx := context.Background()

	// Set a very short timeout
	result := driver.Execute(ctx, "sleep 5", map[string]string{"timeout": "1"})

	if result.Success {
		t.Error("expected failure due to timeout, got success")
	}
	if result.Error != "execution timed out" {
		t.Errorf("expected error 'execution timed out', got '%s'", result.Error)
	}
}

func TestShellDriver_Execute_ContextCanceled(t *testing.T) {
	driver := NewShellDriver("")
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	result := driver.Execute(ctx, "sleep 5", nil)

	if result.Success {
		t.Error("expected failure due to cancellation, got success")
	}
	// The error could be "execution canceled" or contain "signal: killed"
	if !strings.Contains(result.Error, "canceled") && !strings.Contains(result.Error, "killed") {
		t.Errorf("expected error about cancellation, got '%s'", result.Error)
	}
}

func TestShellDriver_Execute_InvalidTimeout(t *testing.T) {
	driver := NewShellDriver("")
	ctx := context.Background()

	// Invalid timeout should use default (30s), so this should succeed quickly
	result := driver.Execute(ctx, "echo 'quick'", map[string]string{"timeout": "invalid"})

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Output != "quick" {
		t.Errorf("expected output 'quick', got '%s'", result.Output)
	}
}

func TestShellDriver_Execute_ZeroTimeout(t *testing.T) {
	driver := NewShellDriver("")
	ctx := context.Background()

	// Zero timeout should use default, not timeout immediately
	result := driver.Execute(ctx, "echo 'quick'", map[string]string{"timeout": "0"})

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Output != "quick" {
		t.Errorf("expected output 'quick', got '%s'", result.Output)
	}
}

func TestShellDriver_Execute_NilConfig(t *testing.T) {
	driver := NewShellDriver("")
	ctx := context.Background()

	// Should handle nil config gracefully
	result := driver.Execute(ctx, "echo 'test'", nil)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Output != "test" {
		t.Errorf("expected output 'test', got '%s'", result.Output)
	}
}

func TestShellDriver_Execute_MultilineScript(t *testing.T) {
	driver := NewShellDriver("")
	ctx := context.Background()

	script := `
x=5
y=10
echo $((x + y))
`
	result := driver.Execute(ctx, script, nil)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Output != "15" {
		t.Errorf("expected output '15', got '%s'", result.Output)
	}
}

func TestShellDriver_Execute_CommandNotFound(t *testing.T) {
	driver := NewShellDriver("")
	ctx := context.Background()

	result := driver.Execute(ctx, "nonexistentcommand12345", nil)

	if result.Success {
		t.Error("expected failure for non-existent command, got success")
	}
	if result.Error == "" {
		t.Error("expected error message, got empty string")
	}
}
