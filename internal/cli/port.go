// Package cli provides the command-line interface for Tap.
package cli

import (
	"errors"
	"fmt"
	"net"
	"syscall"

	"github.com/MiniCodeMonkey/tap/internal/server"
)

// maxPortFallbackAttempts is how many ports above a non-explicit default
// port tap dev and tap serve try before giving up.
const maxPortFallbackAttempts = 20

// isPortInUseError reports whether err, or anything it wraps, is the
// "address already in use" error a failed bind returns.
func isPortInUseError(err error) bool {
	return errors.Is(err, syscall.EADDRINUSE)
}

// portInUseError builds the message a busy, explicitly requested port
// fails with, naming the command a caller most likely already has running
// on it.
func portInUseError(requestedPort int, commandName string) error {
	return fmt.Errorf("port %d is already in use (another %s may be running); pass --port <other>", requestedPort, commandName)
}

// startOnAvailablePort starts a *server.Server on requestedPort, built by
// build (a fresh Server per candidate port, since Server.New fixes its
// address at construction). Server.Start binds synchronously
// (net.Listen), so a busy port is known before this returns - well before
// any success output, TUI start, or browser open.
//
// When explicit is true (the user passed --port themselves), a busy port
// is a hard error naming a fix (see portInUseError); requestedPort is the
// only candidate tried. When explicit is false (the default port), a busy
// port is not fatal: the next maxPortFallbackAttempts ports are tried in
// turn instead, so two tap dev processes (or two decks, or two agents)
// can run side by side without flags.
//
// loopback is whether build binds to loopback rather than the wildcard
// address (true unless the caller passed --lan). On macOS, binding
// 127.0.0.1:P succeeds even when another process already holds the
// wildcard *:P, so a plain bind attempt on a loopback address would miss
// that collision and let two unrelated servers share the port. When
// loopback is true, each candidate port is first probed with a wildcard
// listen; a port the probe finds busy is treated the same as a bind
// failure, without ever building or starting a real server on it.
func startOnAvailablePort(requestedPort int, explicit bool, commandName string, loopback bool, build func(port int) *server.Server) (*server.Server, error) {
	attempts := 1
	if !explicit {
		attempts = maxPortFallbackAttempts + 1
	}

	var lastAttemptErr error
	for i := 0; i < attempts; i++ {
		candidatePort := requestedPort + i
		if loopback {
			busy, probeErr := wildcardPortBusy(candidatePort)
			if probeErr != nil {
				return nil, fmt.Errorf("failed to probe port: %w", probeErr)
			}
			if busy {
				lastAttemptErr = fmt.Errorf("port %d: %w", candidatePort, syscall.EADDRINUSE)
				continue
			}
		}
		startedServer := build(candidatePort)
		err := startedServer.Start()
		if err == nil {
			return startedServer, nil
		}
		if !isPortInUseError(err) {
			return nil, fmt.Errorf("failed to start server: %w", err)
		}
		lastAttemptErr = err
	}

	if explicit {
		return nil, portInUseError(requestedPort, commandName)
	}
	return nil, fmt.Errorf("could not find a free port from %d to %d: %w", requestedPort, requestedPort+maxPortFallbackAttempts, lastAttemptErr)
}

// wildcardPortBusy reports whether port is held by a wildcard listener
// (something bound to *:port), by probing a wildcard listen on it and
// closing the probe listener immediately, before any real bind is
// attempted. On macOS a loopback bind (127.0.0.1:port) succeeds even
// while another process holds the wildcard address on the same port, so
// this probe is what actually catches that collision.
func wildcardPortBusy(port int) (bool, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		if isPortInUseError(err) {
			return true, nil
		}
		return false, err
	}
	_ = listener.Close()
	return false, nil
}

// listenOnAvailablePort binds a TCP listener on requestedPort the same way
// startOnAvailablePort picks a port for the *server.Server-based commands,
// for a caller (tap serve) that drives a raw *http.Server instead: the
// listener is handed back already bound, so the caller can Serve on it
// without a second net.Listen (and the TOCTOU gap that would open between
// probing a port and actually binding it).
func listenOnAvailablePort(requestedPort int, explicit bool, commandName string) (net.Listener, error) {
	attempts := 1
	if !explicit {
		attempts = maxPortFallbackAttempts + 1
	}

	var lastAttemptErr error
	for i := 0; i < attempts; i++ {
		candidatePort := requestedPort + i
		listener, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", candidatePort))
		if err == nil {
			return listener, nil
		}
		if !isPortInUseError(err) {
			return nil, fmt.Errorf("failed to listen: %w", err)
		}
		lastAttemptErr = err
	}

	if explicit {
		return nil, portInUseError(requestedPort, commandName)
	}
	return nil, fmt.Errorf("could not find a free port from %d to %d: %w", requestedPort, requestedPort+maxPortFallbackAttempts, lastAttemptErr)
}
