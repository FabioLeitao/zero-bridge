// Package acptest provides a reusable fake zero acp agent for integration tests.
package acptest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

// Main runs the fake agent on stdio. Call from TestMain when invoked as `testbinary acp` with ZB_FAKE_AGENT=1.
func Main() {
	runAgent(os.Stdin, os.Stdout)
}

var nextSessionID atomic.Int64
var nextPermID atomic.Int64

func runAgent(r io.Reader, w io.Writer) {
	reader := bufio.NewReader(r)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}
		var env struct {
			JSONRPC string          `json:"jsonrpc"`
			ID      json.RawMessage `json:"id"`
			Method  string          `json:"method"`
		}
		if json.Unmarshal(line, &env) != nil || env.Method == "" || env.ID == nil {
			continue
		}
		switch env.Method {
		case "initialize":
			writeResp(w, env.ID, map[string]any{
				"protocolVersion":   1,
				"agentCapabilities": fakeCapabilities(),
				"agentInfo": map[string]string{
					"name":    fakeAgentName(),
					"version": fakeAgentVersion(),
				},
			})
			if os.Getenv("ZB_FAKE_AGENT_EXIT") == "1" {
				os.Exit(0)
			}
		case "session/new":
			n := nextSessionID.Add(1)
			body := map[string]any{"sessionId": fmt.Sprintf("sess-acptest-%d", n)}
			if ids := os.Getenv("ZB_FAKE_MODE_IDS"); ids != "" {
				var modes []map[string]string
				for _, id := range strings.Split(ids, ",") {
					id = strings.TrimSpace(id)
					if id == "" {
						continue
					}
					modes = append(modes, map[string]string{"id": id, "name": id})
				}
				if len(modes) > 0 {
					if name := os.Getenv("ZB_FAKE_MODE_NAME"); name != "" {
						modes[0]["name"] = name
					}
					if desc := os.Getenv("ZB_FAKE_MODE_DESC"); desc != "" {
						modes[0]["description"] = desc
					}
					body["modes"] = map[string]any{
						"currentModeId":  modes[0]["id"],
						"availableModes": modes,
					}
				}
			} else if os.Getenv("ZB_FAKE_NO_MODES") != "1" {
				body["modes"] = modeResult("auto")["modes"]
			}
			writeResp(w, env.ID, body)
		case "session/set_mode":
			var setReq struct {
				Params struct {
					SessionID string `json:"sessionId"`
					ModeID    string `json:"modeId"`
				} `json:"params"`
			}
			_ = json.Unmarshal(line, &setReq)
			notify(w, "session/update", map[string]any{
				"sessionId": setReq.Params.SessionID,
				"update": map[string]any{
					"sessionUpdate": "current_mode_update",
					"currentModeId": setReq.Params.ModeID,
				},
			})
			writeResp(w, env.ID, map[string]any{})
		case "session/list":
			if os.Getenv("ZB_FAKE_LIST_ERROR") == "1" {
				msg := "list failed Bearer secret-token"
				if echo := os.Getenv("ZB_FAKE_ERROR_ECHO"); echo != "" {
					msg += " " + echo
				}
				writeErr(w, env.ID, -32603, msg)
				continue
			}
			var listReq struct {
				Params struct {
					Cwd    string `json:"cwd"`
					Cursor string `json:"cursor"`
				} `json:"params"`
			}
			_ = json.Unmarshal(line, &listReq)
			sessions, next := listPage(listReq.Params.Cwd, listReq.Params.Cursor)
			writeResp(w, env.ID, map[string]any{"sessions": sessions, "nextCursor": next})
		case "session/load":
			if os.Getenv("ZB_FAKE_LOAD_ERROR") == "1" {
				writeErr(w, env.ID, -32603, "load failed Bearer secret-token")
				continue
			}
			var loadReq struct {
				Params struct {
					SessionID string `json:"sessionId"`
				} `json:"params"`
			}
			_ = json.Unmarshal(line, &loadReq)
			sess, ok := findPersisted(loadReq.Params.SessionID)
			if !ok {
				writeErr(w, env.ID, -32602, "unknown session")
				continue
			}
			if ms := envMillis("ZB_FAKE_LOAD_SLOW_MS"); ms > 0 && !sleepUnlessClosed(r, ms) {
				return
			}
			replaySession(w, sess)
			writeResp(w, env.ID, modeResult("ask"))
		case "session/resume":
			var resumeReq struct {
				Params struct {
					SessionID string `json:"sessionId"`
				} `json:"params"`
			}
			_ = json.Unmarshal(line, &resumeReq)
			if _, ok := findPersisted(resumeReq.Params.SessionID); !ok {
				writeErr(w, env.ID, -32602, "unknown session")
				continue
			}
			writeResp(w, env.ID, modeResult("auto"))
		case "session/prompt":
			sid := "sess-acptest-1"
			var req struct {
				Params struct {
					SessionID string `json:"sessionId"`
				} `json:"params"`
			}
			if json.Unmarshal(line, &req) == nil && req.Params.SessionID != "" {
				sid = req.Params.SessionID
			}
			if sess, ok := findPersisted(sid); ok && sess.viewOnly {
				msg := "session is view-only"
				if os.Getenv("ZB_FAKE_VIEWONLY_SECRET") == "1" {
					msg += " Bearer secret-token"
				}
				writeErr(w, env.ID, -32002, msg)
				continue
			}
			if ms := slowMillis(); ms > 0 && !sleepUnlessClosed(r, ms) {
				return
			}
			if os.Getenv("ZB_FAKE_NO_PERMISSION") == "1" {
				notify(w, "session/update", map[string]any{
					"sessionId": sid,
					"update": map[string]any{
						"sessionUpdate": "agent_message_chunk",
						"content":       map[string]string{"type": "text", "text": "OK"},
					},
				})
				writeResp(w, env.ID, map[string]any{"stopReason": "end_turn"})
				continue
			}
			if os.Getenv("ZB_FAKE_SCENARIO") == "rich" {
				richPrompt(reader, w, sid, env.ID)
				continue
			}
			if os.Getenv("ZB_FAKE_PERMISSION") == "1" {
				n := 1
				if os.Getenv("ZB_FAKE_PERMISSION_N") == "2" {
					n = 2
				}
				onlyAlways := os.Getenv("ZB_FAKE_PERMISSION_OPTIONS") == "always"
				var parts []string
				for i := 0; i < n; i++ {
					parts = append(parts, permissionRound(reader, w, sid, onlyAlways))
				}
				notify(w, "session/update", map[string]any{
					"sessionId": sid,
					"update": map[string]any{
						"sessionUpdate": "agent_message_chunk",
						"content":       map[string]string{"type": "text", "text": strings.Join(parts, "\n")},
					},
				})
				writeResp(w, env.ID, map[string]any{"stopReason": "end_turn"})
				continue
			}
			text := os.Getenv("ZB_FAKE_TEXT")
			if text == "" {
				text = "OK"
			}
			notify(w, "session/update", map[string]any{
				"sessionId": sid,
				"update": map[string]any{
					"sessionUpdate": "agent_thought_chunk",
					"content":       map[string]string{"type": "text", "text": "thinking"},
				},
			})
			notify(w, "session/update", map[string]any{
				"sessionId": sid,
				"update": map[string]any{
					"sessionUpdate": "tool_call",
					"toolCallId":    "tc1",
					"title":         "noop",
					"kind":          "other",
					"status":        "pending",
				},
			})
			notify(w, "session/update", map[string]any{
				"sessionId": sid,
				"update": map[string]any{
					"sessionUpdate": "plan",
					"entries":       []map[string]string{{"content": "reply", "status": "pending", "priority": "medium"}},
				},
			})
			notify(w, "session/update", map[string]any{
				"sessionId": sid,
				"update": map[string]any{
					"sessionUpdate": "agent_message_chunk",
					"content":       map[string]string{"type": "text", "text": text},
				},
			})
			writeResp(w, env.ID, map[string]any{"stopReason": "end_turn"})
		case "ping":
			writeResp(w, env.ID, map[string]any{})
		default:
			writeErr(w, env.ID, -32601, "method not found")
		}
	}
}

