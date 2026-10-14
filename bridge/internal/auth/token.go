// Package auth loads the bridge token and compares credentials in constant time.
package auth

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"os"
	"strings"
)

// MinTokenBytes is the minimum token length. openssl rand -hex 32 writes 64 hex
// characters (32 bytes of entropy) and satisfies this check.
const MinTokenBytes = 32

// Validate rejects a token shorter than MinTokenBytes.
func Validate(token string) error {
	if len(token) < MinTokenBytes {
		return fmt.Errorf("token must be at least %d bytes (openssl rand -hex 32)", MinTokenBytes)
	}
	return nil
}

// LoadFile reads a token file. The file mode must not grant group or other
// access. A single trailing newline is removed.
func LoadFile(path string) (string, error) {
	st, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("token file: %w", err)
	}
	if st.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("token file %s is readable by group or others (mode %#o); use 0600", path, st.Mode().Perm())
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("token file: %w", err)
	}
	token := strings.TrimRight(string(b), "\r\n")
	if err := Validate(token); err != nil {
		return "", err
	}
	return token, nil
}

// Equal reports whether a and b are the same secret. The comparison does not
// stop at the first differing byte.
func Equal(a, b string) bool {
	da := sha256.Sum256([]byte(a))
	db := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(da[:], db[:]) == 1
}
