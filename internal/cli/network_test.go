package cli

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestListenHost(t *testing.T) {
	if got := listenHost(false); got != "127.0.0.1" {
		t.Errorf("listenHost(false) = %q, want 127.0.0.1", got)
	}
	if got := listenHost(true); got != "0.0.0.0" {
		t.Errorf("listenHost(true) = %q, want 0.0.0.0", got)
	}
}

// firstLANAddress returns a non-loopback IPv4 address of this machine, or
// skips the test when there is none.
func firstLANAddress(t *testing.T) string {
	t.Helper()
	addresses, err := net.InterfaceAddrs()
	if err != nil {
		t.Skipf("cannot list interface addresses: %v", err)
	}
	for _, address := range addresses {
		prefix, ok := address.(*net.IPNet)
		if ok && !prefix.IP.IsLoopback() && prefix.IP.To4() != nil {
			return prefix.IP.String()
		}
	}
	t.Skip("this machine has no non-loopback IPv4 address")
	return ""
}

// startHeadlessDev starts tap dev --headless on a free port with extra
// arguments, waits until it answers on 127.0.0.1, and stops it when the
// test ends.
func startHeadlessDev(t *testing.T, extra ...string) int {
	t.Helper()
	binary := buildTapBinaryForTest(t)
	deckPath := filepath.Join(t.TempDir(), "deck.md")
	if err := os.WriteFile(deckPath, []byte("---\ntitle: Net\n---\n\n# One\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	port := freePort(t)
	args := append([]string{"dev", deckPath, "--headless", "--port", fmt.Sprint(port)}, extra...)
	command := exec.Command(binary, args...)
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Signal(syscall.SIGINT)
		_ = command.Wait()
	})

	deadline := time.Now().Add(20 * time.Second)
	for {
		connection, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 200*time.Millisecond)
		if err == nil {
			connection.Close()
			return port
		}
		if time.Now().After(deadline) {
			t.Fatalf("tap dev did not start:\n%s", output.String())
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func TestDevListensOnLoopbackOnlyByDefault(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}
	lanAddress := firstLANAddress(t)
	port := startHeadlessDev(t)

	connection, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", lanAddress, port), time.Second)
	if err == nil {
		connection.Close()
		t.Errorf("tap dev answered on %s:%d, want loopback only", lanAddress, port)
	}
}

func TestDevListensOnTheNetworkWithLAN(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}
	lanAddress := firstLANAddress(t)
	port := startHeadlessDev(t, "--lan")

	connection, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", lanAddress, port), time.Second)
	if err != nil {
		t.Errorf("tap dev --lan did not answer on %s:%d: %v", lanAddress, port, err)
		return
	}
	connection.Close()
}

// freePort returns a TCP port that is free right now.
func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}
