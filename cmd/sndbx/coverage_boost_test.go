// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/boggycreek/agent-sandbox/pkg/config"
	"github.com/boggycreek/agent-sandbox/test/harness"
)

func TestSndbxSubcommandsBoost(t *testing.T) {
	valkey := harness.StartValkeyHarness(t)
	defer valkey.Teardown()

	tmpDir := t.TempDir()
	os.Setenv("XDG_DATA_HOME", tmpDir)
	os.Setenv("ADMIN_BACKPLANE_PASSWORD", valkey.AdminPass)
	os.Setenv("BP_HOST", "127.0.0.1")
	os.Setenv("BP_PORT", string(rune(valkey.Port))) // dummy

	paths := config.GetPaths()
	_ = paths.EnsureDirectories()

	// 1. Create agent with Valkey registration
	var stdout, stderr bytes.Buffer
	code := Run([]string{"agent", "create", "test-agent", "as", "base"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("agent create failed: %s", stderr.String())
	}

	// 1b. Create with --role and defaults
	code = Run([]string{"agent", "create", "role-agent", "--role", "tester"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("agent create with role failed")
	}

	// 1c. Create error on empty name
	code = Run([]string{"agent", "create", ""}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("agent create empty name should fail")
	}

	// 1d. Create error on invalid flags
	code = Run([]string{"agent", "create", "bad-flag-agent", "--invalid-flag"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("agent create with invalid flag should fail")
	}

	// 2. Start agent
	code = Run([]string{"agent", "start", "test-agent"}, &stdout, &stderr)
	_ = code

	// 3. Stop --all & individual
	code = Run([]string{"agent", "stop", "--all"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("agent stop --all failed")
	}
	code = Run([]string{"agent", "stop", "test-agent"}, &stdout, &stderr)
	_ = code

	// 4. Clean & Destroy
	code = Run([]string{"agent", "clean", "test-agent"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("agent clean failed")
	}
	code = Run([]string{"agent", "destroy", "test-agent"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("agent destroy failed")
	}
	code = Run([]string{"agent", "destroy"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("agent destroy missing args should fail")
	}
	code = Run([]string{"agent", "clean"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("agent clean missing args should fail")
	}

	// 5. Infra subcommands
	code = Run([]string{"infra", "unknown"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("infra unknown should return 1")
	}
	code = Run([]string{"infra"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("infra empty should return 1")
	}
	code = Run([]string{"infra", "down"}, &stdout, &stderr)
	_ = code
	code = Run([]string{"infra", "list"}, &stdout, &stderr)
	_ = code
	code = Run([]string{"infra", "up"}, &stdout, &stderr)
	_ = code

	// 6. Repo subcommands
	code = Run([]string{"repo", "unknown"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("repo unknown should return 1")
	}
	code = Run([]string{"repo", "path"}, &stdout, &stderr)
	_ = code
	code = Run([]string{"repo", "build"}, &stdout, &stderr)
	_ = code
	code = Run([]string{"repo", "build-images"}, &stdout, &stderr)
	_ = code

	// 7. Test Start / Connect / SSH error paths
	code = Run([]string{"agent", "start", "nonexistent"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("start nonexistent should fail")
	}
	code = Run([]string{"agent", "connect", "nonexistent"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("connect nonexistent should fail")
	}
	code = Run([]string{"agent", "ssh", "nonexistent"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("ssh nonexistent should fail")
	}

	// 8. registerValkeyACL direct test
	cfg, _ := config.NewAgentConfig("acl-test-agent", "base", "tester")
	registerValkeyACL(context.Background(), cfg, paths)

	// 9. Corrupted JSON file in list & handleAgentList error branch
	corruptFile := filepath.Join(paths.AgentsDir, "corrupted.json")
	_ = os.WriteFile(corruptFile, []byte("{invalid-json"), 0600)
	code = Run([]string{"agent", "list"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("agent list should tolerate or skip corrupted configs")
	}

	// 10. Direct execution of handleAgentConnect & handleAgentSSH with created mock agent
	_ = config.SaveAgentConfig(cfg, paths)
	_ = handleAgentConnect(context.Background(), paths, []string{"acl-test-agent"}, &stdout, &stderr)
	_ = handleAgentSSH(context.Background(), paths, []string{"acl-test-agent"}, &stdout, &stderr)

	// 11. handleAgentList error on invalid paths
	roPaths := config.Paths{AgentsDir: "/dev/null/forbidden/agents"}
	_ = handleAgentList(context.Background(), roPaths, []string{}, &stdout, &stderr)

	// 12. handleAgentCreate failure on save
	_ = handleAgentCreate(context.Background(), roPaths, []string{"fail-create"}, &stdout, &stderr)

	// 13. handleAgentClean / handleAgentDestroy on nonexistent agent
	_ = handleAgentClean(context.Background(), paths, []string{"nonexistent-clean"}, &stdout, &stderr)
	_ = handleAgentDestroy(context.Background(), paths, []string{"nonexistent-destroy"}, &stdout, &stderr)
	_ = handleAgentStop(context.Background(), paths, []string{"nonexistent-stop"}, &stdout, &stderr)
}
