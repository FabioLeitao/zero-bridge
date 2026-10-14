package server

import (
	"encoding/json"
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
	if !json.Valid([]byte(Redact(`{"title":"Bearer abc"}`))) {
		t.Fatal("raw redact broke json")
	}
}

func TestRedactJSONValues(t *testing.T) {
	cases := []struct {
		in   string
		leak string
	}{
		{`{"title":"Bearer abc"}`, "abc"},
		{`{"title":"sk-abc123xyz"}`, "sk-abc123xyz"},
		{`{"cmd":"Authorization: secret"}`, "secret"},
		{`{"title":"say \"Bearer abc\""}`, "abc"},
	}
	for _, tc := range cases {
		out := RedactJSON([]byte(tc.in))
		if !json.Valid(out) {
			t.Fatalf("invalid json %s from %s", out, tc.in)
		}
		if strings.Contains(string(out), tc.leak) {
			t.Fatalf("leaked %q in %s", tc.leak, out)
		}
		if !strings.Contains(string(out), "[redacted]") {
			t.Fatalf("not redacted: %s", out)
		}
	}
	if string(RedactJSON([]byte(`{"title":`))) != `{}` {
		t.Fatal("bad json should become {}")
	}
}
