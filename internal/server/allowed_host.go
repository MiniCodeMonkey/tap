// Package server provides HTTP and WebSocket handling for the tap dev
// server.
package server

import (
	"net"
	"net/netip"
	"os"
	"strings"
)

// isAllowedHost reports whether host (a Host header or an Origin header's
// host, both in "host[:port]" form) is safe to treat as this machine
// reaching itself. A plain same-host compare (an Origin's host equals the
// request's own Host header) is not enough on its own: a hostile domain an
// attacker controls can resolve to 127.0.0.1, making that compare pass for
// literally any name (DNS rebinding). Accepted: "localhost", a loopback,
// private, or link-local IP literal (IPv4 and IPv6), a name ending in
// ".local", the machine's own hostname, or an entry from extra (built from
// --allow-origin; see allowedHostsFromOrigins).
func isAllowedHost(host string, extra map[string]struct{}) bool {
	hostname := hostnameWithoutPort(host)

	if hostname == "localhost" {
		return true
	}
	if addr, err := netip.ParseAddr(hostname); err == nil {
		if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() {
			return true
		}
	}
	if strings.HasSuffix(hostname, ".local") {
		return true
	}
	if machineHostname, err := os.Hostname(); err == nil && machineHostname != "" && hostname == machineHostname {
		return true
	}
	if extra != nil {
		if _, ok := extra[host]; ok {
			return true
		}
		if _, ok := extra[hostname]; ok {
			return true
		}
	}
	return false
}

// hostnameWithoutPort strips a trailing ":<port>" and any IPv6 brackets
// from host, tolerating a host with no port (net.SplitHostPort errors on
// that, so this falls back to the original string first).
func hostnameWithoutPort(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}
	return strings.TrimSuffix(strings.TrimPrefix(host, "["), "]")
}

// allowedHostsFromOrigins builds the extra host set isAllowedHost checks,
// from the same --allow-origin values SetAllowedOrigins already takes as
// full origins ("http://localhost:5173"). Each entry contributes its host,
// with and without port, so --allow-origin also works as a plain host
// allow-list entry ("build-box.example.com" or "build-box.example.com:
// 3000") for a request that carries no scheme, such as a plain HTTP Host
// header.
func allowedHostsFromOrigins(origins []string) map[string]struct{} {
	hosts := make(map[string]struct{}, len(origins)*2)
	for _, origin := range origins {
		hostWithPort := origin
		if idx := strings.Index(origin, "://"); idx != -1 {
			hostWithPort = origin[idx+3:]
		}
		if hostWithPort == "" {
			continue
		}
		hosts[hostWithPort] = struct{}{}
		hosts[hostnameWithoutPort(hostWithPort)] = struct{}{}
	}
	return hosts
}
