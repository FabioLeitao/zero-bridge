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
		return denyPermission(params)
	default:
		if strings.HasPrefix(method, "fs/") {
			return nil, &rpcError{Code: codeMethodNotFound, Message: "filesystem capability not advertised"}
		}
		return nil, &rpcError{Code: codeMethodNotFound, Message: "method not supported: " + method}
	}
}

func denyPermission(params json.RawMessage) (any, error) {
	var p struct {
		Options []struct {
			OptionID string `json:"optionId"`
			Kind     string `json:"kind"`
		} `json:"options"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &rpcError{Code: codeInvalidRequest, Message: "invalid permission params"}
	}
	for _, opt := range p.Options {
		if opt.Kind == permRejectOnce && opt.OptionID != "" {
			return map[string]any{
				"outcome": map[string]string{
					"outcome":  "selected",
					"optionId": opt.OptionID,
				},
			}, nil
		}
	}
	return map[string]any{
		"outcome": map[string]string{"outcome": "cancelled"},
	}, nil
}
