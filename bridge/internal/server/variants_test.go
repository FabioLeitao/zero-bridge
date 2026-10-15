package server

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestNoPermissionVariant(t *testing.T) {
	_, base, cleanup := startTestServer(t, []string{"ZB_FAKE_NO_PERMISSION=1"})
	defer cleanup()
	dir := t.TempDir()
	res, err := http.Post(base+"/v1/sessions", "application/json", strings.NewReader(`{"workspace":"`+dir+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var created struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.NewDecoder(res.Body).Decode(&created); err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(http.MethodPost, base+"/v1/sessions/"+created.SessionID+"/prompt", strings.NewReader(`{"text":"hi"}`))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	posted, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	posted.Body.Close()
	if posted.StatusCode != http.StatusAccepted {
		t.Fatalf("prompt %d", posted.StatusCode)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	evReq, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1/sessions/"+created.SessionID+"/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	evRes, err := http.DefaultClient.Do(evReq)
	if err != nil {
		t.Fatal(err)
	}
	defer evRes.Body.Close()
	sawStop := false
	sawText := false
	sc := bufio.NewScanner(evRes.Body)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &obj); err != nil {
			t.Fatal(err)
		}
		if obj["type"] == "permission_request" {
			t.Fatalf("permission request %#v", obj)
		}
		if obj["type"] == "stop" {
			sawStop = true
			break
		}
		raw, _ := json.Marshal(obj)
		if strings.Contains(string(raw), `"text":"OK"`) {
			sawText = true
		}
	}
	if !sawStop || !sawText {
		t.Fatalf("stop %v text %v", sawStop, sawText)
	}
}
