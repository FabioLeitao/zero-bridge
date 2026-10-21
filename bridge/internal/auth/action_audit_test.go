package auth

import (
	"bytes"
	"log"
	"strings"
	"testing"
)

func TestActionAuditDropsSecrets(t *testing.T) {
	secret := "sk-" + strings.Repeat("a", 20)
	sessionID := strings.Repeat("ab", 16)
	kinds := []Kind{
		KindPermissionAllow,
		KindPermissionDeny,
		KindPermissionExpire,
		KindPromptSubmitted,
		KindSessionCreated,
		KindIngress,
	}
	for _, kind := range kinds {
		var buf bytes.Buffer
		sink := NewLogSink(log.New(&buf, "", 0))
		err := sink.Record(Event{
			Kind:   kind,
			Source: secret,
			Method: "POST",
			Path:   sessionID,
			Reason: secret + " " + sessionID,
		})
		if err != nil {
			t.Fatal(err)
		}
		out := buf.String()
		for _, leak := range []string{secret, sessionID} {
			if strings.Contains(out, leak) {
				t.Fatalf("%s leaked %q in %s", kind, leak, out)
			}
		}
		if !strings.Contains(out, "[redacted]") {
			t.Fatalf("%s did not scrub: %s", kind, out)
		}
	}
}
