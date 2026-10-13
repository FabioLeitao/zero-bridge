package server

import (
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	in := "failed sk-abc123xyz and Bearer tok123 and Authorization: secret"
	out := Redact(in)
	if out == in {
		t.Fatal("expected redaction")
	}
	for _, leak := range []string{"sk-abc", "tok123", "secret"} {
		if strings.Contains(out, leak) {
			t.Fatalf("leaked %q in %q", leak, out)
		}
	}
}
