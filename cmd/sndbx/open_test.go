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

	"github.com/boggycreek/sandbox/pkg/config"
	"github.com/boggycreek/sandbox/pkg/ide"
	"github.com/boggycreek/sandbox/pkg/runtime"
)

func TestAgentOpenValidationAndErrors(t *testing.T) {
	tmpDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", tmpDir)
	paths := config.GetPaths()
	_ = paths.EnsureDirectories()

	// Inject a fake lookPath: only "code" is installed.
	restoreLookPath := ide.SetLookPathForTesting(func(name string) (string, error) {
		if name == "code" {
			return "/usr/bin/code", nil
		}
		return "", fmt.Errorf("not found: %s", name)
	})
	defer restoreLookPath()

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

	// 4. Completely unknown IDE (not in catalogue)
	stderr.Reset()
	code = Run([]string{"agent", "open", "myagent", "--ide", "emacs"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected code 1 for unknown ide emacs, got %d", code)
	}
	if !strings.Contains(stderr.String(), `unknown IDE "emacs"`) {
		t.Errorf("expected unknown IDE error, got: %s", stderr.String())
	}

	// 5. Known IDE (cursor) that is not installed on this fake host
	stderr.Reset()
	code = Run([]string{"agent", "open", "myagent", "in", "cursor"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected code 1 for uninstalled ide cursor, got %d", code)
	}
	if !strings.Contains(stderr.String(), "is not installed on this host") {
		t.Errorf("expected not-installed error, got: %s", stderr.String())
	}

	// 6. Nonexistent agent (IDE resolves OK, agent config missing)
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
		t.Fatalf("failed saving agent config: %v\n", err)
	}

	// Inject a fake lookPath: code, webstorm, and goland are "installed".
	restoreLookPath := ide.SetLookPathForTesting(func(name string) (string, error) {
		switch name {
		case "code":
			return "/usr/bin/code", nil
		case "webstorm":
			return "/home/user/.local/share/JetBrains/Toolbox/scripts/webstorm", nil
		case "goland":
			return "/home/user/.local/share/JetBrains/Toolbox/scripts/goland", nil
		}
		return "", fmt.Errorf("not found: %s", name)
	})
	defer restoreLookPath()

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
		// Mock VS Code server presence check
		if name == "podman" && len(args) > 3 && strings.Contains(args[3], "test -d /home/agent/.vscode-server") {
			return exec.Command("true")
		}
		return exec.Command("true")
	}

	execCommandContext = mockFn
	restoreRuntime := runtime.SetExecCommandContextForTesting(mockFn)
	defer restoreRuntime()

	var stdout, stderr bytes.Buffer

	// Test 1: --no-launch with code (default, VS Code family)
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

	// Test 2: --no-launch with webstorm (JetBrains family)
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"agent", "open", "ide-agent", "in", "webstorm", "--no-launch"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("agent open in webstorm --no-launch failed: %d, stderr: %s", code, stderr.String())
	}
	outStr = stdout.String()
	if !strings.Contains(outStr, "Connect via WebStorm") {
		t.Errorf("expected WebStorm connection info, got: %s", outStr)
	}
	if !strings.Contains(outStr, "ssh://agent@localhost:32777/home/agent/workspace") {
		t.Errorf("expected JetBrains SSH URI in no-launch output, got: %s", outStr)
	}

	// Test 3: goland --no-launch (JetBrains, different binary)
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"agent", "open", "ide-agent", "in", "goland", "--no-launch"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("agent open in goland --no-launch failed: %d, stderr: %s", code, stderr.String())
	}
	outStr = stdout.String()
	if !strings.Contains(outStr, "GoLand") {
		t.Errorf("expected GoLand in output, got: %s", outStr)
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
	if !strings.Contains(outStr, "Opening ide-agent in VS Code") {
		t.Errorf("expected opening message for VS Code, got: %s", outStr)
	}
	// Verify code binary was called with --file-uri
	foundCodeCmd := false
	foundExtensionInstall := false
	for _, cmd := range executedCmds {
		if strings.Contains(cmd, "--file-uri") && strings.Contains(cmd, "vscode-remote://ssh-remote+sndbx-ide-agent") {
			foundCodeCmd = true
		}
		if strings.Contains(cmd, "anthropic.claude-code") {
			foundExtensionInstall = true
		}
	}
	if !foundCodeCmd {
		t.Errorf("expected --file-uri vscode-remote:// command, got: %v", executedCmds)
	}
	if !foundExtensionInstall {
		t.Errorf("expected anthropic.claude-code installation command, got: %v", executedCmds)
	}

	// Test 5: webstorm launch (JetBrains family via --remote-dev)
	executedCmds = nil
	stdout.Reset()
	stderr.Reset()
	code = Run([]string{"agent", "open", "ide-agent", "in", "webstorm"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("agent open in webstorm failed: %d, stderr: %s", code, stderr.String())
	}
	outStr = stdout.String()
	if !strings.Contains(outStr, "Opening ide-agent in WebStorm via Remote Development") {
		t.Errorf("expected WebStorm opening message, got: %s", outStr)
	}
	foundRemoteDev := false
	for _, cmd := range executedCmds {
		if strings.Contains(cmd, "--remote-dev") && strings.Contains(cmd, "ssh://agent@localhost") {
			foundRemoteDev = true
		}
	}
	if !foundRemoteDev {
		t.Errorf("expected --remote-dev ssh:// command for webstorm, got: %v", executedCmds)
	}

	// Test 6: VS Code launch failure
	failFn := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "port" {
			return exec.Command("echo", "127.0.0.1:32777")
		}
		if name == "/usr/bin/code" {
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
	if !strings.Contains(stderr.String(), "sndbx error launching") {
		t.Errorf("expected launch error message in stderr, got: %s", stderr.String())
	}

	// Test 7: JetBrains launch failure
	execCommandContext = mockFn
	restoreRuntime()
	jbFailFn := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "port" {
			return exec.Command("echo", "127.0.0.1:32777")
		}
		if strings.Contains(name, "webstorm") {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	execCommandContext = jbFailFn
	restoreRuntime3 := runtime.SetExecCommandContextForTesting(jbFailFn)
	defer restoreRuntime3()

	stderr.Reset()
	stdout.Reset()
	code = Run([]string{"agent", "open", "ide-agent", "in", "webstorm"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected code 1 when launching WebStorm fails, got %d", code)
	}
	if !strings.Contains(stderr.String(), "sndbx error launching") {
		t.Errorf("expected launch error for webstorm, got: %s", stderr.String())
	}

	// Test 8: Container start failure when stopped
	execCommandContext = mockFn
	restoreRuntime3()
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
	restoreRuntime4 := runtime.SetExecCommandContextForTesting(startFailFn)
	defer restoreRuntime4()

	stderr.Reset()
	code = Run([]string{"agent", "open", "ide-agent"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected code 1 when start container fails, got %d", code)
	}

	// Test 9: Container starts, but port remains unavailable
	portFailFn := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "port" {
			return exec.Command("false") // Port always fails
		}
		return exec.Command("true") // Container start succeeds
	}
	execCommandContext = portFailFn
	restoreRuntime5 := runtime.SetExecCommandContextForTesting(portFailFn)
	defer restoreRuntime5()

	stderr.Reset()
	code = Run([]string{"agent", "open", "ide-agent"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("expected code 1 when port remains unavailable, got %d", code)
	}
	if !strings.Contains(stderr.String(), "discovering SSH port") {
		t.Errorf("expected discovering SSH port error, got: %s", stderr.String())
	}

	// Test 10: SyncSSHConfigFile error path
	execCommandContext = mockFn
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
