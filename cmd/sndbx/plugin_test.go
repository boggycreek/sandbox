// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boggycreek/agent-sandbox/pkg/config"
	"github.com/boggycreek/agent-sandbox/pkg/plugin"
)

func TestHandlePluginCommands(t *testing.T) {
	tmpDir := t.TempDir()
	paths := config.Paths{
		DataHome:      filepath.Join(tmpDir, "data"),
		AgentsDir:     filepath.Join(tmpDir, "data", "agents"),
		SSHConfigFile: filepath.Join(tmpDir, "data", "ssh_config"),
	}
	_ = paths.EnsureDirectories()

	ctx := context.Background()

	// 1. No arguments -> error + usage
	{
		var stdout, stderr bytes.Buffer
		code := handlePlugin(ctx, paths, []string{}, &stdout, &stderr)
		if code != 1 {
			t.Errorf("expected exit code 1 for no args, got %d", code)
		}
		if !strings.Contains(stderr.String(), "Usage:") {
			t.Errorf("expected usage in stderr, got: %s", stderr.String())
		}
	}

	// 2. Help
	for _, h := range []string{"help", "-h", "--help"} {
		var stdout, stderr bytes.Buffer
		code := handlePlugin(ctx, paths, []string{h}, &stdout, &stderr)
		if code != 0 {
			t.Errorf("expected exit code 0 for help flag %s, got %d", h, code)
		}
		if !strings.Contains(stdout.String(), "Agent Sandbox Plugin Manager") {
			t.Errorf("expected plugin manager help text, got: %s", stdout.String())
		}
	}

	// 3. List
	{
		var stdout, stderr bytes.Buffer
		code := handlePlugin(ctx, paths, []string{"list"}, &stdout, &stderr)
		if code != 0 {
			t.Errorf("expected exit code 0 for list, got %d", code)
		}
		if !strings.Contains(stdout.String(), "PLUGIN") || !strings.Contains(stdout.String(), "toolbox") {
			t.Errorf("expected plugins list in stdout, got: %s", stdout.String())
		}
	}

	// 4. Install without target
	{
		var stdout, stderr bytes.Buffer
		code := handlePlugin(ctx, paths, []string{"install"}, &stdout, &stderr)
		if code != 1 {
			t.Errorf("expected exit code 1 for install without args, got %d", code)
		}
	}

	// 5. Toolbox direct install
	{
		var stdout, stderr bytes.Buffer
		code := handlePlugin(ctx, paths, []string{"toolbox"}, &stdout, &stderr)
		if code != 0 {
			t.Errorf("expected exit code 0 for toolbox install, got %d (stderr: %s)", code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "Agent Sandbox JetBrains Gateway / Toolbox Plugin") {
			t.Errorf("expected toolbox plugin success text, got: %s", stdout.String())
		}
	}

	// 6. Gateway alias via `install gateway`
	{
		var stdout, stderr bytes.Buffer
		code := handlePlugin(ctx, paths, []string{"install", "gateway"}, &stdout, &stderr)
		if code != 0 {
			t.Errorf("expected exit code 0 for install gateway, got %d", code)
		}
		if !strings.Contains(stdout.String(), "JetBrains Gateway") {
			t.Errorf("expected gateway output, got: %s", stdout.String())
		}
	}

	// 7. VS Code install
	{
		origExec := plugin.ExecCommandContext
		defer func() { plugin.ExecCommandContext = origExec }()
		plugin.ExecCommandContext = func(ctx context.Context, command string, args ...string) *exec.Cmd {
			return exec.Command("true")
		}

		var stdout, stderr bytes.Buffer
		code := handlePlugin(ctx, paths, []string{"vscode"}, &stdout, &stderr)
		if code != 0 {
			t.Errorf("expected exit code 0 for vscode, got %d", code)
		}
		if !strings.Contains(stdout.String(), "Agent Sandbox VS Code Integration") {
			t.Errorf("expected vscode output, got: %s", stdout.String())
		}

		// Alias `install code`
		var stdout2, stderr2 bytes.Buffer
		code2 := handlePlugin(ctx, paths, []string{"install", "code"}, &stdout2, &stderr2)
		if code2 != 0 {
			t.Errorf("expected exit code 0 for install code, got %d", code2)
		}
	}

	// 8. Unknown plugin
	{
		var stdout, stderr bytes.Buffer
		code := handlePlugin(ctx, paths, []string{"unknown-plugin"}, &stdout, &stderr)
		if code != 1 {
			t.Errorf("expected exit code 1 for unknown plugin, got %d", code)
		}
		if !strings.Contains(stderr.String(), "unknown plugin") {
			t.Errorf("expected error message for unknown plugin, got: %s", stderr.String())
		}
	}

	// 9. Error in toolbox install (bad path)
	{
		badPaths := config.Paths{
			DataHome: "/dev/null/impossible",
		}
		var stdout, stderr bytes.Buffer
		code := handlePlugin(ctx, badPaths, []string{"toolbox"}, &stdout, &stderr)
		if code != 1 {
			t.Errorf("expected exit code 1 on toolbox error, got %d", code)
		}
	}
}

func TestSndbxPluginDomainDispatch(t *testing.T) {
	var stdout, stderr bytes.Buffer
	// Test dispatching through Run()
	code := Run([]string{"plugin", "help"}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("expected Run('plugin', 'help') to exit 0, got %d", code)
	}
	if !strings.Contains(stdout.String(), "Agent Sandbox Plugin Manager") {
		t.Errorf("expected plugin manager help text, got: %s", stdout.String())
	}
}
