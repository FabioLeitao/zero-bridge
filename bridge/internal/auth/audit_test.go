package auth

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestEventHasNoSecretField(t *testing.T) {
	typ := reflect.TypeOf(Event{})
	allowed := map[string]bool{
		"time": true, "kind": true, "source": true, "method": true, "path": true, "reason": true,
	}
	if typ.NumField() != len(allowed) {
		t.Fatalf("fields %d", typ.NumField())
	}
	for i := 0; i < typ.NumField(); i++ {
		f := typ.Field(i)
		tag := f.Tag.Get("json")
		name := strings.ToLower(f.Name + " " + tag)
		if strings.Contains(name, "token") || strings.Contains(name, "session") || strings.Contains(name, "cookie") || strings.Contains(name, "secret") {
			t.Fatalf("secret field %s", f.Name)
		}
		if !allowed[tag] {
			t.Fatalf("json tag %q", tag)
		}
	}
}

func TestRecordScrubsOutput(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	sink, err := OpenFileSink(path)
	if err != nil {
		t.Fatal(err)
	}
	secret := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	session := "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := sink.Record(Event{
				Time:   time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC),
				Kind:   KindLoginFailure,
				Source: "127.0.0.1",
				Method: "POST",
				Path:   "/v1/login",
				Reason: "mismatch " + secret + " " + session,
			})
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	if strings.Contains(text, secret) || strings.Contains(text, session) {
		t.Fatalf("secret in audit output: %s", text)
	}
	if !strings.Contains(text, `"kind":"login_failure"`) || !strings.Contains(text, "[redacted]") {
		t.Fatalf("line: %s", text)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm()&0o077 != 0 {
		t.Fatalf("mode %#o", st.Mode().Perm())
	}
}

func TestOpenFileSinkRefusesOpenMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	if err := os.WriteFile(path, []byte("keep\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenFileSink(path); err == nil || !strings.Contains(err.Error(), "group or others") {
		t.Fatalf("open mode: %v", err)
	}
	kept, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(kept) != "keep\n" {
		t.Fatalf("file changed: %q", kept)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		t.Fatal(err)
	}
	sink, err := OpenFileSink(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := sink.Record(Event{Kind: KindLogout, Source: "127.0.0.1", Method: "POST", Path: "/v1/logout", Reason: "ok"}); err != nil {
		t.Fatal(err)
	}
	if err := sink.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(b), "keep\n") || !strings.Contains(string(b), "logout") {
		t.Fatalf("append: %s", b)
	}
}
