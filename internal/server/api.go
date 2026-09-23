// Package server provides API endpoints for the tap dev server.
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/driver"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

// ExecuteRequest names one live code block of the loaded deck. Both
// numbers count from 1: Slide is the slide's number in the deck, and Block
// counts the live code blocks within that slide. tap runs the code the deck
// holds there, never code sent by the page. Revision is the deck's
// revision (see ComputeRevision) the page had rendered when it sent the
// request, so a reference resolved against a deck that has since changed
// can be told apart from one still current; see
// handleAPIExecute's revision check.
type ExecuteRequest struct {
	Slide    int    `json:"slide"`
	Block    int    `json:"block"`
	Revision string `json:"revision"`
}

// ExecuteResponse represents the response from code execution.
type ExecuteResponse struct {
	Output  string                   `json:"output,omitempty"`
	Error   string                   `json:"error,omitempty"`
	Code    string                   `json:"code,omitempty"`
	Data    []map[string]interface{} `json:"data,omitempty"`
	Success bool                     `json:"success"`
}

// DefaultExecuteTimeout is the default timeout for code execution.
const DefaultExecuteTimeout = time.Duration(config.DefaultDriverTimeoutSeconds) * time.Second

// LiveCodePolicy is which drivers a run of tap dev or tap present lets
// /api/execute use. It comes from the approval check at startup.
type LiveCodePolicy struct {
	// Drivers are the drivers the person approved for this deck.
	Drivers []string
	// AllowAll lets every declared driver run, for --allow-code.
	AllowAll bool
}

// Allows reports whether the policy lets a block with driverName run.
func (p LiveCodePolicy) Allows(driverName string) bool {
	return p.AllowAll || slices.Contains(p.Drivers, driverName)
}

// codeInBodyMessage answers a request that sends code instead of a block
// reference.
const codeInBodyMessage = `/api/execute runs a live code block of the deck by reference. Send {"slide": n, "block": n}, not code.`

// notApprovedMessage answers a request for a driver this run does not
// allow. The page shows "Not approved" for the same blocks.
const notApprovedMessage = "Not approved: this deck may not run code with this driver. Approve it when tap dev or tap present asks at startup in a terminal, or pass --allow-code for this run."

// staleRevisionErrorCode marks an ExecuteResponse refused because the
// request's revision does not match the deck currently loaded, so the
// frontend can tell this refusal apart from any other error and show its
// own message rather than a generic one.
const staleRevisionErrorCode = "stale_revision"

// staleRevisionMessage answers a request whose revision does not match the
// deck currently loaded: the slide and block numbers it names may now
// point at different code than the page showed when it was rendered.
const staleRevisionMessage = "The deck changed since this page loaded, so its Run buttons no longer match what is on screen. Reload the page and try again."

// handleAPIExecute handles POST /api/execute: it runs one live code block
// of the loaded deck, named by slide and block number.
func (s *Server) handleAPIExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeExecuteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// A legitimate body is a couple of small integers, well under a
	// hundred bytes. This caps it before the read, so a body far larger
	// than any real request cannot be held in memory: the read below
	// fails once the limit is crossed and falls into the existing 400
	// path, with no new branch or message.
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)

	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeExecuteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(body, &fields); err != nil {
		writeExecuteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}
	if _, sendsCode := fields["code"]; sendsCode {
		writeExecuteError(w, http.StatusBadRequest, codeInBodyMessage)
		return
	}
	var req ExecuteRequest
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeExecuteError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}
	if req.Slide < 1 || req.Block < 1 {
		writeExecuteError(w, http.StatusBadRequest, "slide and block are required, and both count from 1")
		return
	}
	// A reference is only a slide and block number, so it stays meaningful
	// only as long as the deck it was resolved against. The request must
	// carry the revision the page had rendered when it sent the reference;
	// a mismatch, including a missing revision, means the deck may have
	// changed underneath it, so the block is refused rather than run on
	// the chance the reference still names the same code.
	if req.Revision != s.Revision() {
		writeExecuteErrorWithCode(w, http.StatusConflict, staleRevisionErrorCode, staleRevisionMessage)
		return
	}

	// Read the registry once: SetRegistry runs concurrently on every
	// reload, so every use below reads this local snapshot rather than
	// s.registry directly.
	registry := s.GetRegistry()
	if registry == nil {
		writeExecuteError(w, http.StatusInternalServerError, "Driver registry not configured")
		return
	}

	block, err := findLiveBlock(s.GetPresentation(), req.Slide, req.Block)
	if err != nil {
		writeExecuteError(w, http.StatusNotFound, err.Error())
		return
	}
	if block.Problem != "" {
		writeExecuteError(w, http.StatusUnprocessableEntity, block.Problem)
		return
	}
	if !s.LiveCodePolicy().Allows(block.Driver) {
		writeExecuteError(w, http.StatusForbidden, notApprovedMessage)
		return
	}
	if !registry.Has(block.Driver) {
		writeExecuteError(w, http.StatusBadRequest, fmt.Sprintf("driver not found: %s", block.Driver))
		return
	}

	execConfig, err := s.buildExecutionConfig(block.Driver, block.Connection)
	if err != nil {
		writeExecuteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	timeout := s.getExecutionTimeout(block.Driver)
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	result := registry.Execute(ctx, block.Driver, block.Code, execConfig)

	w.Header().Set("Content-Type", "application/json")
	if result.Success {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(http.StatusInternalServerError)
	}
	_ = json.NewEncoder(w).Encode(ExecuteResponse{
		Success: result.Success,
		Output:  result.Output,
		Error:   result.Error,
		Data:    result.Data,
	})
}

