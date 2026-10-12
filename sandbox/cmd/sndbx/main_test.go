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

	"github.com/boggycreek/sandbox/backplane/pkg/libbp"
	"github.com/boggycreek/sandbox/backplane/test/harness"
	"github.com/boggycreek/sandbox/pkg/config"
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
	for _, unknown := range []string{"unknown", "repo"} {
		code, _, errOut := runSndbx([]string{unknown})
		if code != 1 || !strings.Contains(errOut, "unknown command") {
			t.Errorf("expected unknown command error for %q, got: %s", unknown, errOut)
		}
	}
}

func TestSndbxAgentDomain(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("BP_PORT", "65530")
	t.Setenv("GITEA_URL", "http://127.0.0.1:65531")
	t.Setenv("SONAR_HOST_URL", "http://127.0.0.1:65532")

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

	// Create with 'as pig'
	code, out, _ = runSndbx([]string{"agent", "create", "coder-pig", "as", "pig", "--role", "fuzzer"})
	if code != 0 || !strings.Contains(out, "created successfully") || !strings.Contains(out, "agent-sandbox-pig:latest") {
		t.Errorf("agent create as pig failed: %s", out)
	}

	// Create with 'as native'
	code, out, _ = runSndbx([]string{"agent", "create", "coder-native", "as", "native", "--role", "engine"})
	if code != 0 || !strings.Contains(out, "created successfully") || !strings.Contains(out, "agent-sandbox-native:latest") {
		t.Errorf("agent create as native failed: %s", out)
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
	t.Setenv("HOME", tmpDir)
	t.Setenv("XDG_DATA_HOME", tmpDir)
	t.Setenv("BP_HOST", "127.0.0.1")
	t.Setenv("BP_PORT", fmt.Sprintf("%d", valkey.Port))
	t.Setenv("ADMIN_BACKPLANE_PASSWORD", valkey.AdminPass)

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

func TestSndbxUpdateDomain(t *testing.T) {
	// 1. Update help flags
	for _, h := range []string{"help", "-h", "--help"} {
		code, out, _ := runSndbx([]string{"update", h})
		if code != 0 || !strings.Contains(out, "Usage: sndbx update") {
			t.Errorf("update %s failed", h)
		}
	}

	// 2. Update execution with mocked commands in an isolated repository
	origExec := execCommandContext
	defer func() { execCommandContext = origExec }()
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	tempRepo := t.TempDir()
	_ = os.WriteFile(filepath.Join(tempRepo, "Makefile"), []byte("all:\n"), 0644)
	_ = os.MkdirAll(filepath.Join(tempRepo, ".git"), 0755)
	t.Setenv("AGENT_SANDBOX_REPO", tempRepo)
	t.Setenv("SNDBX_UPDATE_TIMEOUT", "15m")

	code, out, errOut := runSndbx([]string{"update"})
	if code != 0 {
		t.Errorf("expected update to succeed with mocked commands, got code %d, err: %s", code, errOut)
	}
	if !strings.Contains(out, "Update complete") {
		t.Errorf("expected update complete output, got: %s", out)
	}
}

func TestSndbxGUIDomain(t *testing.T) {
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
	if os.Getenv("TEST_RUN_MAIN_SB") == "1" {
		os.Args = []string{"sb", "help"}
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestMainFunction")
	cmd.Env = append(os.Environ(), "TEST_RUN_MAIN=1")
	_ = cmd.Run()

	cmdSb := exec.Command(os.Args[0], "-test.run=TestMainFunction")
	cmdSb.Env = append(os.Environ(), "TEST_RUN_MAIN_SB=1")
	_ = cmdSb.Run()
}

func TestBinaryNameResolution(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	tests := []struct {
		args     []string
		expected string
	}{
		{args: []string{}, expected: "sndbx"},
		{args: []string{"sndbx"}, expected: "sndbx"},
		{args: []string{"/usr/local/bin/sndbx"}, expected: "sndbx"},
		{args: []string{"sb"}, expected: "sb"},
		{args: []string{"/usr/local/bin/sb"}, expected: "sb"},
		{args: []string{"./bin/sb"}, expected: "sb"},
		{args: []string{"other"}, expected: "sndbx"},
	}

	for _, tc := range tests {
		os.Args = tc.args
		got := binaryName()
		if got != tc.expected {
			t.Errorf("binaryName() for os.Args=%v: expected %q, got %q", tc.args, tc.expected, got)
		}
	}
}

func TestUsageOutputAdaptsWhenInvokedAsSb(t *testing.T) {
	origArgs := os.Args
	defer func() { os.Args = origArgs }()
	os.Args = []string{"sb"}

	// 1. Empty args
	code, out, _ := runSndbx([]string{})
	if code != 1 {
		t.Errorf("expected exit code 1 on empty args, got %d", code)
	}
	if !strings.Contains(out, "Agent Sandbox CLI (sb)") {
		t.Errorf("expected usage title 'Agent Sandbox CLI (sb)', got: %s", out)
	}
	if strings.Contains(out, "Agent Sandbox CLI (sndbx)") {
		t.Errorf("did not expect 'sndbx' in header when invoked as 'sb'")
	}
	if !strings.Contains(out, "  sb <domain|command> [args...]") {
		t.Errorf("expected 'sb <domain|command>' usage line, got: %s", out)
	}
	if !strings.Contains(out, "  sb agent create") || !strings.Contains(out, "  sb infra up") || !strings.Contains(out, "  sb plugin add") || !strings.Contains(out, "  sb update") {
		t.Errorf("expected command examples to use 'sb', got: %s", out)
	}
	if !strings.Contains(out, "Run 'sb <domain> help'") {
		t.Errorf("expected hint to use 'sb', got: %s", out)
	}

	// 2. Help flags
	for _, h := range []string{"help", "-h", "--help"} {
		code, out, _ = runSndbx([]string{h})
		if code != 0 || !strings.Contains(out, "Agent Sandbox CLI (sb)") {
			t.Errorf("expected help output with 'sb' for %s", h)
		}
	}

	// 3. Agent domain help
	code, _, errOut := runSndbx([]string{"agent"})
	if code != 1 || !strings.Contains(errOut, "Usage: sb agent") {
		t.Errorf("expected 'Usage: sb agent' on empty agent args, got: %s", errOut)
	}

	code, out, _ = runSndbx([]string{"agent", "help"})
	if code != 0 || !strings.Contains(out, "Usage: sb agent <command>") {
		t.Errorf("expected 'Usage: sb agent <command>' on agent help, got: %s", out)
	}

	// 4. Agent subcommands missing required args
	agentSubTests := []struct {
		args     []string
		expected string
	}{
		{[]string{"agent", "create"}, "Usage: sb agent create"},
		{[]string{"agent", "start"}, "Usage: sb agent start"},
		{[]string{"agent", "tmux"}, "Usage: sb agent tmux"},
		{[]string{"agent", "connect"}, "Usage: sb agent connect"},
		{[]string{"agent", "open"}, "Usage: sb agent open"},
		{[]string{"agent", "ssh"}, "Usage: sb agent ssh"},
		{[]string{"agent", "doctor"}, "Usage: sb agent doctor"},
		{[]string{"agent", "stop"}, "Usage: sb agent stop"},
		{[]string{"agent", "clean"}, "Usage: sb agent clean"},
		{[]string{"agent", "retire"}, "Usage: sb agent retire"},
		{[]string{"agent", "provision"}, "Usage: sb agent provision"},
		{[]string{"agent", "clone"}, "Usage: sb agent clone"},
		{[]string{"agent", "remote"}, "Usage: sb agent remote"},
	}
	for _, tc := range agentSubTests {
		code, _, errOut = runSndbx(tc.args)
		if code != 1 || !strings.Contains(errOut, tc.expected) {
			t.Errorf("args %v: expected error to contain %q, got code %d, err: %s", tc.args, tc.expected, code, errOut)
		}
	}

	// 5. Infra domain help
	code, _, errOut = runSndbx([]string{"infra"})
	if code != 1 || !strings.Contains(errOut, "Usage: sb infra") {
		t.Errorf("expected 'Usage: sb infra' on empty infra args, got: %s", errOut)
	}

	code, out, _ = runSndbx([]string{"infra", "help"})
	if code != 0 || !strings.Contains(out, "Usage: sb infra <command>") {
		t.Errorf("expected 'Usage: sb infra <command>' on infra help, got: %s", out)
	}

	// 6. Plugin domain help
	code, out, _ = runSndbx([]string{"plugin", "help"})
	if code != 0 || !strings.Contains(out, "sb plugin <command>") {
		t.Errorf("expected 'sb plugin <command>' on plugin help, got: %s", out)
	}

	code, _, errOut = runSndbx([]string{"plugin", "add"})
	if code != 1 || !strings.Contains(errOut, "Usage: sb plugin add") {
		t.Errorf("expected 'Usage: sb plugin add' on plugin add empty, got: %s", errOut)
	}

	code, _, errOut = runSndbx([]string{"plugin", "remove"})
	if code != 1 || !strings.Contains(errOut, "Usage: sb plugin remove") {
		t.Errorf("expected 'Usage: sb plugin remove' on plugin remove empty, got: %s", errOut)
	}

	// 7. Update domain help
	code, out, _ = runSndbx([]string{"update", "help"})
	if code != 0 || !strings.Contains(out, "Usage: sb update") {
		t.Errorf("expected 'Usage: sb update' on update help, got: %s", out)
	}

	// 8. Unknown command error
	code, _, errOut = runSndbx([]string{"unknowncmd"})
	if code != 1 || !strings.Contains(errOut, "sb: unknown command \"unknowncmd\" (see 'sb help')") {
		t.Errorf("expected 'sb: unknown command' error, got: %s", errOut)
	}
}

func TestSymlinkCreationLogic(t *testing.T) {
	tmpDir := t.TempDir()
	t.Cleanup(func() {
		_ = os.RemoveAll(tmpDir)
	})

	sndbxBin := filepath.Join(tmpDir, "sndbx")
	sbBin := filepath.Join(tmpDir, "sb")

	// 1. Create a dummy executable sndbx
	if err := os.WriteFile(sndbxBin, []byte("#!/bin/sh\necho sndbx\n"), 0755); err != nil {
		t.Fatalf("failed to create dummy sndbx binary: %v", err)
	}

	// 2. Create symlink sb -> sndbx
	if err := os.Symlink(sndbxBin, sbBin); err != nil {
		t.Fatalf("failed to create symlink: %v", err)
	}

	// Verify symlink is valid
	fi, err := os.Lstat(sbBin)
	if err != nil {
		t.Fatalf("failed to lstat symlink: %v", err)
	}
	if fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected sb to be a symlink")
	}

	target, err := os.Readlink(sbBin)
	if err != nil {
		t.Fatalf("failed to readlink: %v", err)
	}
	if target != sndbxBin && filepath.Base(target) != "sndbx" {
		t.Errorf("expected symlink target %q, got %q", sndbxBin, target)
	}

	// 3. Test overwrite logic (idempotency)
	_ = os.Remove(sbBin)
	if err := os.Symlink("sndbx", sbBin); err != nil {
		t.Fatalf("failed to recreate relative symlink: %v", err)
	}
	relTarget, err := os.Readlink(sbBin)
	if err != nil {
		t.Fatalf("failed to readlink relative symlink: %v", err)
	}
	if relTarget != "sndbx" {
		t.Errorf("expected relative symlink target 'sndbx', got %q", relTarget)
	}

	// 4. Test ensureSbSymlink function directly when no external sb exists
	binDir := t.TempDir()
	t.Setenv("PATH", binDir)
	sndbxTarget := filepath.Join(binDir, "sndbx")
	_ = os.WriteFile(sndbxTarget, []byte("#!/bin/sh\n"), 0755)

	ensureSbSymlink(binDir)
	sbPath := filepath.Join(binDir, "sb")
	fi, err = os.Lstat(sbPath)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("expected ensureSbSymlink to create symlink: %v", err)
	}

	// 5. If another sb exists on PATH, ensureSbSymlink skips
	otherBinDir := t.TempDir()
	otherSb := filepath.Join(otherBinDir, "sb")
	_ = os.WriteFile(otherSb, []byte("#!/bin/sh\n"), 0755)
	t.Setenv("PATH", otherBinDir+":"+os.Getenv("PATH"))

	skipBinDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(skipBinDir, "sndbx"), []byte("#!/bin/sh\n"), 0755)
	ensureSbSymlink(skipBinDir)
	if _, err := os.Lstat(filepath.Join(skipBinDir, "sb")); !os.IsNotExist(err) {
		t.Errorf("expected sb symlink to be skipped when another sb exists on PATH")
	}
}
