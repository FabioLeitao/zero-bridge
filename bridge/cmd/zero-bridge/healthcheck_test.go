package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthcheckOK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("{\"ok\":true}\n"))
	}))
	t.Cleanup(srv.Close)
	if code := healthcheck(srv.URL, io.Discard); code != 0 {
		t.Fatalf("exit %d", code)
	}
}

func TestHealthcheckRejectsDownAndRedirect(t *testing.T) {
	var stderr strings.Builder
	if code := healthcheck("http://127.0.0.1:1/health", &stderr); code != 1 {
		t.Fatalf("down %d", code)
	}
	if stderr.Len() == 0 {
		t.Fatal("no error")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/go" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("{\"ok\":true}\n"))
			return
		}
		http.Redirect(w, r, "/go", http.StatusFound)
	}))
	t.Cleanup(srv.Close)
	if code := healthcheck(srv.URL, io.Discard); code != 1 {
		t.Fatal("redirect followed")
	}
}
