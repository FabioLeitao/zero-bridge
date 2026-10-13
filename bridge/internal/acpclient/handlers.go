package acpclient

import (
	"encoding/json"
	"strings"
)

const (
	methodSessionRequestPerm = "session/request_permission"
	permRejectOnce           = "reject_once"
)

// DefaultInboundHandler denies tool permissions and rejects fs/* agent requests.
type DefaultInboundHandler struct{}

func (DefaultInboundHandler) HandleRequest(method string, params json.RawMessage) (any, error) {
	switch method {
	case methodSessionRequestPerm:
		var p struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, &rpcError{Code: codeInvalidRequest, Message: "invalid permission params"}
		}
		return map[string]any{
			"outcome": map[string]string{
				"outcome":  "selected",
				"optionId": permRejectOnce,
			},
		}, nil
	default:
		if strings.HasPrefix(method, "fs/") {
			return nil, &rpcError{Code: codeMethodNotFound, Message: "filesystem capability not advertised"}
		}
		return nil, &rpcError{Code: codeMethodNotFound, Message: "method not supported: " + method}
	}
}
