package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"zero-bridge/bridge/internal/acpclient"
)

func main() {
	os.Exit(run())
}

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	extraEnv := probeProviderEnv()
	client, err := acpclient.Start(ctx, acpclient.Options{ZeroBin: "zero", ExtraEnv: extraEnv})
	if err != nil {
		fmt.Fprintf(os.Stderr, "acpprobe: cannot start zero acp: %v\n", err)
		return 2
	}
	defer client.Close()

	logLine := func(dir string, v any) {
		b, _ := json.Marshal(v)
		fmt.Printf("%s %s\n", dir, string(b))
	}

	initParams := map[string]any{
		"protocolVersion": 1,
		"clientCapabilities": map[string]any{
			"fs": map[string]bool{
				"readTextFile":  false,
				"writeTextFile": false,
			},
			"terminal": false,
		},
		"clientInfo": map[string]string{"name": "zero-bridge-acpprobe", "version": "0"},
	}
	logLine("->", map[string]any{"method": "initialize", "params": initParams})
	var initRes map[string]any
	if err := client.Call(ctx, "initialize", initParams, &initRes); err != nil {
		fmt.Fprintf(os.Stderr, "acpprobe: initialize failed: %v\n%s", err, client.StderrSnippet())
		return 1
	}
	logLine("<-", map[string]any{"result": initRes})

	ws, err := os.MkdirTemp("", "zbridge-acpprobe-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "acpprobe: workspace: %v\n", err)
		return 1
	}
	defer os.RemoveAll(ws)

	newParams := map[string]any{"cwd": ws, "mcpServers": []any{}}
	logLine("->", map[string]any{"method": "session/new", "params": newParams})
	var newRes struct {
		SessionID string `json:"sessionId"`
	}
	if err := client.Call(ctx, "session/new", newParams, &newRes); err != nil {
		fmt.Fprintf(os.Stderr, "acpprobe: session/new failed: %v\n%s", err, client.StderrSnippet())
		return 1
	}
	logLine("<-", map[string]any{"result": newRes})

	client.SetNotifyHandler(func(method string, params json.RawMessage) {
		logLine("<-", map[string]any{"notification": method, "params": json.RawMessage(params)})
	})

	promptParams := map[string]any{
		"sessionId": newRes.SessionID,
		"prompt": []map[string]string{
			{"type": "text", "text": "Reply with exactly: OK"},
		},
	}
	logLine("->", map[string]any{"method": "session/prompt", "params": promptParams})
	var promptRes struct {
		StopReason string `json:"stopReason"`
	}
	if err := client.Call(ctx, "session/prompt", promptParams, &promptRes); err != nil {
		fmt.Fprintf(os.Stderr, "acpprobe: session/prompt failed: %v\n%s", err, client.StderrSnippet())
		return 1
	}
	logLine("<-", map[string]any{"result": promptRes})

	if promptRes.StopReason == "" {
		fmt.Fprintln(os.Stderr, "acpprobe: empty stopReason")
		return 1
	}
	return 0
}

// probeProviderEnv sets ZERO_PROVIDER for the child only (no persistent config edits).
// Prefer granite-local when Ollama is reachable (see internal/config ActiveProviderEnv = ZERO_PROVIDER).
func probeProviderEnv() []string {
	if ollamaUp() {
		return []string{"ZERO_PROVIDER=granite-local"}
	}
	active := strings.TrimSpace(os.Getenv("ZERO_PROVIDER"))
	if strings.EqualFold(active, "openrouter") || strings.Contains(strings.ToLower(active), "free") {
		return nil
	}
	return nil
}

func ollamaUp() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://127.0.0.1:11434/api/tags", nil)
	if err != nil {
		return false
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	defer res.Body.Close()
	return res.StatusCode == http.StatusOK
}
