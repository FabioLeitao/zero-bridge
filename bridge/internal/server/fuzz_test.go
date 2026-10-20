package server

import (
	"encoding/base64"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"zero-bridge/bridge/internal/workroot"
)

func FuzzListen(f *testing.F) {
	f.Add("")
	f.Add("127.0.0.1:8765")
	f.Add("[::1]:8765")
	f.Add("localhost:1")
	f.Add("unix:/tmp/bridge.sock")
	f.Add("npipe:pipe")
	f.Add("0.0.0.0:1")
	f.Fuzz(func(t *testing.T, addr string) {
		if len(addr) > 4096 {
			addr = addr[:4096]
		}
		_, _ = ResolveListen(addr)
	})
}

func FuzzLogin(f *testing.F) {
	f.Add(`{"token":"0123456789abcdef0123456789abcdef"}`)
	f.Add(`{`)
	f.Add(`{"token":1}`)
	f.Add(`{"extra":true}`)
	f.Add("")
	f.Fuzz(func(t *testing.T, body string) {
		if len(body) > 8192 {
			body = body[:8192]
		}
		srv := fuzzServer(t)
		req := httptest.NewRequest(http.MethodPost, "/v1/login", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		srv.handleLogin(httptest.NewRecorder(), req)
	})
}

func FuzzPromptBody(f *testing.F) {
	f.Add(`{"text":"hi"}`)
	f.Add(`{"images":[{"data":"zzzz","mimeType":"image/png"}]}`)
	f.Add(`{"text":"","images":[]}`)
	f.Add(`{"images":[{"data":"UklGRg==","mimeType":"image/webp"}]}`)
	f.Fuzz(func(t *testing.T, body string) {
		if len(body) > 32<<10 {
			body = body[:32<<10]
		}
		var parsed struct {
			Text   string            `json:"text"`
			Images []ImageAttachment `json:"images"`
		}
		if err := json.Unmarshal([]byte(body), &parsed); err != nil {
			return
		}
		srv := fuzzServer(t)
		for _, img := range parsed.Images {
			data, err := base64.StdEncoding.DecodeString(img.Data)
			if err != nil {
				continue
			}
			_ = srv.validateImage(data, img.MIME)
		}
	})
}

func FuzzA2A(f *testing.F) {
	f.Add(`{"jsonrpc":"2.0","id":1,"method":"message/send","params":{"message":{"role":"user","parts":[{"type":"text","text":"hi"}]}}}`)
	f.Add(`{`)
	f.Add(`{"jsonrpc":"1.0","id":1,"method":"message/send"}`)
	f.Add(`{"jsonrpc":"2.0","id":1,"method":"other","params":{}}`)
	f.Add(`{"jsonrpc":"2.0","id":1,"method":"message/send","params":{"message":{"role":"agent","parts":[{"type":"text","text":"hi"}]}}}`)
	f.Add(`{"jsonrpc":"2.0","id":1,"method":"message/send","params":{"message":{"role":"user","parts":[{"type":"file"}]}}}`)
	f.Fuzz(func(t *testing.T, body string) {
		if len(body) > 8192 {
			body = body[:8192]
		}
		srv := fuzzServer(t)
		req := httptest.NewRequest(http.MethodPost, "/a2a/message/send", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		srv.handleA2AMessageSend(httptest.NewRecorder(), req)
	})
}

func FuzzLimitFile(f *testing.F) {
	f.Add("session-ttl 1h\n")
	f.Add("# comment\n\n")
	f.Add("not-a-key 1\n")
	f.Add("session-ttl nope\n")
	f.Add("")
	f.Fuzz(func(t *testing.T, text string) {
		if len(text) > 8192 {
			text = text[:8192]
		}
		path := filepath.Join(t.TempDir(), "limits")
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		file, err := LoadLimitFile(path)
		if err != nil {
			return
		}
		_, _ = ResolveLimits(nil, nil, file)
	})
}

func FuzzRedact(f *testing.F) {
	f.Add("Bearer abcdefgh", "abcdefgh")
	f.Add("authorization: secret", "secret")
	f.Add("sk-short", "sk-short")
	f.Add(`{"a":"Bearer abcdefgh"}`, "abcdefgh")
	f.Add("plain", "")
	f.Fuzz(func(t *testing.T, text, secret string) {
		if len(text) > 8192 {
			text = text[:8192]
		}
		if len(secret) > 256 {
			secret = secret[:256]
		}
		_ = Redact(text)
		_ = RedactSecrets(text, secret)
		out := RedactJSON([]byte(text))
		if !json.Valid(out) {
			t.Fatalf("redact json invalid")
		}
	})
}

func FuzzWorkspace(f *testing.F) {
	root := os.TempDir()
	set, err := workroot.New([]string{root})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(root, root)
	f.Add("relative", "..")
	f.Add("/", "/etc")
	f.Fuzz(func(t *testing.T, path, extra string) {
		if len(path) > 4096 {
			path = path[:4096]
		}
		if len(extra) > 4096 {
			extra = extra[:4096]
		}
		_ = validateWorkspace(path)
		_, _ = set.Check(path)
		_, _, _ = set.Resolve(path, []string{extra})
	})
}

func fuzzServer(t *testing.T) *Server {
	t.Helper()
	srv, err := New(Config{
		Listen:            "127.0.0.1:9",
		LoginFailureDelay: -1,
		Token:             "0123456789abcdef0123456789abcdef",
		Logger:            log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	return srv
}
