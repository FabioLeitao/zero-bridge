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

func validateExposure(cfg Config) error {
	if len(cfg.AllowedHosts) == 0 && len(cfg.PublicOrigins) == 0 {
		return nil
	}
	if cfg.Token == "" {
		return fmt.Errorf("token required when exposed via proxy")
	}
	for _, host := range cfg.AllowedHosts {
		if err := validateAllowedHost(host); err != nil {
			return err
		}
	}
	for _, origin := range cfg.PublicOrigins {
		if err := validatePublicOrigin(origin); err != nil {
			return err
		}
	}
	return nil
}

func validateAllowedHost(host string) error {
	if host == "" || strings.Contains(host, "://") || strings.Contains(host, "/") {
		return fmt.Errorf("allowed host must be host[:port]")
	}
	if _, _, err := net.SplitHostPort(host); err == nil {
		return nil
	}
	if strings.Contains(host, " ") {
		return fmt.Errorf("allowed host must be host[:port]")
	}
	return nil
}

func validatePublicOrigin(origin string) error {
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return fmt.Errorf("public origin must be scheme://host[:port]")
	}
	if (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.User != nil {
		return fmt.Errorf("public origin must be scheme://host[:port]")
	}
	return nil
}

func checkHostOrigin(r *http.Request, loopback, extraHosts, origins []string) bool {
	if !hostAllowed(r.Host, loopback, extraHosts, origins) {
		return false
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	return originAllowed(origin, loopback, origins)
}

func hostAllowed(host string, loopback, extraHosts, origins []string) bool {
	if host == "" {
		return false
	}
	for _, a := range loopback {
		if strings.EqualFold(host, a) {
			return true
		}
	}
	for _, a := range extraHosts {
		if strings.EqualFold(host, a) {
			return true
		}
	}
	for _, origin := range origins {
		u, err := url.Parse(origin)
		if err == nil && strings.EqualFold(host, u.Host) {
			return true
		}
	}
	return false
}

func originAllowed(origin string, loopback, origins []string) bool {
	u, err := url.Parse(origin)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return false
	}
	if len(origins) == 0 {
		for _, a := range loopback {
			if strings.EqualFold(u.Host, a) {
				return true
			}
		}
		return false
	}
	if (u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.User != nil {
		return false
	}
	for _, a := range loopback {
		if sameOrigin(u, "http://"+a) {
			return true
		}
	}
	for _, a := range origins {
		if sameOrigin(u, a) {
			return true
		}
	}
	return false
}

func sameOrigin(got *url.URL, allowed string) bool {
	u, err := url.Parse(allowed)
	if err != nil || u.Host == "" {
		return false
	}
	return strings.EqualFold(got.Scheme, u.Scheme) && strings.EqualFold(got.Host, u.Host)
}

func requireJSONContentType(r *http.Request) bool {
	ct := r.Header.Get("Content-Type")
	mediaType, _, err := mime.ParseMediaType(ct)
	if err != nil {
		return false
	}
	return mediaType == "application/json"
}
