// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boggycreek/sandbox/pkg/config"
	"github.com/boggycreek/sandbox/pkg/runtime"
	"github.com/boggycreek/sandbox/test/harness"
)

func TestAgentDoctorScenarios(t *testing.T) {
	tmpDir := t.TempDir()
	t.Cleanup(func() {
		_ = exec.Command("podman", "unshare", "rm", "-rf", tmpDir).Run()
	})
	t.Setenv("XDG_DATA_HOME", tmpDir)

	paths := config.GetPaths()
	if err := paths.EnsureDirectories(); err != nil {
		t.Fatalf("failed to create paths: %v", err)
	}

	var out, errOut string

	var code int
	// 1. Doctor without agent name argument
	code, _, errOut = runSndbx([]string{"agent", "doctor"})
	if code != 1 || !strings.Contains(errOut, "Usage: sndbx agent doctor") {
		t.Errorf("expected usage error when agent name missing, got code %d: %s", code, errOut)
	}

	// 2. Doctor on nonexistent agent
	code, out, errOut = runSndbx([]string{"agent", "doctor", "missing-agent"})
	if code != 1 || (!strings.Contains(out, "Missing config") && !strings.Contains(out, "unrepairable")) {
		t.Errorf("expected missing agent error, got code %d: out=%s err=%s", code, out, errOut)
	}

	// 3. Doctor on corrupted agent json
	corruptFile := filepath.Join(paths.AgentsDir, "broken.json")
	if err := os.WriteFile(corruptFile, []byte("{invalid-json"), 0600); err != nil {
		t.Fatalf("failed to write corrupted agent json: %v", err)
	}
	code, out, errOut = runSndbx([]string{"agent", "doctor", "broken"})
	if code != 1 || (!strings.Contains(out, "Corrupt JSON") && !strings.Contains(out, "corrupted agent config")) {
		t.Errorf("expected corrupted agent error, got code %d: out=%s err=%s", code, out, errOut)
	}

	// 4. Doctor on healthy agent configuration
	cfg, err := config.NewAgentConfig("doc-bot", "base", "qa-tester")
	if err != nil {
		t.Fatalf("failed creating agent config: %v", err)
	}
	if err := config.SaveAgentConfig(cfg, paths); err != nil {
		t.Fatalf("failed saving agent config: %v", err)
	}

	code, out, _ = runSndbx([]string{"agent", "doctor", "doc-bot"})
	if code != 0 {
		t.Errorf("expected doctor check to pass for healthy config, got code %d: %s", code, out)
	}
	if !strings.Contains(out, "doc-bot") {
		t.Errorf("expected doctor output to summarize doc-bot, got: %s", out)
	}
}

func TestAgentSSHConfigGeneration(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	paths := config.GetPaths()
	_ = paths.EnsureDirectories()

	// 1. ssh-config for nonexistent agent
	code, out, errOut := runSndbx([]string{"agent", "ssh-config", "nonexistent"})
	if code != 1 || (!strings.Contains(errOut, "not found") && !strings.Contains(out, "not found")) {
		t.Errorf("expected error for nonexistent agent ssh-config, got %d: %s", code, errOut)
	}

	// 2. ssh-config with no args when stopped agents exist
	cfg, _ := config.NewAgentConfig("stopped-agent", "base", "tester")
	_ = config.SaveAgentConfig(cfg, paths)

	code, out, _ = runSndbx([]string{"agent", "ssh-config"})
	if code != 0 {
		t.Errorf("expected ssh-config to succeed, got %d: %s", code, out)
	}

	// 3. ssh-config --all flag
	code, out, _ = runSndbx([]string{"agent", "ssh-config", "--all"})
	if code != 0 {
		t.Errorf("expected ssh-config --all to succeed, got %d: %s", code, out)
	}

	// 4. Test printSSHConfigBlock helper output
	var buf bytes.Buffer
	printSSHConfigBlock(&buf, "my-test-agent", 2222, "/home/test/.ssh/agent-sandbox")
	rendered := buf.String()
	if !strings.Contains(rendered, "Host sndbx-my-test-agent") || !strings.Contains(rendered, "Port 2222") {
		t.Errorf("unexpected ssh config block rendering: %s", rendered)
	}
	if !strings.Contains(rendered, "IdentitiesOnly yes") || !strings.Contains(rendered, "StrictHostKeyChecking accept-new") {
		t.Errorf("missing standard security directives in ssh config block: %s", rendered)
	}
}

