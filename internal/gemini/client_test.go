package gemini

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestNewClient_WithAPIKey(t *testing.T) {
	client, err := NewClient("test-api-key")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.apiKey != "test-api-key" {
		t.Errorf("expected apiKey 'test-api-key', got '%s'", client.apiKey)
	}
	if client.model != DefaultModel {
		t.Errorf("expected model '%s', got '%s'", DefaultModel, client.model)
	}
	if client.baseURL != DefaultBaseURL {
		t.Errorf("expected baseURL '%s', got '%s'", DefaultBaseURL, client.baseURL)
	}
}

func TestNewClient_WithEnvAPIKey(t *testing.T) {
	os.Setenv(EnvAPIKey, "env-api-key")
	defer os.Unsetenv(EnvAPIKey)

	client, err := NewClient("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.apiKey != "env-api-key" {
		t.Errorf("expected apiKey 'env-api-key', got '%s'", client.apiKey)
	}
}

func TestNewClient_NoAPIKey(t *testing.T) {
	os.Unsetenv(EnvAPIKey)
	_, err := NewClient("")
	if err == nil {
		t.Fatal("expected error for missing API key")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Type != ErrorTypeAuth {
		t.Errorf("expected error type '%s', got '%s'", ErrorTypeAuth, apiErr.Type)
	}
}

func TestNewClient_WithOptions(t *testing.T) {
	client, err := NewClient("test-key",
		WithBaseURL("https://custom.api.com"),
		WithModel("custom-model"),
		WithTimeout(30*time.Second),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.baseURL != "https://custom.api.com" {
		t.Errorf("expected baseURL 'https://custom.api.com', got '%s'", client.baseURL)
	}
	if client.model != "custom-model" {
		t.Errorf("expected model 'custom-model', got '%s'", client.model)
	}
	if client.timeout != 30*time.Second {
		t.Errorf("expected timeout '30s', got '%s'", client.timeout)
	}
}

func TestNewClient_BaseURLTrailingSlash(t *testing.T) {
	client, err := NewClient("test-key", WithBaseURL("https://api.example.com/"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.baseURL != "https://api.example.com" {
		t.Errorf("expected baseURL without trailing slash, got '%s'", client.baseURL)
	}
}

func TestNewClientFromEnv(t *testing.T) {
	os.Setenv(EnvAPIKey, "from-env")
	defer os.Unsetenv(EnvAPIKey)

	client, err := NewClientFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client.apiKey != "from-env" {
		t.Errorf("expected apiKey 'from-env', got '%s'", client.apiKey)
	}
}

func TestHasAPIKey(t *testing.T) {
	os.Unsetenv(EnvAPIKey)
	if HasAPIKey() {
		t.Error("expected HasAPIKey() to return false when env is not set")
	}

	os.Setenv(EnvAPIKey, "test-key")
	defer os.Unsetenv(EnvAPIKey)
	if !HasAPIKey() {
		t.Error("expected HasAPIKey() to return true when env is set")
	}
}

func TestGenerateImage_Success(t *testing.T) {
	imageData := []byte("fake-png-image-data")
	encodedImage := base64.StdEncoding.EncodeToString(imageData)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if !strings.Contains(r.URL.Path, "generateContent") {
			t.Errorf("expected path to contain 'generateContent', got %s", r.URL.Path)
		}
		if r.Header.Get("x-goog-api-key") != "test-api-key" {
			t.Errorf("expected API key header, got %s", r.Header.Get("x-goog-api-key"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}

		// Verify request body
		var reqBody generateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}
		if len(reqBody.Contents) == 0 || len(reqBody.Contents[0].Parts) == 0 {
			t.Fatal("expected non-empty contents")
		}
		if reqBody.Contents[0].Parts[0].Text != "test prompt" {
			t.Errorf("expected prompt 'test prompt', got '%s'", reqBody.Contents[0].Parts[0].Text)
		}

		// Return success response
		resp := generateContentResponse{
			Candidates: []candidate{
				{
					Content: &contentResponse{
						Parts: []partResponse{
							{
								InlineData: &inlineData{
									MimeType: "image/png",
									Data:     encodedImage,
								},
							},
						},
					},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, err := NewClient("test-api-key", WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	result, err := client.GenerateImage(context.Background(), "test prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(result.Data) != string(imageData) {
		t.Errorf("expected image data '%s', got '%s'", string(imageData), string(result.Data))
	}
	if result.ContentType != "image/png" {
		t.Errorf("expected content type 'image/png', got '%s'", result.ContentType)
	}
}

func TestGenerateImage_WithAspectRatio(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody generateContentRequest
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		if reqBody.GenerationConfig == nil || reqBody.GenerationConfig.ImageConfig == nil {
			t.Fatal("expected imageConfig in request")
		}
		if reqBody.GenerationConfig.ImageConfig.AspectRatio != "16:9" {
			t.Errorf("expected aspectRatio '16:9', got '%s'", reqBody.GenerationConfig.ImageConfig.AspectRatio)
		}

		imageData := base64.StdEncoding.EncodeToString([]byte("image"))
		resp := generateContentResponse{
			Candidates: []candidate{
				{Content: &contentResponse{Parts: []partResponse{{InlineData: &inlineData{MimeType: "image/png", Data: imageData}}}}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, _ := NewClient("test-key", WithBaseURL(server.URL))
	_, err := client.GenerateImageWithAspectRatio(context.Background(), "test", "16:9")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestGenerateImage_EmptyPrompt(t *testing.T) {
	client, _ := NewClient("test-key")
	_, err := client.GenerateImage(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty prompt")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Type != ErrorTypeInvalidRequest {
		t.Errorf("expected error type '%s', got '%s'", ErrorTypeInvalidRequest, apiErr.Type)
	}
}

func TestGenerateImage_AuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    401,
				"message": "Invalid API key",
				"status":  "UNAUTHENTICATED",
			},
		})
	}))
	defer server.Close()

	client, _ := NewClient("invalid-key", WithBaseURL(server.URL))
	_, err := client.GenerateImage(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for invalid API key")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Type != ErrorTypeAuth {
		t.Errorf("expected error type '%s', got '%s'", ErrorTypeAuth, apiErr.Type)
	}
}

func TestGenerateImage_RateLimitError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    429,
				"message": "Resource exhausted",
				"status":  "RESOURCE_EXHAUSTED",
			},
		})
	}))
	defer server.Close()

	client, _ := NewClient("test-key", WithBaseURL(server.URL))
	_, err := client.GenerateImage(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for rate limit")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Type != ErrorTypeRateLimit {
		t.Errorf("expected error type '%s', got '%s'", ErrorTypeRateLimit, apiErr.Type)
	}
}

