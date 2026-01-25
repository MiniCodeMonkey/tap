// Package gemini provides a client for the Gemini API image generation.
package gemini

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	// DefaultTimeout is the default timeout for API requests.
	DefaultTimeout = 60 * time.Second

	// DefaultBaseURL is the base URL for the Gemini API.
	DefaultBaseURL = "https://generativelanguage.googleapis.com/v1beta"

	// DefaultModel is the Nano Banana Pro model for professional image generation.
	DefaultModel = "gemini-3-pro-image-preview"

	// EnvAPIKey is the environment variable name for the Gemini API key.
	EnvAPIKey = "GEMINI_API_KEY"
)

// ErrorType represents different types of API errors.
type ErrorType string

const (
	// ErrorTypeAuth indicates an authentication error (invalid or missing API key).
	ErrorTypeAuth ErrorType = "auth"
	// ErrorTypeRateLimit indicates a rate limit error.
	ErrorTypeRateLimit ErrorType = "rate_limit"
	// ErrorTypeContentPolicy indicates a content policy violation.
	ErrorTypeContentPolicy ErrorType = "content_policy"
	// ErrorTypeInvalidRequest indicates an invalid request.
	ErrorTypeInvalidRequest ErrorType = "invalid_request"
	// ErrorTypeServer indicates a server error.
	ErrorTypeServer ErrorType = "server"
	// ErrorTypeNetwork indicates a network error.
	ErrorTypeNetwork ErrorType = "network"
	// ErrorTypeNoImage indicates the response contained no image.
	ErrorTypeNoImage ErrorType = "no_image"
)

// APIError represents a structured error from the Gemini API.
type APIError struct {
	Type    ErrorType `json:"type"`
	Message string    `json:"message"`
	Code    int       `json:"code,omitempty"`
}

func (e *APIError) Error() string {
	if e.Code != 0 {
		return fmt.Sprintf("%s: %s (code: %d)", e.Type, e.Message, e.Code)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// ImageResult represents a successfully generated image.
type ImageResult struct {
	Data        []byte `json:"data"`
	ContentType string `json:"content_type"`
}

// Client is a client for the Gemini API.
type Client struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	timeout    time.Duration
}

// Option is a function that configures a Client.
type Option func(*Client)

// WithBaseURL sets a custom base URL for the API.
func WithBaseURL(url string) Option {
	return func(c *Client) {
		c.baseURL = strings.TrimSuffix(url, "/")
	}
}

// WithModel sets the model to use for image generation.
func WithModel(model string) Option {
	return func(c *Client) {
		c.model = model
	}
}

// WithTimeout sets the request timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Client) {
		c.timeout = timeout
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Client) {
		c.httpClient = client
	}
}

// NewClient creates a new Gemini API client.
// If apiKey is empty, it reads from the GEMINI_API_KEY environment variable.
func NewClient(apiKey string, opts ...Option) (*Client, error) {
	if apiKey == "" {
		apiKey = os.Getenv(EnvAPIKey)
	}
	if apiKey == "" {
		return nil, &APIError{
			Type:    ErrorTypeAuth,
			Message: "API key is required: set GEMINI_API_KEY environment variable or pass it explicitly",
		}
	}

	c := &Client{
		apiKey:  apiKey,
		baseURL: DefaultBaseURL,
		model:   DefaultModel,
		timeout: DefaultTimeout,
		httpClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}

	for _, opt := range opts {
		opt(c)
	}

	// Sync HTTP client timeout with configured timeout
	c.httpClient.Timeout = c.timeout

	return c, nil
}

// NewClientFromEnv creates a new client reading the API key from environment.
func NewClientFromEnv(opts ...Option) (*Client, error) {
	return NewClient("", opts...)
}

// generateContentRequest is the request body for the generateContent API.
type generateContentRequest struct {
	Contents         []content         `json:"contents"`
	GenerationConfig *generationConfig `json:"generationConfig,omitempty"`
}

type content struct {
	Parts []part `json:"parts"`
}

