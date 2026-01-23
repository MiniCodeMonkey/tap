package driver

import (
	"context"
	"sort"
	"testing"
)

// mockDriver is a test implementation of the Driver interface.
type mockDriver struct {
	name   string
	result Result
}

func (m *mockDriver) Name() string {
	return m.name
}

func (m *mockDriver) Execute(ctx context.Context, code string, config map[string]string) Result {
	return m.result
}

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if r.drivers == nil {
		t.Error("drivers map not initialized")
	}
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()
	driver := &mockDriver{name: "test"}

	r.Register(driver)

	if !r.Has("test") {
		t.Error("driver not registered")
	}
}

func TestRegistry_Register_Overwrites(t *testing.T) {
	r := NewRegistry()
	driver1 := &mockDriver{name: "test", result: Result{Output: "first"}}
	driver2 := &mockDriver{name: "test", result: Result{Output: "second"}}

	r.Register(driver1)
	r.Register(driver2)

	got := r.Get("test")
	if got == nil {
		t.Fatal("driver not found")
	}
	result := got.Execute(context.Background(), "", nil)
	if result.Output != "second" {
		t.Errorf("expected 'second', got '%s'", result.Output)
	}
}

func TestRegistry_Get(t *testing.T) {
	r := NewRegistry()
	driver := &mockDriver{name: "shell"}
	r.Register(driver)

	got := r.Get("shell")
	if got == nil {
		t.Fatal("expected driver, got nil")
	}
	if got.Name() != "shell" {
		t.Errorf("expected 'shell', got '%s'", got.Name())
	}
}

func TestRegistry_Get_NotFound(t *testing.T) {
	r := NewRegistry()

	got := r.Get("nonexistent")
	if got != nil {
		t.Errorf("expected nil for nonexistent driver, got %v", got)
	}
}

func TestRegistry_Has(t *testing.T) {
	r := NewRegistry()
	driver := &mockDriver{name: "mysql"}
	r.Register(driver)

	if !r.Has("mysql") {
		t.Error("expected Has to return true for registered driver")
	}
	if r.Has("postgres") {
		t.Error("expected Has to return false for unregistered driver")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockDriver{name: "shell"})
	r.Register(&mockDriver{name: "sqlite"})
	r.Register(&mockDriver{name: "mysql"})

	got := r.List()
	sort.Strings(got)

	expected := []string{"mysql", "shell", "sqlite"}
	if len(got) != len(expected) {
		t.Fatalf("expected %d drivers, got %d", len(expected), len(got))
	}
	for i, name := range expected {
		if got[i] != name {
			t.Errorf("expected '%s' at index %d, got '%s'", name, i, got[i])
		}
	}
}

func TestRegistry_List_Empty(t *testing.T) {
	r := NewRegistry()

	got := r.List()
	if len(got) != 0 {
		t.Errorf("expected empty list, got %v", got)
	}
}

func TestRegistry_Execute(t *testing.T) {
	r := NewRegistry()
	driver := &mockDriver{
		name: "test",
		result: Result{
			Success: true,
			Output:  "hello world",
		},
	}
	r.Register(driver)

	result := r.Execute(context.Background(), "test", "print('hello')", nil)

	if !result.Success {
		t.Error("expected success")
	}
	if result.Output != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", result.Output)
	}
}

func TestRegistry_Execute_WithConfig(t *testing.T) {
	r := NewRegistry()
	var receivedConfig map[string]string
	driver := &mockDriver{
		name: "configtest",
		result: Result{
			Success: true,
		},
	}
	// Override Execute to capture config
	customDriver := &configCapturingDriver{
		mockDriver:     driver,
		capturedConfig: &receivedConfig,
	}
	r.Register(customDriver)

	config := map[string]string{
		"host":     "localhost",
		"database": "testdb",
	}
	r.Execute(context.Background(), "configtest", "SELECT 1", config)

	if receivedConfig == nil {
		t.Fatal("config not passed to driver")
	}
	if receivedConfig["host"] != "localhost" {
		t.Errorf("expected host 'localhost', got '%s'", receivedConfig["host"])
	}
}

// configCapturingDriver wraps a mockDriver to capture the config passed to Execute.
type configCapturingDriver struct {
	*mockDriver
	capturedConfig *map[string]string
}

func (c *configCapturingDriver) Execute(ctx context.Context, code string, config map[string]string) Result {
	*c.capturedConfig = config
	return c.mockDriver.Execute(ctx, code, config)
}

func TestRegistry_Execute_DriverNotFound(t *testing.T) {
	r := NewRegistry()

	result := r.Execute(context.Background(), "nonexistent", "code", nil)

	if result.Success {
		t.Error("expected failure for nonexistent driver")
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
	if result.Error != "driver not found: nonexistent" {
		t.Errorf("unexpected error message: %s", result.Error)
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	done := make(chan bool)

	// Concurrent writes
	go func() {
		for i := 0; i < 100; i++ {
			r.Register(&mockDriver{name: "driver1"})
		}
		done <- true
	}()

	// Concurrent reads
	go func() {
		for i := 0; i < 100; i++ {
			r.Get("driver1")
			r.Has("driver1")
			r.List()
		}
		done <- true
	}()

	<-done
	<-done
}
