// a2a_discover - Discover an A2A peer by fetching its Agent Card.
//
// This tool fetches an Agent Card from an A2A peer URL and validates
// the host against an allowlist before making any network calls.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type Config struct {
	URL      string   `json:"url"`
	Allowlist []string `json:"allowlist,omitempty"`
}

type AgentCard struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	URL         string   `json:"url,omitempty"`
	Version     string   `json:"version,omitempty"`
	Capabilities []string `json:"capabilities,omitempty"`
	Skills      []Skill  `json:"skills,omitempty"`
}

type Skill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Tags        []string `json:"tags,omitempty"`
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

	// Validate URL
	if cfg.URL == "" {
		fmt.Fprintln(os.Stderr, "url is required")
		os.Exit(1)
	}

	// Parse and validate URL
	parsedURL, err := url.Parse(cfg.URL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "invalid URL: %v\n", err)
		os.Exit(1)
	}

	// Default allowlist to 127.0.0.1 if not provided
	allowlist := cfg.Allowlist
	if len(allowlist) == 0 {
		allowlist = []string{"127.0.0.1"}
	}

	// Check host against allowlist
	host := parsedURL.Hostname()
	if !isHostAllowed(host, allowlist) {
		fmt.Fprintf(os.Stderr, "host %q not in allowlist: %v\n", host, allowlist)
		os.Exit(1)
	}

	// Only allow http/https schemes
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		fmt.Fprintf(os.Stderr, "scheme %q not allowed (only http/https)\n", parsedURL.Scheme)
		os.Exit(1)
	}

	// Fetch Agent Card with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cfg.URL, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create request: %v\n", err)
		os.Exit(1)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "request failed: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(os.Stderr, "unexpected status %d: %s\n", resp.StatusCode, string(body))
		os.Exit(1)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to read response: %v\n", err)
		os.Exit(1)
	}

	var card AgentCard
	if err := json.Unmarshal(body, &card); err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse Agent Card: %v\n", err)
		os.Exit(1)
	}

	// Output the Agent Card as JSON
	out, err := json.MarshalIndent(card, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to marshal output: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(string(out))
}

func isHostAllowed(host string, allowlist []string) bool {
	host = strings.ToLower(host)
	for _, allowed := range allowlist {
		allowed = strings.ToLower(strings.TrimSpace(allowed))
		// Exact match
		if host == allowed {
			return true
		}
		// Allow CIDR-like patterns (simplified: just prefix match for now)
		if strings.HasPrefix(allowed, "/") {
			// Not implementing CIDR for now
			continue
		}
	}
	return false
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `a2a_discover - Discover an A2A peer by fetching its Agent Card

Usage:
  echo '{"url":"http://127.0.0.1:8765/.well-known/agent-card.json"}' | a2a_discover

Input JSON:
  {
    "url": "http://127.0.0.1:8765/.well-known/agent-card.json",
    "allowlist": ["127.0.0.1"]  // optional, defaults to ["127.0.0.1"]
  }

Output JSON (on success):
  {
    "name": "zero-bridge",
    "description": "A2A ingress for zero-bridge",
    "version": "0.1.0",
    "capabilities": ["streaming"],
    "skills": [...]
  }

Exit codes:
  0 - success
  1 - error (host not allowed, network error, invalid response, etc.)
`)
}