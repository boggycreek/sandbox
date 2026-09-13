// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
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

	// Destroy
	code, out, _ = runSndbx([]string{"agent", "destroy", "coder-1"})
	if code != 0 || !strings.Contains(out, "destroyed completely") {
		t.Errorf("agent destroy failed: %s", out)
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

func TestSndbxRepoAndGUIDomains(t *testing.T) {
	// Repo path
	code, out, _ := runSndbx([]string{"repo", "path"})
	if code != 0 || len(strings.TrimSpace(out)) == 0 {
		t.Errorf("repo path failed")
	}

	// Repo empty
	code, _, _ = runSndbx([]string{"repo"})
	if code != 1 {
		t.Errorf("repo empty should return 1")
	}

	// GUI
	code, out, _ = runSndbx([]string{"gui"})
	if code != 0 || !strings.Contains(out, "Launching Backplane GUI") {
		t.Errorf("gui command failed: %s", out)
	}
}
