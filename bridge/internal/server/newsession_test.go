package server

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewSessionForwardsAdditionalDirectories(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	extra := filepath.Join(root, "extra")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(extra, 0o755); err != nil {
		t.Fatal(err)
	}
	capture := filepath.Join(t.TempDir(), "capture")
	srv, base, cleanup := startTestServer(t, []string{"ZB_FAKE_CAPTURE=" + capture})
	defer cleanup()
	if err := srv.SetWorkspaceRoots([]string{root}); err != nil {
		t.Fatal(err)
	}
	body := `{"workspace":"` + sub + `","additionalDirectories":["` + extra + `"]}`
	res, err := http.Post(base+"/v1/sessions", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d %s", res.StatusCode, raw)
	}
	got, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	var msg struct {
		Params struct {
			Cwd   string   `json:"cwd"`
			Extra []string `json:"additionalDirectories"`
		} `json:"params"`
	}
	if err := json.Unmarshal(got, &msg); err != nil {
		t.Fatal(err)
	}
	if msg.Params.Cwd != sub || len(msg.Params.Extra) != 1 || msg.Params.Extra[0] != extra {
		t.Fatalf("acp params %+v", msg.Params)
	}

	_ = os.Remove(capture)
	outside := t.TempDir()
	bad := `{"workspace":"` + root + `","additionalDirectories":["` + outside + `"]}`
	res, err = http.Post(base+"/v1/sessions", "application/json", strings.NewReader(bad))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ = io.ReadAll(res.Body)
	if res.StatusCode != http.StatusBadRequest {
		t.Fatalf("escape %d %s", res.StatusCode, raw)
	}
	if strings.Contains(string(raw), outside) {
		t.Fatalf("response leaked the path: %s", raw)
	}
	if _, err := os.Stat(capture); !os.IsNotExist(err) {
		t.Fatal("rejected request still reached the agent")
	}
}

func TestNewSessionEmptyBodyUsesWorkspace(t *testing.T) {
	dir := t.TempDir()
	capture := filepath.Join(t.TempDir(), "capture")
	srv, base, cleanup := startConfigured(t, Config{
		Workspace: dir,
		ExtraEnv:  []string{"ZB_FAKE_CAPTURE=" + capture},
	})
	defer cleanup()
	if err := srv.PrepareWorkspace(); err != nil {
		t.Fatal(err)
	}
	if err := srv.SetWorkspaceRoots([]string{dir}); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, base+"/v1/sessions", nil)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d %s", res.StatusCode, raw)
	}
	got, err := os.ReadFile(capture)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), dir) {
		t.Fatalf("capture %s", got)
	}
}
