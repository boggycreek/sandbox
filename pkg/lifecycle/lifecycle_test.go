// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package lifecycle

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boggycreek/sandbox/pkg/config"
	"github.com/boggycreek/sandbox/pkg/libbp"
	"github.com/boggycreek/sandbox/test/harness"
)

func TestLifecycleProvisionAndDeprovision(t *testing.T) {
	valkey := harness.StartValkeyHarness(t)
	defer valkey.Teardown()

	tmpDir := t.TempDir()
	paths := config.Paths{
		DataHome:   filepath.Join(tmpDir, "data"),
		AgentsDir:  filepath.Join(tmpDir, "data", "agents"),
		SecretsDir: filepath.Join(tmpDir, "data", "secrets"),
		BinDir:     filepath.Join(tmpDir, "bin"),
		SSHDir:     filepath.Join(tmpDir, "ssh"),
		IDEKeyFile: filepath.Join(tmpDir, "ssh", "agent-sandbox"),
	}
	_ = paths.EnsureDirectories()

	// Write dummy IDE key file and pub file
	_ = os.WriteFile(paths.IDEKeyFile, []byte("dummy-key"), 0o600)
	// #nosec G306 -- test fixture file
	_ = os.WriteFile(fmt.Sprintf("%s.pub", paths.IDEKeyFile), []byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 test-ide-key"), 0o600)

	t.Setenv("ADMIN_BACKPLANE_PASSWORD", valkey.AdminPass)
	t.Setenv("BP_HOST", "127.0.0.1")
	t.Setenv("BP_PORT", fmt.Sprintf("%d", valkey.Port))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := config.NewAgentConfig("test-agent", "base", "coder")
	if err != nil {
		t.Fatalf("failed creating agent config: %v", err)
	}

	// 1. Valkey ACL Provisioning
	if regErr := RegisterValkeyACL(ctx, cfg, paths); regErr != nil {
		t.Fatalf("RegisterValkeyACL failed: %v", regErr)
	}

	// Verify ACL connection works with provisioned credentials
	agentClient, dialErr := libbp.Dial(ctx, libbp.ClientConfig{
		Host:     "127.0.0.1",
		Port:     valkey.Port,
		Username: cfg.Name,
		Password: cfg.Password,
		AgentID:  cfg.Name,
	})
	if dialErr != nil {
		t.Fatalf("failed dialing valkey as provisioned agent: %v", dialErr)
	}
	_ = agentClient.Close()

	// 2. Valkey Deprovisioning
	if deprovErr := DeprovisionValkeyUser(ctx, cfg.Name); deprovErr != nil {
		t.Fatalf("DeprovisionValkeyUser failed: %v", deprovErr)
	}

	// Verify dialing after deprovision fails
	_, err = libbp.Dial(ctx, libbp.ClientConfig{
		Host:     "127.0.0.1",
		Port:     valkey.Port,
		Username: cfg.Name,
		Password: cfg.Password,
		AgentID:  cfg.Name,
	})
	if err == nil {
		t.Fatalf("expected dial to fail after deprovisioning agent user")
	}

	// 3. Dial failure branches for Valkey
	t.Setenv("BP_PORT", "65530") // invalid port
	if err := RegisterValkeyACL(ctx, cfg, paths); err == nil {
		t.Errorf("expected RegisterValkeyACL error on unreachable Valkey")
	}
	if err := DeprovisionValkeyUser(ctx, cfg.Name); err == nil {
		t.Errorf("expected DeprovisionValkeyUser error on unreachable Valkey")
	}
}

func TestGiteaProvisionAndDeprovision(t *testing.T) {
	tmpDir := t.TempDir()
	paths := config.Paths{
		DataHome:   filepath.Join(tmpDir, "data"),
		IDEKeyFile: filepath.Join(tmpDir, "agent-sandbox"),
	}
	// #nosec G306 -- test fixture file
	_ = os.WriteFile(fmt.Sprintf("%s.pub", paths.IDEKeyFile), []byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5 dummy-key"), 0o600)

	// Mock Gitea server
	mockGitea := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/v1/users/test-agent":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id": 1, "username": "test-agent"}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/v1/users/test-agent/keys":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"id": 1, "title": "test-agent-ide-key"}`))
		case r.Method == http.MethodPut && r.URL.Path == "/api/v1/orgs/fleet/members/test-agent":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete && r.URL.Path == "/api/v1/admin/users/test-agent":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer mockGitea.Close()

	t.Setenv("GITEA_URL", mockGitea.URL)
	t.Setenv("ADMIN_BACKPLANE_PASSWORD", "mockpass")

	ctx := context.Background()
	cfg := &config.AgentConfig{Name: "test-agent", Password: "testpassword"}

	if err := RegisterGiteaUser(ctx, cfg, paths); err != nil {
		t.Errorf("RegisterGiteaUser failed: %v", err)
	}

	if err := DeprovisionGiteaUser(ctx, cfg.Name); err != nil {
		t.Errorf("DeprovisionGiteaUser failed: %v", err)
	}

	// Failure branch
	t.Setenv("GITEA_URL", "http://127.0.0.1:65531")
	if err := RegisterGiteaUser(ctx, cfg, paths); err == nil {
		t.Errorf("expected RegisterGiteaUser error on offline Gitea")
	}
}

func TestSonarProvisionAndDeprovision(t *testing.T) {
	tmpDir := t.TempDir()
	paths := config.Paths{
		DataHome:   filepath.Join(tmpDir, "data"),
		AgentsDir:  filepath.Join(tmpDir, "data", "agents"),
		SecretsDir: filepath.Join(tmpDir, "data", "secrets"),
	}
	_ = paths.EnsureDirectories()

	mockSonar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/system/status":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status": "UP"}`))
		case "/api/users/create":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"user": {"login": "test-agent"}}`))
		case "/api/user_tokens/generate":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"token": "test-token-1234"}`))
		case "/api/user_tokens/revoke":
			w.WriteHeader(http.StatusNoContent)
		case "/api/users/deactivate":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"user": {"login": "test-agent", "active": false}}`))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	defer mockSonar.Close()

	t.Setenv("SONAR_HOST_URL", mockSonar.URL)
	t.Setenv("ADMIN_BACKPLANE_PASSWORD", "admin")

	ctx := context.Background()
	cfg := &config.AgentConfig{Name: "test-agent", Password: "testpassword"}
	_ = config.SaveAgentConfig(cfg, paths)

	if err := RegisterSonarUser(ctx, cfg, paths); err != nil {
		t.Errorf("RegisterSonarUser failed: %v", err)
	}

	if cfg.SonarToken != "test-token-1234" {
		t.Errorf("expected token 'test-token-1234', got %q", cfg.SonarToken)
	}

	if err := DeprovisionSonarUser(ctx, cfg.Name); err != nil {
		t.Errorf("DeprovisionSonarUser failed: %v", err)
	}

	// Offline branch
	t.Setenv("SONAR_HOST_URL", "http://127.0.0.1:65532")
	if err := RegisterSonarUser(ctx, cfg, paths); err == nil {
		t.Errorf("expected RegisterSonarUser error on offline Sonar")
	}

	// Environment variable fallback branches
	t.Setenv("GITEA_URL", "")
	t.Setenv("ADMIN_BACKPLANE_PASSWORD", "")
	_ = RegisterGiteaUser(ctx, cfg, paths)
	_ = DeprovisionGiteaUser(ctx, "offline-agent")

	t.Setenv("SONAR_HOST_URL", "")
	t.Setenv("SONARQUBE_URL", "")
	t.Setenv("SONAR_ADMIN_USER", "")
	_ = DeprovisionSonarUser(ctx, "offline-agent")

	// Test SONARQUBE_URL fallback and EnsureUser failure
	mockSonarErr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/system/status" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status": "UP"}`))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer mockSonarErr.Close()
	t.Setenv("SONARQUBE_URL", mockSonarErr.URL)
	t.Setenv("SONAR_HOST_URL", "")
	if err := RegisterSonarUser(ctx, cfg, paths); err == nil {
		t.Errorf("expected error on EnsureUser failure")
	}
}
