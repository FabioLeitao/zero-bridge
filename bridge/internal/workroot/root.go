// Package workroot checks that a client-chosen directory stays inside the
// roots the operator named. A symlink that resolves outside those roots is
// refused. Errors do not include the resolved path.
package workroot

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MaxAdditional is the most extra directories one session may name.
const MaxAdditional = 8

// Set is the allowlist of workspace roots.
type Set struct {
	shown    []string
	resolved []string
}

// New checks each root. A root must be an absolute directory that exists.
// The shown paths are what /v1/info lists. Containment uses EvalSymlinks.
func New(roots []string) (*Set, error) {
	if len(roots) == 0 {
		return nil, fmt.Errorf("workspace root is required")
	}
	set := &Set{}
	for _, root := range roots {
		clean, err := cleanAbs(root)
		if err != nil {
			return nil, err
		}
		if err := dirExists(clean); err != nil {
			return nil, err
		}
		real, err := filepath.EvalSymlinks(clean)
		if err != nil {
			return nil, fmt.Errorf("workspace root is not available")
		}
		set.shown = append(set.shown, clean)
		set.resolved = append(set.resolved, real)
	}
	return set, nil
}

// Paths returns the cleaned roots, in flag order.
func (s *Set) Paths() []string {
	out := make([]string, len(s.shown))
	copy(out, s.shown)
	return out
}

// Check reports whether path is an existing directory inside a root.
// The returned path is cleaned. The error never includes a resolved path.
func (s *Set) Check(path string) (string, error) {
	clean, err := cleanAbs(path)
	if err != nil {
		return "", err
	}
	if err := dirExists(clean); err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(clean)
	if err != nil {
		return "", fmt.Errorf("directory is not available")
	}
	if !s.contains(real) {
		return "", fmt.Errorf("directory is not allowed")
	}
	return clean, nil
}

// Resolve checks a workspace and up to MaxAdditional extra directories.
// Duplicates are refused. Nothing is partially accepted.
func (s *Set) Resolve(workspace string, extra []string) (string, []string, error) {
	if len(extra) > MaxAdditional {
		return "", nil, fmt.Errorf("too many additional directories")
	}
	cwd, err := s.Check(workspace)
	if err != nil {
		return "", nil, err
	}
	seen := map[string]bool{cwd: true}
	out := make([]string, 0, len(extra))
	for _, dir := range extra {
		clean, err := s.Check(dir)
		if err != nil {
			return "", nil, err
		}
		if seen[clean] {
			return "", nil, fmt.Errorf("duplicate directory")
		}
		seen[clean] = true
		out = append(out, clean)
	}
	return cwd, out, nil
}

func (s *Set) contains(real string) bool {
	for _, root := range s.resolved {
		rel, err := filepath.Rel(root, real)
		if err != nil {
			continue
		}
		if rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))) {
			return true
		}
	}
	return false
}

func cleanAbs(path string) (string, error) {
	if strings.Contains(path, "\x00") {
		return "", fmt.Errorf("path must not contain NUL")
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("path must be absolute")
	}
	clean := filepath.Clean(path)
	trimmed := strings.TrimRight(path, string(os.PathSeparator))
	if trimmed == "" {
		trimmed = string(os.PathSeparator)
	}
	if clean != trimmed {
		return "", fmt.Errorf("path must not contain a parent directory")
	}
	return clean, nil
}

func dirExists(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("directory not found")
	}
	if !st.IsDir() {
		return fmt.Errorf("path must be a directory")
	}
	return nil
}