// persistedSession is a fake catalog entry. session/new ids are not added here.
type persistedSession struct {
	id       string
	cwd      string
	title    string
	updated  string
	viewOnly bool
}

func persistedCatalog() []persistedSession {
	hostile := hostilePayload()
	return []persistedSession{
		{id: "sess-a1", cwd: "/tmp/zbridge-ws-a", title: hostile, updated: "2026-10-13T00:00:01Z"},
		{id: "sess-a2", cwd: "/tmp/zbridge-ws-a", title: "alpha", updated: "2026-10-13T00:00:02Z"},
		{id: "sess-a3", cwd: "/tmp/zbridge-ws-a", title: "beta", updated: "2026-10-13T00:00:03Z"},
		{id: "sess-b1", cwd: "/tmp/zbridge-ws-b", title: "other", updated: "2026-10-13T00:00:04Z"},
		{id: "sess-view", cwd: "/tmp/zbridge-ws-a", title: "locked", updated: "2026-10-13T00:00:05Z", viewOnly: true},
	}
}

func findPersisted(id string) (persistedSession, bool) {
	for _, s := range persistedCatalog() {
		if s.id == id {
			return s, true
		}
	}
	return persistedSession{}, false
}

const sessionPageSize = 2

func listPage(cwd, cursor string) ([]map[string]any, string) {
	var filtered []persistedSession
	for _, s := range persistedCatalog() {
		if cwd == "" || s.cwd == cwd {
			filtered = append(filtered, s)
		}
	}
	start := 0
	if cursor != "" {
		start = len(filtered)
		for i, s := range filtered {
			if s.id == cursor {
				start = i + 1
				break
			}
		}
	}
	end := start + sessionPageSize
	if end > len(filtered) {
		end = len(filtered)
	}
	out := make([]map[string]any, 0, end-start)
	for _, s := range filtered[start:end] {
		out = append(out, map[string]any{
			"sessionId": s.id,
			"cwd":       s.cwd,
			"title":     s.title,
			"updatedAt": s.updated,
			"_meta":     map[string]string{"modelId": "fake", "createdAt": s.updated},
		})
	}
	next := ""
	if end < len(filtered) && end > start {
		next = filtered[end-1].id
	}
	return out, next
}

