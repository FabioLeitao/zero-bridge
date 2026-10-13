package server

import (
	"fmt"
	"net"
	"strings"
)

// ResolveListen ensures the address binds only on loopback.
func ResolveListen(addr string) (string, error) {
	if addr == "" {
		addr = "127.0.0.1:8765"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("invalid --listen: %w", err)
	}
	host = strings.Trim(host, "[]")
	switch host {
	case "127.0.0.1", "::1", "localhost":
		return net.JoinHostPort(host, port), nil
	default:
		return "", fmt.Errorf("listen address must be loopback (127.0.0.1, ::1, or localhost), got %q", host)
	}
}
