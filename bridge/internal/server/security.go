package server

import (
	"fmt"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"
)

const (
	headerCSP   = "default-src 'self'; base-uri 'none'; form-action 'none'; frame-ancestors 'none'"
	headerCTO   = "nosniff"
	headerCache = "no-store"
)

func writeSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Security-Policy", headerCSP)
	w.Header().Set("X-Content-Type-Options", headerCTO)
	w.Header().Set("Cache-Control", headerCache)
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Frame-Options", "DENY")
}

func allowedHosts(listenAddr string) ([]string, error) {
	host, port, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return nil, err
	}
	host = strings.Trim(host, "[]")
	switch host {
	case "127.0.0.1", "::1", "localhost":
	default:
		return nil, fmt.Errorf("host not loopback")
	}
	return []string{
		net.JoinHostPort("127.0.0.1", port),
		net.JoinHostPort("localhost", port),
		net.JoinHostPort("[::1]", port),
	}, nil
}

func checkHostOrigin(r *http.Request, allowed []string) bool {
	host := r.Host
	if host == "" {
		return false
	}
	okHost := false
	for _, a := range allowed {
		if strings.EqualFold(host, a) || host == a {
			okHost = true
			break
		}
	}
	if !okHost {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	originHost := u.Host
	for _, a := range allowed {
		if strings.EqualFold(originHost, a) || originHost == a {
			return true
		}
	}
	return false
}

func requireJSONContentType(r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return false
	}
	return mediaType == "application/json"
}
