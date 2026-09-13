// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package harness

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/boggycreek/agent-sandbox/pkg/gitea"
	"github.com/boggycreek/agent-sandbox/pkg/libbp"
	"github.com/boggycreek/agent-sandbox/pkg/libbp/resp"
)

// EphemeralInfraHarness manages isolated Valkey and Gitea containers for end-to-end integration tests.
type EphemeralInfraHarness struct {
	ValkeyPort     int
	GiteaHTTPPort  int
	GiteaSSHPort   int
	AdminPassword  string
	HumanPassword  string
	HumanName      string
	NetworkName    string
	ValkeyName     string
	GiteaName      string
	ValkeyVolName  string
	GiteaVolName   string
	DataHome       string
	BinPath        string
	engine         string
	t              *testing.T
	teardownOnce   sync.Once
}

// StartEphemeralInfraHarness starts an isolated Valkey and Gitea stack with unique ephemeral names and dynamic ports.
func StartEphemeralInfraHarness(t *testing.T) *EphemeralInfraHarness {
	t.Helper()

	engine := detectContainerEngine()
	if engine == "" {
		t.Skip("Podman container engine not found on PATH; skipping ephemeral infra integration test")
	}

	dataHome := t.TempDir()

	valkeyPort, err := getFreePort()
	if err != nil {
		t.Fatalf("failed finding free port for test valkey: %v", err)
	}

	giteaHTTPPort, err := getFreePort()
	if err != nil {
		t.Fatalf("failed finding free port for test gitea HTTP: %v", err)
	}

	giteaSSHPort, err := getFreePort()
	if err != nil {
		t.Fatalf("failed finding free port for test gitea SSH: %v", err)
	}

	suffix := fmt.Sprintf("%d-%d", os.Getpid(), valkeyPort)
	h := &EphemeralInfraHarness{
		ValkeyPort:    valkeyPort,
		GiteaHTTPPort: giteaHTTPPort,
		GiteaSSHPort:  giteaSSHPort,
		AdminPassword: "test_admin_backplane_pass",
		HumanPassword: "test_human_backplane_pass",
		HumanName:     "tester",
		NetworkName:   fmt.Sprintf("test-infra-net-%s", suffix),
		ValkeyName:    fmt.Sprintf("test-infra-valkey-%s", suffix),
		GiteaName:     fmt.Sprintf("test-infra-gitea-%s", suffix),
		ValkeyVolName: fmt.Sprintf("test-infra-valkey-data-%s", suffix),
		GiteaVolName:  fmt.Sprintf("test-infra-gitea-data-%s", suffix),
		DataHome:      dataHome,
		engine:        engine,
		t:             t,
	}

	// Register automatic teardown
	t.Cleanup(func() {
		h.Teardown()
	})

	// Build isolated sndbx binary in temp dir
	h.BinPath = filepath.Join(dataHome, "sndbx")
	buildCmd := exec.Command("go", "build", "-o", h.BinPath, "github.com/boggycreek/agent-sandbox/cmd/sndbx")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed building test sndbx binary: %v (%s)", err, string(out))
	}

	// Initialize ephemeral infrastructure
	h.startContainers()
	h.waitForReady()

	return h
}

func (h *EphemeralInfraHarness) startContainers() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 1. Create isolated bridge network
	_ = exec.CommandContext(ctx, h.engine, "network", "create", h.NetworkName).Run()

	// 2. Create isolated volumes
	_ = exec.CommandContext(ctx, h.engine, "volume", "create", h.ValkeyVolName).Run()
	_ = exec.CommandContext(ctx, h.engine, "volume", "create", h.GiteaVolName).Run()

	// 3. Write Valkey ACL file
	valkeyConfigDir := filepath.Join(h.DataHome, "valkey")
	_ = os.MkdirAll(valkeyConfigDir, 0755)
	aclFile := filepath.Join(valkeyConfigDir, "valkey-users.acl")

	aclContent := fmt.Sprintf(`user default off
user admin on >%s ~* &* +@all
user %s on >%s ~%s:* ~human:name ~liaison:current ~identity:* %%R~*:* &* +@all (+xadd ~*:inbox)
`, h.AdminPassword, h.HumanName, h.HumanPassword, h.HumanName)
	if err := os.WriteFile(aclFile, []byte(aclContent), 0644); err != nil {
		h.t.Fatalf("failed writing test valkey ACL file: %v", err)
	}

	// 4. Start ephemeral Valkey container
	valkeyArgs := []string{
		"run", "-d",
		"--name", h.ValkeyName,
		"--hostname", "valkey",
		"--network", h.NetworkName,
		"-p", fmt.Sprintf("127.0.0.1:%d:6379", h.ValkeyPort),
		"-v", fmt.Sprintf("%s:/etc/valkey:z", valkeyConfigDir),
		"-v", fmt.Sprintf("%s:/data:z", h.ValkeyVolName),
		"docker.io/valkey/valkey:8.0-alpine",
		"valkey-server", "--aclfile", "/etc/valkey/valkey-users.acl", "--appendonly", "yes", "--port", "6379",
	}
	if out, err := exec.CommandContext(ctx, h.engine, valkeyArgs...).CombinedOutput(); err != nil {
		h.t.Fatalf("failed starting ephemeral Valkey container: %v (%s)", err, string(out))
	}

	// 5. Start ephemeral Gitea container
	giteaArgs := []string{
		"run", "-d",
		"--name", h.GiteaName,
		"--hostname", "gitea",
		"--network", h.NetworkName,
		"-p", fmt.Sprintf("127.0.0.1:%d:3000", h.GiteaHTTPPort),
		"-p", fmt.Sprintf("127.0.0.1:%d:2222", h.GiteaSSHPort),
		"-e", "GITEA__server__DOMAIN=gitea",
		"-e", "GITEA__server__HTTP_PORT=3000",
		"-e", "GITEA__server__ROOT_URL=http://gitea:3000/",
		"-e", "GITEA__server__SSH_PORT=2222",
		"-e", "GITEA__server__SSH_LISTEN_PORT=2222",
		"-e", "GITEA__database__DB_TYPE=sqlite3",
		"-e", "GITEA__database__PATH=/var/lib/gitea/data/gitea.db",
		"-e", "GITEA__service__DISABLE_REGISTRATION=false",
		"-e", "GITEA__service__REQUIRE_SIGNIN_VIEW=false",
		"-e", "GITEA__security__INSTALL_LOCK=true",
		"-v", fmt.Sprintf("%s:/var/lib/gitea:z", h.GiteaVolName),
		"docker.io/gitea/gitea:1.22-rootless",
	}
	if out, err := exec.CommandContext(ctx, h.engine, giteaArgs...).CombinedOutput(); err != nil {
		h.t.Fatalf("failed starting ephemeral Gitea container: %v (%s)", err, string(out))
	}
}

