package zeroenv

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProviderEnvWithGraniteListed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"models":[{"name":"granite4.2:3b-cpu-ctx16k"}]}`))
	}))
	defer srv.Close()

	env := ProviderEnvWith(Config{TagsURL: srv.URL, Client: srv.Client()})
	if len(env) != 1 || env[0] != "ZERO_PROVIDER=granite-local" {
		t.Fatalf("env %v", env)
	}
}

func TestProviderEnvWithoutGranite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`{"models":[{"name":"other:tag"}]}`))
	}))
	defer srv.Close()

	env := ProviderEnvWith(Config{TagsURL: srv.URL, Client: srv.Client()})
	if env != nil {
		t.Fatalf("expected nil env, got %v", env)
	}
}

func TestProviderEnvOllamaDown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	if env := ProviderEnvWith(Config{TagsURL: srv.URL, Client: srv.Client()}); env != nil {
		t.Fatalf("expected nil, got %v", env)
	}
}
