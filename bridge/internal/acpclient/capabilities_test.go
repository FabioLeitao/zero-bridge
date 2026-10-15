package acpclient

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"testing"
	"time"
)

func TestFakeAgentCapabilities(t *testing.T) {
	c, done := startFakeClient(t)
	defer done()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, ok := c.InitializeSnapshot(); ok {
		t.Fatal("snapshot before initialize")
	}
	if err := c.Call(ctx, "initialize", map[string]any{"protocolVersion": 1}, &struct{}{}); err != nil {
		t.Fatal(err)
	}
	snap, ok := c.InitializeSnapshot()
	if !ok {
		t.Fatal("missing snapshot")
	}
	if !snap.Capabilities.LoadSession {
		t.Fatal("loadSession")
	}
	if snap.Capabilities.PromptCapabilities.Image {
		t.Fatal("image")
	}
	if snap.Capabilities.List() || snap.Capabilities.Resume() {
		t.Fatalf("session caps %+v", snap.Capabilities.SessionCapabilities)
	}
	var newRes struct {
		SessionID string `json:"sessionId"`
	}
	if err := c.Call(ctx, "session/new", map[string]any{"cwd": "/tmp", "mcpServers": []any{}}, &newRes); err != nil {
		t.Fatal(err)
	}
	offer, ok := c.SessionOffer(newRes.SessionID)
	if !ok {
		t.Fatal("missing offer")
	}
	if len(offer.Modes) != 0 || offer.CurrentModeID != "" || len(offer.ConfigOptions) != 0 {
		t.Fatalf("fake session/new declares no modes: %+v", offer)
	}
}

func TestZeroShapedCapabilities(t *testing.T) {
	stdoutR, stdoutW := io.Pipe()
	stdinR, stdinW := io.Pipe()
	go serveZeroShape(stdinR, stdoutW)
	c := AttachPipes(stdoutR, stdinW, nil)
	defer c.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := c.Call(ctx, "initialize", map[string]any{"protocolVersion": 1}, &struct{}{}); err != nil {
		t.Fatal(err)
	}
	snap, ok := c.InitializeSnapshot()
	if !ok {
		t.Fatal("missing snapshot")
	}
	if snap.ProtocolVersion != 1 || snap.AgentInfo.Name != "zero" {
		t.Fatalf("snap %+v", snap)
	}
	if !snap.Capabilities.LoadSession || !snap.Capabilities.PromptCapabilities.Image {
		t.Fatalf("caps %+v", snap.Capabilities)
	}
	if !snap.Capabilities.List() || !snap.Capabilities.Resume() {
		t.Fatal("list/resume")
	}
	var newRes struct {
		SessionID string `json:"sessionId"`
	}
	if err := c.Call(ctx, "session/new", map[string]any{"cwd": "/tmp/ws", "mcpServers": []any{}}, &newRes); err != nil {
		t.Fatal(err)
	}
	offer, ok := c.SessionOffer(newRes.SessionID)
	if !ok {
		t.Fatal("missing offer")
	}
	if offer.CurrentModeID != "auto" || len(offer.Modes) != 3 || offer.Modes[2].ID != "plan" {
		t.Fatalf("modes %+v", offer)
	}
	if len(offer.ConfigOptions) != 2 || offer.ConfigOptions[0].ID != "model" || offer.ConfigOptions[1].CurrentValue != "auto" {
		t.Fatalf("config %+v", offer.ConfigOptions)
	}
	if err := c.Call(ctx, "session/load", map[string]any{"sessionId": newRes.SessionID, "cwd": "/tmp/ws"}, &struct{}{}); err != nil {
		t.Fatal(err)
	}
	loaded, ok := c.SessionOffer(newRes.SessionID)
	if !ok || loaded.CurrentModeID != "ask" {
		t.Fatalf("loaded %+v", loaded)
	}
}

func serveZeroShape(r io.Reader, w io.Writer) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
		}
		if json.Unmarshal(sc.Bytes(), &msg) != nil || msg.Method == "" {
			continue
		}
		var result string
		switch msg.Method {
		case "initialize":
			result = `{"protocolVersion":1,"agentCapabilities":{"loadSession":true,"promptCapabilities":{"image":true,"audio":false,"embeddedContext":false},"sessionCapabilities":{"list":{},"resume":{}}},"agentInfo":{"name":"zero","version":"0"},"authMethods":[]}`
		case "session/new":
			result = `{"sessionId":"sess-shape","configOptions":[{"id":"model","name":"Model","type":"select","currentValue":"","options":null},{"id":"mode","name":"Mode","type":"select","currentValue":"auto","options":[{"value":"auto","name":"Auto"},{"value":"ask","name":"Ask"},{"value":"plan","name":"Plan"}]}],"modes":{"currentModeId":"auto","availableModes":[{"id":"auto","name":"Auto"},{"id":"ask","name":"Ask"},{"id":"plan","name":"Plan"}]}}`
		case "session/load":
			result = `{"modes":{"currentModeId":"ask","availableModes":[{"id":"ask","name":"Ask"}]},"configOptions":[{"id":"mode","name":"Mode","type":"select","currentValue":"ask","options":[]}]}`
		default:
			continue
		}
		b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(msg.ID), "result": json.RawMessage(result)})
		b = append(b, '\n')
		_, _ = w.Write(b)
	}
}