func (h *EphemeralInfraHarness) waitForReady() {
	// Wait for Valkey
	deadline := time.Now().Add(15 * time.Second)
	valkeyReady := false
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		client, err := libbp.Dial(ctx, libbp.ClientConfig{
			Host:     "127.0.0.1",
			Port:     h.ValkeyPort,
			Username: "admin",
			Password: h.AdminPassword,
		})
		cancel()
		if err == nil {
			val, err := client.Exec(context.Background(), "PING")
			client.Close()
			if err == nil && val.Type == resp.TypeSimpleString && val.Str == "PONG" {
				valkeyReady = true
				break
			}
		}
		time.Sleep(150 * time.Millisecond)
	}
	if !valkeyReady {
		h.t.Fatalf("timed out waiting for test Valkey container on port %d", h.ValkeyPort)
	}

	// Wait for Gitea
	giteaURL := fmt.Sprintf("http://127.0.0.1:%d", h.GiteaHTTPPort)
	httpClient := &http.Client{Timeout: 1 * time.Second}
	deadlineGitea := time.Now().Add(25 * time.Second)
	giteaReady := false
	for time.Now().Before(deadlineGitea) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, giteaURL+"/api/v1/version", nil)
		resp, err := httpClient.Do(req)
		cancel()
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			giteaReady = true
			break
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		time.Sleep(300 * time.Millisecond)
	}
	if !giteaReady {
		h.t.Fatalf("timed out waiting for test Gitea container on port %d", h.GiteaHTTPPort)
	}

	// Bootstrap Gitea admin user & fleet org
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_ = exec.CommandContext(ctx, h.engine, "exec", h.GiteaName,
		"gitea", "admin", "user", "create",
		"--admin",
		"--username", "giteaadmin",
		"--password", h.AdminPassword,
		"--email", "giteaadmin@local.sndbx",
		"--must-change-password=false",
	).Run()

	client := gitea.NewClient(gitea.ClientConfig{
		BaseURL:   giteaURL,
		AdminUser: "giteaadmin",
		AdminPass: h.AdminPassword,
		Timeout:   3 * time.Second,
	})

	_ = client.EnsureOrg(ctx, "fleet")
	_ = client.EnsureRepo(ctx, "fleet", "tools", "Fleet shared tools", true)
	_ = client.EnsureRepo(ctx, "fleet", "tasks", "Fleet shared tasks", true)
}

// Env returns an environment variable slice configured for this ephemeral harness.
func (h *EphemeralInfraHarness) Env() []string {
	return []string{
		fmt.Sprintf("XDG_DATA_HOME=%s", h.DataHome),
		fmt.Sprintf("XDG_CONFIG_HOME=%s", h.DataHome),
		"BP_HOST=127.0.0.1",
		fmt.Sprintf("BP_PORT=%d", h.ValkeyPort),
		fmt.Sprintf("ADMIN_BACKPLANE_PASSWORD=%s", h.AdminPassword),
		fmt.Sprintf("HUMAN_BACKPLANE_PASSWORD=%s", h.HumanPassword),
		fmt.Sprintf("HUMAN_NAME=%s", h.HumanName),
		fmt.Sprintf("GITEA_URL=http://127.0.0.1:%d", h.GiteaHTTPPort),
	}
}

// ExecSndbx runs the test sndbx binary with the harness environment.
func (h *EphemeralInfraHarness) ExecSndbx(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, h.BinPath, args...)
	cmd.Env = append(os.Environ(), h.Env()...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Teardown stops and forcefully removes all ephemeral containers, volumes, networks, and directories.
func (h *EphemeralInfraHarness) Teardown() {
	h.teardownOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		if h.engine != "" {
			_ = exec.CommandContext(ctx, h.engine, "stop", h.ValkeyName, h.GiteaName).Run()
			_ = exec.CommandContext(ctx, h.engine, "rm", "-f", h.ValkeyName, h.GiteaName).Run()
			_ = exec.CommandContext(ctx, h.engine, "volume", "rm", "-f", h.ValkeyVolName, h.GiteaVolName).Run()
			_ = exec.CommandContext(ctx, h.engine, "network", "rm", "-f", h.NetworkName).Run()
		}
	})
}
