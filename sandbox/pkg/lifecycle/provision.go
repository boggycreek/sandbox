// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package lifecycle manages provisioning and deprovisioning of agent infrastructure.
package lifecycle

import (
	"context"
	"crypto/ed25519"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boggycreek/sandbox/backplane/pkg/libbp"
	"github.com/boggycreek/sandbox/pkg/config"
	"github.com/boggycreek/sandbox/pkg/gitea"
	"github.com/boggycreek/sandbox/pkg/runtime"
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
	adminPass := os.Getenv("GITEA_ADMIN_PASSWORD")
	if adminPass == "" {
		adminPass = os.Getenv("ADMIN_BACKPLANE_PASSWORD")
	}
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
		if keyErr := client.AddUserSSHKey(ctx, cfg.Name, fmt.Sprintf("%s-ide-key", cfg.Name), string(keyData)); keyErr != nil {
			return fmt.Errorf("failed registering IDE public key with Gitea: %w", keyErr)
		}
	}

	// 3. Add user to default fleet organization
	return client.AddOrgMember(ctx, "fleet", cfg.Name)
}

// RegisterSonarUser provisions an agent user in SonarQube and generates an analysis token.
//
//nolint:gocritic // paths passed by value for consistency with config package
func RegisterSonarUser(ctx context.Context, cfg *config.AgentConfig, paths config.Paths) error {
	adminPass := os.Getenv("SONAR_ADMIN_PASSWORD")
	if adminPass == "" {
		adminPass = os.Getenv("ADMIN_BACKPLANE_PASSWORD")
	}
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
	if err != nil {
		return fmt.Errorf("failed generating SonarQube analysis token: %w", err)
	}
	if token != "" {
		cfg.SonarToken = token
		return config.SaveAgentConfig(cfg, paths)
	}
	return nil
}

// EnsureAgentKeys verifies that the agent has valid Ed25519 signing keys and on-disk PEM secrets.
// If missing, keys are generated and persisted.
func EnsureAgentKeys(cfg *config.AgentConfig, paths config.Paths) error {
	if cfg.SigningKeyPEM == "" || cfg.PublicKeyB64 == "" {
		priv, pub, err := libbp.GenerateKeypair()
		if err != nil {
			return fmt.Errorf("failed generating Ed25519 keypair for agent %s: %w", cfg.Name, err)
		}
		pemStr, err := libbp.EncodePrivateKeyPEM(priv)
		if err != nil {
			return fmt.Errorf("failed encoding signing key PEM: %w", err)
		}
		cfg.SigningKeyPEM = pemStr
		cfg.PublicKeyB64 = libbp.EncodePublicKeyBase64(pub)
	}

	secretDir := filepath.Join(paths.SecretsDir, cfg.Name)
	if err := os.MkdirAll(secretDir, 0700); err != nil {
		return fmt.Errorf("failed creating secret dir %s: %w", secretDir, err)
	}

	keyPath := filepath.Join(secretDir, "signing-key.pem")
	if _, err := os.Stat(keyPath); os.IsNotExist(err) {
		if err := os.WriteFile(keyPath, []byte(cfg.SigningKeyPEM), 0600); err != nil {
			return fmt.Errorf("failed writing signing key file: %w", err)
		}
	}

	return config.SaveAgentConfig(cfg, paths)
}

// EnsureOperatorKeys provisions the host operator's Ed25519 signing keypair and backplane identity.
func EnsureOperatorKeys(ctx context.Context, paths config.Paths, humanName string) (*libbp.IdentityRecord, error) {
	if humanName == "" {
		humanName = os.Getenv("HUMAN_NAME")
	}
	if humanName == "" {
		humanName = "operator"
	}
	humanName = strings.ToLower(strings.TrimSpace(humanName))

	secretDir := filepath.Join(paths.SecretsDir, humanName)
	if err := os.MkdirAll(secretDir, 0700); err != nil {
		return nil, fmt.Errorf("failed creating operator secret dir %s: %w", secretDir, err)
	}

	keyPath := filepath.Join(secretDir, "signing-key.pem")
	var pubB64 string
	if data, err := os.ReadFile(keyPath); err == nil && len(data) > 0 {
		priv, err := libbp.DecodePrivateKeyPEM(string(data))
		if err == nil {
			pubB64 = libbp.EncodePublicKeyBase64(priv.Public().(ed25519.PublicKey))
		}
	}

	if pubB64 == "" {
		priv, pub, err := libbp.GenerateKeypair()
		if err != nil {
			return nil, fmt.Errorf("failed generating operator keypair: %w", err)
		}
		pemStr, err := libbp.EncodePrivateKeyPEM(priv)
		if err != nil {
			return nil, fmt.Errorf("failed encoding operator signing key: %w", err)
		}
		if err := os.WriteFile(keyPath, []byte(pemStr), 0600); err != nil {
			return nil, fmt.Errorf("failed writing operator signing key: %w", err)
		}
		pubB64 = libbp.EncodePublicKeyBase64(pub)
	}

	record := &libbp.IdentityRecord{
		Name:   humanName,
		Role:   "Human Operator",
		Kind:   "human",
		PubKey: pubB64,
	}

	// Try registering in Valkey if reachable
	bpCfg := libbp.LoadClientFromEnv()
	client, err := libbp.Dial(ctx, libbp.ClientConfig{
		Host:     bpCfg.Host,
		Port:     bpCfg.Port,
		Username: "admin",
		Password: os.Getenv("ADMIN_BACKPLANE_PASSWORD"),
	})
	if err == nil {
		defer func() { _ = client.Close() }()
		_ = client.RegisterIdentity(ctx, *record)
	}

	return record, nil
}

