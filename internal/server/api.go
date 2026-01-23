// Package server provides API endpoints for the tap dev server.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/tapsh/tap/internal/driver"
)

// ExecuteRequest represents a request to execute code via a driver.
type ExecuteRequest struct {
	Driver     string `json:"driver"`
	Code       string `json:"code"`
	Connection string `json:"connection,omitempty"`
}

// ExecuteResponse represents the response from code execution.
type ExecuteResponse struct {
	Output  string                   `json:"output,omitempty"`
	Error   string                   `json:"error,omitempty"`
	Data    []map[string]interface{} `json:"data,omitempty"`
	Success bool                     `json:"success"`
}

// DefaultExecuteTimeout is the default timeout for code execution.
const DefaultExecuteTimeout = 30 * time.Second

// handleAPIExecute handles POST /api/execute requests to execute code via a driver.
func (s *Server) handleAPIExecute(w http.ResponseWriter, r *http.Request) {
	// Only allow POST method
	if r.Method != http.MethodPost {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusMethodNotAllowed)
		_ = json.NewEncoder(w).Encode(ExecuteResponse{
			Success: false,
			Error:   "Method not allowed",
		})
		return
	}

	// Parse request body
	var req ExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	// Validate required fields
	if req.Driver == "" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ExecuteResponse{
			Success: false,
			Error:   "driver field is required",
		})
		return
	}

	// Check if registry is set
	if s.registry == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(ExecuteResponse{
			Success: false,
			Error:   "Driver registry not configured",
		})
		return
	}

	// Check if driver exists
	if !s.registry.Has(req.Driver) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("driver not found: %s", req.Driver),
		})
		return
	}

	// Build config from connection
	config := s.buildExecutionConfig(req.Driver, req.Connection)

	// Create context with timeout
	timeout := s.getExecutionTimeout(req.Driver)
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	// Execute code
	result := s.registry.Execute(ctx, req.Driver, req.Code, config)

	// Determine HTTP status based on result
	w.Header().Set("Content-Type", "application/json")
	if result.Success {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}

	// Return response
	_ = json.NewEncoder(w).Encode(ExecuteResponse{
		Success: result.Success,
		Output:  result.Output,
		Error:   result.Error,
		Data:    result.Data,
	})
}

// buildExecutionConfig builds the config map for driver execution
// by looking up connection details from the presentation config.
func (s *Server) buildExecutionConfig(driverName, connectionName string) map[string]string {
	config := make(map[string]string)

	// Get presentation to access config
	pres := s.GetPresentation()
	if pres == nil {
		return config
	}

	// Look up driver config
	driverConfig, exists := pres.Config.Drivers[driverName]
	if !exists {
		return config
	}

	// Look up connection config if specified
	if connectionName != "" {
		if connConfig, exists := driverConfig.Connections[connectionName]; exists {
			// Map connection config fields to driver config keys
			if connConfig.Host != "" {
				config["host"] = connConfig.Host
			}
			if connConfig.Port != 0 {
				config["port"] = strconv.Itoa(connConfig.Port)
			}
			if connConfig.User != "" {
				config["user"] = connConfig.User
			}
			if connConfig.Password != "" {
				config["password"] = connConfig.Password
			}
			if connConfig.Database != "" {
				config["database"] = connConfig.Database
			}
			if connConfig.Path != "" {
				config["path"] = connConfig.Path
			}
		}
	}

	// Add timeout from driver config if specified
	if driverConfig.Timeout > 0 {
		config["timeout"] = strconv.Itoa(driverConfig.Timeout)
	}

	return config
}

// getExecutionTimeout returns the timeout for a driver execution.
func (s *Server) getExecutionTimeout(driverName string) time.Duration {
	pres := s.GetPresentation()
	if pres == nil {
		return DefaultExecuteTimeout
	}

	// Check if driver has custom timeout
	if driverConfig, exists := pres.Config.Drivers[driverName]; exists {
		if driverConfig.Timeout > 0 {
			return time.Duration(driverConfig.Timeout) * time.Second
		}
	}

	return DefaultExecuteTimeout
}

// SetRegistry sets the driver registry for the server.
// This must be called before SetupRoutes() if you want the execute endpoint to work.
func (s *Server) SetRegistry(registry *driver.Registry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.registry = registry
}

// GetRegistry returns the driver registry.
func (s *Server) GetRegistry() *driver.Registry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.registry
}
