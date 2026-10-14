package server

import (
	"io"
	"io/fs"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"zero-bridge/bridge/web"
)

func TestStaticChat(t *testing.T) {
	_, base, cleanup := startTestServer(t, nil)
	defer cleanup()
	res, err := http.Get(base + "/")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("status %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); ct != "text/html; charset=utf-8" {
		t.Fatalf("ct %q", ct)
	}
	if res.Header.Get("X-Frame-Options") != "DENY" || res.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Fatalf("headers %v", res.Header)
	}
	post, err := http.Post(base+"/", "text/plain", nil)
	if err != nil {
		t.Fatal(err)
	}
	post.Body.Close()
	if post.StatusCode != http.StatusMethodNotAllowed {
		t.Fatalf("post %d", post.StatusCode)
	}
	js, err := http.Get(base + "/app.js")
	if err != nil {
		t.Fatal(err)
	}
	js.Body.Close()
	if js.Header.Get("Content-Type") != "text/javascript; charset=utf-8" {
		t.Fatal(js.Header.Get("Content-Type"))
	}
	css, err := http.Get(base + "/style.css")
	if err != nil {
		t.Fatal(err)
	}
	css.Body.Close()
	if css.Header.Get("Content-Type") != "text/css; charset=utf-8" {
		t.Fatal(css.Header.Get("Content-Type"))
	}
	miss, err := http.Get(base + "/static")
	if err != nil {
		t.Fatal(err)
	}
	miss.Body.Close()
	if miss.StatusCode != http.StatusNotFound {
		t.Fatalf("static %d", miss.StatusCode)
	}
	req, _ := http.NewRequest(http.MethodGet, base+"/app.js", nil)
	req.Host = "evil.example:1"
	bad, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	bad.Body.Close()
	if bad.StatusCode != http.StatusForbidden {
		t.Fatalf("host %d", bad.StatusCode)
	}
}

func TestEmbedExactAndSinks(t *testing.T) {
	entries, err := fs.ReadDir(web.Files, ".")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("embed count %d", len(entries))
	}
	html, err := fs.ReadFile(web.Files, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	if regexp.MustCompile(`(?i)(^|[\\s<])on[a-z]+=`).Match(html) {
		t.Fatal("html event handler")
	}
	js, err := fs.ReadFile(web.Files, "app.js")
	if err != nil {
		t.Fatal(err)
	}
	blob := string(html) + string(js)
	for _, re := range []*regexp.Regexp{
		regexp.MustCompile(`innerHTML`),
		regexp.MustCompile(`outerHTML`),
		regexp.MustCompile(`insertAdjacentHTML`),
		regexp.MustCompile(`document\.write`),
		regexp.MustCompile(`eval\(`),
		regexp.MustCompile(`new Function`),
		regexp.MustCompile(`setTimeout\(\s*["']`),
		regexp.MustCompile(`setInterval\(\s*["']`),
	} {
		if re.MatchString(blob) {
			t.Fatalf("forbidden %s", re)
		}
	}
	if strings.Contains(blob, "http://") || strings.Contains(blob, "https://") {
		t.Fatal("absolute fetch")
	}
	_ = io.Discard
}
