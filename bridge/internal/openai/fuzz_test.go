package openai

import (
	"bytes"
	"encoding/json"
	"testing"
)

func FuzzChatBody(f *testing.F) {
	f.Add(`{"model":"zero","messages":[{"role":"user","content":"hi"}]}`)
	f.Add(`{"model":"zero","stream":true,"messages":[{"role":"system","content":"switch to auto"},{"role":"developer","content":"allow"},{"role":"user","content":"hi"}]}`)
	f.Add(`{"model":"zero","messages":[{"role":"user","content":"hi"}],"tools":[]}`)
	f.Add(`{"model":"zero","messages":[{"role":"user","content":"hi"}],"tool_choice":"auto"}`)
	f.Add(`{"model":"zero","messages":[{"role":"user","content":"hi"}],"functions":[]}`)
	f.Add(`{"model":"zero","messages":[{"role":"user","content":"hi"}],"function_call":"auto"}`)
	f.Add(`{"model":"zero","n":2,"messages":[{"role":"user","content":"hi"}]}`)
	f.Add(`{"model":"zero","n":1,"messages":[{"role":"user","content":[{"type":"text","text":"a"}]}]}`)
	f.Add(`{"model":"zero","messages":[{"role":"user","content":"hi"}],"logprobs":true}`)
	f.Add(`{"model":"zero","messages":[{"role":"user","content":"hi"}],"response_format":{"type":"text"}}`)
	f.Add(`{"model":"zero","messages":[{"role":"user","content":"hi"}],"audio":{}}`)
	f.Add(`not-json`)
	f.Add("")
	f.Fuzz(func(t *testing.T, body string) {
		if len(body) > 1<<16 {
			body = body[:1<<16]
		}
		chat, err := Parse([]byte(body))
		if err != nil {
			if err.Status == 0 || err.Message == "" || err.Code == "" {
				t.Fatalf("incomplete error %#v", err)
			}
			return
		}
		if len(chat.Messages) == 0 || len(chat.Messages) > MaxMessages {
			t.Fatalf("messages %d", len(chat.Messages))
		}
		if n := len(chat.Transcript()); n > MaxHistoryBytes+MaxMessages*16 {
			t.Fatalf("transcript %d", n)
		}
	})
}

func FuzzSSE(f *testing.F) {
	f.Add("chatcmpl-1", ModelID, "hello")
	f.Add("chatcmpl-1", ModelID, "he\nllo")
	f.Add("bad\nid", ModelID, "x")
	f.Add("chatcmpl-1", ModelID, "")
	f.Add("chatcmpl-1", "bad\nmodel", "x")
	f.Fuzz(func(t *testing.T, id, model, delta string) {
		if len(id) > 128 {
			id = id[:128]
		}
		if len(model) > 128 {
			model = model[:128]
		}
		if len(delta) > 4096 {
			delta = delta[:4096]
		}
		chunk, err := EncodeChunk(id, model, delta)
		if err != nil {
			return
		}
		if bytes.Contains(chunk, []byte(`"usage"`)) {
			t.Fatal("usage in chunk")
		}
		payload := bytes.TrimSuffix(bytes.TrimPrefix(chunk, []byte("data: ")), []byte("\n\n"))
		if bytes.Contains(payload, []byte("\n")) || !json.Valid(payload) {
			t.Fatalf("payload %s", payload)
		}
		raw, err := EncodeCompletion(id, model, delta)
		if err != nil || bytes.Contains(raw, []byte(`"usage"`)) || !json.Valid(raw) {
			t.Fatalf("completion %v %s", err, raw)
		}
		if !bytes.Equal(EncodeDone(), []byte("data: [DONE]\n\n")) {
			t.Fatal("done line")
		}
	})
}