type part struct {
	Text       string      `json:"text,omitempty"`
	InlineData *inlineData `json:"inline_data,omitempty"`
}

type inlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"`
}

type generationConfig struct {
	ResponseModalities []string     `json:"responseModalities,omitempty"`
	ImageConfig        *imageConfig `json:"imageConfig,omitempty"`
}

type imageConfig struct {
	AspectRatio string `json:"aspectRatio,omitempty"`
}

// generateContentResponse is the response body from the generateContent API.
type generateContentResponse struct {
	Candidates    []candidate    `json:"candidates,omitempty"`
	PromptFeedback *promptFeedback `json:"promptFeedback,omitempty"`
	Error         *apiErrorResponse `json:"error,omitempty"`
}

type candidate struct {
	Content      *contentResponse `json:"content,omitempty"`
	FinishReason string           `json:"finishReason,omitempty"`
}

type contentResponse struct {
	Parts []partResponse `json:"parts,omitempty"`
}

type partResponse struct {
	Text       string      `json:"text,omitempty"`
	InlineData *inlineData `json:"inlineData,omitempty"`
}

type promptFeedback struct {
	BlockReason string `json:"blockReason,omitempty"`
}

type apiErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

// GenerateImage generates an image from a text prompt.
func (c *Client) GenerateImage(ctx context.Context, prompt string) (*ImageResult, error) {
	return c.GenerateImageWithAspectRatio(ctx, prompt, "")
}

