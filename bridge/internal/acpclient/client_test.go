package acpclient

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") == "1" {
		runFakeAgent()
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestHelperProcess(t *testing.T) {
	t.Skip("re-exec helper for fake zero acp")
}

func startFakeClient(t *testing.T) (*Client, func()) {
	return startFakeClientOpts(t, Options{})
}

func startFakeClientOpts(t *testing.T, opts Options) (*Client, func()) {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	c := attachPipes(stdout, stdin, cmd, opts)
	return c, func() {
		_ = c.Close()
	}
}

func TestNotifyOptionBeforeInitialize(t *testing.T) {
	var got atomic.Int32
	c, done := startFakeClientOpts(t, Options{
		Notify: func(method string, _ json.RawMessage) {
			if method == "session/update" {
				got.Add(1)
			}
		},
	})
	defer done()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Call(ctx, "initialize", map[string]any{
		"protocolVersion":    1,
		"clientCapabilities": map[string]any{"fs": map[string]bool{"readTextFile": false, "writeTextFile": false}},
	}, &struct{}{}); err != nil {
		t.Fatal(err)
	}
	if got.Load() < 1 {
		t.Fatalf("expected warmup notification, got %d", got.Load())
	}
}

func TestFramingAndCall(t *testing.T) {
	c, done := startFakeClient(t)
	defer done()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var initRes struct {
		ProtocolVersion int `json:"protocolVersion"`
	}
	if err := c.Call(ctx, "initialize", map[string]any{
		"protocolVersion":    1,
		"clientCapabilities": map[string]any{"fs": map[string]bool{"readTextFile": false, "writeTextFile": false}},
	}, &initRes); err != nil {
		t.Fatal(err)
	}
	if initRes.ProtocolVersion != 1 {
		t.Fatalf("protocol %d", initRes.ProtocolVersion)
	}
}

func TestNotificationCallback(t *testing.T) {
	c, done := startFakeClient(t)
	defer done()
	var got int
	c.SetNotifyHandler(func(method string, _ json.RawMessage) {
		if method == "session/update" {
			got++
		}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = c.Call(ctx, "initialize", map[string]any{
		"protocolVersion":    1,
		"clientCapabilities": map[string]any{"fs": map[string]bool{"readTextFile": false, "writeTextFile": false}},
	}, &struct{}{})
	var newRes struct {
		SessionID string `json:"sessionId"`
	}
	if err := c.Call(ctx, "session/new", map[string]any{"cwd": "/tmp", "mcpServers": []any{}}, &newRes); err != nil {
		t.Fatal(err)
	}
	var promptRes struct {
		StopReason string `json:"stopReason"`
	}
	if err := c.Call(ctx, "session/prompt", map[string]any{
		"sessionId": newRes.SessionID,
		"prompt":    []map[string]string{{"type": "text", "text": "hi"}},
	}, &promptRes); err != nil {
		t.Fatal(err)
	}
	if got < 1 {
		t.Fatalf("expected session/update notification, got %d", got)
	}
	if promptRes.StopReason != "end_turn" {
		t.Fatalf("stopReason %q", promptRes.StopReason)
	}
}

func TestPermissionDenied(t *testing.T) {
	h := DefaultInboundHandler{}
	out, err := h.HandleRequest(methodSessionRequestPerm, json.RawMessage(`{"sessionId":"s1"}`))
	if err != nil {
		t.Fatal(err)
	}
	b, _ := json.Marshal(out)
	if !json.Valid(b) {
		t.Fatalf("invalid json %s", b)
	}
	var decoded struct {
		Outcome struct {
			Outcome  string `json:"outcome"`
			OptionID string `json:"optionId"`
		} `json:"outcome"`
	}
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.Outcome.OptionID != permRejectOnce {
		t.Fatalf("option %q", decoded.Outcome.OptionID)
	}
}

func TestFSRejected(t *testing.T) {
	h := DefaultInboundHandler{}
	_, err := h.HandleRequest("fs/read_text_file", json.RawMessage(`{"path":"x"}`))
	if err == nil {
		t.Fatal("expected error")
	}
	if re, ok := err.(*rpcError); !ok || re.Code != codeMethodNotFound {
		t.Fatalf("got %v", err)
	}
}

func TestCallTimeout(t *testing.T) {
	pr, pw := io.Pipe()
	c := AttachPipes(pr, pw, exec.Command("sleep", "60"))
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	err := c.Call(ctx, "initialize", map[string]any{}, &struct{}{})
	if err == nil {
		t.Fatal("expected timeout")
	}
	_ = pw.Close()
	_ = c.Close()
}

func TestCallEOFError(t *testing.T) {
	stdoutR, stdoutW := io.Pipe()
	stdinR, stdinW := io.Pipe()
	go func() { _, _ = io.Copy(io.Discard, stdinR) }()
	c := AttachPipes(stdoutR, stdinW, nil)
	_ = stdoutW.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := c.Call(ctx, "ping", nil, &struct{}{})
	if err == nil {
		t.Fatal("expected error when agent closes stdout without response")
	}
	_ = c.Close()
}

func TestCallRace200Pings(t *testing.T) {
	c, done := startFakeClient(t)
	defer done()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for i := 0; i < 200; i++ {
		if err := c.Call(ctx, "ping", nil, &struct{}{}); err != nil {
			t.Fatalf("ping %d: %v", i, err)
		}
	}
}

func TestCloseIdempotent(t *testing.T) {
	c, _ := startFakeClient(t)
	if err := c.Close(); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}