// writeExecuteError writes a failed /api/execute response with no error
// code: the frontend has no reason to branch on this failure beyond
// showing the message.
func writeExecuteError(w http.ResponseWriter, status int, message string) {
	writeExecuteErrorWithCode(w, status, "", message)
}

// writeExecuteErrorWithCode writes a failed /api/execute response carrying
// an error code the frontend can match on, distinct from message text that
// may be reworded.
func writeExecuteErrorWithCode(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ExecuteResponse{Success: false, Error: message, Code: code})
}

// findLiveBlock returns live code block blockNumber of slide slideNumber,
// both counted from 1. A slide is found by its number in the deck, not by
// its position in the list, so a list that leaves slides out still finds
// the right one.
func findLiveBlock(pres *transformer.TransformedPresentation, slideNumber, blockNumber int) (transformer.TransformedCodeBlock, error) {
	if pres == nil {
		//nolint:staticcheck // the message is shown in the page as a sentence
		return transformer.TransformedCodeBlock{}, errors.New("No presentation loaded")
	}
	for _, slide := range pres.Slides {
		if slide.Index+1 != slideNumber {
			continue
		}
		for _, block := range slide.CodeBlocks {
			if block.Block == blockNumber {
				return block, nil
			}
		}
		//nolint:staticcheck // the message is shown in the page as a sentence
		return transformer.TransformedCodeBlock{}, fmt.Errorf("Slide %d has no live code block %d", slideNumber, blockNumber)
	}
	//nolint:staticcheck // the message is shown in the page as a sentence
	return transformer.TransformedCodeBlock{}, fmt.Errorf("The deck has no slide %d", slideNumber)
}

// buildExecutionConfig builds the config map for driver execution by
// looking up connection details from the presentation config. ${NAME} in
// the connection expands here, when the block runs, so the page never
// receives the value.
func (s *Server) buildExecutionConfig(driverName, connectionName string) (map[string]string, error) {
	config := make(map[string]string)

	pres := s.GetPresentation()
	if pres == nil {
		return config, nil
	}

	driverConfig, exists := pres.Config.Drivers[driverName]
	if !exists {
		return config, nil
	}

	if connectionName != "" {
		if connConfig, exists := driverConfig.Connections[connectionName]; exists {
			expanded, err := connConfig.Expanded(fmt.Sprintf("drivers.%s.connections.%s", driverName, connectionName), os.LookupEnv)
			if err != nil {
				return nil, err
			}
			connConfig = expanded
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

	if driverConfig.Timeout > 0 {
		config["timeout"] = strconv.Itoa(driverConfig.Timeout)
	}

	return config, nil
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

// SetLiveCodePolicy sets which drivers /api/execute may run. tap dev and
// tap present set it once at startup, from the approval check.
func (s *Server) SetLiveCodePolicy(policy LiveCodePolicy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.liveCodePolicy = policy
}

// LiveCodePolicy returns which drivers /api/execute may run.
func (s *Server) LiveCodePolicy() LiveCodePolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.liveCodePolicy
}
