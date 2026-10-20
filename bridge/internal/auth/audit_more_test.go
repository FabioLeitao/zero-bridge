package auth

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLogSinkAndMulti(t *testing.T) {
	var buf bytes.Buffer
	logSink := NewLogSink(log.New(&buf, "", 0))
	if err := NewLogSink(nil).Record(Event{Kind: KindRateLimited, Reason: "sk-live-secret and quiet"}); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "audit.log")
	fileSink, err := OpenFileSink(path)
	if err != nil {
		t.Fatal(err)
	}
	multi := NewMulti(nil, logSink, fileSink)
	if err := multi.Record(Event{
		Kind:   KindBearerFailure,
		Source: "local",
		Method: "GET",
		Path:   "/v1/info",
		Reason: "sk-live-secret",
	}); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "sk-live") || !strings.Contains(buf.String(), "[redacted]") {
		t.Fatalf("log sink %q", buf.String())
	}
	if err := multi.Close(); err != nil {
		t.Fatal(err)
	}
	if err := fileSink.Close(); err != nil {
		t.Fatal(err)
	}
	if err := multi.Record(Event{Kind: KindLogout}); err == nil {
		t.Fatal("closed file sink still accepted a record")
	}
}

func TestOpenFileSinkRefusesSymlinkAndDirectory(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenFileSink(link); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("symlink: %v", err)
	}
	if _, err := OpenFileSink(dir); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("directory: %v", err)
	}
}

func TestLoadFileMissing(t *testing.T) {
	if _, err := LoadFile(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("missing file accepted")
	}
}
