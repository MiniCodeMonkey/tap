// Package server provides API endpoints for the tap dev server.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/driver"
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
const DefaultExecuteTimeout = time.Duration(config.DefaultDriverTimeoutSeconds) * time.Second

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
	// Read the whole body before parsing it, so a body over the front
	// door's limit is caught even when it is not valid JSON: a decoder
	// reading straight from the request can hit a syntax error on the
	// first bad byte, before ever reading far enough to trip
	// http.MaxBytesReader.
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		if IsBodyTooLarge(err) {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			_ = json.NewEncoder(w).Encode(ExecuteResponse{
				Success: false,
				Error:   "Request body too large",
			})
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ExecuteResponse{
			Success: false,
			Error:   fmt.Sprintf("Invalid request body: %v", err),
		})
		return
	}

	var req ExecuteRequest
	if err := json.Unmarshal(body, &req); err != nil {
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

	// Read the registry once: SetRegistry runs concurrently on every
	// reload, so every use below reads this local snapshot rather than
	// s.registry directly.
	registry := s.GetRegistry()

	// Check if registry is set
	if registry == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(ExecuteResponse{
			Success: false,
			Error:   "Driver registry not configured",
		})
		return
	}

	// Only code that is a live block in the loaded deck runs. Other
	// devices can reach the server when tap dev is started with --lan or
	// --tunnel, and a client outside a browser can send any Origin
	// header, so the same-origin check alone does not stop a request from
	// running arbitrary code.
	if !s.deckHasLiveBlock(req.Driver, req.Connection, req.Code) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(ExecuteResponse{
			Success: false,
			Error:   "This code is not a live code block in the loaded deck",
		})
		return
	}

	// Check if driver exists
	if !registry.Has(req.Driver) {
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
	result := registry.Execute(ctx, req.Driver, req.Code, config)

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

// deckHasLiveBlock reports whether the loaded deck has a live code block
// with exactly this driver, connection and code.
func (s *Server) deckHasLiveBlock(driverName, connection, code string) bool {
	presentation := s.GetPresentation()
	if presentation == nil {
		return false
	}
	for _, slide := range presentation.Slides {
		for _, block := range slide.CodeBlocks {
			if block.Driver == driverName && block.Connection == connection && block.Code == code {
				return true
			}
		}
	}
	return false
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
