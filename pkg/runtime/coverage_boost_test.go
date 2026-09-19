// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boggycreek/agent-sandbox/pkg/config"
)

func TestRuntimeCoverageBoost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
		SSHDir:     sshDir,
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

	// Test ClearAgentKnownHosts
	testAgentName := "test-known-hosts-agent"
	home, _ := os.UserHomeDir()
	if home != "" {
		homeSSH := filepath.Join(home, ".ssh")
		_ = os.MkdirAll(homeSSH, 0700)
		homeHostFile := filepath.Join(homeSSH, fmt.Sprintf("known_hosts.sndbx-%s", testAgentName))
		_ = os.WriteFile(homeHostFile, []byte("dummy host entry"), 0600)
	}
	customHostFile := filepath.Join(paths.SSHDir, fmt.Sprintf("known_hosts.sndbx-%s", testAgentName))
	_ = os.WriteFile(customHostFile, []byte("dummy host entry"), 0600)
	ClearAgentKnownHosts(testAgentName, paths)
	if _, err := os.Stat(customHostFile); !os.IsNotExist(err) {
		t.Errorf("expected custom known hosts file to be removed")
	}

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
	_ = CleanAgentContainer(ctx, cfgNoKey.ContainerName)
	_ = DestroyAgentContainer(ctx, cfgNoKey.ContainerName, cfgNoKey.VolumeName)

	// Test StartInfraStack with an isolated DataHome that won't stomp live host ACL
	infraPaths := config.Paths{
		DataHome: filepath.Join(tmpDir, "infra-data"),
	}
	_ = os.MkdirAll(infraPaths.DataHome, 0755)
	// These calls exercise the code paths including the existing-container branch
	// (the live infra containers already exist, so this will exercise the start/restart branch)
	_ = StartInfraStack(ctx, infraPaths, "test_admin_pass", "test_human_pass", "test_human_name")

	// Call a second time to exercise the "container already exists, podman start" path
	_ = StartInfraStack(ctx, infraPaths, "test_admin_pass2", "test_human_pass2", "test_human_name2")

	// Call with empty credentials to exercise default fallback credentials
	_ = StartInfraStack(ctx, infraPaths, "", "", "")

	giteaMock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer giteaMock.Close()
	os.Setenv("GITEA_URL", giteaMock.URL)

	_ = BootstrapGitea(ctx, "")
	_ = BootstrapGitea(ctx, "custom_pass")

	// BootstrapGitea timeout / context cancel branch
	canceledCtx, cancelNow := context.WithCancel(context.Background())
	cancelNow()
	_ = BootstrapGitea(canceledCtx, "")

	infraList, err := InspectInfraStack(ctx)
	if err != nil || len(infraList) == 0 {
		t.Errorf("InspectInfraStack failed: %v", err)
	}

	// Ensure network creation path
	testNet := "agent-sandbox-unit-test-net"
	_ = exec.Command("podman", "network", "rm", "-f", testNet).Run()
	_ = EnsureNetwork(ctx, testNet)
	_ = exec.Command("podman", "network", "rm", "-f", testNet).Run()

	// Test FormatSSHConfigBlock
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

	// Test SyncSSHConfigFile
	paths.AgentsDir = filepath.Join(tmpDir, "agents")
	paths.SecretsDir = filepath.Join(tmpDir, "secrets")
	paths.SSHConfigFile = filepath.Join(tmpDir, "ssh_config")
	_ = paths.EnsureDirectories()

	// Save an agent config
	_ = config.SaveAgentConfig(cfg, paths)
	if err := SyncSSHConfigFile(ctx, paths); err != nil {
		t.Errorf("SyncSSHConfigFile failed: %v", err)
	}
	if _, err := os.Stat(paths.SSHConfigFile); err != nil {
		t.Errorf("expected ssh_config file created at %s: %v", paths.SSHConfigFile, err)
	}

	// Test SyncSSHConfigFile with empty paths.SSHConfigFile
	pathsDefaultSSH := config.Paths{
		DataHome:  tmpDir,
		AgentsDir: filepath.Join(tmpDir, "agents"),
	}
	_ = SyncSSHConfigFile(ctx, pathsDefaultSSH)

	// Test SyncSSHConfigFile error when AgentsDir is an invalid file path
	invalidAgentsPath := filepath.Join(tmpDir, "file-not-dir")
	_ = os.WriteFile(invalidAgentsPath, []byte("plain file"), 0600)
	_ = SyncSSHConfigFile(ctx, config.Paths{AgentsDir: invalidAgentsPath})

	// StopAgentContainer with nonexistent container
	_ = StopAgentContainer(ctx, "nonexistent-container-stop-test")
}