func fakeAgentName() string {
	if name := os.Getenv("ZB_FAKE_AGENT_NAME"); name != "" {
		return name
	}
	return "acptest"
}

func fakeAgentVersion() string {
	if version := os.Getenv("ZB_FAKE_AGENT_VERSION"); version != "" {
		return version
	}
	return "0"
}

func fakeCapabilities() map[string]any {
	caps := map[string]any{
		"loadSession": os.Getenv("ZB_FAKE_NO_LOAD") != "1",
		"promptCapabilities": map[string]any{
			"image":           false,
			"audio":           false,
			"embeddedContext": false,
		},
	}
	sess := map[string]any{}
	if os.Getenv("ZB_FAKE_NO_LIST") != "1" {
		sess["list"] = map[string]any{}
	}
	if os.Getenv("ZB_FAKE_NO_RESUME") != "1" {
		sess["resume"] = map[string]any{}
	}
	if len(sess) > 0 {
		caps["sessionCapabilities"] = sess
	}
	return caps
}

func modeResult(current string) map[string]any {
	return map[string]any{
		"modes": map[string]any{
			"currentModeId": current,
			"availableModes": []map[string]string{
				{"id": "auto", "name": "Auto"},
				{"id": "ask", "name": "Ask"},
				{"id": "plan", "name": "Plan"},
			},
		},
	}
}

