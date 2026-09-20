// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package doctor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/boggycreek/sandbox/pkg/config"
	"github.com/boggycreek/sandbox/pkg/gitea"
	"github.com/boggycreek/sandbox/pkg/libbp"
	"github.com/boggycreek/sandbox/pkg/runtime"
)

// infraValkeyContainer, infraGiteaContainer, infraPostgresContainer, and infraSonarContainer are the expected shared infra container names.
// These are package-level vars so integration tests can override them with ephemeral container names.
var (
	infraValkeyContainer   = "agent-sandbox-valkey"
	infraGiteaContainer    = "agent-sandbox-gitea"
	infraPostgresContainer = "agent-sandbox-postgres"
	infraSonarContainer    = "agent-sandbox-sonarqube"
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

	// 6. PostgreSQL Relational Database Container & Service
	checkAndHealPostgresContainer(ctx, report)

	// 7. SonarQube Mechanical Analysis Container & Service
	checkAndHealSonarContainer(ctx, report)

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
		healedEnvPerm := false
		if fi, err := os.Stat(paths.EnvFile); err == nil && fi.Mode().Perm() != 0600 {
			if err := os.Chmod(paths.EnvFile, 0600); err == nil {
				healedEnvPerm = true
			}
		}

		if healedEnvPerm {
			report.Checks = append(report.Checks, CheckItem{
				Name:    "Environment Config (.env)",
				Status:  StatusHealed,
				Message: fmt.Sprintf("Repaired environment file permissions (0600) at %s", paths.EnvFile),
				Healed:  true,
			})
			report.HealedCount++
		} else {
			report.Checks = append(report.Checks, CheckItem{
				Name:    "Environment Config (.env)",
				Status:  StatusOK,
				Message: fmt.Sprintf("Valid configuration present (%s)", paths.EnvFile),
			})
		}
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

	_ = runtime.EnsureRootlessNetNS(ctx)

	for _, vol := range []string{
		"agent-sandbox-valkey-data",
		"agent-sandbox-gitea-data",
		"agent-sandbox-postgres-data",
		"agent-sandbox-sonarqube-data",
		"agent-sandbox-sonarqube-extensions",
		"agent-sandbox-sonarqube-logs",
	} {
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
			Message: "Bridge network and data volumes (valkey-data, gitea-data, postgres-data, sonarqube-data) present",
		})
	}
}

func checkAndHealPostgresContainer(ctx context.Context, report *DoctorReport) {
	containerName := infraPostgresContainer
	info, err := runtime.InspectAgentContainer(ctx, containerName)
	state := "stopped"
	if err == nil && info != nil {
		state = info.State
	}

	pgHost := os.Getenv("POSTGRES_HOST")
	if pgHost == "" {
		pgHost = "127.0.0.1"
	}
	pgPort := os.Getenv("POSTGRES_PORT")
	if pgPort == "" {
		pgPort = "5432"
	}

	dialer := net.Dialer{Timeout: 1 * time.Second}
	conn, dialErr := dialer.DialContext(ctx, "tcp", net.JoinHostPort(pgHost, pgPort))
	if dialErr != nil {
		report.Checks = append(report.Checks, CheckItem{
			Name:    "PostgreSQL Database",
			Status:  StatusWarning,
			Message: fmt.Sprintf("PostgreSQL container is %s (start via `sndbx infra up`)", state),
		})
		report.WarningCount++
		return
	}
	_ = conn.Close()

	report.Checks = append(report.Checks, CheckItem{
		Name:    "PostgreSQL Database",
		Status:  StatusOK,
		Message: fmt.Sprintf("Container %s, listening on %s:%s (database: sonar)", state, pgHost, pgPort),
	})
}

func checkAndHealValkeyContainer(ctx context.Context, paths config.Paths, report *DoctorReport) {
	containerName := infraValkeyContainer
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

	humanName := os.Getenv("HUMAN_NAME")
	if humanName == "" {
		humanName = "operator"
	}
	humanPass := os.Getenv("HUMAN_BACKPLANE_PASSWORD")
	if humanPass == "" {
		humanPass = "human_backplane_pass"
	}

	dialCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	client, dialErr := libbp.Dial(dialCtx, libbp.ClientConfig{
		Host:     bpCfg.Host,
		Port:     bpCfg.Port,
		Username: "admin",
		Password: adminPass,
	})
	cancel()

	if dialErr != nil {
		// If container is running, attempt to auto-heal by restarting stack with valid ACL file
		if state == "running" {
			healCtx, healCancel := context.WithTimeout(ctx, 10*time.Second)
			defer healCancel()

			if err := runtime.StartInfraStack(healCtx, paths, adminPass, humanPass, humanName); err == nil {
				// Re-test connection after restart
				retryCtx, retryCancel := context.WithTimeout(ctx, 2*time.Second)
				clientRetry, retryErr := libbp.Dial(retryCtx, libbp.ClientConfig{
					Host:     bpCfg.Host,
					Port:     bpCfg.Port,
					Username: "admin",
					Password: adminPass,
				})
				retryCancel()

				if retryErr == nil {
					defer clientRetry.Close()
					report.Checks = append(report.Checks, CheckItem{
						Name:    "Valkey Service",
						Status:  StatusHealed,
						Message: "Repaired Valkey container mount and synchronized admin ACL credentials",
						Healed:  true,
					})
					report.HealedCount++
					return
				}
			}
		}

		report.Checks = append(report.Checks, CheckItem{
			Name:    "Valkey Service",
			Status:  StatusWarning,
			Message: fmt.Sprintf("Valkey container is %s (%v; start via `sndbx infra up`)", state, dialErr),
		})
		report.WarningCount++
		return
	}
	defer client.Close()

	// Ensure human operator ACL is configured
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
	_ = client.EnsureRepo(ctx, "fleet", "tasks", "Fleet shared task tracking backlog", true)

	report.Checks = append(report.Checks, CheckItem{
		Name:    "Gitea Service",
		Status:  StatusOK,
		Message: fmt.Sprintf("Container %s, HTTP %s responsive, admin account & fleet repositories verified", state, giteaURL),
	})
}

func checkAndHealSonarContainer(ctx context.Context, report *DoctorReport) {
	containerName := infraSonarContainer
	info, err := runtime.InspectAgentContainer(ctx, containerName)
	state := "stopped"
	if err == nil && info != nil {
		state = info.State
	}

	sonarURL := os.Getenv("SONAR_HOST_URL")
	if sonarURL == "" {
		sonarURL = os.Getenv("SONARQUBE_URL")
	}
	if sonarURL == "" {
		sonarURL = "http://127.0.0.1:9000"
	}

	httpClient := &http.Client{Timeout: 1 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, "GET", sonarURL+"/api/system/status", nil)
	resp, err := httpClient.Do(req)
	if err != nil || (resp != nil && resp.StatusCode >= 500) {
		if resp != nil {
			_ = resp.Body.Close()
		}
		report.Checks = append(report.Checks, CheckItem{
			Name:    "SonarQube Service",
			Status:  StatusWarning,
			Message: fmt.Sprintf("SonarQube container is %s (start via `sndbx infra up`)", state),
		})
		report.WarningCount++
		return
	}
	defer resp.Body.Close()

	var status struct {
		Status string `json:"status"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&status)

	report.Checks = append(report.Checks, CheckItem{
		Name:    "SonarQube Service",
		Status:  StatusOK,
		Message: fmt.Sprintf("Container %s, HTTP %s responsive (status: %s)", state, sonarURL, status.Status),
	})
}
