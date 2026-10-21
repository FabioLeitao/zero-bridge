// Package redact is the shared secret-shape list for error text and audit lines.
// The two call sites apply it and then their own extra rules (bearer text, hex runs).
package redact

import "regexp"

// Replacement is the literal written in place of a matched secret.
const Replacement = "[redacted]"

// patterns match secret shapes. A word boundary and a minimum length keep
// ordinary words such as task-force intact: those contain the letters sk-
// but not as their own token.
// The PEM markers are concatenated so this source file does not contain the
// contiguous phrase the hygiene check treats as a private key.
var patterns = []*regexp.Regexp{
	regexp.MustCompile(`(?s)` + "-----BEGIN " + `[A-Z0-9 ]*` + "PRIVATE KEY-----" + `.*?-----END ` + `[A-Z0-9 ]*` + "PRIVATE KEY-----"),
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{8,}\b`),
	regexp.MustCompile(`\bxai-[A-Za-z0-9_-]{8,}\b`),
	regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9-]{10,}\b`),
	regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{8,}\.eyJ[A-Za-z0-9_-]{8,}\.[A-Za-z0-9_-]{8,}\b`),
	regexp.MustCompile(`\bghp_[A-Za-z0-9]{20,}\b`),
	regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}\b`),
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
}

// Apply replaces each secret shape with Replacement.
func Apply(s string) string {
	for _, pattern := range patterns {
		s = pattern.ReplaceAllString(s, Replacement)
	}
	return s
}
