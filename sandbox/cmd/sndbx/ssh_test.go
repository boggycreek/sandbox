// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/boggycreek/sandbox/pkg/config"
	"github.com/boggycreek/sandbox/pkg/runtime"
)

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
