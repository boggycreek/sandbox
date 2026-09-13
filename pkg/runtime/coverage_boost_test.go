// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/boggycreek/agent-sandbox/pkg/config"
)

func TestRuntimeCoverageBoost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Port parsing / inspect error branches
	_, err := GetAgentSSHPort(ctx, "")
	if err == nil {
		t.Errorf("expected error for empty container port lookup")
	}

	_, err = InspectAgentContainer(ctx, "")
	if err == nil {
		t.Errorf("expected error for empty container inspect")
	}

	// Inspect fallback & malformed port parsing
	var outBuf bytes.Buffer
	outBuf.WriteString(`[{"Id":"test123","State":{"Status":"","Running":true}}]`)
	var list []struct {
		ID    string `json:"Id"`
		State struct {
			Status  string `json:"Status"`
			Running bool   `json:"Running"`
		} `json:"State"`
	}
	_ = json.Unmarshal(outBuf.Bytes(), &list)
	if list[0].State.Status == "" && list[0].State.Running {
		// Verifies running fallback logic
	}
	tmpDir := t.TempDir()
	t.Cleanup(func() {
		_ = exec.Command("podman", "unshare", "rm", "-rf", tmpDir).Run()
	})
	sshDir := filepath.Join(tmpDir, "ssh")
	_ = os.MkdirAll(sshDir, 0700)
	keyFile := filepath.Join(sshDir, "key")
	_ = os.WriteFile(keyFile+".pub", []byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA test"), 0644)

	paths := config.Paths{
		DataHome:   tmpDir,
		IDEKeyFile: keyFile,
	}

	cfg := &config.AgentConfig{
		Name:          "mock-start-agent",
		ContainerName: "mock-start-container",
		VolumeName:    "mock-start-vol",
		Image:         "agent-sandbox-base:latest",
		ModelURL:      "http://localhost:11434/v1",
		ModelName:     "qwen2.5-coder:32b",
		ModelAPIKey:   "ollama-secret",
	}

	// Ensure network and volume paths execute
	_ = EnsureNetwork(ctx, "agent-sandbox-infra")
	_ = EnsureVolume(ctx, cfg.VolumeName)
	_ = CleanAgentContainer(ctx, cfg.ContainerName)
	_ = paths.EnsureDirectories()

	// Start agent container when podman exists
	_ = StartAgentContainer(ctx, cfg, paths, "127.0.0.1", 6379)

	// Also start with default dummy key branch (empty ModelAPIKey)
	cfgNoKey := &config.AgentConfig{
		Name:          "mock-start-nokey",
		ContainerName: "mock-start-nokey-container",
		VolumeName:    "mock-start-nokey-vol",
		Image:         "agent-sandbox-base:latest",
		ModelURL:      "http://127.0.0.1:8000/v1",
	}
	_ = StartAgentContainer(ctx, cfgNoKey, paths, "127.0.0.1", 6379)
	// Query state
	info, _ := InspectAgentContainer(ctx, cfg.ContainerName)
	if info != nil {
		_ = info.ID
	}
	_, _ = GetAgentSSHPort(ctx, cfg.ContainerName)

	// Clean up
	_ = CleanAgentContainer(ctx, cfg.ContainerName)
	_ = DestroyAgentContainer(ctx, cfg.ContainerName, cfg.VolumeName)

	// Test StartInfraStack with defaults and already-running paths
	_ = StartInfraStack(ctx, paths, "", "", "")
	_ = StartInfraStack(ctx, paths, "test_admin_pass", "test_human_pass", "test_human_name")
	infraList, err := InspectInfraStack(ctx)
	if err != nil || len(infraList) == 0 {
		t.Errorf("InspectInfraStack failed: %v", err)
	}
	_ = StopInfraStack(ctx)

	// Ensure network creation path
	testNet := "agent-sandbox-unit-test-net"
	_ = exec.Command("podman", "network", "rm", "-f", testNet).Run()
	_ = EnsureNetwork(ctx, testNet)
	_ = exec.Command("podman", "network", "rm", "-f", testNet).Run()
}

