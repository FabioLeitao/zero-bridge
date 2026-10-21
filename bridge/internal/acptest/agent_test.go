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

func TestSessionListPages(t *testing.T) {
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"session/list","params":{"cwd":"/tmp/zbridge-ws-a"}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"session/list","params":{"cwd":"/tmp/zbridge-ws-b"}}` + "\n" +
		`{"jsonrpc":"2.0","id":3,"method":"session/list","params":{}}` + "\n")
	var out bytes.Buffer
	runAgent(in, &out)
	lines := splitLines(t, out.String())
	if len(lines) != 3 {
		t.Fatalf("lines %d", len(lines))
	}
	var first struct {
		Result struct {
			Sessions []struct {
				SessionID string `json:"sessionId"`
				Title     string `json:"title"`
				Cwd       string `json:"cwd"`
			} `json:"sessions"`
			NextCursor string `json:"nextCursor"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatal(err)
	}
	if len(first.Result.Sessions) != 2 || first.Result.NextCursor != "sess-a2" {
		t.Fatalf("page %+v", first.Result)
	}
	if first.Result.Sessions[0].Title != hostilePayload() {
		t.Fatalf("title %q", first.Result.Sessions[0].Title)
	}
	var other struct {
		Result struct {
			Sessions []struct {
				Cwd string `json:"cwd"`
			} `json:"sessions"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &other); err != nil {
		t.Fatal(err)
	}
	if len(other.Result.Sessions) != 1 || other.Result.Sessions[0].Cwd != "/tmp/zbridge-ws-b" {
		t.Fatalf("filter %+v", other.Result.Sessions)
	}
	var all struct {
		Result struct {
			Sessions   []struct{} `json:"sessions"`
			NextCursor string     `json:"nextCursor"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[2]), &all); err != nil {
		t.Fatal(err)
	}
	if len(all.Result.Sessions) != 2 || all.Result.NextCursor == "" {
		t.Fatalf("all %+v", all.Result)
	}
	page2 := `{"jsonrpc":"2.0","id":4,"method":"session/list","params":{"cursor":"` + all.Result.NextCursor + `"}}` + "\n"
	var out2 bytes.Buffer
	runAgent(strings.NewReader(page2), &out2)
	if !strings.Contains(out2.String(), "sess-a3") {
		t.Fatalf("cursor page %s", out2.String())
	}
}

