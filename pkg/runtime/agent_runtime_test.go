// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boggycreek/sandbox/pkg/config"
)

func TestAgentContainerInspectionAndPortLookup(t *testing.T) {
	ctx := context.Background()

	// 1. Empty container name lookups fail
	if _, err := GetAgentSSHPort(ctx, ""); err == nil {
		t.Errorf("expected error for empty container port lookup")
	}
	if _, err := InspectAgentContainer(ctx, ""); err == nil {
		t.Errorf("expected error for empty container inspect")
	}

	// 2. Mocked inspect fallback
	mockInspect := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 0 && args[0] == "inspect" {
			jsonPayload := `[{"Id":"c1","Name":"agent-container","State":{"Status":"","Running":false}}]`
			return exec.Command("echo", jsonPayload)
		}
		return exec.Command("true")
	}
	restore := SetExecCommandContextForTesting(mockInspect)
	defer restore()

	info, err := InspectAgentContainer(ctx, "agent-container")
	if err != nil || info == nil {
		t.Fatalf("InspectAgentContainer failed: %v", err)
	}
	if info.State != "stopped" {
		t.Errorf("expected stopped state, got %s", info.State)
	}
	if len(info.Names) == 0 || info.Names[0] != "agent-container" {
		t.Errorf("expected name agent-container, got %v", info.Names)
	}
}

func TestClearAgentKnownHosts(t *testing.T) {
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, "ssh")
	_ = os.MkdirAll(sshDir, 0700)

	paths := config.Paths{
		DataHome: tmpDir,
		SSHDir:   sshDir,
	}

	testAgentName := "test-known-hosts-agent"
	customHostFile := filepath.Join(paths.SSHDir, fmt.Sprintf("known_hosts.sndbx-%s", testAgentName))
	_ = os.WriteFile(customHostFile, []byte("dummy host entry"), 0600)

	ClearAgentKnownHosts(testAgentName, paths)
	if _, err := os.Stat(customHostFile); !os.IsNotExist(err) {
		t.Errorf("expected custom known hosts file to be removed")
	}
}

func TestFormatSSHConfigBlock(t *testing.T) {
	block := FormatSSHConfigBlock("alpha", 34567, "/home/test/.ssh/agent-sandbox")
	if !strings.Contains(block, "Host sndbx-alpha") || !strings.Contains(block, "Port 34567") {
		t.Errorf("unexpected FormatSSHConfigBlock output: %s", block)
	}
	if !strings.Contains(block, "IdentitiesOnly yes") || !strings.Contains(block, "StrictHostKeyChecking accept-new") {
		t.Errorf("expected IdentitiesOnly and accept-new in FormatSSHConfigBlock: %s", block)
	}
	if !strings.Contains(block, "known_hosts.sndbx-alpha") {
		t.Errorf("expected known_hosts.sndbx-alpha in UserKnownHostsFile: %s", block)
	}
}

func TestSyncSSHConfigFileWithMockPorts(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	mockPort := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "port" {
			return exec.Command("echo", "127.0.0.1:22222")
		}
		return exec.Command("true")
	}
	restore := SetExecCommandContextForTesting(mockPort)
	defer restore()

	paths := config.Paths{
		DataHome:      tmpDir,
		AgentsDir:     filepath.Join(tmpDir, "agents"),
		SecretsDir:    filepath.Join(tmpDir, "secrets"),
		SSHConfigFile: filepath.Join(tmpDir, "ssh_config"),
	}
	_ = paths.EnsureDirectories()

	cfg1 := &config.AgentConfig{Name: "agent-1", ContainerName: "sndbx-agent-1"}
	cfg2 := &config.AgentConfig{Name: "agent-2", ContainerName: "sndbx-agent-2"}
	_ = config.SaveAgentConfig(cfg1, paths)
	_ = config.SaveAgentConfig(cfg2, paths)

	err := SyncSSHConfigFile(ctx, paths)
	if err != nil {
		t.Errorf("SyncSSHConfigFile failed: %v", err)
	}

	data, err := os.ReadFile(paths.SSHConfigFile)
	if err != nil {
		t.Fatalf("failed reading synced ssh config: %v", err)
	}
	if !strings.Contains(string(data), "Host sndbx-agent-1") || !strings.Contains(string(data), "Host sndbx-agent-2") {
		t.Errorf("expected both host blocks in sync'd config, got: %s", string(data))
	}

	// Test fallback when SSHConfigFile is empty
	pathsDefaultSSH := config.Paths{
		DataHome:  tmpDir,
		AgentsDir: filepath.Join(tmpDir, "agents"),
	}
	if err := SyncSSHConfigFile(ctx, pathsDefaultSSH); err != nil {
		t.Errorf("SyncSSHConfigFile with default path failed: %v", err)
	}
}

func TestAgentContainerLifecycleMocks(t *testing.T) {
	ctx := context.Background()

	// 1. Container already exists -> podman start
	mockExisting := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "container" && args[1] == "exists" {
			return exec.Command("true")
		}
		return exec.Command("true")
	}
	restore1 := SetExecCommandContextForTesting(mockExisting)
	cfg := &config.AgentConfig{
		Name:          "exists-agent",
		ContainerName: "sndbx-exists-agent",
		VolumeName:    "sndbx-exists-vol",
		Image:         "base",
	}
	err := StartAgentContainer(ctx, cfg, config.Paths{}, "127.0.0.1", 6379)
	restore1()
	if err != nil {
		t.Errorf("StartAgentContainer existing failed: %v", err)
	}

	// 2. Existing container start failure
	mockExistingFail := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "container" && args[1] == "exists" {
			return exec.Command("true")
		}
		if name == "podman" && len(args) > 0 && args[0] == "start" {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	restore2 := SetExecCommandContextForTesting(mockExistingFail)
	err = StartAgentContainer(ctx, cfg, config.Paths{}, "127.0.0.1", 6379)
	restore2()
	if err == nil {
		t.Errorf("expected error when starting existing container fails")
	}
}

