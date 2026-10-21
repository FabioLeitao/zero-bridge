package auth

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestAuditScrubsSecretShapes(t *testing.T) {
	// Pieces are joined so the source does not hold a live-looking token.
	// This file is not on the sk- allow list, so that fixture is joined too.
	sk := "sk-" + "abcdefghijklmnopqrst"
	risk := "risk-" + "assessment ok"
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
		{name: "compound", in: risk, keep: risk},
		{name: "sk", in: "key " + sk, drop: sk},
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
			var buf bytes.Buffer
			sink := NewLogSink(log.New(&buf, "", 0))
			if err := sink.Record(Event{Kind: KindLoginFailure, Reason: tc.in}); err != nil {
				t.Fatal(err)
			}
			out := buf.String()
			if tc.keep != "" && !strings.Contains(out, tc.keep) {
				t.Fatalf("changed %q", out)
			}
			if tc.drop != "" && (strings.Contains(out, tc.drop) || !strings.Contains(out, "[redacted]")) {
				t.Fatalf("missed %q in %q", tc.drop, out)
			}
		})
	}
}
