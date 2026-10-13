// Package acptest provides a reusable fake zero acp agent for integration tests.
package acptest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"sync/atomic"
	"time"
)

// Main runs the fake agent on stdio. Call from TestMain when invoked as `testbinary acp` with ZB_FAKE_AGENT=1.
func Main() {
	runAgent(os.Stdin, os.Stdout)
}

var nextSessionID atomic.Int64

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
				"protocolVersion": 1,
				"agentCapabilities": map[string]any{
					"loadSession": false,
					"promptCapabilities": map[string]any{
						"image": false,
					},
				},
				"agentInfo": map[string]string{"name": "acptest", "version": "0"},
			})
			if os.Getenv("ZB_FAKE_AGENT_EXIT") == "1" {
				os.Exit(0)
			}
		case "session/new":
			n := nextSessionID.Add(1)
			writeResp(w, env.ID, map[string]any{"sessionId": fmt.Sprintf("sess-acptest-%d", n)})
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
			if ms := slowMillis(); ms > 0 {
				time.Sleep(time.Duration(ms) * time.Millisecond)
			}
			notify(w, "session/update", map[string]any{
				"sessionId": sid,
				"update": map[string]any{
					"sessionUpdate": "agent_message_chunk",
					"content":       map[string]string{"type": "text", "text": "one"},
				},
			})
			notify(w, "session/update", map[string]any{
				"sessionId": sid,
				"update": map[string]any{
					"sessionUpdate": "agent_message_chunk",
					"content":       map[string]string{"type": "text", "text": "two"},
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

func slowMillis() int {
	s := os.Getenv("ZB_FAKE_AGENT_SLOW_MS")
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
