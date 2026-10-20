package acpclient

import (
	"encoding/json"
	"testing"
)

func FuzzACPFrame(f *testing.F) {
	f.Add([]byte(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`))
	f.Add([]byte(`{"jsonrpc":"2.0","method":"session/update","params":{}}`))
	f.Add([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}`))
	f.Add([]byte(`{"jsonrpc":"2.0","id":"nope","method":"ping"}`))
	f.Add([]byte("{"))
	f.Fuzz(func(t *testing.T, line []byte) {
		if len(line) > 65536 {
			line = line[:65536]
		}
		var msg rpcMessage
		if err := json.Unmarshal(line, &msg); err != nil {
			return
		}
		_ = msg.isResponse()
		_ = msg.isRequest()
		_ = msg.isNotify()
		_, _ = parseID(msg.ID)
	})
}
