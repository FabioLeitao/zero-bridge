package acptest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestFakeAgentInitializeAndPrompt(t *testing.T) {
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"session/new","params":{"cwd":"/tmp"}}` + "\n" +
		`{"jsonrpc":"2.0","id":3,"method":"session/prompt","params":{"sessionId":"s","prompt":[]}}` + "\n")
	var out bytes.Buffer
	runAgent(in, &out)
	sc := bufio.NewScanner(&out)
	var lines []string
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if len(lines) < 4 {
		t.Fatalf("expected updates+responses, got %d lines", len(lines))
	}
	var last map[string]any
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &last); err != nil {
		t.Fatal(err)
	}
	res, _ := last["result"].(map[string]any)
	if res["stopReason"] != "end_turn" {
		t.Fatalf("stopReason %v", res)
	}
}

func TestFakeTextAndKinds(t *testing.T) {
	t.Setenv("ZB_FAKE_TEXT", "PAYLOAD")
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"session/prompt","params":{"sessionId":"s"}}` + "\n")
	var out bytes.Buffer
	runAgent(in, &out)
	s := out.String()
	for _, want := range []string{"PAYLOAD", "agent_thought_chunk", "tool_call", "plan"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %s in %s", want, s)
		}
	}
}

func TestSlowMillisEnv(t *testing.T) {
	t.Setenv("ZB_FAKE_AGENT_SLOW_MS", "not-a-number")
	if slowMillis() != 0 {
		t.Fatal("expected 0")
	}
	t.Setenv("ZB_FAKE_AGENT_SLOW_MS", "5")
	if slowMillis() != 5 {
		t.Fatal("expected 5")
	}
}

func TestMainDoesNotRunWithoutEnv(t *testing.T) {
	// MaybeMain is a no-op in normal test runs.
	MaybeMain()
}
