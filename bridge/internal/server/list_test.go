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

func TestListSearchArchiveAndPin(t *testing.T) {
	st, err := sessionmeta.Open(filepath.Join(t.TempDir(), "state"), nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Set("sess-a2", sessionmeta.Update{Alias: strPtr("Apelido"), Pinned: boolPtr(true)}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Set("sess-a1", sessionmeta.Update{Archived: boolPtr(true)}); err != nil {
		t.Fatal(err)
	}
	_, base, cleanup := startConfigured(t, Config{Meta: st})
	defer cleanup()

	page := getList(t, base, "/v1/sessions?cwd=/tmp/zbridge-ws-a&limit=2")
	if len(page.Sessions) != 2 || page.Sessions[0].ID != "sess-a2" || !page.Sessions[0].Pinned || page.Sessions[0].Alias != "Apelido" {
		t.Fatalf("pinned page %+v", page.Sessions)
	}
	for _, row := range page.Sessions {
		if row.ID == "sess-a1" {
			t.Fatal("archived row was included")
		}
	}

	only := getList(t, base, "/v1/sessions?cwd=/tmp/zbridge-ws-a&archived=only")
	if len(only.Sessions) != 1 || only.Sessions[0].ID != "sess-a1" || !only.Sessions[0].Archived {
		t.Fatalf("only %+v", only.Sessions)
	}
	if !strings.Contains(only.Sessions[0].Title, "<img") {
		t.Fatalf("hostile title %q", only.Sessions[0].Title)
	}

	found := getList(t, base, "/v1/sessions?q=apelido")
	if len(found.Sessions) != 1 || found.Sessions[0].ID != "sess-a2" {
		t.Fatalf("alias search %+v", found.Sessions)
	}
	byBase := getList(t, base, "/v1/sessions?q=ZBRIDGE-WS-B")
	if len(byBase.Sessions) != 1 || byBase.Sessions[0].ID != "sess-b1" {
		t.Fatalf("basename search %+v", byBase.Sessions)
	}

	badQ := strings.Repeat("x", 101)
	status, _ := getListStatus(t, base, "/v1/sessions?q="+badQ)
	if status != http.StatusBadRequest {
		t.Fatalf("long q %d", status)
	}
	status, _ = getListStatus(t, base, "/v1/sessions?archived=maybe")
	if status != http.StatusBadRequest {
		t.Fatalf("archived %d", status)
	}
	status, _ = getListStatus(t, base, "/v1/sessions?limit=101")
	if status != http.StatusBadRequest {
		t.Fatalf("limit %d", status)
	}
}

func TestListTruncatesAtPageCap(t *testing.T) {
	prev := listACPPageCap
	listACPPageCap = 1
	t.Cleanup(func() { listACPPageCap = prev })
	_, base, cleanup := startTestServer(t, nil)
	defer cleanup()
	page := getList(t, base, "/v1/sessions?cwd=/tmp/zbridge-ws-a")
	if !page.Truncated || page.Next == "" || len(page.Sessions) != 2 {
		t.Fatalf("truncated %+v n=%d", page.Sessions, len(page.Sessions))
	}
	rest := getList(t, base, "/v1/sessions?cwd=/tmp/zbridge-ws-a&cursor="+url.QueryEscape(page.Next))
	if rest.Truncated || len(rest.Sessions) != 2 || rest.Sessions[0].ID != "sess-a3" {
		t.Fatalf("rest %+v truncated %v", rest.Sessions, rest.Truncated)
	}
}

type listBody struct {
	Sessions []struct {
		ID       string `json:"sessionId"`
		Title    string `json:"title"`
		Alias    string `json:"alias"`
		Archived bool   `json:"archived"`
		Pinned   bool   `json:"pinned"`
	} `json:"sessions"`
	Next      string `json:"nextCursor"`
	Truncated bool   `json:"truncated"`
	Raw       string
}

func getList(t *testing.T, base, path string) listBody {
	t.Helper()
	status, raw := getListStatus(t, base, path)
	if status != http.StatusOK {
		t.Fatalf("GET %s %d %s", path, status, raw)
	}
	var page listBody
	if err := json.Unmarshal([]byte(raw), &page); err != nil {
		t.Fatal(err)
	}
	page.Raw = raw
	return page
}

func strPtr(s string) *string { return &s }
func boolPtr(v bool) *bool    { return &v }

func getListStatus(t *testing.T, base, path string) (int, string) {
	t.Helper()
	res, err := http.Get(base + path)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(b)
}