func TestStartEgressFilterContainerScenarios(t *testing.T) {
	ctx := context.Background()

	// 1. Existing egress filter container
	mockExisting := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "container" && args[1] == "exists" {
			return exec.Command("true")
		}
		return exec.Command("true")
	}
	restore1 := SetExecCommandContextForTesting(mockExisting)
	if err := StartEgressFilterContainer(ctx, "agent-1"); err != nil {
		t.Errorf("expected StartEgressFilterContainer to succeed for existing container: %v", err)
	}
	restore1()

	// 2. New egress filter container
	mockNew := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "container" && args[1] == "exists" {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	restore2 := SetExecCommandContextForTesting(mockNew)
	if err := StartEgressFilterContainer(ctx, "agent-2"); err != nil {
		t.Errorf("expected StartEgressFilterContainer to succeed for new container: %v", err)
	}
	restore2()

	// 3. Egress filter container run failure
	mockFail := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "container" && args[1] == "exists" {
			return exec.Command("false")
		}
		if name == "podman" && len(args) > 0 && args[0] == "run" {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	restore3 := SetExecCommandContextForTesting(mockFail)
	if err := StartEgressFilterContainer(ctx, "agent-fail"); err == nil {
		t.Errorf("expected error when egress filter run fails")
	}
	restore3()
}

func TestStartAgentContainerWithModelAndMounts(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	pubKeyPath := filepath.Join(tmpDir, "agent-sandbox.pub")
	if err := os.WriteFile(pubKeyPath, []byte("ssh-ed25519 AAAAC3... test"), 0644); err != nil {
		t.Fatalf("failed writing mock pubkey: %v", err)
	}

	paths := config.Paths{
		DataHome:   tmpDir,
		IDEKeyFile: filepath.Join(tmpDir, "agent-sandbox"),
	}

	cfg := &config.AgentConfig{
		Name:          "model-agent",
		ContainerName: "sndbx-model-agent",
		VolumeName:    "sndbx-model-vol",
		Image:         "agent-sandbox-base:latest",
		ModelURL:      "http://localhost:11434/v1",
		ModelName:     "qwen2.5-coder:32b",
		ModelAPIKey:   "secret-ollama-key",
		SonarToken:    "sqa_full_token",
	}

	var capturedArgs []string
	mockRun := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "container" && args[1] == "exists" {
			return exec.Command("false")
		}
		if name == "podman" && len(args) > 0 && args[0] == "run" {
			capturedArgs = args
			return exec.Command("true")
		}
		return exec.Command("true")
	}

	restore := SetExecCommandContextForTesting(mockRun)
	defer restore()

	if err := StartAgentContainer(ctx, cfg, paths, "localhost", 6379); err != nil {
		t.Fatalf("StartAgentContainer failed: %v", err)
	}

	argsJoined := strings.Join(capturedArgs, " ")
	if !strings.Contains(argsJoined, "llm-gateway:11434/v1") {
		t.Errorf("expected llm-gateway URL translation in args: %s", argsJoined)
	}
	if !strings.Contains(argsJoined, "SONAR_TOKEN=sqa_full_token") {
		t.Errorf("expected SONAR_TOKEN in args: %s", argsJoined)
	}
	if !strings.Contains(argsJoined, "qwen2.5-coder:32b") {
		t.Errorf("expected model name in args: %s", argsJoined)
	}
	if !strings.Contains(argsJoined, "host-keys/agent-sandbox.pub:ro") {
		t.Errorf("expected pubkey mount in args: %s", argsJoined)
	}
}

func TestResolveAgentImageTiers(t *testing.T) {
	ctx := context.Background()

	// 1. Preset base image
	mockBase := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}
	restore1 := SetExecCommandContextForTesting(mockBase)
	img, isLocal := ResolveAgentImage(ctx, "base")
	restore1()
	if img != "agent-sandbox-base:latest" || !isLocal {
		t.Errorf("expected base image local, got %s, %v", img, isLocal)
	}

	// 2. Empty string defaults to base
	restore2 := SetExecCommandContextForTesting(mockBase)
	img, isLocal = ResolveAgentImage(ctx, "")
	restore2()
	if img != "agent-sandbox-base:latest" || !isLocal {
		t.Errorf("expected empty string to resolve to base, got %s", img)
	}

	// 3. Local candidate without tag
	mockLocal := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if len(args) > 2 && strings.Contains(args[2], "my-custom") {
			return exec.Command("true")
		}
		return exec.Command("false")
	}
	restore3 := SetExecCommandContextForTesting(mockLocal)
	_, isLocal = ResolveAgentImage(ctx, "my-custom")
	restore3()
	if !isLocal {
		t.Errorf("expected my-custom to resolve to local candidate")
	}

	// 4. Remote image with tag not found locally
	mockRemote := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}
	restore4 := SetExecCommandContextForTesting(mockRemote)
	img, isLocal = ResolveAgentImage(ctx, "docker.io/library/ubuntu:22.04")
	restore4()
	if isLocal || img != "docker.io/library/ubuntu:22.04" {
		t.Errorf("expected remote image reference, got %s, %v", img, isLocal)
	}
}