func replaySession(w io.Writer, s persistedSession) {
	notify(w, "session/update", map[string]any{
		"sessionId": s.id,
		"update": map[string]any{
			"sessionUpdate": "user_message_chunk",
			"content":       map[string]string{"type": "text", "text": "replay-user"},
		},
	})
	notify(w, "session/update", map[string]any{
		"sessionId": s.id,
		"update": map[string]any{
			"sessionUpdate": "agent_message_chunk",
			"content":       map[string]string{"type": "text", "text": "replay-agent"},
		},
	})
	notify(w, "session/update", map[string]any{
		"sessionId": s.id,
		"update": map[string]any{
			"sessionUpdate": "tool_call",
			"toolCallId":    "tc-replay",
			"title":         "replay-tool",
			"kind":          "read",
			"status":        "completed",
		},
	})
}

// hostilePayload is untrusted text the UI must render as text, never as HTML.
func hostilePayload() string {
	return `<img src=x onerror="window.__xss=1"></script><script>window.__xss=2</script>`
}

func richPrompt(r *bufio.Reader, w io.Writer, sid string, id json.RawMessage) {
	hostile := hostilePayload()
	notify(w, "session/update", map[string]any{
		"sessionId": sid,
		"update": map[string]any{
			"sessionUpdate": "agent_thought_chunk",
			"content":       map[string]string{"type": "text", "text": "thinking"},
		},
	})
	notify(w, "session/update", map[string]any{
		"sessionId": sid,
		"update": map[string]any{
			"sessionUpdate": "tool_call",
			"toolCallId":    "tc-text",
			"title":         "list",
			"kind":          "read",
			"status":        "pending",
			"rawInput":      map[string]string{"command": hostile},
		},
	})
	notify(w, "session/update", map[string]any{
		"sessionId": sid,
		"update": map[string]any{
			"sessionUpdate": "tool_call_update",
			"toolCallId":    "tc-text",
			"status":        "completed",
			"content": []any{
				map[string]any{"type": "content", "content": map[string]string{"type": "text", "text": "listed"}},
			},
		},
	})
	notify(w, "session/update", map[string]any{
		"sessionId": sid,
		"update": map[string]any{
			"sessionUpdate": "tool_call",
			"toolCallId":    "tc-edit",
			"title":         "edit",
			"kind":          "edit",
			"status":        "completed",
			"content": []any{
				map[string]any{"type": "diff", "path": "hello.txt", "oldText": "old", "newText": hostile},
			},
		},
	})
	notify(w, "session/update", map[string]any{
		"sessionId": sid,
		"update": map[string]any{
			"sessionUpdate": "plan",
			"entries": []map[string]string{
				{"content": "look", "priority": "high", "status": "pending"},
				{"content": hostile, "priority": "medium", "status": "in_progress"},
				{"content": "finish", "priority": "low", "status": "completed"},
			},
		},
	})
	text := "rich-done"
	if os.Getenv("ZB_FAKE_PERMISSION") == "1" {
		text = permissionRoundTool(r, w, sid, false, hostile, map[string]string{"path": hostile})
	}
	notify(w, "session/update", map[string]any{
		"sessionId": sid,
		"update": map[string]any{
			"sessionUpdate": "agent_message_chunk",
			"content":       map[string]string{"type": "text", "text": text},
		},
	})
	writeResp(w, id, map[string]any{"stopReason": "end_turn"})
}

func permissionRound(r *bufio.Reader, w io.Writer, sessionID string, onlyAlways bool) string {
	return permissionRoundTool(r, w, sessionID, onlyAlways, "write", map[string]string{"path": "x"})
}

