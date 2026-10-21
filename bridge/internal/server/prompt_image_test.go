package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func imageFixture(kind string, n int) ([]byte, string) {
	if n < 12 {
		n = 12
	}
	buf := make([]byte, n)
	switch kind {
	case "png":
		copy(buf, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A})
		return buf, "image/png"
	case "jpeg":
		copy(buf, []byte{0xFF, 0xD8, 0xFF})
		return buf, "image/jpeg"
	case "webp":
		copy(buf, []byte{0x52, 0x49, 0x46, 0x46})
		copy(buf[8:12], []byte{0x57, 0x45, 0x42, 0x50})
		return buf, "image/webp"
	case "gif":
		copy(buf, []byte{0x47, 0x49, 0x46, 0x38, 0x39, 0x61})
		return buf, "image/gif"
	default:
		panic(kind)
	}
}

func TestPromptImageBlockKeepsBase64(t *testing.T) {
	for _, kind := range []string{"png", "jpeg", "webp", "gif"} {
		t.Run(kind, func(t *testing.T) {
			raw, mime := imageFixture(kind, 32)
			b64 := base64.StdEncoding.EncodeToString(raw)
			capture := filepath.Join(t.TempDir(), "prompt")
			_, base, stop := startTestServer(t, []string{"ZB_FAKE_CAPTURE_PROMPT=" + capture})
			defer stop()
			sid := createSession(t, base, t.TempDir())
			status, body := postPromptJSON(t, base, sid, map[string]any{
				"text": "look",
				"images": []map[string]string{{
					"data":     b64,
					"mimeType": mime,
				}},
			})
			if status != http.StatusAccepted {
				t.Fatalf("status %d %s", status, body)
			}
			got := imageDataFromCapture(t, capture)
			if got != b64 || strings.ContainsRune(got, '\uFFFD') || !utf8.ValidString(got) {
				t.Fatalf("agent image data is not the submitted base64 (has U+FFFD: %v)", strings.ContainsRune(got, '\uFFFD'))
			}
		})
	}
}

func TestPromptFourImagesAtLimitAreAccepted(t *testing.T) {
	srv, base, stop := startTestServer(t, nil)
	defer stop()
	limit := int(srv.cfg.MaxImageBytes)
	images := make([]map[string]string, 4)
	for i, kind := range []string{"png", "jpeg", "webp", "gif"} {
		raw, mime := imageFixture(kind, limit)
		images[i] = map[string]string{
			"data":     base64.StdEncoding.EncodeToString(raw),
			"mimeType": mime,
		}
	}
	sid := createSession(t, base, t.TempDir())
	status, body := postPromptJSON(t, base, sid, map[string]any{"text": "four", "images": images})
	if status != http.StatusAccepted {
		t.Fatalf("status %d %s", status, body)
	}
}

func TestPromptBodyOverCapIsClear(t *testing.T) {
	_, base, stop := startConfigured(t, Config{MaxImageBytes: 32, MaxImages: 1})
	defer stop()
	sid := createSession(t, base, t.TempDir())
	raw := `{"text":"` + strings.Repeat("a", int(promptBodyLimit(32, 1)))
	res, err := http.Post(base+"/v1/sessions/"+sid+"/prompt", "application/json", strings.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), "body too large") {
		t.Fatalf("status %d %s", res.StatusCode, body)
	}
}

func TestPromptFifthImageIsRefused(t *testing.T) {
	_, base, stop := startTestServer(t, nil)
	defer stop()
	images := make([]map[string]string, 5)
	for i := range images {
		raw, mime := imageFixture("png", 12)
		images[i] = map[string]string{
			"data":     base64.StdEncoding.EncodeToString(raw),
			"mimeType": mime,
		}
	}
	sid := createSession(t, base, t.TempDir())
	status, body := postPromptJSON(t, base, sid, map[string]any{"images": images})
	if status != http.StatusBadRequest || !strings.Contains(body, "too many images") {
		t.Fatalf("status %d %s", status, body)
	}
}

func TestPromptImageOverDecodedLimitIsRefused(t *testing.T) {
	srv, base, stop := startTestServer(t, nil)
	defer stop()
	raw, mime := imageFixture("png", int(srv.cfg.MaxImageBytes)+1)
	sid := createSession(t, base, t.TempDir())
	status, body := postPromptJSON(t, base, sid, map[string]any{
		"images": []map[string]string{{
			"data":     base64.StdEncoding.EncodeToString(raw),
			"mimeType": mime,
		}},
	})
	if status != http.StatusBadRequest || !strings.Contains(body, "exceeds limit") {
		t.Fatalf("status %d %s", status, body)
	}
}

func postPromptJSON(t *testing.T, base, sid string, payload any) (int, string) {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	res, err := http.Post(base+"/v1/sessions/"+sid+"/prompt", "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, string(body)
}

func imageDataFromCapture(t *testing.T, path string) string {
	t.Helper()
	var raw []byte
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		b, err := os.ReadFile(path)
		if err == nil && len(bytes.TrimSpace(b)) > 0 {
			raw = b
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if len(raw) == 0 {
		t.Fatal("agent did not capture the prompt")
	}
	var env struct {
		Params struct {
			Prompt []struct {
				Type string `json:"type"`
				Data string `json:"data"`
			} `json:"prompt"`
		} `json:"params"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	for _, block := range env.Params.Prompt {
		if block.Type == "image" {
			return block.Data
		}
	}
	t.Fatalf("no image block in %s", raw)
	return ""
}
