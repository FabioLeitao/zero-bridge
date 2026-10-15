package workroot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckDir(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(root, "file")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	set, err := New([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := set.Check(sub); err != nil {
		t.Fatal(err)
	}
	if _, err := set.Check(root); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		path string
		want string
	}{
		{path: "relative", want: "absolute"},
		{path: root + "/../" + filepath.Base(root), want: "parent"},
		{path: filepath.Join(root, "missing"), want: "not found"},
		{path: file, want: "directory"},
		{path: link, want: "not allowed"},
		{path: root + "\x00", want: "NUL"},
	}
	for _, tc := range cases {
		_, err := set.Check(tc.path)
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("path %q err %v, want %s", tc.path, err, tc.want)
		}
		if strings.Contains(err.Error(), outside) {
			t.Fatalf("error leaked the resolved path: %v", err)
		}
	}
}

func TestResolveAdditional(t *testing.T) {
	root := t.TempDir()
	var extra []string
	for i := 0; i < 8; i++ {
		dir := filepath.Join(root, "d"+string(rune('a'+i)))
		if err := os.Mkdir(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		extra = append(extra, dir)
	}
	set, err := New([]string{root})
	if err != nil {
		t.Fatal(err)
	}
	cwd, got, err := set.Resolve(root, extra)
	if err != nil || cwd != root || len(got) != 8 {
		t.Fatalf("resolve %v %v %v", cwd, got, err)
	}
	if _, _, err := set.Resolve(root, append(extra, extra[0])); err == nil || !strings.Contains(err.Error(), "too many") {
		t.Fatalf("nine %v", err)
	}
	if _, _, err := set.Resolve(root, []string{extra[0], extra[0]}); err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("dup %v", err)
	}
}
