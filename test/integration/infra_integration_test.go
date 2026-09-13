// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInfraLifecycleIntegration(t *testing.T) {
	if _, err := exec.LookPath("podman"); err != nil {
		t.Skip("podman not installed or not in PATH; skipping infra lifecycle integration test")
	}

	tmpDir := t.TempDir()

	// Build sndbx binary
	binPath := filepath.Join(tmpDir, "sndbx")
	buildCmd := exec.Command("go", "build", "-o", binPath, "github.com/boggycreek/agent-sandbox/cmd/sndbx")
	out, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed building sndbx binary: %v (%s)", err, string(out))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	// Ensure clean initial state and cleanup at end of test
	stopInfra := func() {
		downCtx, downCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer downCancel()
		cleanupCmd := exec.CommandContext(downCtx, binPath, "infra", "down")
		cleanupCmd.Env = os.Environ()
		_ = cleanupCmd.Run()
	}
	stopInfra()
	defer stopInfra()

	// 1. sndbx infra up
	t.Log("Starting infrastructure via 'sndbx infra up'...")
	upCmd := exec.CommandContext(ctx, binPath, "infra", "up")
	upCmd.Env = os.Environ()
	out, err = upCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("'sndbx infra up' failed: %v\nOutput: %s", err, string(out))
	}
	upOutput := string(out)
	if !strings.Contains(upOutput, "Shared infrastructure is online") {
		t.Errorf("unexpected up output: %s", upOutput)
	}

	// 2. sndbx infra list (while running)
	t.Log("Inspecting infrastructure via 'sndbx infra list'...")
	listCmd := exec.CommandContext(ctx, binPath, "infra", "list")
	listCmd.Env = os.Environ()
	out, err = listCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("'sndbx infra list' failed: %v\nOutput: %s", err, string(out))
	}
	listOutput := string(out)
	if !strings.Contains(listOutput, "agent-sandbox-valkey") {
		t.Errorf("expected agent-sandbox-valkey in list output: %s", listOutput)
	}
	if !strings.Contains(listOutput, "agent-sandbox-gitea") {
		t.Errorf("expected agent-sandbox-gitea in list output: %s", listOutput)
	}
	if !strings.Contains(listOutput, "running") {
		t.Errorf("expected 'running' status in list output: %s", listOutput)
	}

	// 3. sndbx infra down
	t.Log("Stopping infrastructure via 'sndbx infra down'...")
	downCmd := exec.CommandContext(ctx, binPath, "infra", "down")
	downCmd.Env = os.Environ()
	out, err = downCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("'sndbx infra down' failed: %v\nOutput: %s", err, string(out))
	}
	downOutput := string(out)
	if !strings.Contains(downOutput, "Shared infrastructure stopped") {
		t.Errorf("unexpected down output: %s", downOutput)
	}

	// 4. sndbx infra list (after shutdown)
	t.Log("Inspecting infrastructure after down...")
	listAfterCmd := exec.CommandContext(ctx, binPath, "infra", "list")
	listAfterCmd.Env = os.Environ()
	out, err = listAfterCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("'sndbx infra list' after down failed: %v\nOutput: %s", err, string(out))
	}
	listAfterOutput := string(out)
	if strings.Contains(listAfterOutput, "running") {
		t.Errorf("expected no running containers in list output after down: %s", listAfterOutput)
	}
}
