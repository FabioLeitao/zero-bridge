package server

import (
	"regexp"
)

var (
	reSk         = regexp.MustCompile(`sk-[A-Za-z0-9_-]+`)
	reBearer     = regexp.MustCompile(`Bearer\s+\S+`)
	reAuthHeader = regexp.MustCompile(`(?i)authorization:\s*\S+`)
)

// Redact removes likely secrets from error text shown to clients or logs.
func Redact(s string) string {
	s = reSk.ReplaceAllString(s, "[redacted]")
	s = reBearer.ReplaceAllString(s, "[redacted]")
	s = reAuthHeader.ReplaceAllString(s, "authorization: [redacted]")
	return s
}