func TestGenerateImage_ContentPolicyError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := generateContentResponse{
			PromptFeedback: &promptFeedback{
				BlockReason: "SAFETY",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, _ := NewClient("test-key", WithBaseURL(server.URL))
	_, err := client.GenerateImage(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for content policy violation")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Type != ErrorTypeContentPolicy {
		t.Errorf("expected error type '%s', got '%s'", ErrorTypeContentPolicy, apiErr.Type)
	}
}

func TestGenerateImage_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer server.Close()

	client, _ := NewClient("test-key", WithBaseURL(server.URL))
	_, err := client.GenerateImage(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for server error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Type != ErrorTypeServer {
		t.Errorf("expected error type '%s', got '%s'", ErrorTypeServer, apiErr.Type)
	}
}

func TestGenerateImage_NoImageInResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := generateContentResponse{
			Candidates: []candidate{
				{Content: &contentResponse{Parts: []partResponse{{Text: "I cannot generate that image"}}}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, _ := NewClient("test-key", WithBaseURL(server.URL))
	_, err := client.GenerateImage(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for no image in response")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Type != ErrorTypeNoImage {
		t.Errorf("expected error type '%s', got '%s'", ErrorTypeNoImage, apiErr.Type)
	}
}

func TestGenerateImage_EmptyCandidates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := generateContentResponse{
			Candidates: []candidate{},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, _ := NewClient("test-key", WithBaseURL(server.URL))
	_, err := client.GenerateImage(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for empty candidates")
	}
	apiErr := err.(*APIError)
	if apiErr.Type != ErrorTypeNoImage {
		t.Errorf("expected error type '%s', got '%s'", ErrorTypeNoImage, apiErr.Type)
	}
}

func TestGenerateImage_ContextTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewClient("test-key", WithBaseURL(server.URL), WithTimeout(50*time.Millisecond))
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := client.GenerateImage(ctx, "test")
	if err == nil {
		t.Fatal("expected timeout error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Type != ErrorTypeNetwork {
		t.Errorf("expected error type '%s', got '%s'", ErrorTypeNetwork, apiErr.Type)
	}
}

func TestGenerateImage_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client, _ := NewClient("test-key", WithBaseURL(server.URL))
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	_, err := client.GenerateImage(ctx, "test")
	if err == nil {
		t.Fatal("expected canceled error")
	}
}

func TestMaskAPIKey(t *testing.T) {
	apiKey := "secret-api-key-12345"
	message := "Error: invalid request with key secret-api-key-12345 failed"
	masked := maskAPIKey(message, apiKey)
	if strings.Contains(masked, apiKey) {
		t.Error("API key should be masked in error message")
	}
	if !strings.Contains(masked, "[REDACTED]") {
		t.Error("masked message should contain [REDACTED]")
	}
}

func TestMaskAPIKey_EmptyKey(t *testing.T) {
	message := "Error message"
	masked := maskAPIKey(message, "")
	if masked != message {
		t.Errorf("expected unchanged message when key is empty, got '%s'", masked)
	}
}

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *APIError
		expected string
	}{
		{
			name:     "with code",
			err:      &APIError{Type: ErrorTypeAuth, Message: "invalid key", Code: 401},
			expected: "auth: invalid key (code: 401)",
		},
		{
			name:     "without code",
			err:      &APIError{Type: ErrorTypeNetwork, Message: "connection failed"},
			expected: "network: connection failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, got)
			}
		})
	}
}