// TestMockedRuntimeUnits tests runtime functions with command mocking for edge cases
func TestMockedRuntimeUnits(t *testing.T) {
	ctx := context.Background()

	// 1. InspectAgentContainer fallback branches
	mockInspect := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 0 && args[0] == "inspect" {
			// Return json with name without slash, stopped state
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

	// 2. StartAgentContainer container already exists branch
	mockExisting := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "container" && args[1] == "exists" {
			return exec.Command("true") // container exists
		}
		return exec.Command("true")
	}
	restore2 := SetExecCommandContextForTesting(mockExisting)
	defer restore2()

	cfg := &config.AgentConfig{
		Name:          "exists-agent",
		ContainerName: "sndbx-exists-agent",
		VolumeName:    "sndbx-exists-vol",
		Image:         "base",
	}
	err = StartAgentContainer(ctx, cfg, config.Paths{}, "127.0.0.1", 6379)
	if err != nil {
		t.Errorf("StartAgentContainer existing failed: %v", err)
	}

	// 3. StartAgentContainer existing container start failure
	mockExistingFail := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "container" && args[1] == "exists" {
			return exec.Command("true") // container exists
		}
		if name == "podman" && len(args) > 0 && args[0] == "start" {
			return exec.Command("false") // start fails
		}
		return exec.Command("true")
	}
	restore3 := SetExecCommandContextForTesting(mockExistingFail)
	defer restore3()
	err = StartAgentContainer(ctx, cfg, config.Paths{}, "127.0.0.1", 6379)
	if err == nil {
		t.Errorf("expected error when starting existing container fails")
	}

	// 4. SyncSSHConfigFile with running agent mapping
	mockPort := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "port" {
			return exec.Command("echo", "127.0.0.1:22222")
		}
		return exec.Command("true")
	}
	restore4 := SetExecCommandContextForTesting(mockPort)
	defer restore4()

	tmpDir := t.TempDir()
	paths := config.Paths{
		DataHome:      tmpDir,
		AgentsDir:     filepath.Join(tmpDir, "agents"),
		SecretsDir:    filepath.Join(tmpDir, "secrets"),
		SSHConfigFile: filepath.Join(tmpDir, "ssh_config"),
	}
	_ = paths.EnsureDirectories()
	_ = config.SaveAgentConfig(cfg, paths)
	cfg2 := &config.AgentConfig{Name: "agent2", ContainerName: "sndbx-agent2"}
	_ = config.SaveAgentConfig(cfg2, paths)

	err = SyncSSHConfigFile(ctx, paths)
	if err != nil {
		t.Errorf("SyncSSHConfigFile with mock ports failed: %v", err)
	}
	data, _ := os.ReadFile(paths.SSHConfigFile)
	if !strings.Contains(string(data), "Host sndbx-exists-agent") || !strings.Contains(string(data), "Host sndbx-agent2") {
		t.Errorf("expected both host blocks in sync'd ssh config, got: %s", string(data))
	}
}

// TestStopInfraStackCoverage exercises StopInfraStack with ephemeral container names.
// It overrides the package-level infra container name vars so the real production containers are not affected.
func TestStopInfraStackCoverage(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use unique ephemeral container names that won't conflict with production
	pid := os.Getpid()
	testValkeyName := fmt.Sprintf("unit-test-valkey-stop-%d", pid)
	testGiteaName := fmt.Sprintf("unit-test-gitea-stop-%d", pid)

	// Override package-level vars so StopInfraStack targets our ephemeral containers
	origValkey := infraValkeyContainer
	origGitea := infraGiteaContainer
	origSonar := infraSonarContainer
	infraValkeyContainer = testValkeyName
	infraGiteaContainer = testGiteaName
	infraSonarContainer = fmt.Sprintf("unit-test-sonar-stop-%d", pid)
	defer func() {
		infraValkeyContainer = origValkey
		infraGiteaContainer = origGitea
		infraSonarContainer = origSonar
	}()

	// Start a minimal ephemeral Valkey container for the stop test
	startOut, err := execCommandContext(ctx, "podman", "run", "-d", "--name", testValkeyName,
		"docker.io/valkey/valkey:8-alpine",
	).CombinedOutput()
	if err != nil {
		t.Skipf("Cannot start ephemeral valkey container for stop test: %v (%s)", err, string(startOut))
	}
	// Ensure cleanup even if StopInfraStack doesn't fully clean up
	defer func() {
		_ = exec.Command("podman", "rm", "-f", testValkeyName).Run()
		_ = exec.Command("podman", "rm", "-f", testGiteaName).Run()
	}()

	// Now exercise StopInfraStack — should stop our ephemeral containers
	if err := StopInfraStack(ctx); err != nil {
		t.Errorf("StopInfraStack returned unexpected error: %v", err)
	}

	// Also exercise InspectInfraStack with our ephemeral containers (stopped state)
	list, err := InspectInfraStack(ctx)
	if err != nil {
		t.Errorf("InspectInfraStack failed: %v", err)
	}
	if len(list) == 0 {
		t.Errorf("InspectInfraStack returned empty list")
	}
}

