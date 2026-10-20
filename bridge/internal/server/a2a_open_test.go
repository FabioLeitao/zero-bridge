package server

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestA2AMessageSendWithoutTokenStaysAllowed(t *testing.T) {
	_, base, stop := startConfigured(t, Config{})
	defer stop()
	body := `{"jsonrpc":"2.0","id":1,"method":"message/send","params":{"message":{"role":"user","parts":[{"type":"text","text":"hi"}]}}}`
	res, err := http.Post(base+"/a2a/message/send", "application/json", strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode == http.StatusUnauthorized {
		t.Fatalf("empty token rejected the send: %s", raw)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d body %s", res.StatusCode, raw)
	}
	if !strings.Contains(string(raw), `"completed"`) {
		t.Fatalf("body %s", raw)
	}
}
