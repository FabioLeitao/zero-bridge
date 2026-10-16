package server

import (
	"fmt"
	"net"
	"strings"
)

// Endpoint is one listen address. Network is "tcp" or "unix".
// A unix Addr is an absolute path. The public info object reports only
// the network name, never this path.
type Endpoint struct {
	Network string
	Addr    string
}

// ResolveListen accepts a TCP loopback address or unix:<absolute path>.
// The default is 127.0.0.1:8765. Any other TCP host is refused.
// A Windows named pipe is not accepted.
func ResolveListen(addr string) (Endpoint, error) {
	if addr == "" {
		addr = "127.0.0.1:8765"
	}
	if rest, ok := strings.CutPrefix(addr, "unix:"); ok {
		return resolveUnix(rest)
	}
	if strings.HasPrefix(addr, "npipe:") {
		return Endpoint{}, fmt.Errorf("named pipes are not supported")
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return Endpoint{}, fmt.Errorf("invalid --listen: %w", err)
	}
	host = strings.Trim(host, "[]")
	switch host {
	case "127.0.0.1", "::1", "localhost":
		return Endpoint{Network: "tcp", Addr: net.JoinHostPort(host, port)}, nil
	default:
		return Endpoint{}, fmt.Errorf("listen address must be loopback (127.0.0.1, ::1, or localhost), got %q", host)
	}
}

func resolveUnix(path string) (Endpoint, error) {
	if path == "" || !strings.HasPrefix(path, "/") {
		return Endpoint{}, fmt.Errorf("unix listen path must be absolute")
	}
	if strings.Contains(path, "\x00") {
		return Endpoint{}, fmt.Errorf("unix listen path must be absolute")
	}
	return Endpoint{Network: "unix", Addr: path}, nil
}
