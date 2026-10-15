// Package sessionmeta stores bridge-local session labels.
// Alias, archived, and pinned stay in this file. They are never written to
// the agent session store.
package sessionmeta

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	fileName   = "sessions-meta.json"
	version    = 1
	maxEntries = 5000
	maxAlias   = 120
	maxID      = 128
)

// ErrFull is returned when a new session id would exceed maxEntries.
var ErrFull = errors.New("too many session metadata entries")

var idPattern = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

// Entry is one session's local labels. UpdatedAt is RFC3339.
type Entry struct {
	Alias     string `json:"alias"`
	Archived  bool   `json:"archived"`
	Pinned    bool   `json:"pinned"`
	UpdatedAt string `json:"updatedAt"`
}

// Update changes the fields whose pointers are set. An empty Alias clears it.
type Update struct {
	Alias    *string
	Archived *bool
	Pinned   *bool
}

// WarnFunc receives a short notice. The notice does not include file bytes.
type WarnFunc func(msg string)

type fileDoc struct {
	Version  int              `json:"version"`
	Sessions map[string]Entry `json:"sessions"`
}

// Store is a process-local view of sessions-meta.json.
type Store struct {
	mu   sync.Mutex
	dir  string
	path string
	doc  fileDoc
}

// Open creates dir at mode 0700 when it is missing. An existing directory
// more open than 0700, or an existing file more open than 0600, is refused.
// A file that does not parse is renamed aside and the store starts empty.
func Open(dir string, warn WarnFunc) (*Store, error) {
	if warn == nil {
		warn = func(string) {}
	}
	info, err := os.Stat(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("state directory: %w", err)
		}
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("state directory: %w", err)
		}
		if err := os.Chmod(dir, 0o700); err != nil {
			return nil, fmt.Errorf("state directory: %w", err)
		}
		info, err = os.Stat(dir)
		if err != nil {
			return nil, fmt.Errorf("state directory: %w", err)
		}
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("state directory %s is not a directory", dir)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, fmt.Errorf("state directory %s is more open than 0700 (mode %04o)", dir, info.Mode().Perm())
	}
	path := filepath.Join(dir, fileName)
	st := &Store{
		dir:  dir,
		path: path,
		doc:  fileDoc{Version: version, Sessions: map[string]Entry{}},
	}
	if err := st.load(warn); err != nil {
		return nil, err
	}
	return st, nil
}

func (s *Store) load(warn WarnFunc) error {
	raw, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("session metadata: %w", err)
	}
	info, err := os.Stat(s.path)
	if err != nil {
		return fmt.Errorf("session metadata: %w", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("state file %s is more open than 0600 (mode %04o)", s.path, info.Mode().Perm())
	}
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	var doc fileDoc
	if err := dec.Decode(&doc); err != nil || doc.Version != version || doc.Sessions == nil {
		return s.quarantine(warn)
	}
	if dec.More() {
		return s.quarantine(warn)
	}
	for id, entry := range doc.Sessions {
		if !validID(id) || !validStored(entry) {
			return s.quarantine(warn)
		}
	}
	s.doc = doc
	return nil
}

func validStored(entry Entry) bool {
	if _, err := time.Parse(time.RFC3339, entry.UpdatedAt); err != nil {
		return false
	}
	if entry.Alias == "" && !entry.Archived && !entry.Pinned {
		return false
	}
	return validAlias(entry.Alias) == nil || entry.Alias == ""
}

func (s *Store) quarantine(warn WarnFunc) error {
	dest := filepath.Join(s.dir, fmt.Sprintf("%s.corrupt-%d", fileName, time.Now().Unix()))
	if err := os.Rename(s.path, dest); err != nil {
		return fmt.Errorf("session metadata: %w", err)
	}
	warn("session metadata file is unreadable; the previous bytes were kept aside and the store started empty")
	s.doc = fileDoc{Version: version, Sessions: map[string]Entry{}}
	return nil
}

// Get returns a copy of the entry.
func (s *Store) Get(id string) (Entry, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.doc.Sessions[id]
	return entry, ok
}

// All returns a copy of every stored entry.
func (s *Store) All() map[string]Entry {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]Entry, len(s.doc.Sessions))
	for id, entry := range s.doc.Sessions {
		out[id] = entry
	}
	return out
}

// Set applies u and writes the file. An entry that is all defaults is removed.
func (s *Store) Set(id string, u Update) (Entry, error) {
	if !validID(id) {
		return Entry{}, fmt.Errorf("invalid session id")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	prev, exists := s.doc.Sessions[id]
	cur := prev
	if u.Alias != nil {
		if err := validAlias(*u.Alias); err != nil {
			return Entry{}, err
		}
		cur.Alias = *u.Alias
	}
	if u.Archived != nil {
		cur.Archived = *u.Archived
	}
	if u.Pinned != nil {
		cur.Pinned = *u.Pinned
	}
	restore := func() {
		if exists {
			s.doc.Sessions[id] = prev
			return
		}
		delete(s.doc.Sessions, id)
	}
	if cur.Alias == "" && !cur.Archived && !cur.Pinned {
		delete(s.doc.Sessions, id)
		if err := s.write(); err != nil {
			restore()
			return Entry{}, err
		}
		return Entry{}, nil
	}
	if !exists && len(s.doc.Sessions) >= maxEntries {
		return Entry{}, ErrFull
	}
	cur.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if s.doc.Sessions == nil {
		s.doc.Sessions = map[string]Entry{}
	}
	s.doc.Sessions[id] = cur
	if err := s.write(); err != nil {
		restore()
		return Entry{}, err
	}
	return cur, nil
}

func validID(id string) bool {
	return idPattern.MatchString(id) && len(id) <= maxID
}

func validAlias(alias string) error {
	if alias == "" {
		return nil
	}
	if !utf8.ValidString(alias) {
		return fmt.Errorf("invalid alias")
	}
	if utf8.RuneCountInString(alias) > maxAlias {
		return fmt.Errorf("invalid alias")
	}
	if strings.TrimSpace(alias) == "" {
		return fmt.Errorf("invalid alias")
	}
	for _, r := range alias {
		if r == 0 || unicode.IsControl(r) {
			return fmt.Errorf("invalid alias")
		}
	}
	return nil
}

func (s *Store) write() error {
	if s.doc.Sessions == nil {
		s.doc.Sessions = map[string]Entry{}
	}
	s.doc.Version = version
	raw, err := json.Marshal(s.doc)
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	tmp, err := os.CreateTemp(s.dir, ".sessions-meta-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, s.path); err != nil {
		return err
	}
	ok = true
	dir, err := os.Open(s.dir)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