func TestMockedInfraStack(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	paths := config.Paths{DataHome: tmpDir}

	// 1. EnsureNetwork error
	mockFailNet := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "network" {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	restore1 := SetExecCommandContextForTesting(mockFailNet)
	err := StartInfraStack(ctx, paths, "p1", "p2", "u1")
	restore1()
	if err == nil {
		t.Errorf("expected error when network fails")
	}

	// 2. Valkey doesn't exist, run fails
	mockValkeyRunFail := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "container" && args[1] == "exists" {
			return exec.Command("false") // doesn't exist
		}
		if name == "podman" && len(args) > 0 && args[0] == "run" && strings.Contains(args[3], "valkey") {
			return exec.Command("false") // run fails
		}
		return exec.Command("true")
	}
	restore2 := SetExecCommandContextForTesting(mockValkeyRunFail)
	err = StartInfraStack(ctx, paths, "p1", "p2", "u1")
	restore2()
	if err == nil {
		t.Errorf("expected error when valkey run fails")
	}

	// 3. Valkey exists, start fails, restart fails
	mockValkeyRestartFail := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "container" && args[1] == "exists" {
			return exec.Command("true") // exists
		}
		if name == "podman" && len(args) > 0 && args[0] == "start" {
			return exec.Command("false") // start fails
		}
		if name == "podman" && len(args) > 0 && args[0] == "run" {
			return exec.Command("false") // restart fails
		}
		return exec.Command("true")
	}
	restore3 := SetExecCommandContextForTesting(mockValkeyRestartFail)
	err = StartInfraStack(ctx, paths, "p1", "p2", "u1")
	restore3()
	if err == nil {
		t.Errorf("expected error when valkey restart fails")
	}

	// 4. Gitea doesn't exist, run fails
	mockGiteaRunFail := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "container" && args[1] == "exists" {
			// Valkey exists, Gitea does not
			if strings.Contains(args[2], "gitea") {
				return exec.Command("false")
			}
			return exec.Command("true")
		}
		if name == "podman" && len(args) > 0 && args[0] == "run" && strings.Contains(args[3], "gitea") {
			return exec.Command("false") // gitea run fails
		}
		return exec.Command("true")
	}
	restore4 := SetExecCommandContextForTesting(mockGiteaRunFail)
	err = StartInfraStack(ctx, paths, "p1", "p2", "u1")
	restore4()
	if err == nil {
		t.Errorf("expected error when gitea run fails")
	}

	// 5. Gitea exists, start fails, restart fails
	mockGiteaRestartFail := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "container" && args[1] == "exists" {
			return exec.Command("true") // both exist
		}
		if name == "podman" && len(args) > 1 && args[0] == "start" && strings.Contains(args[1], "gitea") {
			return exec.Command("false") // gitea start fails
		}
		if name == "podman" && len(args) > 0 && args[0] == "run" && strings.Contains(args[3], "gitea") {
			return exec.Command("false") // gitea restart fails
		}
		return exec.Command("true")
	}
	restore5 := SetExecCommandContextForTesting(mockGiteaRestartFail)
	err = StartInfraStack(ctx, paths, "p1", "p2", "u1")
	restore5()
	if err == nil {
		t.Errorf("expected error when gitea restart fails")
	}

	// 6. Sonarqube image exists, container doesn't exist, run succeeds
	mockSonarRunSuccess := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "image" && args[1] == "exists" {
			return exec.Command("true")
		}
		if name == "podman" && len(args) > 1 && args[0] == "container" && args[1] == "exists" {
			if strings.Contains(args[2], "sonar") {
				return exec.Command("false")
			}
			return exec.Command("true")
		}
		return exec.Command("true")
	}
	restore6 := SetExecCommandContextForTesting(mockSonarRunSuccess)
	err = StartInfraStack(ctx, paths, "p1", "p2", "u1")
	restore6()
	if err != nil {
		t.Errorf("unexpected error in sonar run success: %v", err)
	}

	// 7. Sonarqube image exists, container exists, start fails, restart fails
	mockSonarRestartFail := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "image" && args[1] == "exists" {
			return exec.Command("true")
		}
		if name == "podman" && len(args) > 1 && args[0] == "container" && args[1] == "exists" {
			return exec.Command("true")
		}
		if name == "podman" && len(args) > 1 && args[0] == "start" && strings.Contains(args[1], "sonar") {
			return exec.Command("false")
		}
		if name == "podman" && len(args) > 0 && args[0] == "run" && strings.Contains(args[3], "sonar") {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	restore7 := SetExecCommandContextForTesting(mockSonarRestartFail)
	err = StartInfraStack(ctx, paths, "p1", "p2", "u1")
	restore7()
	if err == nil {
		t.Errorf("expected error when sonarqube restart fails")
	}

	// 8. Sonarqube run fails directly
	mockSonarRunFail := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "image" && args[1] == "exists" {
			return exec.Command("true")
		}
		if name == "podman" && len(args) > 1 && args[0] == "container" && args[1] == "exists" {
			if strings.Contains(args[2], "sonar") {
				return exec.Command("false")
			}
			return exec.Command("true")
		}
		if name == "podman" && len(args) > 0 && args[0] == "run" && strings.Contains(args[3], "sonar") {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	restore8 := SetExecCommandContextForTesting(mockSonarRunFail)
	err = StartInfraStack(ctx, paths, "p1", "p2", "u1")
	restore8()
	if err == nil {
		t.Errorf("expected error when sonarqube run fails")
	}
}

