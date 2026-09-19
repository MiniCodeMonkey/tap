package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

// TestIsAllowedHost covers each class isAllowedHost accepts or rejects.
func TestIsAllowedHost(t *testing.T) {
	machineHostname, _ := os.Hostname()

	tests := []struct {
		name string
		host string
		want bool
	}{
		{"localhost with port", "localhost:3000", true},
		{"localhost without port", "localhost", true},
		{"loopback IPv4", "127.0.0.1:3000", true},
		{"loopback IPv6", "[::1]:3000", true},
		{"private IPv4 (LAN presenter)", "192.168.1.20:3000", true},
		{"private IPv4 10.x", "10.0.0.5:3000", true},
		{"private IPv4 172.16-31.x", "172.20.0.5:3000", true},
		{"link-local IPv4", "169.254.1.1:3000", true},
		{"link-local IPv6", "[fe80::1]:3000", true},
		{".local name", "my-laptop.local:3000", true},
		{"a public IP literal", "8.8.8.8:3000", false},
		{"an arbitrary domain", "evil.example.com:3000", false},
		{"an arbitrary domain resolving to loopback (DNS rebinding)", "evil.example.com:3000", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAllowedHost(tt.host, nil); got != tt.want {
				t.Errorf("isAllowedHost(%q, nil) = %v, want %v", tt.host, got, tt.want)
			}
		})
	}

	if machineHostname != "" {
		t.Run("the machine's own hostname", func(t *testing.T) {
			if !isAllowedHost(machineHostname+":3000", nil) {
				t.Errorf("isAllowedHost(%q, nil) = false, want true", machineHostname+":3000")
			}
		})
	}

	t.Run("an entry from --allow-origin (full origin form)", func(t *testing.T) {
		extra := allowedHostsFromOrigins([]string{"http://build-box.example.com:5173"})
		if !isAllowedHost("build-box.example.com:5173", extra) {
			t.Error("expected a host matching an --allow-origin entry to be allowed")
		}
	})

	t.Run("an entry from --allow-origin (bare host form)", func(t *testing.T) {
		extra := allowedHostsFromOrigins([]string{"build-box.example.com:5173"})
		if !isAllowedHost("build-box.example.com:5173", extra) {
			t.Error("expected a host matching a bare --allow-origin entry to be allowed")
		}
	})

	t.Run("a host not on any list is rejected", func(t *testing.T) {
		extra := allowedHostsFromOrigins([]string{"http://build-box.example.com:5173"})
		if isAllowedHost("evil.example.com:3000", extra) {
			t.Error("expected an unlisted host to be rejected")
		}
	})
}

// TestRequireAllowedHost_RejectsDisallowedHostHeader checks the HTTP
// middleware directly: a request whose Host header is not on the
// allow-list gets 403 with a body that names --allow-origin, and a
// request with an allowed Host reaches the wrapped handler.
func TestRequireAllowedHost_RejectsDisallowedHostHeader(t *testing.T) {
	s := New(0)

	called := false
	handler := s.requireAllowedHost(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	t.Run("disallowed host is rejected", func(t *testing.T) {
		called = false
		req := httptest.NewRequest(http.MethodGet, "/presenter", nil)
		req.Host = "evil.example.com:3000"
		w := httptest.NewRecorder()
		handler(w, req)

		resp := w.Result()
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusForbidden)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "--allow-origin") {
			t.Errorf("body = %q, want it to name --allow-origin", body)
		}
		if called {
			t.Error("expected the wrapped handler not to run")
		}
	})

	t.Run("allowed host reaches the handler", func(t *testing.T) {
		called = false
		req := httptest.NewRequest(http.MethodGet, "/presenter", nil)
		req.Host = "127.0.0.1:3000"
		w := httptest.NewRecorder()
		handler(w, req)

		if !called {
			t.Error("expected the wrapped handler to run for an allowed host")
		}
	})

	t.Run("a host allowed via --allow-origin reaches the handler", func(t *testing.T) {
		s.SetAllowedOrigins([]string{"build-box.example.com:5173"})
		called = false
		req := httptest.NewRequest(http.MethodGet, "/presenter", nil)
		req.Host = "build-box.example.com:5173"
		w := httptest.NewRecorder()
		handler(w, req)

		if !called {
			t.Error("expected the wrapped handler to run for a host allowed via --allow-origin")
		}
	})
}
