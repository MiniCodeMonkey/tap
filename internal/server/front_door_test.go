package server

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

const frontDoorOrigin = "http://127.0.0.1:3000"

// frontDoorServer has the page routes, an app source route and one extra
// mutating route, the way tap dev --app registers them.
func frontDoorServer() *Server {
	s := NewWithHost(0, "127.0.0.1")
	s.SetupRoutes()
	s.RegisterHandlerFunc("PUT "+AppSourcePath, func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			if IsBodyTooLarge(err) {
				WriteBodyTooLarge(w)
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	s.RegisterHandlerFunc("DELETE /api/test-only", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	return s
}

func frontDoorRequest(method, path string, body []byte, contentType, origin string) *http.Request {
	request := httptest.NewRequest(method, frontDoorOrigin+path, bytes.NewReader(body))
	if contentType != "" {
		request.Header.Set("Content-Type", contentType)
	}
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	return request
}

func serveFrontDoor(s *Server, request *http.Request) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	s.httpServer.Handler.ServeHTTP(recorder, request)
	return recorder
}

func TestFrontDoorGuardsEveryMutatingRoute(t *testing.T) {
	s := frontDoorServer()
	for _, route := range []struct{ method, path string }{
		{http.MethodPost, "/api/execute"},
		{http.MethodPut, AppSourcePath},
		{http.MethodDelete, "/api/test-only"},
		{http.MethodPatch, "/api/not-a-route"},
	} {
		crossSite := serveFrontDoor(s, frontDoorRequest(route.method, route.path, []byte("{}"), "application/json", "https://evil.example"))
		if crossSite.Code != http.StatusForbidden {
			t.Errorf("%s %s from another site: status %d, want 403", route.method, route.path, crossSite.Code)
		}
		plainText := serveFrontDoor(s, frontDoorRequest(route.method, route.path, []byte("{}"), "text/plain", frontDoorOrigin))
		if plainText.Code != http.StatusUnsupportedMediaType {
			t.Errorf("%s %s with a text/plain body: status %d, want 415", route.method, route.path, plainText.Code)
		}
	}
}

func TestFrontDoorLetsASameOriginJSONRequestThrough(t *testing.T) {
	s := frontDoorServer()
	response := serveFrontDoor(s, frontDoorRequest(http.MethodDelete, "/api/test-only", []byte("{}"), "application/json", frontDoorOrigin))
	if response.Code != http.StatusNoContent {
		t.Errorf("status %d, want 204", response.Code)
	}
}

func TestFrontDoorLimitsRequestBodies(t *testing.T) {
	s := frontDoorServer()
	for _, test := range []struct {
		name, method, path string
		size, want         int
	}{
		{"an execute body over 64 KB", http.MethodPost, "/api/execute", RequestBodyLimit + 1, http.StatusRequestEntityTooLarge},
		{"a buffer over 64 KB", http.MethodPut, AppSourcePath, RequestBodyLimit + 1, http.StatusNoContent},
		{"a buffer over 8 MB", http.MethodPut, AppSourcePath, AppSourceBodyLimit + 1, http.StatusRequestEntityTooLarge},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := bytes.Repeat([]byte("a"), test.size)
			response := serveFrontDoor(s, frontDoorRequest(test.method, test.path, body, "application/json", frontDoorOrigin))
			if response.Code != test.want {
				t.Errorf("status %d, want %d", response.Code, test.want)
			}
		})
	}
}

// TestFrontDoorLimitsABodyThatDeclaresNoLength sends a body without a
// Content-Length, so only http.MaxBytesReader can stop it.
func TestFrontDoorLimitsABodyThatDeclaresNoLength(t *testing.T) {
	s := frontDoorServer()
	for _, test := range []struct {
		method, path string
		size         int
	}{
		{http.MethodPost, "/api/execute", RequestBodyLimit + 1},
		{http.MethodPut, AppSourcePath, AppSourceBodyLimit + 1},
	} {
		request := frontDoorRequest(test.method, test.path, nil, "application/json", frontDoorOrigin)
		request.Body = io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("a"), test.size)))
		request.ContentLength = -1
		if response := serveFrontDoor(s, request); response.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("%s %s: status %d, want 413", test.method, test.path, response.Code)
		}
	}
}

// TestSetupRoutesRegistersOnlyPageRoutes lists every route a tap server
// has. None of them may answer a question from tap --app or change the
// recording: those travel only over the tap process's standard input and
// output, which script on a page cannot reach. Add a route here only when
// it does neither.
func TestSetupRoutesRegistersOnlyPageRoutes(t *testing.T) {
	s := New(0)
	s.SetupRoutes()
	want := []string{
		"GET /api/custom-theme.css",
		"GET /api/presentation",
		"GET /assets/",
		"GET /components/",
		"GET /index.html",
		"GET /local/",
		"GET /presenter",
		"GET /presenter.html",
		"GET /presenter/",
		"GET /qr",
		"GET /{$}",
		"POST /api/execute",
	}
	if got := s.Routes(); !slices.Equal(got, want) {
		t.Errorf("routes = %q\nwant %q", got, want)
	}
}