// ProvisionAgent performs all infrastructure provisioning steps for an agent and returns a structured ProvisioningReport.
//
//nolint:gocritic // paths passed by value for consistency with config package
func ProvisionAgent(ctx context.Context, cfg *config.AgentConfig, paths config.Paths) *ProvisioningReport {
	report := NewProvisioningReport(cfg.Name, "provision")

	// 1. Cryptographic Keys
	if err := EnsureAgentKeys(cfg, paths); err == nil {
		report.AddStep("Cryptographic Signing Keys", StatusOK, "Verified Ed25519 signing keypair and on-disk secret", nil)
	} else {
		report.AddStep("Cryptographic Signing Keys", StatusError, fmt.Sprintf("Failed ensuring signing keys: %v", err), err)
	}

	// 2. Valkey ACL & Identity
	if err := RegisterValkeyACL(ctx, cfg, paths); err == nil {
		report.AddStep("Valkey ACL & Identity", StatusOK, "Registered ACL user and backplane identity", nil)
	} else if IsOfflineError(err) {
		report.AddStep("Valkey ACL & Identity", StatusSkipped, fmt.Sprintf("Valkey infrastructure is offline (skipped; start via 'sndbx infra up'): %v", err), err)
	} else {
		report.AddStep("Valkey ACL & Identity", StatusError, fmt.Sprintf("Failed registering Valkey ACL: %v", err), err)
	}

	// 3. Gitea Account & Keys
	if os.Getenv("GITEA_DISABLED") == "1" {
		report.AddStep("Gitea User & Keys", StatusSkipped, "Gitea infrastructure is disabled via GITEA_DISABLED (skipped)", nil)
	} else if err := RegisterGiteaUser(ctx, cfg, paths); err == nil {
		report.AddStep("Gitea User & Keys", StatusOK, "Registered user account, IDE key, and fleet organization", nil)
	} else if IsOfflineError(err) {
		report.AddStep("Gitea User & Keys", StatusSkipped, fmt.Sprintf("Gitea infrastructure is offline (skipped; start via 'sndbx infra up'): %v", err), err)
	} else {
		report.AddStep("Gitea User & Keys", StatusError, fmt.Sprintf("Failed registering Gitea user: %v", err), err)
	}

	// 4. SonarQube User & Analysis Token
	if os.Getenv("SONAR_DISABLED") == "1" {
		report.AddStep("SonarQube Account & Token", StatusSkipped, "SonarQube infrastructure is disabled via SONAR_DISABLED (skipped)", nil)
	} else if err := RegisterSonarUser(ctx, cfg, paths); err == nil {
		report.AddStep("SonarQube Account & Token", StatusOK, "Registered user account and generated analysis token", nil)
	} else if IsOfflineError(err) {
		report.AddStep("SonarQube Account & Token", StatusSkipped, fmt.Sprintf("SonarQube infrastructure is offline (skipped; start via 'sndbx infra up'): %v", err), err)
	} else {
		report.AddStep("SonarQube Account & Token", StatusError, fmt.Sprintf("Failed registering SonarQube user: %v", err), err)
	}

	// 5. Host SSH Configuration Sync
	if err := runtime.SyncSSHConfigFile(ctx, paths); err == nil {
		report.AddStep("SSH Configuration", StatusOK, "Synchronized host SSH config", nil)
	} else {
		report.AddStep("SSH Configuration", StatusWarning, fmt.Sprintf("Failed to synchronize SSH config: %v", err), err)
	}

	return report
}
