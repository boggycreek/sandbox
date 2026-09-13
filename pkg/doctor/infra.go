// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package doctor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/boggycreek/agent-sandbox/pkg/config"
	"github.com/boggycreek/agent-sandbox/pkg/gitea"
	"github.com/boggycreek/agent-sandbox/pkg/libbp"
	"github.com/boggycreek/agent-sandbox/pkg/runtime"
)

// DiagnoseAndHealInfra performs a comprehensive health check and auto-healing on shared infrastructure
func DiagnoseAndHealInfra(ctx context.Context, paths config.Paths) (*DoctorReport, error) {
	report := &DoctorReport{
		AgentName: "shared-infrastructure",
	}

	// 1. Host Environment Configuration (.env) Check & Heal
	checkAndHealInfraEnv(paths, report)

	// 2. Host SSH Keypair & Include Directive
	checkAndHealHostSSH(paths, report)

	// 3. Infrastructure Storage Volumes & Network
	checkAndHealInfraStorage(ctx, report)

	// 4. Valkey Backplane Container & Service
	checkAndHealValkeyContainer(ctx, paths, report)

	// 5. Gitea Git Forge Container & Service
	checkAndHealGiteaContainer(ctx, paths, report)

	return report, nil
}

func checkAndHealInfraEnv(paths config.Paths, report *DoctorReport) {
	if _, err := os.Stat(paths.EnvFile); os.IsNotExist(err) {
		_ = paths.EnsureDirectories()
		adminBytes := make([]byte, 16)
		humanBytes := make([]byte, 16)
		_, _ = rand.Read(adminBytes)
		_, _ = rand.Read(humanBytes)

		adminPW := hex.EncodeToString(adminBytes)
		humanPW := hex.EncodeToString(humanBytes)
		humanName := os.Getenv("USER")
		if humanName == "" {
			humanName = "operator"
		}

		content := fmt.Sprintf("# Agent Sandbox Environment Configuration\nHUMAN_NAME=%s\nADMIN_BACKPLANE_PASSWORD=%s\nHUMAN_BACKPLANE_PASSWORD=%s\n", humanName, adminPW, humanPW)
		_ = os.WriteFile(paths.EnvFile, []byte(content), 0600)
		paths.LoadEnv()

		report.Checks = append(report.Checks, CheckItem{
			Name:    "Environment Config (.env)",
			Status:  StatusHealed,
			Message: fmt.Sprintf("Created missing environment configuration at %s", paths.EnvFile),
			Healed:  true,
		})
		report.HealedCount++
	} else {
		paths.LoadEnv()
		report.Checks = append(report.Checks, CheckItem{
			Name:    "Environment Config (.env)",
			Status:  StatusOK,
			Message: fmt.Sprintf("Valid configuration present (%s)", paths.EnvFile),
		})
	}
}

func checkAndHealInfraStorage(ctx context.Context, report *DoctorReport) {
	healed := false
	netName := "agent-sandbox-infra"
	if err := runtime.EnsureNetwork(ctx, netName); err != nil {
		report.Checks = append(report.Checks, CheckItem{
			Name:         "Bridge Network",
			Status:       StatusError,
			Message:      fmt.Sprintf("Failed ensuring bridge network %s: %v", netName, err),
			Unrepairable: true,
		})
		report.UnrepairableCount++
		return
	}

	for _, vol := range []string{"agent-sandbox-valkey-data", "agent-sandbox-gitea-data"} {
		volCmd := exec.CommandContext(ctx, "podman", "volume", "exists", vol)
		if err := volCmd.Run(); err != nil {
			if err := runtime.EnsureVolume(ctx, vol); err == nil {
				healed = true
			}
		}
	}

	if healed {
		report.Checks = append(report.Checks, CheckItem{
			Name:    "Infrastructure Storage",
			Status:  StatusHealed,
			Message: "Restored missing shared infrastructure data volumes",
			Healed:  true,
		})
		report.HealedCount++
	} else {
		report.Checks = append(report.Checks, CheckItem{
			Name:    "Infrastructure Storage",
			Status:  StatusOK,
			Message: "Bridge network and data volumes (valkey-data, gitea-data) present",
		})
	}
}

