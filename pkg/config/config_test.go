// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestPathsAndAgentConfig(t *testing.T) {
	tmpDir := t.TempDir()
	paths := Paths{
		DataHome:   filepath.Join(tmpDir, "data"),
		AgentsDir:  filepath.Join(tmpDir, "data", "agents"),
		SecretsDir: filepath.Join(tmpDir, "data", "secrets"),
		BinDir:     filepath.Join(tmpDir, "bin"),
		SSHDir:     filepath.Join(tmpDir, "ssh"),
		IDEKeyFile: filepath.Join(tmpDir, "ssh", "key"),
	}

	if err := paths.EnsureDirectories(); err != nil {
		t.Fatalf("EnsureDirectories failed: %v", err)
	}

	// 1. Resolve image
	if img := ResolveImage("opencode"); img != "agent-sandbox-opencode:latest" {
		t.Errorf("expected opencode preset image, got %s", img)
	}
	if img := ResolveImage("claude"); img != "agent-sandbox-claude:latest" {
		t.Errorf("expected claude preset image, got %s", img)
	}
	if img := ResolveImage("agy"); img != "agent-sandbox-agy:latest" {
		t.Errorf("expected agy preset image, got %s", img)
	}
	if img := ResolveImage("egress"); img != "agent-sandbox-egress:latest" {
		t.Errorf("expected egress preset image, got %s", img)
	}
	if img := ResolveImage("custom/image:tag"); img != "custom/image:tag" {
		t.Errorf("expected custom image passthrough, got %s", img)
	}
	if img := ResolveImage(""); img != "agent-sandbox-base:latest" {
		t.Errorf("expected default base image, got %s", img)
	}

	// 2. NewAgentConfig errors & creation
	if _, err := NewAgentConfig("", "base", "role"); err == nil {
		t.Errorf("expected error for empty agent name")
	}

	cfg, err := NewAgentConfig("test-agent", "opencode", "", "http://localhost:11434/v1", "llama3", "secret-key")
	if err != nil {
		t.Fatalf("NewAgentConfig failed: %v", err)
	}
	if cfg.Name != "test-agent" || cfg.Role != "coding-agent" || cfg.ModelURL != "http://localhost:11434/v1" || cfg.ModelName != "llama3" || cfg.ModelAPIKey != "secret-key" || !strings.Contains(cfg.ContainerName, "test-agent") {
		t.Errorf("unexpected agent config: %+v", cfg)
	}

	// 3. Save & Load & List
	if err := SaveAgentConfig(cfg, paths); err != nil {
		t.Fatalf("SaveAgentConfig failed: %v", err)
	}

	loaded, err := LoadAgentConfig("test-agent", paths)
	if err != nil {
		t.Fatalf("LoadAgentConfig failed: %v", err)
	}
	if loaded.Name != cfg.Name || loaded.Password != cfg.Password {
		t.Errorf("loaded config mismatch: %+v vs %+v", loaded, cfg)
	}

	list, err := ListAgentConfigs(paths)
	if err != nil || len(list) != 1 {
		t.Errorf("ListAgentConfigs failed: len=%d, err=%v", len(list), err)
	}

	// 4. Missing agent error
	if _, err := LoadAgentConfig("nonexistent", paths); err == nil {
		t.Errorf("expected error for nonexistent agent")
	}

	// 5. Delete agent
	if err := DeleteAgentConfig("test-agent", paths); err != nil {
		t.Errorf("DeleteAgentConfig error: %v", err)
	}
	listAfter, _ := ListAgentConfigs(paths)
	if len(listAfter) != 0 {
		t.Errorf("expected 0 agents after delete, got %d", len(listAfter))
	}

	// 6. GetPaths
	defaultPaths := GetPaths()
	if defaultPaths.DataHome == "" || defaultPaths.SSHConfigFile == "" {
		t.Errorf("GetPaths returned empty DataHome or SSHConfigFile: %+v", defaultPaths)
	}
}
