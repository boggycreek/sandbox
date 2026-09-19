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
	"strings"
	"testing"

	"github.com/boggycreek/agent-sandbox/pkg/config"
	"github.com/boggycreek/agent-sandbox/pkg/runtime"
)

func TestAgentTmuxAndConnect(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", tmpDir)
	paths := config.GetPaths()
	_ = paths.EnsureDirectories()

	// 1. Missing args
	var stdout, stderr bytes.Buffer
	code := Run([]string{"agent", "tmux"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected code 1 for agent tmux without args, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Usage: sndbx agent tmux <name>") {
		t.Errorf("expected usage output in stderr, got: %s", stderr.String())
	}

	stderr.Reset()
	code = Run([]string{"agent", "connect"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected code 1 for agent connect without args, got %d", code)
	}
	if !strings.Contains(stderr.String(), "deprecated") {
		t.Errorf("expected deprecation notice in stderr, got: %s", stderr.String())
	}

	// 2. Nonexistent agent
	stderr.Reset()
	code = Run([]string{"agent", "tmux", "nonexistent-agent"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected code 1 for nonexistent agent, got %d", code)
	}

	stderr.Reset()
	code = Run([]string{"agent", "connect", "nonexistent-agent"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected code 1 for nonexistent agent via connect, got %d", code)
	}
	if !strings.Contains(stderr.String(), "deprecated") {
		t.Errorf("expected deprecation notice, got: %s", stderr.String())
	}

	// 3. Valid agent with mock exec
	cfg, err := config.NewAgentConfig("tmux-test-agent", "base", "developer")
	if err != nil {
		t.Fatalf("failed creating agent config: %v", err)
	}
	if err := config.SaveAgentConfig(cfg, paths); err != nil {
		t.Fatalf("failed saving agent config: %v", err)
	}

	origExec := execCommandContext
	defer func() { execCommandContext = origExec }()

	var executedCmds []string
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		executedCmds = append(executedCmds, fmt.Sprintf("%s %s", name, strings.Join(args, " ")))
		return exec.Command("true")
	}

	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"agent", "tmux", "tmux-test-agent"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("agent tmux failed: %d, stderr: %s", code, stderr.String())
	}
	expectedAttach := fmt.Sprintf("podman exec -it %s tmux attach", cfg.ContainerName)
	if len(executedCmds) == 0 || !strings.Contains(executedCmds[len(executedCmds)-1], expectedAttach) {
		t.Errorf("expected %s command, got: %v", expectedAttach, executedCmds)
	}

	// Connect alias
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"agent", "connect", "tmux-test-agent"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("agent connect failed: %d, stderr: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "Notice: 'sndbx agent connect' is deprecated. Please use 'sndbx agent tmux tmux-test-agent' instead.") {
		t.Errorf("expected deprecation notice in stderr, got: %s", stderr.String())
	}

	// Error when podman exec fails
	execCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}
	stderr.Reset()
	code = Run([]string{"agent", "tmux", "tmux-test-agent"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected error when exec fails, got %d", code)
	}
}

func TestAgentOpenValidationAndErrors(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", tmpDir)
	paths := config.GetPaths()
	_ = paths.EnsureDirectories()

	var stdout, stderr bytes.Buffer

	// 1. Missing args
	code := Run([]string{"agent", "open"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected code 1 for open without args, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Usage: sndbx agent open") {
		t.Errorf("expected usage message, got: %s", stderr.String())
	}

	// 2. Dangling 'in'
	stderr.Reset()
	code = Run([]string{"agent", "open", "myagent", "in"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected code 1 for dangling in, got %d", code)
	}

	// 3. Invalid flag
	stderr.Reset()
	code = Run([]string{"agent", "open", "myagent", "--unrecognized-flag"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected code 1 for invalid flag, got %d", code)
	}

	// 4. Unsupported IDE via flag
	stderr.Reset()
	code = Run([]string{"agent", "open", "myagent", "--ide", "emacs"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected code 1 for unsupported ide emacs, got %d", code)
	}
	if !strings.Contains(stderr.String(), `unsupported IDE "emacs"`) {
		t.Errorf("expected unsupported IDE error, got: %s", stderr.String())
	}

	// 5. Unsupported IDE via natural language 'in'
	stderr.Reset()
	code = Run([]string{"agent", "open", "myagent", "in", "cursor"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected code 1 for unsupported ide cursor, got %d", code)
	}
	if !strings.Contains(stderr.String(), `unsupported IDE "cursor"`) {
		t.Errorf("expected unsupported IDE error, got: %s", stderr.String())
	}

	// 6. Nonexistent agent
	stderr.Reset()
	code = Run([]string{"agent", "open", "nonexistent-agent"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected code 1 for nonexistent agent, got %d", code)
	}
	if !strings.Contains(stderr.String(), "sndbx error:") {
		t.Errorf("expected sndbx error, got: %s", stderr.String())
	}
}

func TestAgentOpenExecution(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", tmpDir)
	paths := config.GetPaths()
	_ = paths.EnsureDirectories()

	cfg, err := config.NewAgentConfig("ide-agent", "base", "developer")
	if err != nil {
		t.Fatalf("failed creating agent config: %v", err)
	}
	if err := config.SaveAgentConfig(cfg, paths); err != nil {
		t.Fatalf("failed saving agent config: %v", err)
	}

	origExec := execCommandContext
	defer func() { execCommandContext = origExec }()

	var executedCmds []string
	mockFn := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		cmdStr := fmt.Sprintf("%s %s", name, strings.Join(args, " "))
		executedCmds = append(executedCmds, cmdStr)

		// Mock podman port to return host port 32777
		if name == "podman" && len(args) > 1 && args[0] == "port" {
			return exec.Command("echo", "127.0.0.1:32777")
		}
		// Mock VS code server presence check
		if name == "podman" && len(args) > 3 && strings.Contains(args[3], "test -d /home/agent/.vscode-server") {
			return exec.Command("true")
		}
		return exec.Command("true")
	}

	execCommandContext = mockFn
	restoreRuntime := runtime.SetExecCommandContextForTesting(mockFn)
	defer restoreRuntime()

	var stdout, stderr bytes.Buffer

	// Test 1: --no-launch with code (default)
	stdout.Reset()
	stderr.Reset()
	code := Run([]string{"agent", "open", "ide-agent", "--no-launch"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("agent open --no-launch failed: %d, stderr: %s", code, stderr.String())
	}
	outStr := stdout.String()
	if !strings.Contains(outStr, "SSH host alias prepared: sndbx-ide-agent (Port: 32777)") {
		t.Errorf("expected host alias prepared output, got: %s", outStr)
	}
	if !strings.Contains(outStr, "vscode-remote://ssh-remote+sndbx-ide-agent/home/agent/workspace") {
		t.Errorf("expected VS Code Remote URI, got: %s", outStr)
	}

	// Test 2: --no-launch with webstorm
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"agent", "open", "ide-agent", "in", "webstorm", "--no-launch"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("agent open in webstorm --no-launch failed: %d, stderr: %s", code, stderr.String())
	}
	outStr = stdout.String()
	if !strings.Contains(outStr, `Connect via JetBrains Gateway with Host "sndbx-ide-agent"`) {
		t.Errorf("expected JetBrains Gateway instructions, got: %s", outStr)
	}
	if !strings.Contains(outStr, "Full JetBrains Dev Containers is not used due to container lifecycle management.") {
		t.Errorf("expected dev containers disclaimer, got: %s", outStr)
	}

	// Test 3: webstorm without --no-launch
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"agent", "open", "ide-agent", "--ide", "webstorm"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("agent open --ide webstorm failed: %d, stderr: %s", code, stderr.String())
	}
	outStr = stdout.String()
	if !strings.Contains(outStr, `Connect to ide-agent via JetBrains Gateway with Host "sndbx-ide-agent"`) {
		t.Errorf("expected webstorm connection prompt, got: %s", outStr)
	}

	// Test 4: code launch (VS Code) with extension installation
	executedCmds = nil
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"agent", "open", "ide-agent", "in", "code"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("agent open in code failed: %d, stderr: %s", code, stderr.String())
	}
	outStr = stdout.String()
	if !strings.Contains(outStr, "Opening ide-agent in VS Code (Host: sndbx-ide-agent)...") {
		t.Errorf("expected opening message, got: %s", outStr)
	}

	// Verify code command was called with --file-uri
	foundCodeCmd := false
	foundExtensionInstall := false
	for _, cmd := range executedCmds {
		if strings.HasPrefix(cmd, "code --file-uri vscode-remote://ssh-remote+sndbx-ide-agent/home/agent/workspace") {
			foundCodeCmd = true
		}
		if strings.Contains(cmd, "anthropic.claude-code") {
			foundExtensionInstall = true
		}
	}
	if !foundCodeCmd {
		t.Errorf("expected code --file-uri command to be executed, got: %v", executedCmds)
	}
	if !foundExtensionInstall {
		t.Errorf("expected anthropic.claude-code installation command, got: %v", executedCmds)
	}

	// Test 5: code launch failure
	failFn := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "port" {
			return exec.Command("echo", "127.0.0.1:32777")
		}
		if name == "code" {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	execCommandContext = failFn
	restoreRuntime2 := runtime.SetExecCommandContextForTesting(failFn)
	defer restoreRuntime2()

	stderr.Reset()
	code = Run([]string{"agent", "open", "ide-agent"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected code 1 when launching VS Code fails, got %d", code)
	}
	if !strings.Contains(stderr.String(), "sndbx error launching VS Code:") {
		t.Errorf("expected launch error message in stderr, got: %s", stderr.String())
	}

	// Test 6: Container start failure when stopped
	startFailFn := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "port" {
			return exec.Command("false") // Port fails
		}
		if name == "podman" && len(args) > 0 && args[0] == "run" {
			return exec.Command("false") // Start fails
		}
		return exec.Command("true")
	}
	execCommandContext = startFailFn
	restoreRuntime3 := runtime.SetExecCommandContextForTesting(startFailFn)
	defer restoreRuntime3()

	stderr.Reset()
	code = Run([]string{"agent", "open", "ide-agent"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected code 1 when start container fails, got %d", code)
	}

	// Test 7: Container starts, but port remains unavailable
	portFailFn := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "port" {
			return exec.Command("false") // Port always fails
		}
		return exec.Command("true") // Container start succeeds
	}
	execCommandContext = portFailFn
	restoreRuntime4 := runtime.SetExecCommandContextForTesting(portFailFn)
	defer restoreRuntime4()

	stderr.Reset()
	code = Run([]string{"agent", "open", "ide-agent"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected code 1 when port remains unavailable, got %d", code)
	}
	if !strings.Contains(stderr.String(), "discovering SSH port") {
		t.Errorf("expected discovering SSH port error, got: %s", stderr.String())
	}

	// Test 8: SyncSSHConfigFile error path
	badPaths := config.Paths{
		DataHome:      "/dev/null/forbidden",
		SSHConfigFile: "/dev/null/forbidden/ssh_config",
		AgentsDir:     paths.AgentsDir,
	}
	stderr.Reset()
	c := handleAgentOpen(context.Background(), badPaths, []string{"ide-agent"}, &stdout, &stderr)
	if c != 1 {
		t.Errorf("expected code 1 on sync ssh config failure, got %d", c)
	}

	// Direct tests for runtime.ClearAgentKnownHosts
	runtime.ClearAgentKnownHosts("ide-agent", paths)
}

func TestAgentSSHDirect(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", tmpDir)
	paths := config.GetPaths()
	_ = paths.EnsureDirectories()

	cfg, _ := config.NewAgentConfig("ssh-agent", "base", "developer")
	_ = config.SaveAgentConfig(cfg, paths)

	mockSSH := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "port" {
			return exec.Command("echo", "127.0.0.1:44444")
		}
		if name == "ssh" {
			return exec.Command("true")
		}
		return exec.Command("true")
	}
	origExec := execCommandContext
	defer func() { execCommandContext = origExec }()
	execCommandContext = mockSSH
	restore := runtime.SetExecCommandContextForTesting(mockSSH)
	defer restore()

	var stdout, stderr bytes.Buffer
	code := Run([]string{"agent", "ssh", "ssh-agent"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("agent ssh should succeed with mock port and mock ssh, got %d", code)
	}

	// Failure branch
	failSSH := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "port" {
			return exec.Command("echo", "127.0.0.1:44444")
		}
		if name == "ssh" {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	execCommandContext = failSSH
	restoreFail := runtime.SetExecCommandContextForTesting(failSSH)
	defer restoreFail()

	code = Run([]string{"agent", "ssh", "ssh-agent"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("agent ssh should return 1 when ssh command fails, got %d", code)
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
