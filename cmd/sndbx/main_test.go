// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boggycreek/sandbox/pkg/config"
	"github.com/boggycreek/sandbox/pkg/libbp"
	"github.com/boggycreek/sandbox/test/harness"
)

func runSndbx(args []string) (int, string, string) {
	var stdout, stderr bytes.Buffer
	code := Run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func TestSndbxCLIUsageAndHelp(t *testing.T) {
	// 1. Empty args
	code, out, _ := runSndbx([]string{})
	if code != 1 || !strings.Contains(out, "Agent Sandbox CLI (sndbx)") {
		t.Errorf("expected usage output on empty args")
	}

	// 2. Help
	for _, h := range []string{"help", "-h", "--help"} {
		code, out, _ = runSndbx([]string{h})
		if code != 0 || !strings.Contains(out, "Usage:") {
			t.Errorf("%s failed", h)
		}
	}

	// 3. Unknown command
	code, _, errOut := runSndbx([]string{"unknown"})
	if code != 1 || !strings.Contains(errOut, "unknown command") {
		t.Errorf("expected unknown command error")
	}
}

func TestSndbxAgentDomain(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", tmpDir)

	// Agent help / empty
	code, _, errOut := runSndbx([]string{"agent"})
	if code != 1 || !strings.Contains(errOut, "Usage: sndbx agent") {
		t.Errorf("agent empty failed")
	}

	for _, h := range []string{"help", "-h", "--help"} {
		code, out, _ := runSndbx([]string{"agent", h})
		if code != 0 || !strings.Contains(out, "Usage: sndbx agent") {
			t.Errorf("agent %s failed", h)
		}
	}

	// Unknown agent sub
	code, _, errOut = runSndbx([]string{"agent", "unknownsub"})
	if code != 1 || !strings.Contains(errOut, "unknown command") {
		t.Errorf("agent unknown sub failed")
	}

	// Create without args
	code, _, errOut = runSndbx([]string{"agent", "create"})
	if code != 1 || !strings.Contains(errOut, "Usage: sndbx agent create") {
		t.Errorf("agent create empty failed")
	}

	// Create with 'as opencode'
	code, out, _ := runSndbx([]string{"agent", "create", "coder-1", "as", "opencode", "--role", "developer"})
	if code != 0 || !strings.Contains(out, "created successfully") || !strings.Contains(out, "agent-sandbox-opencode:latest") {
		t.Errorf("agent create as opencode failed: %s", out)
	}

	// Create with '--image claude'
	code, out, _ = runSndbx([]string{"agent", "create", "coder-2", "--image", "claude"})
	if code != 0 || !strings.Contains(out, "created successfully") || !strings.Contains(out, "agent-sandbox-claude:latest") {
		t.Errorf("agent create --image claude failed: %s", out)
	}

	// Create with '--model-url' and '--model-name'
	code, out, _ = runSndbx([]string{"agent", "create", "coder-ollama", "as", "opencode", "--model-url", "http://localhost:11434/v1", "--model-name", "qwen2.5-coder:32b", "--model-key", "ollama-key"})
	if code != 0 || !strings.Contains(out, "created successfully") || !strings.Contains(out, "http://localhost:11434/v1") || !strings.Contains(out, "qwen2.5-coder:32b") {
		t.Errorf("agent create with model flags failed: %s", out)
	}

	// List
	code, out, _ = runSndbx([]string{"agent", "list"})
	if code != 0 || !strings.Contains(out, "coder-1") || !strings.Contains(out, "coder-2") {
		t.Errorf("agent list failed: %s", out)
	}

	// List JSON
	code, out, _ = runSndbx([]string{"agent", "list", "--json"})
	if code != 0 || !strings.Contains(out, "[") || !strings.Contains(out, "coder-1") {
		t.Errorf("agent list json failed: %s", out)
	}

	// Stop missing args
	code, _, errOut = runSndbx([]string{"agent", "stop"})
	if code != 1 || !strings.Contains(errOut, "Usage: sndbx agent stop") {
		t.Errorf("agent stop empty failed")
	}

	// Stop nonexistent
	code, _, errOut = runSndbx([]string{"agent", "stop", "nonexistent"})
	if code != 1 {
		t.Errorf("expected error stopping nonexistent agent")
	}

	// Clean nonexistent
	code, _, errOut = runSndbx([]string{"agent", "clean", "nonexistent"})
	if code != 1 {
		t.Errorf("expected error cleaning nonexistent agent")
	}

	// Retire
	code, out, _ = runSndbx([]string{"agent", "retire", "coder-1", "--force"})
	if code != 0 || !strings.Contains(out, "retired and deprovisioned successfully") {
		t.Errorf("agent retire failed: %s", out)
	}

	// Missing commands
	code, _, _ = runSndbx([]string{"agent", "start"})
	if code != 1 {
		t.Errorf("agent start missing args should fail")
	}
	code, _, _ = runSndbx([]string{"agent", "connect"})
	if code != 1 {
		t.Errorf("agent connect missing args should fail")
	}
	code, _, _ = runSndbx([]string{"agent", "ssh"})
	if code != 1 {
		t.Errorf("agent ssh missing args should fail")
	}
}

func TestSndbxWithLiveValkey(t *testing.T) {
	valkey := harness.StartValkeyHarness(t)
	defer valkey.Teardown()

	tmpDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", tmpDir)
	os.Setenv("BP_HOST", "127.0.0.1")
	os.Setenv("BP_PORT", fmt.Sprintf("%d", valkey.Port))
	os.Setenv("ADMIN_BACKPLANE_PASSWORD", valkey.AdminPass)

	// Create agent with live Valkey connected
	code, out, _ := runSndbx([]string{"agent", "create", "valkey-bot", "as", "base", "--role", "worker"})
	if code != 0 || !strings.Contains(out, "created successfully") {
		t.Errorf("agent create with live valkey failed: %s", out)
	}

	// Verify agent ACL was registered by connecting as valkey-bot
	loaded, err := config.LoadAgentConfig("valkey-bot", config.GetPaths())
	if err != nil {
		t.Fatalf("failed loading agent config: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	agentClient, err := libbp.Dial(ctx, libbp.ClientConfig{
		Host:     "127.0.0.1",
		Port:     valkey.Port,
		Username: "valkey-bot",
		Password: loaded.Password,
		AgentID:  "valkey-bot",
	})
	if err != nil {
		t.Fatalf("failed dialing valkey as registered agent: %v", err)
	}
	defer agentClient.Close()

	// Stop all agents flag test
	code, out, _ = runSndbx([]string{"agent", "stop", "--all"})
	if code != 0 || !strings.Contains(out, "Stopped valkey-bot") {
		t.Errorf("agent stop --all failed: %s", out)
	}

	// Infra subcommands
	code, _, _ = runSndbx([]string{"infra"})
	if code != 1 {
		t.Errorf("infra empty should fail")
	}

	for _, h := range []string{"help", "-h", "--help"} {
		code, out, _ := runSndbx([]string{"infra", h})
		if code != 0 || !strings.Contains(out, "Usage: sndbx infra") {
			t.Errorf("infra %s failed", h)
		}
	}

	code, _, errOut := runSndbx([]string{"infra", "unknown"})
	if code != 1 || !strings.Contains(errOut, "unknown command") {
		t.Errorf("infra unknown failed")
	}

	// Infra up / list / down execution test
	code, out, _ = runSndbx([]string{"infra", "list"})
	if code != 0 || !strings.Contains(out, "SERVICE") {
		t.Errorf("infra list failed: %s", out)
	}

	code, out, _ = runSndbx([]string{"infra", "down"})
	if code != 0 || !strings.Contains(out, "infrastructure stopped") {
		t.Errorf("infra down failed: %s", out)
	}
}

func TestSndbxUpdateAndGUIDomains(t *testing.T) {
	// Update help
	for _, h := range []string{"help", "-h", "--help"} {
		code, out, _ := runSndbx([]string{"update", h})
		if code != 0 || !strings.Contains(out, "Usage: sndbx update") {
			t.Errorf("update %s failed", h)
		}
	}

	// Update command execution with invalid git/repo or dry behavior
	tempRepo := t.TempDir()
	_ = os.MkdirAll(filepath.Join(tempRepo, ".git"), 0755)
	t.Setenv("AGENT_SANDBOX_REPO", tempRepo)
	code, _, errOut := runSndbx([]string{"update"})
	// TempDir has no go.mod or Makefile, but handleUpdate will attempt git pull, compile, and image build
	// We expect either error or execution failure
	if code == 0 {
		t.Logf("update in temp dir succeeded: %v", code)
	} else {
		if !strings.Contains(errOut, "failed") && !strings.Contains(errOut, "warning") {
			t.Errorf("unexpected error output for update: %s", errOut)
		}
	}

	// Unknown domain: repo should now fail as unknown command
	code, _, errOut = runSndbx([]string{"repo"})
	if code != 1 || !strings.Contains(errOut, "unknown command \"repo\"") {
		t.Errorf("repo should be unknown command: %s", errOut)
	}

	// GUI
	code, out, _ := runSndbx([]string{"gui"})
	if code != 0 || !strings.Contains(out, "Launching Backplane GUI") {
		t.Errorf("gui command failed: %s", out)
	}
}

func TestMainFunction(t *testing.T) {
	if os.Getenv("TEST_RUN_MAIN") == "1" {
		os.Args = []string{"sndbx", "help"}
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestMainFunction")
	cmd.Env = append(os.Environ(), "TEST_RUN_MAIN=1")
	_ = cmd.Run()
}
