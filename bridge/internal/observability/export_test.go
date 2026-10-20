package observability

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPlaintextExportRequiresOptIn(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "")
	if err := UseEndpoint("http://127.0.0.1:9"); err == nil {
		t.Fatal("plaintext export accepted")
	}
	if err := UseEndpoint("http://user:secret@127.0.0.1:9"); err == nil {
		t.Fatal("credentials in the endpoint accepted")
	}
}

func TestExportDropsPromptPathTokenAndSession(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_INSECURE", "true")
	got := make(chan string, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(io.LimitReader(r.Body, 1<<16))
		got <- string(b)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()
	if err := UseEndpoint(srv.URL); err != nil {
		t.Fatal(err)
	}
	const (
		prompt  = "PROMPT-MARKER"
		path    = "PATH-MARKER"
		token   = "TOKEN-MARKER"
		session = "SESSION-MARKER"
	)
	ctx, end := StartHTTPSpan(context.Background(), "/v1/sessions/{id}/prompt", "POST", WithAttributes(map[string]string{
		"prompt":     prompt,
		"path":       path,
		"token":      token,
		"session.id": session,
		"http.route": "/tmp/" + path + "/" + session,
	}))
	end()
	RecordHTTPMetric("/v1/sessions/{id}/prompt", "POST", 202, 1.5)
	RecordACPMetric("session/prompt", 2, nil)
	LogWithContext(ctx, "info", prompt, "token", token, "path", path, "session", session)
	deadline := time.After(2 * time.Second)
	var body string
	for len(body) == 0 || !strings.Contains(body, "session/prompt") {
		select {
		case part := <-got:
			body += part
		case <-deadline:
			t.Fatalf("export %q", body)
		}
	}
	for _, marker := range []string{prompt, path, token, session} {
		if strings.Contains(body, marker) {
			t.Fatalf("exported %s in %s", marker, body)
		}
	}
	if !strings.Contains(body, "/v1/sessions/{id}/prompt") || !strings.Contains(body, `"http.method":"POST"`) {
		t.Fatalf("missing allowed fields: %s", body)
	}
}
