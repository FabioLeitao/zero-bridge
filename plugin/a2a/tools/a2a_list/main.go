// a2a_list - List configured A2A peers.
//
// This tool reads the peers configuration and outputs the list of
// configured A2A peers as JSON. It reads stdin (expects empty object {})
// and outputs the list of peers.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type PeerConfig struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	Description string `json:"description,omitempty"`
	Allowed     bool   `json:"allowed"`
}

type PluginConfig struct {
	Peers []PeerConfig `json:"peers"`
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "-h" || os.Args[1] == "--help") {
		printUsage()
		os.Exit(0)
	}

	// Read stdin (expects empty object {})
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read stdin: %v\n", err)
		os.Exit(1)
	}

	// Parse input (should be empty object)
	var inputObj map[string]any
	if len(input) > 0 {
		if err := json.Unmarshal(input, &inputObj); err != nil {
			fmt.Fprintf(os.Stderr, "invalid input JSON: %v\n", err)
			os.Exit(1)
		}
	}

	// Load plugin config
	pluginConfig, err := loadPluginConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load plugin config: %v\n", err)
		os.Exit(1)
	}

	// Filter and output only allowed peers (or all peers)
	// For v1, we output all peers but mark allowed status
	peers := make([]map[string]any, 0, len(pluginConfig.Peers))
	for _, p := range pluginConfig.Peers {
		peer := map[string]any{
			"name":        p.Name,
			"url":         p.URL,
			"description": p.Description,
			"allowed":     p.Allowed,
		}
		peers = append(peers, peer)
	}

	// Output as JSON array
	out, err := json.MarshalIndent(peers, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal output: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(out))
}

func loadPluginConfig() (*PluginConfig, error) {
	// Look for plugin config in standard locations
	paths := []string{
		"plugin/a2a/peers.json",                    // relative to project root
		".zero/plugins/zero-bridge-a2a/peers.json", // user install
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
	// Return empty config if not found
	return &PluginConfig{Peers: []PeerConfig{}}, nil
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `a2a_list - List configured A2A peers

Usage:
  echo '{}' | a2a_list

Input JSON:
  {}

Output JSON (on success):
  [
    {
      "name": "local-bridge",
      "url": "http://127.0.0.1:8765/a2a/message/send",
      "description": "Local zero-bridge A2A ingress",
      "allowed": true
    }
  ]

Exit codes:
  0 - success
  1 - error (config not found, parse error, etc.)
`)
}