// GenerateImageWithAspectRatio generates an image with a specific aspect ratio.
// Valid aspect ratios: "1:1", "16:9", "9:16", "4:3", "3:4"
func (c *Client) GenerateImageWithAspectRatio(ctx context.Context, prompt string, aspectRatio string) (*ImageResult, error) {
	if prompt == "" {
		return nil, &APIError{
			Type:    ErrorTypeInvalidRequest,
			Message: "prompt cannot be empty",
		}
	}

	reqBody := generateContentRequest{
		Contents: []content{
			{
				Parts: []part{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: &generationConfig{
			ResponseModalities: []string{"TEXT", "IMAGE"},
		},
	}

	if aspectRatio != "" {
		reqBody.GenerationConfig.ImageConfig = &imageConfig{
			AspectRatio: aspectRatio,
		}
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, &APIError{
			Type:    ErrorTypeInvalidRequest,
			Message: fmt.Sprintf("failed to marshal request: %v", err),
		}
	}

	url := fmt.Sprintf("%s/models/%s:generateContent", c.baseURL, c.model)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, &APIError{
			Type:    ErrorTypeNetwork,
			Message: fmt.Sprintf("failed to create request: %v", err),
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-goog-api-key", c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, &APIError{
				Type:    ErrorTypeNetwork,
				Message: "request timed out",
			}
		}
		if ctx.Err() == context.Canceled {
			return nil, &APIError{
				Type:    ErrorTypeNetwork,
				Message: "request was canceled",
			}
		}
		return nil, &APIError{
			Type:    ErrorTypeNetwork,
			Message: fmt.Sprintf("failed to send request: %v", maskAPIKey(err.Error(), c.apiKey)),
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, &APIError{
			Type:    ErrorTypeNetwork,
			Message: fmt.Sprintf("failed to read response: %v", err),
		}
	}

	// Handle HTTP error status codes
	if resp.StatusCode != http.StatusOK {
		return nil, c.parseHTTPError(resp.StatusCode, body)
	}

	var genResp generateContentResponse
	if err := json.Unmarshal(body, &genResp); err != nil {
		return nil, &APIError{
			Type:    ErrorTypeServer,
			Message: fmt.Sprintf("failed to parse response: %v", err),
		}
	}

	// Check for API error in response body
	if genResp.Error != nil {
		return nil, c.classifyAPIError(genResp.Error)
	}

	// Check for content policy block
	if genResp.PromptFeedback != nil && genResp.PromptFeedback.BlockReason != "" {
		return nil, &APIError{
			Type:    ErrorTypeContentPolicy,
			Message: fmt.Sprintf("prompt was blocked: %s", genResp.PromptFeedback.BlockReason),
		}
	}

	// Extract image from response
	return c.extractImage(&genResp)
}

// parseHTTPError converts an HTTP error status to a structured APIError.
func (c *Client) parseHTTPError(statusCode int, body []byte) *APIError {
	// Try to parse the error response
	var errResp struct {
		Error *apiErrorResponse `json:"error"`
	}
	if err := json.Unmarshal(body, &errResp); err == nil && errResp.Error != nil {
		return c.classifyAPIError(errResp.Error)
	}

	// Fall back to status code classification
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &APIError{
			Type:    ErrorTypeAuth,
			Message: "invalid or missing API key",
			Code:    statusCode,
		}
	case http.StatusTooManyRequests:
		return &APIError{
			Type:    ErrorTypeRateLimit,
			Message: "rate limit exceeded, please try again later",
			Code:    statusCode,
		}
	case http.StatusBadRequest:
		return &APIError{
			Type:    ErrorTypeInvalidRequest,
			Message: maskAPIKey(string(body), c.apiKey),
			Code:    statusCode,
		}
	default:
		return &APIError{
			Type:    ErrorTypeServer,
			Message: fmt.Sprintf("server error: %s", maskAPIKey(string(body), c.apiKey)),
			Code:    statusCode,
		}
	}
}

// classifyAPIError converts an API error response to a structured APIError.
func (c *Client) classifyAPIError(errResp *apiErrorResponse) *APIError {
	message := maskAPIKey(errResp.Message, c.apiKey)

	switch errResp.Code {
	case 401, 403:
		return &APIError{
			Type:    ErrorTypeAuth,
			Message: message,
			Code:    errResp.Code,
		}
	case 429:
		return &APIError{
			Type:    ErrorTypeRateLimit,
			Message: message,
			Code:    errResp.Code,
		}
	case 400:
		// Check for content policy in message
		if strings.Contains(strings.ToLower(message), "safety") ||
			strings.Contains(strings.ToLower(message), "policy") ||
			strings.Contains(strings.ToLower(message), "blocked") {
			return &APIError{
				Type:    ErrorTypeContentPolicy,
				Message: message,
				Code:    errResp.Code,
			}
		}
		return &APIError{
			Type:    ErrorTypeInvalidRequest,
			Message: message,
			Code:    errResp.Code,
		}
	default:
		return &APIError{
			Type:    ErrorTypeServer,
			Message: message,
			Code:    errResp.Code,
		}
	}
}

// extractImage extracts the generated image from the API response.
func (c *Client) extractImage(resp *generateContentResponse) (*ImageResult, error) {
	if len(resp.Candidates) == 0 {
		return nil, &APIError{
			Type:    ErrorTypeNoImage,
			Message: "no candidates in response",
		}
	}

	for _, candidate := range resp.Candidates {
		if candidate.Content == nil {
			continue
		}
		for _, part := range candidate.Content.Parts {
			if part.InlineData != nil && part.InlineData.Data != "" {
				// Decode base64 image data
				imageData, err := base64.StdEncoding.DecodeString(part.InlineData.Data)
				if err != nil {
					return nil, &APIError{
						Type:    ErrorTypeServer,
						Message: fmt.Sprintf("failed to decode image data: %v", err),
					}
				}
				return &ImageResult{
					Data:        imageData,
					ContentType: part.InlineData.MimeType,
				}, nil
			}
		}
	}

	return nil, &APIError{
		Type:    ErrorTypeNoImage,
		Message: "response did not contain an image",
	}
}

// maskAPIKey replaces the API key in a string with [REDACTED].
func maskAPIKey(s, apiKey string) string {
	if apiKey == "" {
		return s
	}
	return strings.ReplaceAll(s, apiKey, "[REDACTED]")
}

// HasAPIKey checks if the GEMINI_API_KEY environment variable is set.
func HasAPIKey() bool {
	return os.Getenv(EnvAPIKey) != ""
}
