package acpclient

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"
)

func TestStartHelperAndDeclaredModes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	c, err := Start(ctx, Options{
		AgentCmd:  os.Args[0],
		AgentArgs: []string{"-test.run=^TestHelperProcess$"},
		ExtraEnv:  []string{"GO_WANT_HELPER_PROCESS=1"},
		Timeout:   5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	c.SetInboundHandler(DefaultInboundHandler{})
	if err := c.Call(ctx, "initialize", map[string]any{"protocolVersion": 1}, &struct{}{}); err != nil {
		t.Fatal(err)
	}
	if err := c.Call(ctx, "session/new", map[string]any{"cwd": "/tmp", "mcpServers": []any{}}, &struct{}{}); err != nil {
		t.Fatal(err)
	}
	_ = c.DeclaredModes()
	if err := c.Call(ctx, "nope", make(chan int), nil); err == nil {
		t.Fatal("unmarshalable params accepted")
	}
	select {
	case <-c.Done():
		t.Fatal("reader exited")
	default:
	}
	_ = c.StderrSnippet()
}

func TestStartMissingBinary(t *testing.T) {
	_, err := Start(context.Background(), Options{AgentCmd: "/no/such/zero-bridge-acp"})
	if err == nil {
		t.Fatal("missing binary started")
	}
}

func TestJSONRPCCodeAndLimitedWriter(t *testing.T) {
	if _, ok := JSONRPCCode(errors.New("plain")); ok {
		t.Fatal("plain error looked like jsonrpc")
	}
	re := &rpcError{Code: -32601, Message: "missing"}
	code, ok := JSONRPCCode(re)
	if !ok || code != -32601 || re.Error() == "" {
		t.Fatalf("code %d ok %v", code, ok)
	}
	var nilErr *rpcError
	if nilErr.Error() != "<nil rpc error>" {
		t.Fatal(nilErr.Error())
	}
	w := &limitedWriter{limit: 4}
	if n, err := w.Write([]byte("abcdef")); err != nil || n != 4 || w.String() != "abcd" {
		t.Fatalf("n %d err %v buf %q", n, err, w.String())
	}
	if _, err := w.Write([]byte("z")); err != nil || w.String() != "abcd" {
		t.Fatalf("overflow %q %v", w.String(), err)
	}
}

func TestSessionIDParam(t *testing.T) {
	if sessionIDParam(nil) != "" || sessionIDParam(make(chan int)) != "" || sessionIDParam(1) != "" {
		t.Fatal("expected empty session id")
	}
	if sessionIDParam(map[string]any{"sessionId": "s1"}) != "s1" {
		t.Fatal("session id")
	}
}