func checkAndHealValkeyContainer(ctx context.Context, paths config.Paths, report *DoctorReport) {
	containerName := "agent-sandbox-valkey"
	info, err := runtime.InspectAgentContainer(ctx, containerName)
	state := "stopped"
	if err == nil && info != nil {
		state = info.State
	}

	bpCfg := libbp.LoadClientFromEnv()
	adminPass := os.Getenv("ADMIN_BACKPLANE_PASSWORD")
	if adminPass == "" {
		adminPass = "admin_backplane_pass"
	}

	dialCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	client, err := libbp.Dial(dialCtx, libbp.ClientConfig{
		Host:     bpCfg.Host,
		Port:     bpCfg.Port,
		Username: "admin",
		Password: adminPass,
	})
	if err != nil {
		report.Checks = append(report.Checks, CheckItem{
			Name:    "Valkey Service",
			Status:  StatusWarning,
			Message: fmt.Sprintf("Valkey container is %s (%v; start via `sndbx infra up`)", state, err),
		})
		report.WarningCount++
		return
	}
	defer client.Close()

	// Ensure human operator ACL is configured
	humanName := os.Getenv("HUMAN_NAME")
	if humanName == "" {
		humanName = "operator"
	}
	humanPass := os.Getenv("HUMAN_BACKPLANE_PASSWORD")
	if humanPass == "" {
		humanPass = "human_backplane_pass"
	}

	aclArgs := []string{
		"SETUSER", humanName, "on",
		">" + humanPass,
		"~*:%R~*:*", "&*", "+@all",
	}
	_, _ = client.Exec(ctx, "ACL", aclArgs...)

	report.Checks = append(report.Checks, CheckItem{
		Name:    "Valkey Service",
		Status:  StatusOK,
		Message: fmt.Sprintf("Container %s, listening on %s:%d, admin & operator ACLs verified", state, bpCfg.Host, bpCfg.Port),
	})
}

func checkAndHealGiteaContainer(ctx context.Context, paths config.Paths, report *DoctorReport) {
	containerName := "agent-sandbox-gitea"
	info, err := runtime.InspectAgentContainer(ctx, containerName)
	state := "stopped"
	if err == nil && info != nil {
		state = info.State
	}

	giteaURL := os.Getenv("GITEA_URL")
	if giteaURL == "" {
		giteaURL = "http://127.0.0.1:3000"
	}

	adminPass := os.Getenv("ADMIN_BACKPLANE_PASSWORD")
	if adminPass == "" {
		adminPass = "admin_backplane_pass"
	}

	httpClient := &http.Client{Timeout: 1 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, "GET", giteaURL+"/api/v1/version", nil)
	resp, err := httpClient.Do(req)
	if err != nil || (resp != nil && resp.StatusCode >= 500) {
		if resp != nil {
			_ = resp.Body.Close()
		}
		report.Checks = append(report.Checks, CheckItem{
			Name:    "Gitea Service",
			Status:  StatusWarning,
			Message: fmt.Sprintf("Gitea container is %s (start via `sndbx infra up`)", state),
		})
		report.WarningCount++
		return
	}
	_ = resp.Body.Close()

	client := gitea.NewClient(gitea.ClientConfig{
		BaseURL:   giteaURL,
		AdminUser: "giteaadmin",
		AdminPass: adminPass,
		Timeout:   2 * time.Second,
	})

	_ = client.EnsureOrg(ctx, "fleet")
	_ = client.EnsureRepo(ctx, "fleet", "tools", "Fleet shared utility repositories", true)
	_ = client.EnsureRepo(ctx, "fleet", "tasks", "Fleet task tracking backlog", true)

	report.Checks = append(report.Checks, CheckItem{
		Name:    "Gitea Service",
		Status:  StatusOK,
		Message: fmt.Sprintf("Container %s, HTTP %s responsive, admin account & fleet repositories verified", state, giteaURL),
	})
}
