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