func TestGenerateImage_InvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not valid json"))
	}))
	defer server.Close()

	client, _ := NewClient("test-key", WithBaseURL(server.URL))
	_, err := client.GenerateImage(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for invalid JSON response")
	}
	apiErr := err.(*APIError)
	if apiErr.Type != ErrorTypeServer {
		t.Errorf("expected error type '%s', got '%s'", ErrorTypeServer, apiErr.Type)
	}
}

func TestGenerateImage_ErrorInResponseBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := generateContentResponse{
			Error: &apiErrorResponse{
				Code:    400,
				Message: "Invalid prompt",
				Status:  "INVALID_ARGUMENT",
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, _ := NewClient("test-key", WithBaseURL(server.URL))
	_, err := client.GenerateImage(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error from response body")
	}
	apiErr := err.(*APIError)
	if apiErr.Type != ErrorTypeInvalidRequest {
		t.Errorf("expected error type '%s', got '%s'", ErrorTypeInvalidRequest, apiErr.Type)
	}
}

func TestGenerateImage_ContentPolicyInErrorMessage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"code":    400,
				"message": "Request was blocked due to safety policy",
				"status":  "INVALID_ARGUMENT",
			},
		})
	}))
	defer server.Close()

	client, _ := NewClient("test-key", WithBaseURL(server.URL))
	_, err := client.GenerateImage(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for content policy")
	}
	apiErr := err.(*APIError)
	if apiErr.Type != ErrorTypeContentPolicy {
		t.Errorf("expected error type '%s', got '%s'", ErrorTypeContentPolicy, apiErr.Type)
	}
}

func TestGenerateImage_NilContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := generateContentResponse{
			Candidates: []candidate{
				{Content: nil},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client, _ := NewClient("test-key", WithBaseURL(server.URL))
	_, err := client.GenerateImage(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for nil content")
	}
	apiErr := err.(*APIError)
	if apiErr.Type != ErrorTypeNoImage {
		t.Errorf("expected error type '%s', got '%s'", ErrorTypeNoImage, apiErr.Type)
	}
}

func TestGenerateImage_ForbiddenError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("Forbidden"))
	}))
	defer server.Close()

	client, _ := NewClient("test-key", WithBaseURL(server.URL))
	_, err := client.GenerateImage(context.Background(), "test")
	if err == nil {
		t.Fatal("expected error for forbidden")
	}
	apiErr := err.(*APIError)
	if apiErr.Type != ErrorTypeAuth {
		t.Errorf("expected error type '%s', got '%s'", ErrorTypeAuth, apiErr.Type)
	}
}

func TestWithHTTPClient(t *testing.T) {
	customClient := &http.Client{Timeout: 5 * time.Second}
	client, _ := NewClient("test-key", WithHTTPClient(customClient))
	if client.httpClient != customClient {
		t.Error("expected custom HTTP client to be set")
	}
}
