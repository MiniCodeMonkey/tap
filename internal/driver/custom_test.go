package driver

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCustomDriver_Name(t *testing.T) {
	driver := NewCustomDriver(CustomDriverConfig{
		Name:    "python",
		Command: "python3",
	})
	if driver.Name() != "python" {
		t.Errorf("expected driver name 'python', got '%s'", driver.Name())
	}
}

func TestCustomDriver_Execute_SimpleCommand(t *testing.T) {
	// Use cat to echo input
	driver := NewCustomDriver(CustomDriverConfig{
		Name:    "cat",
		Command: "cat",
	})
	ctx := context.Background()

	result := driver.Execute(ctx, "hello world", nil)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Output != "hello world" {
		t.Errorf("expected output 'hello world', got '%s'", result.Output)
	}
}

func TestCustomDriver_Execute_WithArgs(t *testing.T) {
	// Use sed with an argument to transform input
	driver := NewCustomDriver(CustomDriverConfig{
		Name:    "sed",
		Command: "sed",
		Args:    []string{"s/hello/goodbye/"},
	})
	ctx := context.Background()

	result := driver.Execute(ctx, "hello world", nil)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Output != "goodbye world" {
		t.Errorf("expected output 'goodbye world', got '%s'", result.Output)
	}
}

func TestCustomDriver_Execute_ShellScript(t *testing.T) {
	// Use sh -c to run shell commands from stdin
	driver := NewCustomDriver(CustomDriverConfig{
		Name:    "shell-stdin",
		Command: "sh",
	})
	ctx := context.Background()

	result := driver.Execute(ctx, "echo 'from stdin'", nil)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Output != "from stdin" {
		t.Errorf("expected output 'from stdin', got '%s'", result.Output)
	}
}

func TestCustomDriver_Execute_MultilineInput(t *testing.T) {
	driver := NewCustomDriver(CustomDriverConfig{
		Name:    "cat",
		Command: "cat",
	})
	ctx := context.Background()

	input := "line1\nline2\nline3"
	result := driver.Execute(ctx, input, nil)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if result.Output != input {
		t.Errorf("expected output '%s', got '%s'", input, result.Output)
	}
}

func TestCustomDriver_Execute_Failure(t *testing.T) {
	driver := NewCustomDriver(CustomDriverConfig{
		Name:    "false",
		Command: "false",
	})
	ctx := context.Background()

	result := driver.Execute(ctx, "", nil)

	if result.Success {
		t.Error("expected failure, got success")
	}
}

func TestCustomDriver_Execute_Stderr(t *testing.T) {
	// Use sh to write to stderr
	driver := NewCustomDriver(CustomDriverConfig{
		Name:    "sh",
		Command: "sh",
	})
	ctx := context.Background()

	result := driver.Execute(ctx, "echo 'error message' >&2; exit 1", nil)

	if result.Success {
		t.Error("expected failure, got success")
	}
	if result.Error != "error message" {
		t.Errorf("expected error 'error message', got '%s'", result.Error)
	}
}

func TestCustomDriver_Execute_StderrWithSuccess(t *testing.T) {
	driver := NewCustomDriver(CustomDriverConfig{
		Name:    "sh",
		Command: "sh",
	})
	ctx := context.Background()

	result := driver.Execute(ctx, "echo 'output'; echo 'warning' >&2", nil)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	if !strings.Contains(result.Output, "output") {
		t.Errorf("expected output to contain 'output', got '%s'", result.Output)
	}
	if !strings.Contains(result.Output, "warning") {
		t.Errorf("expected output to contain 'warning', got '%s'", result.Output)
	}
}

func TestCustomDriver_Execute_Timeout(t *testing.T) {
	driver := NewCustomDriver(CustomDriverConfig{
		Name:    "sh",
		Command: "sh",
		Timeout: 1, // 1 second timeout
	})
	ctx := context.Background()

	result := driver.Execute(ctx, "sleep 5", nil)

	if result.Success {
		t.Error("expected failure due to timeout, got success")
	}
	if result.Error != "execution timed out" {
		t.Errorf("expected error 'execution timed out', got '%s'", result.Error)
	}
}

func TestCustomDriver_Execute_TimeoutFromConfig(t *testing.T) {
	driver := NewCustomDriver(CustomDriverConfig{
		Name:    "sh",
		Command: "sh",
		Timeout: 30, // Default 30 second timeout
	})
	ctx := context.Background()

	// Override with shorter timeout via config
	result := driver.Execute(ctx, "sleep 5", map[string]string{"timeout": "1"})

	if result.Success {
		t.Error("expected failure due to timeout, got success")
	}
	if result.Error != "execution timed out" {
		t.Errorf("expected error 'execution timed out', got '%s'", result.Error)
	}
}

func TestCustomDriver_Execute_ContextCanceled(t *testing.T) {
	driver := NewCustomDriver(CustomDriverConfig{
		Name:    "sh",
		Command: "sh",
	})
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	result := driver.Execute(ctx, "sleep 5", nil)

	if result.Success {
		t.Error("expected failure due to cancellation, got success")
	}
	if !strings.Contains(result.Error, "canceled") && !strings.Contains(result.Error, "killed") {
		t.Errorf("expected error about cancellation, got '%s'", result.Error)
	}
}