func TestAgentCleanAndDeprovisioningWithMocks(t *testing.T) {
	valkey := harness.StartValkeyHarness(t)
	defer valkey.Teardown()

	tmpDir := t.TempDir()
	t.Cleanup(func() {
		_ = exec.Command("podman", "unshare", "rm", "-rf", tmpDir).Run()
	})
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("ADMIN_BACKPLANE_PASSWORD", valkey.AdminPass)
	t.Setenv("BP_HOST", "127.0.0.1")
	t.Setenv("BP_PORT", fmt.Sprintf("%d", valkey.Port))

	paths := config.GetPaths()
	_ = paths.EnsureDirectories()
	_ = os.WriteFile(paths.IDEKeyFile+".pub", []byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA test"), 0644)

	// Mock Gitea server
	giteaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer giteaServer.Close()
	t.Setenv("GITEA_URL", giteaServer.URL)

	// Mock Sonar server
	sonarServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/system/status":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"UP"}`))
		case "/api/user_tokens/generate":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"token":"sqa_mock_token"}`))
		case "/api/user_tokens/revoke":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer sonarServer.Close()
	t.Setenv("SONAR_HOST_URL", sonarServer.URL)

	// 1. Create agent with Gitea and Sonar integration active
	code, out, errOut := runSndbx([]string{"agent", "create", "fleet-agent", "as", "base", "--role", "integrator"})
	if code != 0 {
		t.Fatalf("agent create with forge mocks failed: code=%d, err=%s", code, errOut)
	}
	if !strings.Contains(out, "fleet-agent") || !strings.Contains(out, "created successfully") {
		t.Errorf("expected successful creation output, got: %s", out)
	}

	// 2. Clean agent container
	code, out, errOut = runSndbx([]string{"agent", "clean", "fleet-agent"})
	if code != 0 {
		t.Errorf("agent clean failed: code=%d err=%s", code, errOut)
	}
	if !strings.Contains(out, "fleet-agent") || !strings.Contains(out, "removed") {
		t.Errorf("expected clean success output, got: %s", out)
	}

	// 3. Retire agent with --force
	code, out, errOut = runSndbx([]string{"agent", "retire", "fleet-agent", "--force"})
	if code != 0 {
		t.Fatalf("agent retire failed: code=%d err=%s", code, errOut)
	}
	if !strings.Contains(out, "fleet-agent") || !strings.Contains(out, "retired") {
		t.Errorf("expected retire success output, got: %s", out)
	}

	// Verify agent configuration was removed
	if _, err := config.LoadAgentConfig("fleet-agent", paths); err == nil {
		t.Errorf("expected agent config to be deleted after retire")
	}
}

func TestInfraDoctorScenario(t *testing.T) {
	code, out, errOut := runSndbx([]string{"infra", "doctor"})
	if code != 0 {
		t.Errorf("infra doctor failed: code=%d err=%s", code, errOut)
	}
	if !strings.Contains(out, "infrastructure") {
		t.Errorf("expected infra doctor report, got: %s", out)
	}
}

func TestInfraUpScenario(t *testing.T) {
	origExec := execCommandContext
	defer func() { execCommandContext = origExec }()
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	code, out, errOut := runSndbx([]string{"infra", "up"})
	if code != 0 {
		t.Errorf("infra up failed: code=%d err=%s", code, errOut)
	}
	if !strings.Contains(out, "Starting shared infrastructure") || !strings.Contains(out, "online") {
		t.Errorf("expected online message from infra up, got: %s", out)
	}
}

func TestAgentStartAndStopLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	paths := config.GetPaths()
	_ = paths.EnsureDirectories()

	// 1. Start nonexistent agent fails
	code, _, errOut := runSndbx([]string{"agent", "start", "nonexistent-start"})
	if code != 1 || !strings.Contains(errOut, "not found") {
		t.Errorf("expected start nonexistent agent to fail, got %d: %s", code, errOut)
	}

	// 2. Start existing agent with mocked podman
	restore := runtime.SetExecCommandContextForTesting(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	})
	defer restore()

	cfg, _ := config.NewAgentConfig("run-bot", "base", "developer")
	_ = config.SaveAgentConfig(cfg, paths)

	code, out, errOut := runSndbx([]string{"agent", "start", "run-bot"})
	if code != 0 {
		t.Errorf("agent start run-bot failed: code=%d err=%s", code, errOut)
	}
	if !strings.Contains(out, "started") {
		t.Errorf("expected start success output, got: %s", out)
	}

	// 3. Stop existing agent
	code, out, errOut = runSndbx([]string{"agent", "stop", "run-bot"})
	if code != 0 {
		t.Errorf("agent stop run-bot failed: code=%d err=%s", code, errOut)
	}
	if !strings.Contains(out, "stopped") {
		t.Errorf("expected stop success output, got: %s", out)
	}
}

func TestAgentListTableMultipleAgents(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("XDG_DATA_HOME", tmpDir)

	paths := config.GetPaths()
	_ = paths.EnsureDirectories()

	cfg1, _ := config.NewAgentConfig("alpha-bot", "base", "researcher")
	cfg2, _ := config.NewAgentConfig("beta-bot", "opencode", "coder")
	_ = config.SaveAgentConfig(cfg1, paths)
	_ = config.SaveAgentConfig(cfg2, paths)

	code, out, errOut := runSndbx([]string{"agent", "list"})
	if code != 0 {
		t.Fatalf("agent list failed: code=%d err=%s", code, errOut)
	}
	if !strings.Contains(out, "alpha-bot") || !strings.Contains(out, "beta-bot") {
		t.Errorf("expected both agents in table output, got: %s", out)
	}
	if !strings.Contains(out, "AGENT_NAME") || !strings.Contains(out, "STATUS") {
		t.Errorf("expected table header in list output, got: %s", out)
	}
}

func TestUpdateFallbackAndSanitizeEnv(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	origExec := execCommandContext
	origHTTP := httpClientForDownload
	defer func() {
		execCommandContext = origExec
		httpClientForDownload = origHTTP
	}()

	// 1. Success via prebuilt binary download
	httpClientForDownload = &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(strings.NewReader("#!/bin/sh\nexit 0\n")),
			}, nil
		},
	}
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	code, out, _ := runSndbx([]string{"update"})
	if code != 0 {
		t.Errorf("expected update to succeed via prebuilt download, got code %d", code)
	}
	if !strings.Contains(out, "Update complete") {
		t.Errorf("expected update complete message, got: %s", out)
	}

	// 2. Prebuilt 404 fallback to local build
	httpClientForDownload = &mockHTTPClient{
		doFunc: func(req *http.Request) (*http.Response, error) {
			return &http.Response{
				StatusCode: http.StatusNotFound,
				Body:       io.NopCloser(strings.NewReader("404")),
			}, nil
		},
	}
	code, _, _ = runSndbx([]string{"update"})
	if code != 0 {
		t.Errorf("expected update to succeed via local build fallback, got code %d", code)
	}

	// 3. Test sanitizeGoEnv
	cleaned := sanitizeGoEnv([]string{"PATH=/bin", "GOROOT=/opt/bad/goroot", "HOME=/home/user"})
	for _, env := range cleaned {
		if strings.HasPrefix(env, "GOROOT=") {
			t.Errorf("expected GOROOT to be removed by sanitizeGoEnv, got: %s", env)
		}
	}
}

type mockHTTPClient struct {
	doFunc func(req *http.Request) (*http.Response, error)
}

func (m *mockHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if m.doFunc != nil {
		return m.doFunc(req)
	}
	return &http.Response{
		StatusCode: http.StatusNotFound,
		Body:       io.NopCloser(strings.NewReader("404 Not Found")),
	}, nil
}
