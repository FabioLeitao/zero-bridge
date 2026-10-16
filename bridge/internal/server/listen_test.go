package server

import "testing"

func TestResolveListen(t *testing.T) {
	def, err := ResolveListen("")
	if err != nil || def.Network != "tcp" || def.Addr != "127.0.0.1:8765" {
		t.Fatalf("default %#v %v", def, err)
	}
	local, err := ResolveListen("localhost:9")
	if err != nil || local.Addr != "localhost:9" {
		t.Fatalf("localhost %#v %v", local, err)
	}
	if _, err := ResolveListen("0.0.0.0:8765"); err == nil {
		t.Fatal("non-loopback accepted")
	}
	if _, err := ResolveListen("unix:relative"); err == nil {
		t.Fatal("relative unix accepted")
	}
	if _, err := ResolveListen("npipe:\\\\.\\pipe\\zero-bridge"); err == nil {
		t.Fatal("named pipe accepted")
	}
	got, err := ResolveListen("unix:/tmp/zero-bridge.sock")
	if err != nil || got.Network != "unix" || got.Addr != "/tmp/zero-bridge.sock" {
		t.Fatalf("unix %#v %v", got, err)
	}
}
