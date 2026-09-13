// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package integration

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boggycreek/agent-sandbox/test/harness"
)

func TestSndbxCLIEndToEnd(t *testing.T) {
	valkey := harness.StartValkeyHarness(t)
	defer valkey.Teardown()

	tmpDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", tmpDir)
	os.Setenv("BP_HOST", "127.0.0.1")
	os.Setenv("BP_PORT", fmt.Sprintf("%d", valkey.Port))
	os.Setenv("ADMIN_BACKPLANE_PASSWORD", valkey.AdminPass)

	// Compile sndbx binary to a temporary path
	binPath := filepath.Join(tmpDir, "sndbx")
	cmd := exec.Command("go", "build", "-o", binPath, "github.com/boggycreek/agent-sandbox/cmd/sndbx")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed building sndbx binary: %v (%s)", err, string(out))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Create Agent via sndbx CLI
	createCmd := exec.CommandContext(ctx, binPath, "agent", "create", "test-bot", "as", "base", "--role", "tester")
	createCmd.Env = os.Environ()
	out, err = createCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sndbx agent create failed: %v (%s)", err, string(out))
	}
	if !strings.Contains(string(out), "Agent \"test-bot\" created successfully") {
		t.Errorf("unexpected create output: %s", string(out))
	}

	// 2. List agents via sndbx CLI
	listCmd := exec.CommandContext(ctx, binPath, "agent", "list")
	listCmd.Env = os.Environ()
	out, err = listCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sndbx agent list failed: %v (%s)", err, string(out))
	}
	if !strings.Contains(string(out), "test-bot") {
		t.Errorf("expected test-bot in agent list: %s", string(out))
	}

	// 3. Destroy agent via sndbx CLI
	destroyCmd := exec.CommandContext(ctx, binPath, "agent", "destroy", "test-bot")
	destroyCmd.Env = os.Environ()
	out, err = destroyCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("sndbx agent destroy failed: %v (%s)", err, string(out))
	}
	if !strings.Contains(string(out), "destroyed completely") {
		t.Errorf("unexpected destroy output: %s", string(out))
	}
}
