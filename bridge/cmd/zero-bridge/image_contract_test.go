package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestRuntimeImageContract(t *testing.T) {
	root := repoRoot(t)
	docker := string(mustRead(t, filepath.Join(root, "Dockerfile")))
	runtime := docker[strings.LastIndex(docker, "FROM "):]
	if strings.Contains(runtime, "acpfake") {
		t.Fatal("runtime stage still mentions acpfake")
	}
	if strings.Contains(runtime, "wget") {
		t.Fatal("HEALTHCHECK still depends on wget")
	}
	if !strings.Contains(runtime, "--healthcheck") {
		t.Fatal("HEALTHCHECK does not call zero-bridge --healthcheck")
	}
	compose := string(mustRead(t, filepath.Join(root, "docker-compose.yml")))
	for _, needle := range []string{"read_only: true", "cap_drop:", "- ALL", "no-new-privileges:true"} {
		if !strings.Contains(compose, needle) {
			t.Fatalf("compose missing %q", needle)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("no caller")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(filepath.Dir(file))))
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
