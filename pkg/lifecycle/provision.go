// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package lifecycle manages provisioning and deprovisioning of agent infrastructure.
package lifecycle

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/boggycreek/sandbox/pkg/config"
	"github.com/boggycreek/sandbox/pkg/gitea"
	"github.com/boggycreek/sandbox/pkg/libbp"
	"github.com/boggycreek/sandbox/pkg/sonar"
)

// RegisterValkeyACL configures Valkey ACL permissions and registers identity for a newly provisioned agent.
//
//nolint:gocritic // paths passed by value for consistency with config package
func RegisterValkeyACL(ctx context.Context, cfg *config.AgentConfig, paths config.Paths) error {
	bpCfg := libbp.LoadClientFromEnv()
	client, err := libbp.Dial(ctx, libbp.ClientConfig{
		Host:     bpCfg.Host,
		Port:     bpCfg.Port,
		Username: "admin",
		Password: os.Getenv("ADMIN_BACKPLANE_PASSWORD"),
	})
	if err != nil {
		return err
	}
	defer func() { _ = client.Close() }()

	// Set Valkey ACL for agent
	// Format: ACL SETUSER <name> on ><password> ~<name>:* %R~*:* &* +@all -@admin -@dangerous (+xadd ~*:inbox)
	aclArgs := []string{
		"SETUSER", cfg.Name, "on",
		">" + cfg.Password,
		fmt.Sprintf("~%s:*", cfg.Name),
		fmt.Sprintf("~identity:%s", cfg.Name),
		"%R~*:*", "&*", "+@all", "-@admin", "-@dangerous",
		"(+xadd ~*:inbox)",
	}
	if _, err := client.Exec(ctx, "ACL", aclArgs...); err != nil {
		return err
	}

	// Register Identity
	return client.RegisterIdentity(ctx, libbp.IdentityRecord{
		Name:   cfg.Name,
		Role:   cfg.Role,
		Kind:   "agent",
		PubKey: cfg.PublicKeyB64,
	})
}

// RegisterGiteaUser provisions an agent account, registers IDE public SSH keys, and adds the agent to the fleet org.
//
//nolint:gocritic // paths passed by value for consistency with config package
func RegisterGiteaUser(ctx context.Context, cfg *config.AgentConfig, paths config.Paths) error {
	adminPass := os.Getenv("ADMIN_BACKPLANE_PASSWORD")
	if adminPass == "" {
		adminPass = "admin_backplane_pass"
	}

	giteaURL := os.Getenv("GITEA_URL")
	if giteaURL == "" {
		giteaURL = "http://127.0.0.1:3000"
	}

	client := gitea.NewClient(gitea.ClientConfig{
		BaseURL:   giteaURL,
		AdminUser: "giteaadmin",
		AdminPass: adminPass,
		Timeout:   2 * time.Second,
	})

	// 1. Provision user account in Gitea
	if err := client.EnsureUser(ctx, cfg.Name, cfg.Password, fmt.Sprintf("%s@local.sndbx", cfg.Name)); err != nil {
		return err
	}

	// 2. Add public SSH key if host IDE key exists
	sshKeyPub := filepath.Clean(fmt.Sprintf("%s.pub", paths.IDEKeyFile))
	// #nosec G304 -- reading controlled IDE public key file
	if keyData, err := os.ReadFile(sshKeyPub); err == nil && len(keyData) > 0 {
		_ = client.AddUserSSHKey(ctx, cfg.Name, fmt.Sprintf("%s-ide-key", cfg.Name), string(keyData))
	}

	// 3. Add user to default fleet organization
	return client.AddOrgMember(ctx, "fleet", cfg.Name)
}

// RegisterSonarUser provisions an agent user in SonarQube and generates an analysis token.
//
//nolint:gocritic // paths passed by value for consistency with config package
func RegisterSonarUser(ctx context.Context, cfg *config.AgentConfig, paths config.Paths) error {
	adminPass := os.Getenv("ADMIN_BACKPLANE_PASSWORD")
	if adminPass == "" {
		adminPass = "admin"
	}
	adminUser := os.Getenv("SONAR_ADMIN_USER")
	if adminUser == "" {
		adminUser = "admin"
	}

	sonarURL := os.Getenv("SONAR_HOST_URL")
	if sonarURL == "" {
		sonarURL = os.Getenv("SONARQUBE_URL")
	}
	if sonarURL == "" {
		sonarURL = "http://127.0.0.1:9000"
	}

	client := sonar.NewClient(sonar.ClientConfig{
		BaseURL:   sonarURL,
		AdminUser: adminUser,
		AdminPass: adminPass,
		Timeout:   2 * time.Second,
	})

	// Check if SonarQube is responsive
	if _, err := client.GetSystemStatus(ctx); err != nil {
		return err
	}

	// 1. Provision user in SonarQube
	if err := client.EnsureUser(ctx, cfg.Name, cfg.Password, cfg.Name, fmt.Sprintf("%s@local.sndbx", cfg.Name)); err != nil {
		return err
	}

	// 2. Generate agent-specific analysis token
	token, err := client.GenerateUserToken(ctx, cfg.Name, fmt.Sprintf("%s-agent-token", cfg.Name))
	if err == nil && token != "" {
		cfg.SonarToken = token
		return config.SaveAgentConfig(cfg, paths)
	}
	return err
}
