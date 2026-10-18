// a2a_call - Call an A2A peer with a message.
//
// This tool sends a message to an A2A peer via the message/send endpoint
// and returns the peer's response. The peer must be pre-configured in the
// plugin's peers configuration.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type Config struct {
	Peer    string `json:"peer"`
	Message string `json:"message"`
}

type PeerConfig struct {
	Name    string `json:"name"`
	URL     string `json:"url"`
	Allowed bool   `json:"allowed"`
}

type PluginConfig struct {
	Peers []PeerConfig `json:"peers"`
}

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
	ID      json.RawMessage `json:"id,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type MessageSendParams struct {
	Message Message `json:"message"`
}

type Message struct {
	Role  string `json:"role"`
	Parts []Part `json:"parts"`
}

type Part struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		printUsage()
		os.Exit(0)
	}

	// Read stdin for input
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read stdin: %v\n", err)
		os.Exit(1)
	}

	var cfg Config
	if err := json.Unmarshal(input, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "invalid input JSON: %v\n", err)
		os.Exit(1)
	}

	// Validate required fields
	if cfg.Peer == "" {
		fmt.Fprintln(os.Stderr, "peer is required")
		os.Exit(1)
	}
	if cfg.Message == "" {
		fmt.Fprintln(os.Stderr, "message is required")
		os.Exit(1)
	}

	// Load plugin config to get peer URL
	pluginConfig, err := loadPluginConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load plugin config: %v\n", err)
		os.Exit(1)
	}

	// Find peer
	var peer *PeerConfig
	for _, p := range pluginConfig.Peers {
		if p.Name == cfg.Peer {
			if !p.Allowed {
				fmt.Fprintf(os.Stderr, "peer %q is not allowed\n", cfg.Peer)
				os.Exit(1)
			}
			peer = &p
			break
		}
	}
	if peer == nil {
		fmt.Fprintf(os.Stderr, "peer %q not found in configuration\n", cfg.Peer)
		os.Exit(1)
	}

	// Build message/send request
	req := Request{
		JSONRPC: "2.0",
		Method:  "message/send",
		Params: mustMarshal(MessageSendParams{
			Message: Message{
				Role: "user",
				Parts: []Part{
					{Type: "text", Text: cfg.Message},
				},
			},
		}),
		ID: mustMarshal(fmt.Sprintf("call-%d", time.Now().UnixNano())),
	}

	// Send request to peer
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	reqBody, _ := json.Marshal(req)
	reqHTTP, err := http.NewRequestWithContext(ctx, http.MethodPost, peer.URL, bytes.NewReader(reqBody))
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create request: %v\n", err)
		os.Exit(1)
	}
	reqHTTP.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(reqHTTP)
	if err != nil {
		fmt.Fprintf(os.Stderr, "request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read response: %v\n", err)
		os.Exit(1)
	}

	if resp.StatusCode >= 500 {
		// Don't include token substrings in stderr
		fmt.Fprintf(os.Stderr, "peer returned %d\n", resp.StatusCode)
		os.Exit(1)
	}

	var respObj Response
	if err := json.Unmarshal(respBody, &respObj); err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse response: %v\n", err)
		os.Exit(1)
	}

	if respObj.Error != nil {
		fmt.Fprintf(os.Stderr, "peer error: %s\n", respObj.Error.Message)
		os.Exit(1)
	}

	// Extract reply from result
	var task struct {
		Artifacts []struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"artifacts"`
	}
	if err := json.Unmarshal(respObj.Result, &task); err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse task result: %v\n", err)
		os.Exit(1)
	}

	var reply string
	for _, artifact := range task.Artifacts {
		for _, part := range artifact.Parts {
			reply += part.Text
		}
	}

	// Output reply
	out := map[string]string{"reply": reply}
	outBytes, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(outBytes))
}

func loadPluginConfig() (*PluginConfig, error) {
	// Look for plugin config in standard locations
	paths := []string{
		"plugin/a2a/plugin.json",                    // relative to project root
		".zero/plugins/zero-bridge-a2a/plugin.json", // user install
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err == nil {
			var cfg PluginConfig
			if err := json.Unmarshal(data, &cfg); err == nil {
				return &cfg, nil
			}
		}
	}
	return &PluginConfig{Peers: []PeerConfig{}}, nil
}

func mustMarshal(v any) json.RawMessage {
	data, _ := json.Marshal(v)
	return data
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `a2a_call - Call an A2A peer with a message

Usage:
  echo '{"peer":"test","message":"hi"}' | a2a_call

Input JSON:
  {
    "peer": "test",
    "message": "hi"
  }

Output JSON (on success):
  {
    "reply": "peer response text"
  }

Exit codes:
  0 - success
  1 - error (peer not found, not allowed, network error, peer error, etc.)
`)
}
