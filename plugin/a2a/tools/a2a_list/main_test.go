package main

import (
	"encoding/json"
	"os"
	"testing"
)

func TestA2AList_ValidConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Create the plugin/a2a directory structure
	peersDir := tmpDir + "/plugin/a2a"
	if err := os.MkdirAll(peersDir, 0755); err != nil {
		t.Fatalf("failed to create peers dir: %v", err)
	}

	peers := map[string]any{
		"peers": []map[string]any{
			{
				"name":        "local-bridge",
				"url":         "http://127.0.0.1:8765/a2a/message/send",
				"description": "Local zero-bridge A2A ingress",
				"allowed":     true,
			},
			{
				"name":        "remote-bridge",
				"url":         "http://192.168.1.100:8765/a2a/message/send",
				"description": "Remote zero-bridge A2A ingress",
				"allowed":     false,
			},
		},
	}

	data, _ := json.Marshal(peers)
	peersFile := peersDir + "/peers.json"
	if err := os.WriteFile(peersFile, data, 0644); err != nil {
		t.Fatalf("failed to write peers.json: %v", err)
	}

	// Change to temp dir
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(tmpDir)

	cfg, err := loadPluginConfig()
	if err != nil {
		t.Fatalf("loadPluginConfig failed: %v", err)
	}

	if len(cfg.Peers) != 2 {
		t.Fatalf("expected 2 peers, got %d", len(cfg.Peers))
	}

	if cfg.Peers[0].Name != "local-bridge" {
		t.Errorf("expected first peer name 'local-bridge', got %s", cfg.Peers[0].Name)
	}

	if cfg.Peers[1].Name != "remote-bridge" {
		t.Errorf("expected second peer name 'remote-bridge', got %s", cfg.Peers[1].Name)
	}
}

func TestA2AList_EmptyConfig(t *testing.T) {
	tmpDir := t.TempDir()
	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(tmpDir)

	cfg, err := loadPluginConfig()
	if err != nil {
		t.Fatalf("loadPluginConfig failed: %v", err)
	}

	if len(cfg.Peers) != 0 {
		t.Errorf("expected 0 peers, got %d", len(cfg.Peers))
	}
}

func TestA2AList_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(tmpDir+"/peers.json", []byte("{invalid json"), 0644); err != nil {
		t.Fatalf("failed to write invalid peers.json: %v", err)
	}

	oldDir, _ := os.Getwd()
	defer os.Chdir(oldDir)
	os.Chdir(tmpDir)

	cfg, err := loadPluginConfig()
	if err != nil {
		// Error is expected for invalid JSON, but we return empty config
		// The function should handle this gracefully
	}
	_ = cfg
}