func TestCustomDriver_Execute_WorkingDirectory(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "custom-driver-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	driver := NewCustomDriver(CustomDriverConfig{
		Name:       "sh",
		Command:    "sh",
		WorkingDir: tmpDir,
	})
	ctx := context.Background()

	result := driver.Execute(ctx, "pwd", nil)

	if !result.Success {
		t.Errorf("expected success, got error: %s", result.Error)
	}
	expectedDir, _ := filepath.EvalSymlinks(tmpDir)
	actualDir, _ := filepath.EvalSymlinks(result.Output)
	if actualDir != expectedDir {
		t.Errorf("expected working dir '%s', got '%s'", expectedDir, actualDir)
	}
}

func TestCustomDriver_Execute_ConfigWorkDir(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "custom-driver-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	driver := NewCustomDriver(CustomDriverConfig{
		Name:    "sh",
		Command: "sh",
	})
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

func TestCustomDriver_Execute_CommandNotFound(t *testing.T) {
	driver := NewCustomDriver(CustomDriverConfig{
		Name:    "nonexistent",
		Command: "nonexistentcommand12345",
	})
	ctx := context.Background()

	result := driver.Execute(ctx, "test", nil)

	if result.Success {
		t.Error("expected failure for non-existent command, got success")
	}
	if result.Error == "" {
		t.Error("expected error message, got empty string")
	}
}

func TestCustomDriver_DefaultTimeout(t *testing.T) {
	driver := NewCustomDriver(CustomDriverConfig{
		Name:    "test",
		Command: "cat",
		Timeout: 0, // Should use default
	})

	if driver.Timeout != DefaultCustomTimeout {
		t.Errorf("expected default timeout %v, got %v", DefaultCustomTimeout, driver.Timeout)
	}
}

func TestCustomDriver_CustomTimeout(t *testing.T) {
	driver := NewCustomDriver(CustomDriverConfig{
		Name:    "test",
		Command: "cat",
		Timeout: 60, // 60 seconds
	})

	expected := 60 * time.Second
	if driver.Timeout != expected {
		t.Errorf("expected timeout %v, got %v", expected, driver.Timeout)
	}
}

func TestRegisterCustomDrivers_Basic(t *testing.T) {
	registry := NewRegistry()

	drivers := map[string]DriverConfigInput{
		"python": {
			Command: "python3",
			Args:    []string{"-"},
			Timeout: 30,
		},
		"node": {
			Command: "node",
			Args:    []string{"-e"},
		},
	}

	RegisterCustomDrivers(registry, drivers)

	if !registry.Has("python") {
		t.Error("expected python driver to be registered")
	}
	if !registry.Has("node") {
		t.Error("expected node driver to be registered")
	}
}

func TestRegisterCustomDrivers_SkipsBuiltins(t *testing.T) {
	registry := NewRegistry()

	// Try to register built-in names with custom config
	drivers := map[string]DriverConfigInput{
		"shell": {
			Command: "custom-shell",
		},
		"sqlite": {
			Command: "custom-sqlite",
		},
		"mysql": {
			Command: "custom-mysql",
		},
		"postgres": {
			Command: "custom-postgres",
		},
		"ruby": {
			Command: "ruby",
		},
	}

	RegisterCustomDrivers(registry, drivers)

	// Built-in names should not be registered
	if registry.Has("shell") {
		t.Error("shell should not be registered as custom driver")
	}
	if registry.Has("sqlite") {
		t.Error("sqlite should not be registered as custom driver")
	}
	if registry.Has("mysql") {
		t.Error("mysql should not be registered as custom driver")
	}
	if registry.Has("postgres") {
		t.Error("postgres should not be registered as custom driver")
	}

	// Non-builtin should be registered
	if !registry.Has("ruby") {
		t.Error("ruby should be registered as custom driver")
	}
}

func TestRegisterCustomDrivers_SkipsEmptyCommand(t *testing.T) {
	registry := NewRegistry()

	drivers := map[string]DriverConfigInput{
		"empty": {
			Command: "",
			Args:    []string{"-e"},
		},
		"valid": {
			Command: "cat",
		},
	}

	RegisterCustomDrivers(registry, drivers)

	if registry.Has("empty") {
		t.Error("driver with empty command should not be registered")
	}
	if !registry.Has("valid") {
		t.Error("valid driver should be registered")
	}
}

func TestRegisterCustomDrivers_PreservesArgs(t *testing.T) {
	registry := NewRegistry()

	drivers := map[string]DriverConfigInput{
		"awk": {
			Command: "awk",
			Args:    []string{"{print $1}"},
		},
	}

	RegisterCustomDrivers(registry, drivers)

	driver := registry.Get("awk")
	if driver == nil {
		t.Fatal("awk driver should be registered")
	}

	customDriver, ok := driver.(*CustomDriver)
	if !ok {
		t.Fatal("expected CustomDriver type")
	}

	if len(customDriver.Args) != 1 || customDriver.Args[0] != "{print $1}" {
		t.Errorf("expected args ['{print $1}'], got %v", customDriver.Args)
	}
}

func TestRegisterCustomDrivers_EmptyMap(t *testing.T) {
	registry := NewRegistry()
	RegisterCustomDrivers(registry, map[string]DriverConfigInput{})

	if len(registry.List()) != 0 {
		t.Errorf("expected no drivers registered, got %d", len(registry.List()))
	}
}

func TestRegisterCustomDrivers_NilMap(t *testing.T) {
	registry := NewRegistry()
	RegisterCustomDrivers(registry, nil)

	if len(registry.List()) != 0 {
		t.Errorf("expected no drivers registered, got %d", len(registry.List()))
	}
}
