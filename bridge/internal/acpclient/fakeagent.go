package acpclient

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
)

// RunFakeAgent implements a minimal `zero acp` peer for tests (stdio ndjson).
func RunFakeAgent() {
	RunFakeAgentIO(os.Stdin, os.Stdout)
}

// RunFakeAgentIO is the test helper body (newline-delimited JSON-RPC).
func RunFakeAgentIO(r io.Reader, w io.Writer) {
	reader := bufio.NewReader(r)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			return
		}
		var msg rpcMessage
		if json.Unmarshal(line, &msg) != nil {
			continue
		}
		if !msg.isRequest() {
			continue
		}
		switch msg.Method {
		case "initialize":
			writeFake(w, rpcMessage{
				JSONRPC: jsonRPCVersion,
				ID:      msg.ID,
				Result:  json.RawMessage(`{"protocolVersion":1,"agentCapabilities":{"loadSession":true,"promptCapabilities":{"image":false}},"authMethods":[]}`),
			})
		case "session/new":
			writeFake(w, rpcMessage{
				JSONRPC: jsonRPCVersion,
				ID:      msg.ID,
				Result:  json.RawMessage(`{"sessionId":"sess-test-1"}`),
			})
		case "session/prompt":
			notifyFake(w, "session/update", `{"sessionId":"sess-test-1","update":{"sessionUpdate":"agent_message_chunk","content":{"type":"text","text":"OK"}}}`)
			writeFake(w, rpcMessage{
				JSONRPC: jsonRPCVersion,
				ID:      msg.ID,
				Result:  json.RawMessage(`{"stopReason":"end_turn"}`),
			})
		case "ping":
			writeFake(w, rpcMessage{JSONRPC: jsonRPCVersion, ID: msg.ID, Result: json.RawMessage(`{}`)})
		default:
			writeFake(w, rpcMessage{
				JSONRPC: jsonRPCVersion,
				ID:      msg.ID,
				Error:   &rpcError{Code: codeMethodNotFound, Message: "unknown method"},
			})
		}
	}
}

func writeFake(w io.Writer, msg rpcMessage) {
	b, _ := json.Marshal(msg)
	b = append(b, '\n')
	_, _ = w.Write(b)
}

func notifyFake(w io.Writer, method string, params string) {
	b, _ := json.Marshal(rpcMessage{JSONRPC: jsonRPCVersion, Method: method, Params: json.RawMessage(params)})
	b = append(b, '\n')
	_, _ = w.Write(b)
}
