package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/driver"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

// mockDriver is a test driver that returns predefined results.
type mockDriver struct {
	name   string
	result driver.Result
}

func (m *mockDriver) Name() string {
	return m.name
}

func (m *mockDriver) Execute(_ context.Context, _ string, _ map[string]string) driver.Result {
	return m.result
}

func TestHandleAPIExecute_MethodNotAllowed(t *testing.T) {
	s := New(0)

	req := httptest.NewRequest(http.MethodGet, "/api/execute", nil)
	w := httptest.NewRecorder()

	s.handleAPIExecute(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status %d, got %d", http.StatusMethodNotAllowed, w.Code)
	}

	var resp ExecuteResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Success {
		t.Error("expected Success to be false")
	}
	if resp.Error != "Method not allowed" {
		t.Errorf("expected error 'Method not allowed', got %q", resp.Error)
	}
}

func TestHandleAPIExecute_InvalidJSON(t *testing.T) {
	s := New(0)

	req := httptest.NewRequest(http.MethodPost, "/api/execute", strings.NewReader("invalid json"))
	w := httptest.NewRecorder()

	s.handleAPIExecute(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp ExecuteResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Success {
		t.Error("expected Success to be false")
	}
	if !strings.Contains(resp.Error, "Invalid request body") {
		t.Errorf("expected error to contain 'Invalid request body', got %q", resp.Error)
	}
}

func TestHandleAPIExecute_MissingDriver(t *testing.T) {
	s := New(0)

	body := ExecuteRequest{
		Code: "SELECT 1",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/execute", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	s.handleAPIExecute(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp ExecuteResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Success {
		t.Error("expected Success to be false")
	}
	if resp.Error != "driver field is required" {
		t.Errorf("expected error 'driver field is required', got %q", resp.Error)
	}
}

func TestHandleAPIExecute_NoRegistry(t *testing.T) {
	s := New(0)

	body := ExecuteRequest{
		Driver: "shell",
		Code:   "echo hello",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/execute", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	s.handleAPIExecute(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var resp ExecuteResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Success {
		t.Error("expected Success to be false")
	}
	if resp.Error != "Driver registry not configured" {
		t.Errorf("expected error 'Driver registry not configured', got %q", resp.Error)
	}
}

func TestHandleAPIExecute_DriverNotFound(t *testing.T) {
	s := New(0)
	s.SetRegistry(driver.NewRegistry())

	body := ExecuteRequest{
		Driver: "nonexistent",
		Code:   "echo hello",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/execute", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	s.handleAPIExecute(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
	}

	var resp ExecuteResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Success {
		t.Error("expected Success to be false")
	}
	if !strings.Contains(resp.Error, "driver not found") {
		t.Errorf("expected error to contain 'driver not found', got %q", resp.Error)
	}
}

func TestHandleAPIExecute_Success(t *testing.T) {
	s := New(0)
	reg := driver.NewRegistry()
	reg.Register(&mockDriver{
		name: "test",
		result: driver.Result{
			Success: true,
			Output:  "Hello, World!",
		},
	})
	s.SetRegistry(reg)

	body := ExecuteRequest{
		Driver: "test",
		Code:   "print('Hello, World!')",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/execute", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	s.handleAPIExecute(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp ExecuteResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Error("expected Success to be true")
	}
	if resp.Output != "Hello, World!" {
		t.Errorf("expected output 'Hello, World!', got %q", resp.Output)
	}
}

func TestHandleAPIExecute_ExecutionError(t *testing.T) {
	s := New(0)
	reg := driver.NewRegistry()
	reg.Register(&mockDriver{
		name: "test",
		result: driver.Result{
			Success: false,
			Error:   "command not found",
		},
	})
	s.SetRegistry(reg)

	body := ExecuteRequest{
		Driver: "test",
		Code:   "invalid command",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/execute", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	s.handleAPIExecute(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	var resp ExecuteResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Success {
		t.Error("expected Success to be false")
	}
	if resp.Error != "command not found" {
		t.Errorf("expected error 'command not found', got %q", resp.Error)
	}
}

func TestHandleAPIExecute_WithData(t *testing.T) {
	s := New(0)
	reg := driver.NewRegistry()
	reg.Register(&mockDriver{
		name: "sql",
		result: driver.Result{
			Success: true,
			Output:  "| id | name |",
			Data: []map[string]interface{}{
				{"id": 1, "name": "Alice"},
				{"id": 2, "name": "Bob"},
			},
		},
	})
	s.SetRegistry(reg)

	body := ExecuteRequest{
		Driver: "sql",
		Code:   "SELECT * FROM users",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest(http.MethodPost, "/api/execute", bytes.NewReader(bodyBytes))
	w := httptest.NewRecorder()

	s.handleAPIExecute(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	var resp ExecuteResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if !resp.Success {
		t.Error("expected Success to be true")
	}
	if len(resp.Data) != 2 {
		t.Errorf("expected 2 data rows, got %d", len(resp.Data))
	}
}

func TestBuildExecutionConfig_NoPresentation(t *testing.T) {
	s := New(0)

	config := s.buildExecutionConfig("shell", "")

	if len(config) != 0 {
		t.Errorf("expected empty config, got %v", config)
	}
}

func TestBuildExecutionConfig_NoDriverConfig(t *testing.T) {
	s := New(0)
	cfg := config.DefaultConfig()
	pres := &transformer.TransformedPresentation{
		Config: *cfg,
	}
	s.SetPresentation(pres)

	result := s.buildExecutionConfig("shell", "")

	if len(result) != 0 {
		t.Errorf("expected empty config, got %v", result)
	}
}

func TestBuildExecutionConfig_WithConnection(t *testing.T) {
	s := New(0)
	cfg := config.DefaultConfig()
	cfg.Drivers = map[string]config.DriverConfig{
		"mysql": {
			Timeout: 60,
			Connections: map[string]config.ConnectionConfig{
				"local": {
					Host:     "localhost",
					Port:     3306,
					User:     "root",
					Password: "secret",
					Database: "testdb",
				},
			},
		},
	}
	pres := &transformer.TransformedPresentation{
		Config: *cfg,
	}
	s.SetPresentation(pres)

	result := s.buildExecutionConfig("mysql", "local")

	if result["host"] != "localhost" {
		t.Errorf("expected host 'localhost', got %q", result["host"])
	}
	if result["port"] != "3306" {
		t.Errorf("expected port '3306', got %q", result["port"])
	}
	if result["user"] != "root" {
		t.Errorf("expected user 'root', got %q", result["user"])
	}
	if result["password"] != "secret" {
		t.Errorf("expected password 'secret', got %q", result["password"])
	}
	if result["database"] != "testdb" {
		t.Errorf("expected database 'testdb', got %q", result["database"])
	}
	if result["timeout"] != "60" {
		t.Errorf("expected timeout '60', got %q", result["timeout"])
	}
}

func TestBuildExecutionConfig_ConnectionNotFound(t *testing.T) {
	s := New(0)
	cfg := config.DefaultConfig()
	cfg.Drivers = map[string]config.DriverConfig{
		"mysql": {
			Connections: map[string]config.ConnectionConfig{},
		},
	}
	pres := &transformer.TransformedPresentation{
		Config: *cfg,
	}
	s.SetPresentation(pres)

	result := s.buildExecutionConfig("mysql", "nonexistent")

	// Should return empty config when connection not found
	if result["host"] != "" {
		t.Errorf("expected empty host, got %q", result["host"])
	}
}

func TestGetExecutionTimeout_Default(t *testing.T) {
	s := New(0)

	timeout := s.getExecutionTimeout("shell")

	if timeout != DefaultExecuteTimeout {
		t.Errorf("expected default timeout %v, got %v", DefaultExecuteTimeout, timeout)
	}
}

func TestGetExecutionTimeout_CustomTimeout(t *testing.T) {
	s := New(0)
	cfg := config.DefaultConfig()
	cfg.Drivers = map[string]config.DriverConfig{
		"mysql": {
			Timeout: 120,
		},
	}
	pres := &transformer.TransformedPresentation{
		Config: *cfg,
	}
	s.SetPresentation(pres)

	timeout := s.getExecutionTimeout("mysql")

	if timeout.Seconds() != 120 {
		t.Errorf("expected timeout 120s, got %v", timeout)
	}
}

func TestSetGetRegistry(t *testing.T) {
	s := New(0)

	if s.GetRegistry() != nil {
		t.Error("expected nil registry initially")
	}

	reg := driver.NewRegistry()
	s.SetRegistry(reg)

	if s.GetRegistry() != reg {
		t.Error("expected to get the same registry back")
	}
}
