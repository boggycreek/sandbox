// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package config

import (
	"os"
	"path/filepath"
	"strings"
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

	// 5b. SaveAgentConfig JSON file write failure (directory with same name exists)
	jsonBlockPaths := Paths{
		DataHome:   filepath.Join(tmpDir, "jsonfail"),
		AgentsDir:  filepath.Join(tmpDir, "jsonfail", "agents"),
		SecretsDir: filepath.Join(tmpDir, "jsonfail", "secrets"),
	}
	_ = jsonBlockPaths.EnsureDirectories()
	_ = os.MkdirAll(filepath.Join(jsonBlockPaths.AgentsDir, "jsonblock.json"), 0700)
	jsonBlockCfg, _ := NewAgentConfig("jsonblock", "base", "tester")
	if err := SaveAgentConfig(jsonBlockCfg, jsonBlockPaths); err == nil {
		t.Errorf("expected error writing json file when directory conflicts")
	}

	// 5c. SaveAgentConfig key file write failure
	secretDir := filepath.Join(tmpDir, "keyfail", "secrets", "keyfail-agent")
	_ = os.MkdirAll(secretDir, 0700)
	// Create signing-key.pem as a directory to force os.WriteFile failure
	_ = os.MkdirAll(filepath.Join(secretDir, "signing-key.pem"), 0700)
	keyFailPaths := Paths{
		DataHome:   filepath.Join(tmpDir, "keyfail"),
		AgentsDir:  filepath.Join(tmpDir, "keyfail", "agents"),
		SecretsDir: filepath.Join(tmpDir, "keyfail", "secrets"),
	}
	keyCfg, _ := NewAgentConfig("keyfail-agent", "base", "tester")
	if err := SaveAgentConfig(keyCfg, keyFailPaths); err == nil {
		t.Errorf("expected error writing key file")
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

	// 8. GetPaths with custom XDG_DATA_HOME and empty home
	os.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "custom-xdg"))
	customPaths := GetPaths()
	if customPaths.DataHome != filepath.Join(tmpDir, "custom-xdg", "agent-sandbox") {
		t.Errorf("unexpected custom data home: %s", customPaths.DataHome)
	}

	// Default GetPaths without XDG_DATA_HOME
	os.Unsetenv("XDG_DATA_HOME")
	_ = GetPaths()

	// 9. LoadEnv test
	envFile := filepath.Join(tmpDir, "test.env")
	envContent := "# Comment\nTEST_ENV_VAR_1=hello\nTEST_ENV_VAR_2=world\n\nINVALID_LINE"
	_ = os.WriteFile(envFile, []byte(envContent), 0600)
	envPaths := Paths{EnvFile: envFile}
	envPaths.LoadEnv()
	if os.Getenv("TEST_ENV_VAR_1") != "hello" || os.Getenv("TEST_ENV_VAR_2") != "world" {
		t.Errorf("LoadEnv failed to parse environment variables")
	}

	// LoadEnv on missing file
	missingEnvPaths := Paths{EnvFile: filepath.Join(tmpDir, "missing.env")}
	missingEnvPaths.LoadEnv()

	// 10. ResolveRepoDir tests
	// a. AGENT_SANDBOX_REPO env var
	mockRepoDir := filepath.Join(tmpDir, "mock-repo")
	_ = os.MkdirAll(mockRepoDir, 0755)
	_ = os.WriteFile(filepath.Join(mockRepoDir, "Makefile"), []byte("# mock"), 0644)
	os.Setenv("AGENT_SANDBOX_REPO", mockRepoDir)
	if r := paths.ResolveRepoDir(); r != mockRepoDir {
		t.Errorf("expected %s from AGENT_SANDBOX_REPO, got %s", mockRepoDir, r)
	}
	os.Unsetenv("AGENT_SANDBOX_REPO")

	// b. SANDBOX_ROOT env var
	os.Setenv("SANDBOX_ROOT", mockRepoDir)
	if r := paths.ResolveRepoDir(); r != mockRepoDir {
		t.Errorf("expected %s from SANDBOX_ROOT, got %s", mockRepoDir, r)
	}
	os.Unsetenv("SANDBOX_ROOT")

	// c. Standard XDG repo directory
	stdRepo := filepath.Join(paths.DataHome, "repo")
	_ = os.MkdirAll(stdRepo, 0755)
	_ = os.WriteFile(filepath.Join(stdRepo, "Makefile"), []byte("# mock std"), 0644)
	if r := paths.ResolveRepoDir(); r != stdRepo && !strings.Contains(r, "agent-sandbox") {
		t.Errorf("expected standard repo fallback, got %s", r)
	}
}
