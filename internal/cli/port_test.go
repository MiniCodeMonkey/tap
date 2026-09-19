package cli

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/server"
)

// heldPort opens a real listener on an ephemeral port, bound the same way
// server.New does (0.0.0.0), so a later attempt to bind that exact port
// genuinely fails with "address in use" - not a stand-in, the real OS-level
// collision two tap dev (or tap serve) processes would hit.
func heldPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatalf("failed to hold a port for the test: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	return listener.Addr().(*net.TCPAddr).Port
}

func TestStartOnAvailablePort_ExplicitPortBusyFails(t *testing.T) {
	busyPort := heldPort(t)

	build := func(port int) *server.Server { return server.New(port) }

	srv, err := startOnAvailablePort(busyPort, true, "tap dev", build)
	if err == nil {
		_ = srv.Shutdown(context.Background())
		t.Fatal("expected an error for an explicitly requested, already-busy port")
	}
	if !strings.Contains(err.Error(), fmt.Sprintf("port %d is already in use", busyPort)) {
		t.Errorf("error = %q, want it to name the busy port", err.Error())
	}
	if !strings.Contains(err.Error(), "--port") {
		t.Errorf("error = %q, want it to suggest --port <other>", err.Error())
	}
}

func TestStartOnAvailablePort_DefaultPortBusyFallsBack(t *testing.T) {
	busyPort := heldPort(t)

	build := func(port int) *server.Server { return server.New(port) }

	srv, err := startOnAvailablePort(busyPort, false, "tap dev", build)
	if err != nil {
		t.Fatalf("expected the default port to fall back instead of failing, got: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	if srv.Port() == busyPort {
		t.Errorf("Port() = %d, want a different port than the busy one (%d)", srv.Port(), busyPort)
	}
	if srv.Port() <= busyPort || srv.Port() > busyPort+maxPortFallbackAttempts {
		t.Errorf("Port() = %d, want it within (%d, %d]", srv.Port(), busyPort, busyPort+maxPortFallbackAttempts)
	}
}
