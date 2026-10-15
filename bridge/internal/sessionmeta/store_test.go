package sessionmeta

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestOpenCreatesTightModes(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	st, err := Open(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Set("sess-1", Update{Alias: strPtr("alpha")}); err != nil {
		t.Fatal(err)
	}
	assertMode(t, dir, 0o700)
	assertMode(t, filepath.Join(dir, fileName), 0o600)
	matches, err := filepath.Glob(filepath.Join(dir, ".sessions-meta-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("temp files left: %v", matches)
	}
}

func TestOpenRefusesLooseDirAndFile(t *testing.T) {
	root := t.TempDir()
	looseDir := filepath.Join(root, "loose-dir")
	if err := os.Mkdir(looseDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(looseDir, nil); err == nil || !strings.Contains(err.Error(), "0700") {
		t.Fatalf("loose dir err = %v", err)
	}

	tight := filepath.Join(root, "tight")
	if err := os.Mkdir(tight, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tight, fileName), []byte(`{"version":1,"sessions":{}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(tight, nil); err == nil || !strings.Contains(err.Error(), "0600") {
		t.Fatalf("loose file err = %v", err)
	}
}

func TestSetDropsDefaultsAndKeepsHostileAlias(t *testing.T) {
	st := openTight(t)
	hostile := `<img src=x onerror="window.__xss=1">`
	got, err := st.Set("id.1", Update{Alias: &hostile, Archived: boolPtr(false), Pinned: boolPtr(true)})
	if err != nil {
		t.Fatal(err)
	}
	if got.Alias != hostile || !got.Pinned || got.Archived {
		t.Fatalf("got %+v", got)
	}
	if _, err := time.Parse(time.RFC3339, got.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Open(st.dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	again, ok := reloaded.Get("id.1")
	if !ok || again.Alias != hostile {
		t.Fatalf("reloaded %+v ok=%v", again, ok)
	}
	cleared, err := st.Set("id.1", Update{Alias: strPtr(""), Pinned: boolPtr(false)})
	if err != nil {
		t.Fatal(err)
	}
	if cleared.Alias != "" || cleared.Pinned || cleared.Archived {
		t.Fatalf("cleared %+v", cleared)
	}
	if _, ok := st.Get("id.1"); ok {
		t.Fatal("default entry kept")
	}
	raw := readFile(t, st)
	if strings.Contains(string(raw), "id.1") {
		t.Fatalf("file still has id: %s", raw)
	}
	if !strings.Contains(string(raw), `"version":1`) {
		t.Fatalf("file %s", raw)
	}
}

func TestAliasAndIDValidation(t *testing.T) {
	st := openTight(t)
	long := strings.Repeat("é", 121)
	cases := []struct {
		id    string
		alias string
	}{
		{id: "", alias: "a"},
		{id: "../x", alias: "a"},
		{id: "ok", alias: "   "},
		{id: "ok", alias: "a\nb"},
		{id: "ok", alias: "a\x00b"},
		{id: "ok", alias: long},
	}
	for _, tc := range cases {
		if _, err := st.Set(tc.id, Update{Alias: &tc.alias}); err == nil {
			t.Fatalf("accepted id %q alias %q", tc.id, tc.alias)
		}
	}
	ok := strings.Repeat("é", 120)
	if _, err := st.Set("Az09._:-", Update{Alias: &ok}); err != nil {
		t.Fatal(err)
	}
}

func TestFullAtFiveThousand(t *testing.T) {
	st := openTight(t)
	stamp := time.Now().UTC().Format(time.RFC3339)
	st.mu.Lock()
	for i := 0; i < maxEntries; i++ {
		st.doc.Sessions["s"+itoa(i)] = Entry{Pinned: true, UpdatedAt: stamp}
	}
	st.mu.Unlock()
	if _, err := st.Set("overflow", Update{Pinned: boolPtr(true)}); err != ErrFull {
		t.Fatalf("overflow err = %v", err)
	}
	if _, err := st.Set("s0", Update{Alias: strPtr("still")}); err != nil {
		t.Fatalf("update at cap: %v", err)
	}
}

func TestCorruptFileIsQuarantined(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "state")
	if err := os.Mkdir(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	payload := []byte("{not-json-secret-looking")
	if err := os.WriteFile(filepath.Join(dir, fileName), payload, 0o600); err != nil {
		t.Fatal(err)
	}
	var notes []string
	st, err := Open(dir, func(msg string) { notes = append(notes, msg) })
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := st.Get("anything"); ok {
		t.Fatal("corrupt store was not empty")
	}
	if len(notes) != 1 || strings.Contains(notes[0], "not-json") || strings.Contains(notes[0], "{") {
		t.Fatalf("warning %q", notes)
	}
	matches, err := filepath.Glob(filepath.Join(dir, fileName+".corrupt-*"))
	if err != nil || len(matches) != 1 {
		t.Fatalf("quarantine %v %v", matches, err)
	}
	kept, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	if string(kept) != string(payload) {
		t.Fatal("quarantine dropped bytes")
	}
	if _, err := os.Stat(filepath.Join(dir, fileName)); !os.IsNotExist(err) {
		t.Fatalf("original file still present: %v", err)
	}
}

func TestConcurrentSets(t *testing.T) {
	st := openTight(t)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := "c" + itoa(i)
			alias := "n" + itoa(i)
			if _, err := st.Set(id, Update{Alias: &alias}); err != nil {
				t.Errorf("set %s: %v", id, err)
			}
		}(i)
	}
	wg.Wait()
	if len(st.All()) != 32 {
		t.Fatalf("entries %d", len(st.All()))
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for n := 0; n < 40; n++ {
			raw, err := os.ReadFile(st.path)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				t.Errorf("read: %v", err)
				return
			}
			var doc fileDoc
			if json.Unmarshal(raw, &doc) != nil || doc.Version != 1 {
				t.Errorf("partial file")
				return
			}
		}
	}()
	for n := 0; n < 40; n++ {
		if _, err := st.Set("c0", Update{Alias: strPtr("again")}); err != nil {
			t.Fatal(err)
		}
	}
	<-done
}

func openTight(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "state"), nil)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func assertMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != want {
		t.Fatalf("%s mode %o, want %o", path, st.Mode().Perm(), want)
	}
}

func readFile(t *testing.T, st *Store) []byte {
	t.Helper()
	b, err := os.ReadFile(st.path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func strPtr(s string) *string { return &s }
func boolPtr(b bool) *bool    { return &b }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [16]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
