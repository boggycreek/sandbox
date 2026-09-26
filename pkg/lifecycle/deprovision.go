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
	"time"

	"github.com/boggycreek/sandbox/pkg/config"
	"github.com/boggycreek/sandbox/pkg/gitea"
	"github.com/boggycreek/sandbox/pkg/libbp"
	"github.com/boggycreek/sandbox/pkg/runtime"
	"github.com/boggycreek/sandbox/pkg/sonar"
)

// DeprovisionValkeyUser removes the agent's Valkey ACL user, streams, and identity registration.
func DeprovisionValkeyUser(ctx context.Context, agentName string) error {
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

	// Delete ACL user
	if _, execErr := client.Exec(ctx, "ACL", "DELUSER", agentName); execErr != nil {
		return execErr
	}

	// Clean up backplane keys & identity
	_, err = client.Exec(ctx, "DEL",
		fmt.Sprintf("identity:%s", agentName),
		fmt.Sprintf("%s:inbox", agentName),
		fmt.Sprintf("%s:out", agentName),
		fmt.Sprintf("%s:seq", agentName),
		fmt.Sprintf("%s:finger", agentName),
		fmt.Sprintf("%s:status", agentName),
	)
	return err
}

// DeprovisionGiteaUser removes the agent account and public keys from the local Gitea instance.
func DeprovisionGiteaUser(ctx context.Context, agentName string) error {
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
		Timeout:   3 * time.Second,
	})

	return client.DeleteUser(ctx, agentName, true)
}

// DeprovisionSonarUser revokes the agent's analysis token and deactivates the agent account in SonarQube.
func DeprovisionSonarUser(ctx context.Context, agentName string) error {
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

	if tokenErr := client.RevokeUserToken(ctx, agentName, fmt.Sprintf("%s-agent-token", agentName)); tokenErr != nil && IsOfflineError(tokenErr) {
		return tokenErr
	}
	return client.DeactivateUser(ctx, agentName)
}

// DeprovisionAgent deprovisions container, storage, configuration, and infrastructure for an agent.
//
//nolint:gocritic // paths passed by value for consistency with config package
func DeprovisionAgent(ctx context.Context, cfg *config.AgentConfig, paths config.Paths) *ProvisioningReport {
	report := NewProvisioningReport(cfg.Name, "deprovision")

	// 1. Destroy container and persistent volume
	if err := runtime.DestroyAgentContainer(ctx, cfg.ContainerName, cfg.VolumeName); err != nil {
		report.AddStep("Container & Volume", StatusError, fmt.Sprintf("Failed to remove container/volume: %v", err), err)
	} else {
		report.AddStep("Container & Volume", StatusOK, fmt.Sprintf("Removed container (%s) and volume (%s)", cfg.ContainerName, cfg.VolumeName), nil)
	}

	// 2. Delete local config and secrets
	if err := config.DeleteAgentConfig(cfg.Name, paths); err != nil {
		report.AddStep("Local Configuration", StatusError, fmt.Sprintf("Failed to delete local configuration: %v", err), err)
	} else {
		report.AddStep("Local Configuration", StatusOK, "Purged local configuration and secrets", nil)
	}

	// 3. Clear per-agent known hosts
	runtime.ClearAgentKnownHosts(cfg.Name, paths)
	report.AddStep("Known Hosts", StatusOK, "Cleared agent SSH known hosts entry", nil)

	// 4. Deprovision Valkey ACL user & streams
	if err := DeprovisionValkeyUser(ctx, cfg.Name); err == nil {
		report.AddStep("Valkey Identity", StatusOK, "Removed Valkey ACL user and backplane identity", nil)
	} else if IsOfflineError(err) {
		report.AddStep("Valkey Identity", StatusSkipped, fmt.Sprintf("Valkey infrastructure is offline (skipped): %v", err), err)
	} else {
		report.AddStep("Valkey Identity", StatusError, fmt.Sprintf("Failed to remove Valkey identity: %v", err), err)
	}

	// 5. Deprovision Gitea user & keys
	if err := DeprovisionGiteaUser(ctx, cfg.Name); err == nil {
		report.AddStep("Gitea Account", StatusOK, "Purged Gitea user account and authorized keys", nil)
	} else if IsOfflineError(err) {
		report.AddStep("Gitea Account", StatusSkipped, fmt.Sprintf("Gitea infrastructure is offline (skipped): %v", err), err)
	} else {
		report.AddStep("Gitea Account", StatusError, fmt.Sprintf("Failed to purge Gitea user: %v", err), err)
	}

	// 6. Deprovision SonarQube user & analysis tokens
	if err := DeprovisionSonarUser(ctx, cfg.Name); err == nil {
		report.AddStep("SonarQube Account", StatusOK, "Purged SonarQube user account and analysis tokens", nil)
	} else if IsOfflineError(err) {
		report.AddStep("SonarQube Account", StatusSkipped, fmt.Sprintf("SonarQube infrastructure is offline (skipped): %v", err), err)
	} else {
		report.AddStep("SonarQube Account", StatusError, fmt.Sprintf("Failed to deprovision SonarQube user: %v", err), err)
	}

	// 7. Update SSH config file
	if err := runtime.SyncSSHConfigFile(ctx, paths); err != nil {
		report.AddStep("SSH Configuration", StatusWarning, fmt.Sprintf("Failed to synchronize SSH config: %v", err), err)
	} else {
		report.AddStep("SSH Configuration", StatusOK, "Synchronized host SSH config", nil)
	}

	return report
}
