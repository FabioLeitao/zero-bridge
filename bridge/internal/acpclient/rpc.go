package acpclient

import (
	"encoding/json"
	"fmt"
)

const jsonRPCVersion = "2.0"

const (
	codeParseError     = -32700
	codeInvalidRequest = -32600
	codeMethodNotFound = -32601
)

type rpcError struct {
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data,omitempty"`
}

func (e *rpcError) Error() string {
	if e == nil {
		return "<nil rpc error>"
	}
	return fmt.Sprintf("jsonrpc error %d: %s", e.Code, e.Message)
}

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func (m rpcMessage) isResponse() bool { return m.Method == "" && len(m.ID) > 0 }
func (m rpcMessage) isRequest() bool  { return m.Method != "" && len(m.ID) > 0 }
func (m rpcMessage) isNotify() bool   { return m.Method != "" && len(m.ID) == 0 }
