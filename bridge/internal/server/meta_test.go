package server

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"zero-bridge/bridge/internal/sessionmeta"
)

func TestSessionMetaRoundTrip(t *testing.T) {
	st := openMeta(t)
	_, base, cleanup := startConfigured(t, Config{Meta: st})
	defer cleanup()

	hostile := `<img src=x onerror="window.__xss=1">`
	res := postMeta(t, base, "sess-1", `{"alias":`+mustJSON(t, hostile)+`,"pinned":true}`)
	if res.Alias != hostile || !res.Pinned || res.Archived || res.UpdatedAt == "" {
		t.Fatalf("meta %+v", res)
	}

	res = postMeta(t, base, "sess-1", `{"alias":null,"pinned":false}`)
	if res.Alias != "" || res.Pinned {
		t.Fatalf("cleared %+v", res)
	}
	if _, ok := st.Get("sess-1"); ok {
		t.Fatal("default entry kept")
	}

	res = postMeta(t, base, "sess-2", `{"archived":true}`)
	if !res.Archived || res.Alias != "" {
		t.Fatalf("archived %+v", res)
	}
}

func TestSessionMetaRejects(t *testing.T) {
	st := openMeta(t)
	_, base, cleanup := startConfigured(t, Config{Meta: st})
	defer cleanup()

	cases := []struct {
		name   string
		id     string
		body   string
		ctype  string
		status int
	}{
		{"bad id", url.PathEscape("bad id"), `{"alias":"a"}`, "application/json", http.StatusBadRequest},
		{"spaces", "ok", `{"alias":"   "}`, "application/json", http.StatusBadRequest},
		{"unknown", "ok", `{"nickname":"a"}`, "application/json", http.StatusBadRequest},
		{"type", "ok", `{"alias":"a"}`, "text/plain", http.StatusUnsupportedMediaType},
		{"empty alias", "ok", `{"alias":""}`, "application/json", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, base+"/v1/sessions/"+tc.id+"/meta", strings.NewReader(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", tc.ctype)
			res, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			b, _ := io.ReadAll(res.Body)
			if res.StatusCode != tc.status {
				t.Fatalf("status %d body %s", res.StatusCode, b)
			}
		})
	}
}

func TestSessionMetaUnavailableAndUnsupported(t *testing.T) {
	t.Run("no store", func(t *testing.T) {
		_, base, cleanup := startTestServer(t, nil)
		defer cleanup()
		status, body := postMetaRaw(t, base, "sess-1", `{"pinned":true}`)
		if status != http.StatusServiceUnavailable || !strings.Contains(body, "unavailable") {
			t.Fatalf("no store %d %s", status, body)
		}
	})
	t.Run("no list", func(t *testing.T) {
		st := openMeta(t)
		_, base, cleanup := startConfigured(t, Config{Meta: st, ExtraEnv: []string{"ZB_FAKE_NO_LIST=1"}})
		defer cleanup()
		status, body := postMetaRaw(t, base, "sess-1", `{"pinned":true}`)
		if status != http.StatusNotImplemented || !strings.Contains(body, "not supported by agent") {
			t.Fatalf("no list %d %s", status, body)
		}
	})
}

func TestSessionMetaPaddedBody(t *testing.T) {
	st := openMeta(t)
	_, base, cleanup := startConfigured(t, Config{Meta: st})
	defer cleanup()
	body := `{"alias":"x"}` + strings.Repeat(" ", 4096)
	status, raw := postMetaRaw(t, base, "sess-pad", body)
	if status != http.StatusBadRequest {
		t.Fatalf("status %d %s", status, raw)
	}
	if got, ok := st.Get("sess-pad"); ok {
		t.Fatalf("stored %+v", got)
	}
}

func TestSessionMetaBodyCap(t *testing.T) {
	st := openMeta(t)
	_, base, cleanup := startConfigured(t, Config{Meta: st})
	defer cleanup()
	body := `{"alias":"` + strings.Repeat("a", 5000) + `"}`
	status, raw := postMetaRaw(t, base, "sess-1", body)
	if status != http.StatusBadRequest {
		t.Fatalf("status %d %s", status, raw)
	}
}

func openMeta(t *testing.T) *sessionmeta.Store {
	t.Helper()
	st, err := sessionmeta.Open(filepath.Join(t.TempDir(), "state"), nil)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

type metaView struct {
	Alias     string `json:"alias"`
	Archived  bool   `json:"archived"`
	Pinned    bool   `json:"pinned"`
	UpdatedAt string `json:"updatedAt"`
}

func postMetaRaw(t *testing.T, base, id, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/v1/sessions/"+id+"/meta", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}

func postMeta(t *testing.T, base, id, body string) metaView {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, base+"/v1/sessions/"+id+"/meta", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d body %s", res.StatusCode, b)
	}
	var view metaView
	if err := json.Unmarshal(b, &view); err != nil {
		t.Fatal(err)
	}
	return view
}

func mustJSON(t *testing.T, v string) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
