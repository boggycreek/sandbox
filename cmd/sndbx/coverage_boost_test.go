// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boggycreek/agent-sandbox/pkg/config"
	"github.com/boggycreek/agent-sandbox/test/harness"
)

func TestSndbxSubcommandsBoost(t *testing.T) {
	valkey := harness.StartValkeyHarness(t)
	defer valkey.Teardown()

	tmpDir := t.TempDir()
	t.Cleanup(func() {
		_ = exec.Command("podman", "unshare", "rm", "-rf", tmpDir).Run()
	})
	os.Setenv("XDG_DATA_HOME", tmpDir)
	os.Setenv("ADMIN_BACKPLANE_PASSWORD", valkey.AdminPass)
	os.Setenv("BP_HOST", "127.0.0.1")
	os.Setenv("BP_PORT", fmt.Sprintf("%d", valkey.Port))

	paths := config.GetPaths()
	_ = paths.EnsureDirectories()

	// 1. Create agent with Valkey registration
	var stdout, stderr bytes.Buffer
	code := Run([]string{"agent", "create", "test-agent", "as", "base"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("agent create failed: %s", stderr.String())
	}

	// 1b. Create with --role, --model-url, --model-name, --model-api-key
	code = Run([]string{
		"agent", "create", "role-agent",
		"--role", "tester",
		"--model-url", "http://localhost:11434/v1",
		"--model-name", "qwen2.5-coder:32b",
		"--model-api-key", "secret-key",
	}, &stdout, &stderr)
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

	// 1e. Create agent help
	code = Run([]string{"agent", "help"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("agent help failed")
	}
	code = Run([]string{"agent"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("agent empty should return 1")
	}
	code = Run([]string{"agent", "unknown"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("agent unknown should return 1")
	}

	// 2. Start subcommands
	code = Run([]string{"agent", "start"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("agent start empty should return 1")
	}
	code = Run([]string{"agent", "start", "--all"}, &stdout, &stderr)
	_ = code
	code = Run([]string{"agent", "start", "role-agent"}, &stdout, &stderr)
	_ = code

	// 3. Stop --all & individual
	code = Run([]string{"agent", "stop", "--all"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("agent stop --all failed")
	}
	code = Run([]string{"agent", "stop", "test-agent"}, &stdout, &stderr)
	_ = code

	// 4. Clean, Destroy & Retire
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
	code = Run([]string{"agent", "retire"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("agent retire missing args should fail")
	}
	code = Run([]string{"agent", "retire", "role-agent"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("agent retire role-agent failed: %s", stderr.String())
	}
	code = Run([]string{"agent", "retire", "nonexistent-agent"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("agent retire nonexistent should fail")
	}

	// 5. Infra subcommands
	code = Run([]string{"infra", "help"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("infra help should return 0")
	}
	code = Run([]string{"infra", "-h"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("infra -h should return 0")
	}
	code = Run([]string{"infra", "unknown"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("infra unknown should return 1")
	}
	code = Run([]string{"infra"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("infra empty should return 1")
	}
	code = Run([]string{"infra", "up"}, &stdout, &stderr)
	_ = code
	code = Run([]string{"infra", "down"}, &stdout, &stderr)
	_ = code
	code = Run([]string{"infra", "list"}, &stdout, &stderr)
	_ = code
	code = Run([]string{"infra", "doctor"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("infra doctor failed: %s", stderr.String())
	}

	// 6. Repo subcommands
	code = Run([]string{"repo", "help"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("repo help should return 0")
	}
	code = Run([]string{"repo", "-h"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("repo -h should return 0")
	}
	code = Run([]string{"repo", "unknown"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("repo unknown should return 1")
	}
	code = Run([]string{"repo"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("repo empty should return 1")
	}
	code = Run([]string{"repo", "path"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("repo path should return 0")
	}
	code = Run([]string{"repo", "build"}, &stdout, &stderr)
	_ = code
	code = Run([]string{"repo", "build-images"}, &stdout, &stderr)
	_ = code

	// 7. Test Start / Connect / SSH error paths
	code = Run([]string{"agent", "start", "nonexistent"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("start nonexistent should fail")
	}
	code = Run([]string{"agent", "connect"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("connect empty should fail")
	}
	code = Run([]string{"agent", "connect", "nonexistent"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("connect nonexistent should fail")
	}
	code = Run([]string{"agent", "ssh"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("ssh empty should fail")
	}
	code = Run([]string{"agent", "ssh", "nonexistent"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("ssh nonexistent should fail")
	}

	// 8. registerValkeyACL & registerGiteaUser direct test
	cfg, _ := config.NewAgentConfig("acl-test-agent", "base", "tester")
	registerValkeyACL(context.Background(), cfg, paths)

	// Mock Gitea server for registerGiteaUser and deprovisionGiteaUser
	giteaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer giteaServer.Close()
	os.Setenv("GITEA_URL", giteaServer.URL)

	_ = os.WriteFile(paths.IDEKeyFile+".pub", []byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA test"), 0644)
	registerGiteaUser(context.Background(), cfg, paths)
	deprovisionGiteaUser(context.Background(), "acl-test-agent")

	// 9. Corrupted JSON file in list & handleAgentList error branch
	corruptFile := filepath.Join(paths.AgentsDir, "corrupted.json")
	_ = os.WriteFile(corruptFile, []byte("{invalid-json"), 0600)
	code = Run([]string{"agent", "list"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("agent list should tolerate or skip corrupted configs")
	}

	// 10. Direct execution of handleAgentConnect, handleAgentSSH, handleAgentStart with created mock agent
	_ = config.SaveAgentConfig(cfg, paths)
	_ = handleAgentStart(context.Background(), paths, []string{}, &stdout, &stderr)
	_ = handleAgentStart(context.Background(), paths, []string{"nonexistent-start"}, &stdout, &stderr)
	_ = handleAgentConnect(context.Background(), paths, []string{"acl-test-agent"}, &stdout, &stderr)
	_ = handleAgentSSH(context.Background(), paths, []string{"acl-test-agent"}, &stdout, &stderr)
	_ = handleAgentStop(context.Background(), paths, []string{"acl-test-agent"}, &stdout, &stderr)

	// 11. handleAgentList error on invalid paths
	roPaths := config.Paths{AgentsDir: "/dev/null/forbidden/agents"}
	_ = handleAgentList(context.Background(), roPaths, []string{}, &stdout, &stderr)

	// 12. handleAgentCreate failure on save
	_ = handleAgentCreate(context.Background(), roPaths, []string{"fail-create"}, &stdout, &stderr)

	// 13. handleAgentClean / handleAgentDestroy on nonexistent agent
	_ = handleAgentClean(context.Background(), paths, []string{"nonexistent-clean"}, &stdout, &stderr)
	_ = handleAgentDestroy(context.Background(), paths, []string{"nonexistent-destroy"}, &stdout, &stderr)
	_ = handleAgentStop(context.Background(), paths, []string{"nonexistent-stop"}, &stdout, &stderr)

	// 14. Root CLI options
	code = Run([]string{"help"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("sndbx help failed")
	}
	code = Run([]string{"-h"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("sndbx -h failed")
	}
	code = Run([]string{"--help"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("sndbx --help failed")
	}
	code = Run([]string{"unknown-root-cmd"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("sndbx unknown-root-cmd failed")
	}

	// 15. Agent ssh-config command
	code = Run([]string{"agent", "ssh-config", "nonexistent"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("ssh-config nonexistent should fail")
	}
	code = Run([]string{"agent", "ssh-config", "acl-test-agent"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("ssh-config on stopped agent should fail")
	}
	code = Run([]string{"agent", "ssh-config"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("ssh-config with no args should succeed: %s", stderr.String())
	}
	code = Run([]string{"agent", "ssh-config", "--all"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("ssh-config --all should succeed: %s", stderr.String())
	}

	// Direct tests for handleAgentSSHConfig & printSSHConfigBlock
	_ = handleAgentSSHConfig(context.Background(), roPaths, []string{}, &stdout, &stderr)
	var configOut bytes.Buffer
	printSSHConfigBlock(&configOut, "my-agent", 2222, "/home/test/.ssh/agent-sandbox")
	if !strings.Contains(configOut.String(), "Host sndbx-my-agent") || !strings.Contains(configOut.String(), "Port 2222") {
		t.Errorf("printSSHConfigBlock output unexpected: %s", configOut.String())
	}

	// 16. GUI command
	code = Run([]string{"gui"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("gui command failed")
	}

	// 17. List with --json flag
	code = Run([]string{"agent", "list", "--json"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("agent list --json failed")
	}

	// 18. handleAgentClean and handleAgentDestroy on existing config
	_ = config.SaveAgentConfig(cfg, paths)
	_ = handleAgentClean(context.Background(), paths, []string{"acl-test-agent"}, &stdout, &stderr)
	_ = handleAgentDestroy(context.Background(), paths, []string{"acl-test-agent"}, &stdout, &stderr)

	// 19. Agent doctor command
	code = Run([]string{"agent", "doctor"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("agent doctor with empty args should return 1")
	}
	code = Run([]string{"agent", "doctor", "nonexistent"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("agent doctor nonexistent should return 1")
	}
	_ = config.SaveAgentConfig(cfg, paths)
	code = Run([]string{"agent", "doctor", "acl-test-agent"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("agent doctor acl-test-agent failed: %s", stderr.String())
	}

	// 20. Doctor on corrupted agent (unrepairable error path)
	_ = os.WriteFile(filepath.Join(paths.AgentsDir, "broken.json"), []byte("{invalid"), 0600)
	code = Run([]string{"agent", "doctor", "broken"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("agent doctor on broken JSON should return 1")
	}

	// 21. Table formatting for agent list with multiple agents
	_ = config.SaveAgentConfig(cfg, paths)
	cfg2, _ := config.NewAgentConfig("second-agent", "opencode", "developer")
	_ = config.SaveAgentConfig(cfg2, paths)
	code = Run([]string{"agent", "list"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("agent list table output failed")
	}

	// 22. Infra doctor command
	code = Run([]string{"infra", "doctor"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("infra doctor failed: %s", stderr.String())
	}

	// 23. Repo commands
	code = Run([]string{"repo"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("repo empty args should return 1")
	}
	code = Run([]string{"repo", "help"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("repo help should return 0")
	}
	code = Run([]string{"repo", "path"}, &stdout, &stderr)
	if code != 0 {
		t.Errorf("repo path should return 0")
	}
	code = Run([]string{"repo", "unknown"}, &stdout, &stderr)
	if code != 1 {
		t.Errorf("repo unknown should return 1")
	}
}