func permissionRoundTool(r *bufio.Reader, w io.Writer, sessionID string, onlyAlways bool, title string, rawInput any) string {
	id := nextPermID.Add(1)
	options := []map[string]string{
		{"optionId": "allow", "name": "Allow", "kind": "allow_once"},
		{"optionId": "deny", "name": "Deny", "kind": "reject_once"},
		{"optionId": "always_allow", "name": "Always", "kind": "allow_always"},
	}
	if onlyAlways {
		options = []map[string]string{
			{"optionId": "always_allow", "name": "Always", "kind": "allow_always"},
			{"optionId": "deny", "name": "Deny", "kind": "reject_once"},
		}
	}
	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  "session/request_permission",
		"params": map[string]any{
			"sessionId": sessionID,
			"toolCall": map[string]any{
				"toolCallId": "tc-perm",
				"title":      title,
				"kind":       "edit",
				"status":     "pending",
				"rawInput":   rawInput,
			},
			"options": options,
		},
	}
	b, _ := json.Marshal(req)
	_, _ = w.Write(append(b, '\n'))
	for {
		line, err := r.ReadBytes('\n')
		if err != nil {
			return "DENIED:read"
		}
		var msg struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(line, &msg) != nil || msg.Method != "" || len(msg.ID) == 0 {
			continue
		}
		var got float64
		if json.Unmarshal(msg.ID, &got) != nil || int64(got) != id {
			continue
		}
		if msg.Error != nil {
			return "DENIED:" + msg.Error.Message
		}
		var result struct {
			Outcome struct {
				Outcome  string `json:"outcome"`
				OptionID string `json:"optionId"`
			} `json:"outcome"`
		}
		_ = json.Unmarshal(msg.Result, &result)
		if result.Outcome.Outcome == "cancelled" || result.Outcome.OptionID == "" {
			return "DENIED:cancelled"
		}
		if result.Outcome.OptionID == "allow" {
			return "ALLOWED optionId=allow"
		}
		return "DENIED optionId=" + result.Outcome.OptionID
	}
}

func slowMillis() int {
	return envMillis("ZB_FAKE_AGENT_SLOW_MS")
}

// sleepUnlessClosed waits ms, or returns false when stdin closes.
// The parent's Shutdown closes that pipe. select picks the timer or the close.
// /dev/stdin rejects SetReadDeadline, so a poll watches POLLHUP and does not read.
func sleepUnlessClosed(r io.Reader, ms int) bool {
	if ms <= 0 {
		return true
	}
	f, ok := r.(*os.File)
	if !ok {
		time.Sleep(time.Duration(ms) * time.Millisecond)
		return true
	}
	closed := make(chan struct{})
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		fd := int(f.Fd())
		for {
			select {
			case <-stop:
				return
			default:
			}
			revents, err := waitHUP(fd, 20)
			if err == syscall.EINTR {
				continue
			}
			if revents&hupBit != 0 {
				close(closed)
				return
			}
			if err != nil {
				return
			}
		}
	}()
	timer := time.NewTimer(time.Duration(ms) * time.Millisecond)
	defer timer.Stop()
	select {
	case <-timer.C:
		close(stop)
		<-done
		return true
	case <-closed:
		close(stop)
		<-done
		return false
	}
}

const hupBit int16 = 0x10

type pollfd struct {
	fd      int32
	events  int16
	revents int16
}

func waitHUP(fd, timeoutMS int) (int16, error) {
	p := pollfd{fd: int32(fd), events: hupBit}
	_, _, errno := syscall.Syscall(syscall.SYS_POLL, uintptr(unsafe.Pointer(&p)), 1, uintptr(timeoutMS))
	if errno != 0 {
		return 0, errno
	}
	return p.revents, nil
}

func envMillis(key string) int {
	s := os.Getenv(key)
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0
	}
	return n
}

func writeResp(w io.Writer, id json.RawMessage, result any) {
	out := map[string]any{"jsonrpc": "2.0", "id": id, "result": result}
	b, _ := json.Marshal(out)
	b = append(b, '\n')
	_, _ = w.Write(b)
}

func writeErr(w io.Writer, id json.RawMessage, code int, msg string) {
	out := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"error":   map[string]any{"code": code, "message": msg},
	}
	b, _ := json.Marshal(out)
	b = append(b, '\n')
	_, _ = w.Write(b)
}

func notify(w io.Writer, method string, params any) {
	out := map[string]any{"jsonrpc": "2.0", "method": method, "params": params}
	b, _ := json.Marshal(out)
	b = append(b, '\n')
	_, _ = w.Write(b)
}
