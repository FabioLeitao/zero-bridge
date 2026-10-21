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

func TestRedactSecretShapes(t *testing.T) {
	// Pieces are joined so the source does not hold a live-looking token.
	ghp := "ghp_" + strings.Repeat("z", 36)
	pat := "github_pat_" + strings.Repeat("a", 22)
	akia := "AKIA" + "IOSFODNN7EXAMPLE"
	slack := "xox" + "b-1234567890-abcdefghij"
	pem := "-----BEGIN " + "OPENSSH " + "PRIVATE KEY-----\n" +
		"b3BlbnNzaC1rZXkBAAAA\n" +
		"-----END " + "OPENSSH " + "PRIVATE KEY-----"
	cases := []struct {
		name string
		in   string
		drop string
		keep string
	}{
		{name: "task-force", in: "task-force failed", keep: "task-force failed"},
		{name: "risk-assessment", in: "risk-assessment ok", keep: "risk-assessment ok"},
		{name: "sk", in: "key sk-abcdefghijklmnopqrst", drop: "sk-abcdefghijklmnopqrst"},
		{name: "ghp", in: ghp, drop: ghp},
		{name: "github_pat", in: pat, drop: pat},
		{name: "akia", in: akia, drop: akia},
		{name: "xai", in: "xai-ABCDEFGHIJKLMNOP", drop: "xai-ABCDEFGHIJKLMNOP"},
		{name: "slack", in: slack, drop: slack},
		{name: "jwt", in: "eyJhbGciOiJub25lIn0.eyJzdWIiOiJ0ZXN0In0.signatureok", drop: "eyJzdWIiOiJ0ZXN0In0"},
		{name: "pem", in: pem, drop: "b3BlbnNzaC1rZXkBAAAA"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := Redact(tc.in)
			if tc.keep != "" && out != tc.keep {
				t.Fatalf("changed %q", out)
			}
			if tc.drop != "" && (strings.Contains(out, tc.drop) || !strings.Contains(out, "[redacted]")) {
				t.Fatalf("missed %q in %q", tc.drop, out)
			}
		})
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