func TestSessionLoadAndResume(t *testing.T) {
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"session/load","params":{"sessionId":"sess-a1","cwd":"/tmp/zbridge-ws-a","mcpServers":[]}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"session/resume","params":{"sessionId":"sess-a1","cwd":"/tmp/zbridge-ws-a","mcpServers":[]}}` + "\n" +
		`{"jsonrpc":"2.0","id":3,"method":"session/load","params":{"sessionId":"missing"}}` + "\n" +
		`{"jsonrpc":"2.0","id":4,"method":"session/prompt","params":{"sessionId":"sess-view","prompt":[]}}` + "\n")
	var out bytes.Buffer
	runAgent(in, &out)
	s := out.String()
	loadAt := strings.Index(s, "replay-user")
	agentAt := strings.Index(s, "replay-agent")
	toolAt := strings.Index(s, "replay-tool")
	if loadAt < 0 || agentAt < loadAt || toolAt < agentAt {
		t.Fatalf("replay order %s", s)
	}
	lines := splitLines(t, s)
	var loadResult, resumeResult map[string]any
	for _, line := range lines {
		var msg map[string]any
		if json.Unmarshal([]byte(line), &msg) != nil {
			t.Fatal(line)
		}
		id, _ := msg["id"].(float64)
		switch id {
		case 1:
			loadResult = msg
		case 2:
			resumeResult = msg
			if strings.Contains(line, "session/update") {
				t.Fatal("resume replayed")
			}
		case 3:
			errObj, _ := msg["error"].(map[string]any)
			if errObj["code"].(float64) != -32602 {
				t.Fatalf("unknown %v", msg)
			}
		case 4:
			errObj, _ := msg["error"].(map[string]any)
			if errObj["message"] != "session is view-only" {
				t.Fatalf("view %v", msg)
			}
		}
	}
	if loadResult["result"] == nil || resumeResult["result"] == nil {
		t.Fatalf("missing results load=%v resume=%v", loadResult, resumeResult)
	}
	between := s[strings.Index(s, `"id":2`):strings.Index(s, `"id":3`)]
	if strings.Contains(between, "replay-user") {
		t.Fatalf("resume included replay %s", between)
	}
}

func splitLines(t *testing.T, s string) []string {
	t.Helper()
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
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

func TestSessionListOmitsCreatedWithoutEnv(t *testing.T) {
	in := strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"session/new","params":{"cwd":"/tmp/zbridge-ws-a","title":"kept-out"}}` + "\n" +
		`{"jsonrpc":"2.0","id":2,"method":"session/list","params":{"cwd":"/tmp/zbridge-ws-a"}}` + "\n")
	var out bytes.Buffer
	runAgent(in, &out)
	lines := splitLines(t, out.String())
	if len(lines) != 2 {
		t.Fatalf("lines %d", len(lines))
	}
	if strings.Contains(lines[1], "kept-out") {
		t.Fatalf("created title leaked into the catalog: %s", lines[1])
	}
	var page struct {
		Result struct {
			Sessions []struct {
				SessionID string `json:"sessionId"`
			} `json:"sessions"`
			NextCursor string `json:"nextCursor"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(lines[1]), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Result.Sessions) != 2 || page.Result.Sessions[0].SessionID != "sess-a1" || page.Result.NextCursor != "sess-a2" {
		t.Fatalf("catalog page %+v", page.Result)
	}
}

func TestCreatedSessionsListedNewestFirst(t *testing.T) {
	t.Setenv("ZB_FAKE_LIST", "created")
	stdinR, stdinW := io.Pipe()
	stdoutR, stdoutW := io.Pipe()
	go func() {
		runAgent(stdinR, stdoutW)
		_ = stdoutW.Close()
	}()
	t.Cleanup(func() { _ = stdinW.Close() })
	sc := bufio.NewScanner(stdoutR)
	write := func(line string) {
		t.Helper()
		if _, err := io.WriteString(stdinW, line+"\n"); err != nil {
			t.Fatal(err)
		}
	}
	readID := func(id float64) (map[string]any, string) {
		t.Helper()
		var seen strings.Builder
		for sc.Scan() {
			line := sc.Text()
			seen.WriteString(line)
			seen.WriteByte('\n')
			var msg map[string]any
			if json.Unmarshal([]byte(line), &msg) != nil {
				t.Fatalf("line %s", line)
			}
			got, _ := msg["id"].(float64)
			if got == id {
				return msg, seen.String()
			}
		}
		t.Fatalf("response %v: %v", id, sc.Err())
		return nil, ""
	}
	sessionID := func(msg map[string]any) string {
		t.Helper()
		res, _ := msg["result"].(map[string]any)
		id, _ := res["sessionId"].(string)
		if id == "" {
			t.Fatalf("session/new %v", msg)
		}
		return id
	}
	write(`{"jsonrpc":"2.0","id":1,"method":"session/new","params":{"cwd":"/tmp/created-a","title":"first"}}`)
	firstID := sessionID(func() map[string]any { msg, _ := readID(1); return msg }())
	write(`{"jsonrpc":"2.0","id":2,"method":"session/new","params":{"cwd":"/tmp/created-b","title":"second"}}`)
	secondID := sessionID(func() map[string]any { msg, _ := readID(2); return msg }())
	write(`{"jsonrpc":"2.0","id":3,"method":"session/list","params":{}}`)
	listed, _ := readID(3)
	res, _ := listed["result"].(map[string]any)
	raw, _ := res["sessions"].([]any)
	if len(raw) < 2 {
		t.Fatalf("list %v", listed)
	}
	type row struct {
		SessionID string `json:"sessionId"`
		Title     string `json:"title"`
		Cwd       string `json:"cwd"`
	}
	var sessions []row
	b, _ := json.Marshal(raw)
	if err := json.Unmarshal(b, &sessions); err != nil {
		t.Fatal(err)
	}
	if sessions[0].SessionID != secondID || sessions[0].Title != "second" || sessions[0].Cwd != "/tmp/created-b" {
		t.Fatalf("newest session = %+v, want id %s title second cwd /tmp/created-b", sessions[0], secondID)
	}
	if sessions[1].SessionID != firstID || sessions[1].Title != "first" || sessions[1].Cwd != "/tmp/created-a" {
		t.Fatalf("older session = %+v, want id %s title first cwd /tmp/created-a", sessions[1], firstID)
	}
	for _, s := range sessions {
		if s.SessionID == "sess-a1" {
			t.Fatalf("catalog replaced the created list: %+v", sessions)
		}
	}
	write(`{"jsonrpc":"2.0","id":4,"method":"session/load","params":{"sessionId":"` + secondID + `"}}`)
	_, loadText := readID(4)
	if !strings.Contains(loadText, "replay-user") || strings.Contains(loadText, "unknown session") {
		t.Fatalf("load %s", loadText)
	}
	write(`{"jsonrpc":"2.0","id":5,"method":"session/resume","params":{"sessionId":"` + secondID + `"}}`)
	resumed, resumeText := readID(5)
	if resumed["result"] == nil || strings.Contains(resumeText, "unknown session") {
		t.Fatalf("resume %s", resumeText)
	}
}
