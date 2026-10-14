package acptest

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
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

func TestFakeSetMode(t *testing.T) {
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"session/new","params":{"cwd":"/tmp"}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"session/set_mode","params":{"sessionId":"s","modeId":"ask"}}` + "\n")
	var out bytes.Buffer
	runAgent(in, &out)
	s := out.String()
	if !strings.Contains(s, `"currentModeId":"auto"`) || !strings.Contains(s, `"id":"plan"`) {
		t.Fatalf("session/new modes %s", s)
	}
	if !strings.Contains(s, `"currentModeId":"ask"`) || !strings.Contains(s, "current_mode_update") {
		t.Fatalf("set_mode %s", s)
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

func TestRichScenario(t *testing.T) {
	t.Setenv("ZB_FAKE_SCENARIO", "rich")
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"session/prompt","params":{"sessionId":"s"}}` + "\n")
	var out bytes.Buffer
	runAgent(in, &out)
	s := out.String()
	hostile := hostileJSON(hostilePayload())
	for _, want := range []string{
		"agent_thought_chunk",
		`"toolCallId":"tc-text"`,
		"tool_call_update",
		`"status":"completed"`,
		`"type":"content"`,
		`"type":"diff"`,
		`"oldText":"old"`,
		`"sessionUpdate":"plan"`,
		`"status":"pending"`,
		`"status":"in_progress"`,
		hostile,
		"rich-done",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q in %s", want, s)
		}
	}
}

func TestRichPermissionHostile(t *testing.T) {
	t.Setenv("ZB_FAKE_SCENARIO", "rich")
	t.Setenv("ZB_FAKE_PERMISSION", "1")
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	go runAgent(stdinR, stdoutW)
	if _, err := io.WriteString(stdinW, `{"jsonrpc":"2.0","id":1,"method":"session/prompt","params":{"sessionId":"s"}}`+"\n"); err != nil {
		t.Fatal(err)
	}
	sc := bufio.NewScanner(stdoutR)
	hostile := hostileJSON(hostilePayload())
	var sawReq bool
	var final strings.Builder
	for sc.Scan() {
		line := sc.Text()
		final.WriteString(line)
		final.WriteByte('\n')
		if !strings.Contains(line, "session/request_permission") {
			if strings.Contains(line, "end_turn") {
				break
			}
			continue
		}
		sawReq = true
		if !strings.Contains(line, hostile) {
			t.Fatalf("permission missing hostile text: %s", line)
		}
		var msg struct {
			ID json.RawMessage `json:"id"`
		}
		if json.Unmarshal([]byte(line), &msg) != nil || len(msg.ID) == 0 {
			t.Fatal("permission id")
		}
		if _, err := io.WriteString(stdinW, `{"jsonrpc":"2.0","id":`+string(msg.ID)+`,"result":{"outcome":{"outcome":"selected","optionId":"deny"}}}`+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	if !sawReq {
		t.Fatal("no permission request")
	}
	out := final.String()
	if !strings.Contains(out, `"type":"diff"`) || !strings.Contains(out, hostile) || !strings.Contains(out, "DENIED optionId=deny") {
		t.Fatalf("transcript %s", out)
	}
}

func hostileJSON(s string) string {
	b, _ := json.Marshal(s)
	return string(b[1 : len(b)-1])
}

func TestPermissionRound(t *testing.T) {
	pr, pw := io.Pipe()
	or, ow := io.Pipe()
	reqCh := make(chan string, 1)
	go func() { reqCh <- replySelected(pw, or, "deny") }()
	got := permissionRound(bufio.NewReader(pr), ow, "s", false)
	req := <-reqCh
	if got != "DENIED optionId=deny" {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(req, "always_allow") || !strings.Contains(req, `"optionId":"allow"`) {
		t.Fatalf("request %s", req)
	}
}

// replySelected answers the permission request with the id that request used.
// nextPermID is process-global, so a hardcoded id fails on later -count runs.
func replySelected(w io.WriteCloser, req io.Reader, optionID string) string {
	sc := bufio.NewScanner(req)
	if !sc.Scan() {
		_ = w.Close()
		return ""
	}
	line := sc.Text()
	var msg struct {
		ID json.RawMessage `json:"id"`
	}
	if json.Unmarshal([]byte(line), &msg) != nil || len(msg.ID) == 0 {
		_ = w.Close()
		return line
	}
	_, _ = io.WriteString(w, `{"jsonrpc":"2.0","id":`+string(msg.ID)+`,"result":{"outcome":{"outcome":"selected","optionId":"`+optionID+`"}}}`+"\n")
	_ = w.Close()
	return line
}

func TestPermissionRoundAlwaysOptions(t *testing.T) {
	resp := `{"jsonrpc":"2.0","id":1,"result":{"outcome":{"outcome":"selected","optionId":"deny"}}}` + "\n"
	pr, pw := io.Pipe()
	go func() {
		_, _ = io.WriteString(pw, resp)
		_ = pw.Close()
	}()
	var out bytes.Buffer
	_ = permissionRound(bufio.NewReader(pr), &out, "s", true)
	if strings.Contains(out.String(), `"optionId":"allow"`) {
		t.Fatal("allow_once should be omitted")
	}
	if !strings.Contains(out.String(), "always_allow") {
		t.Fatal(out.String())
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
