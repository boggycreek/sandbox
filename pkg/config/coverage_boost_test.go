package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigCoverageBoost(t *testing.T) {
	tmpDir := t.TempDir()
	paths := Paths{
		DataHome:   filepath.Join(tmpDir, "data"),
		AgentsDir:  filepath.Join(tmpDir, "data", "agents"),
		SecretsDir: filepath.Join(tmpDir, "data", "secrets"),
		BinDir:     filepath.Join(tmpDir, "bin"),
		SSHDir:     filepath.Join(tmpDir, "ssh"),
		IDEKeyFile: filepath.Join(tmpDir, "ssh", "key"),
	}

	// 1. Corrupted JSON file in LoadAgentConfig
	_ = paths.EnsureDirectories()
	corrupted := filepath.Join(paths.AgentsDir, "bad.json")
	_ = os.WriteFile(corrupted, []byte("{bad-json"), 0600)
	if _, err := LoadAgentConfig("bad", paths); err == nil {
		t.Errorf("expected error loading corrupted agent config")
	}

	// 2. Resolve image with empty / custom / whitespace
	if img := ResolveImage("  "); img != "agent-sandbox-base:latest" {
		t.Errorf("expected base image for whitespace, got %s", img)
	}
	if img := ResolveImage("random-custom:v1"); img != "random-custom:v1" {
		t.Errorf("expected random-custom:v1, got %s", img)
	}

	// 3. SaveAgentConfig write error simulation (read-only dir)
	roPaths := Paths{
		DataHome:   "/dev/null/forbidden",
		AgentsDir:  "/dev/null/forbidden/agents",
		SecretsDir: "/dev/null/forbidden/secrets",
	}
	cfg, _ := NewAgentConfig("ro-agent", "base", "tester")
	if err := SaveAgentConfig(cfg, roPaths); err == nil {
		t.Errorf("expected error saving to read only path")
	}

	// 4. ListAgentConfigs error on invalid directory
	if _, err := ListAgentConfigs(roPaths); err == nil {
		t.Errorf("expected error listing invalid directory")
	}

	// 5. SaveAgentConfig secret dir failure
	secFailPaths := Paths{
		DataHome:   filepath.Join(tmpDir, "secfail"),
		AgentsDir:  filepath.Join(tmpDir, "secfail", "agents"),
		SecretsDir: "/dev/null/forbidden/secrets",
	}
	_ = secFailPaths.EnsureDirectories()
	if err := SaveAgentConfig(cfg, secFailPaths); err == nil {
		t.Errorf("expected error saving to invalid secrets path")
	}

	// 6. List with corrupted JSON entries
	list, err := ListAgentConfigs(paths)
	if err != nil {
		t.Errorf("unexpected error on ListAgentConfigs with bad json present: %v", err)
	}
	_ = list

	// 7. EnsureDirectories failure on forbidden path
	if err := roPaths.EnsureDirectories(); err == nil {
		t.Errorf("expected error from EnsureDirectories on read-only path")
	}

	// 8. GetPaths with custom XDG_DATA_HOME
	os.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "custom-xdg"))
	customPaths := GetPaths()
	if customPaths.DataHome != filepath.Join(tmpDir, "custom-xdg", "agent-sandbox") {
		t.Errorf("unexpected custom data home: %s", customPaths.DataHome)
	}
}
