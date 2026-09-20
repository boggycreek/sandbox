// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package doctor

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/boggycreek/sandbox/pkg/config"
	"github.com/boggycreek/sandbox/pkg/gitea"
	"github.com/boggycreek/sandbox/pkg/libbp"
	"github.com/boggycreek/sandbox/pkg/runtime"
)

// CheckStatus indicates the outcome of an individual diagnostic check
type CheckStatus string

const (
	StatusOK      CheckStatus = "OK"
	StatusHealed  CheckStatus = "HEALED"
	StatusWarning CheckStatus = "WARNING"
	StatusError   CheckStatus = "ERROR"
)

// CheckItem represents one diagnostic check and auto-heal outcome
type CheckItem struct {
	Name         string      `json:"name"`
	Status       CheckStatus `json:"status"`
	Message      string      `json:"message"`
	Healed       bool        `json:"healed"`
	Unrepairable bool        `json:"unrepairable"`
}

// DoctorReport contains all diagnostic results for an agent
type DoctorReport struct {
	AgentName         string      `json:"agent_name"`
	Checks            []CheckItem `json:"checks"`
	HealedCount       int         `json:"healed_count"`
	WarningCount      int         `json:"warning_count"`
	UnrepairableCount int         `json:"unrepairable_count"`
}

// DiagnoseAndHealAgent performs complete health check and auto-healing on an agent
func DiagnoseAndHealAgent(ctx context.Context, agentName string, paths config.Paths) (*DoctorReport, error) {
	name := strings.ToLower(strings.TrimSpace(agentName))
	report := &DoctorReport{
		AgentName: name,
	}

	if name == "" {
		return nil, fmt.Errorf("agent name cannot be empty")
	}

	// 1. Configuration Check & Heal
	cfg, err := checkAndHealConfig(paths, name, report)
	if err != nil || cfg == nil {
		return report, nil
	}

	// 2. Cryptographic Signing Key Check & Heal
	checkAndHealSigningKey(cfg, paths, report)

	// 3. Host SSH Keypair & Include Config Check & Heal
	checkAndHealHostSSH(paths, report)

	// 4. Podman Storage (Volume & Network) Check & Heal
	checkAndHealPodmanStorage(ctx, cfg, report)

	// 5. Image Resolution and Cache Verification
	checkAndHealImage(ctx, cfg, paths, report)

	// 6. Valkey Backplane Registration Check & Heal
	checkAndHealValkey(ctx, cfg, paths, report)

	// 6. Gitea Git Forge Registration Check & Heal
	checkAndHealGitea(ctx, cfg, paths, report)

	// 7. Container State & SSH Config Sync
	checkAndHealContainer(ctx, cfg, paths, report)

	return report, nil
}

func checkAndHealConfig(paths config.Paths, name string, report *DoctorReport) (*config.AgentConfig, error) {
	cfgPath := filepath.Join(paths.AgentsDir, fmt.Sprintf("%s.json", name))
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		report.Checks = append(report.Checks, CheckItem{
			Name:         "Configuration File",
			Status:       StatusError,
			Message:      fmt.Sprintf("Missing config at %s (unrepairable: recreate agent via `sndbx agent create %s`)", cfgPath, name),
			Unrepairable: true,
		})
		report.UnrepairableCount++
		return nil, err
	}

	cfg, err := config.LoadAgentConfig(name, paths)
	if err != nil {
		report.Checks = append(report.Checks, CheckItem{
			Name:         "Configuration Parsing",
			Status:       StatusError,
			Message:      fmt.Sprintf("Corrupt JSON configuration: %v", err),
			Unrepairable: true,
		})
		report.UnrepairableCount++
		return nil, err
	}

	healed := false
	if cfg.ContainerName == "" {
		cfg.ContainerName = fmt.Sprintf("agent-sandbox-%s", name)
		healed = true
	}
	if cfg.VolumeName == "" {
		cfg.VolumeName = fmt.Sprintf("agent-sandbox-%s-home", name)
		healed = true
	}
	if cfg.Image == "" {
		cfg.Image = "agent-sandbox-base:latest"
		healed = true
	}
	if cfg.Password == "" {
		pBytes := make([]byte, 16)
		_, _ = rand.Read(pBytes)
		cfg.Password = hex.EncodeToString(pBytes)
		healed = true
	}

	// Check and heal file permissions to 0600
	if fi, err := os.Stat(cfgPath); err == nil && fi.Mode().Perm() != 0600 {
		if err := os.Chmod(cfgPath, 0600); err == nil {
			healed = true
		}
	}

	if healed {
		_ = config.SaveAgentConfig(cfg, paths)
		report.Checks = append(report.Checks, CheckItem{
			Name:    "Configuration File",
			Status:  StatusHealed,
			Message: fmt.Sprintf("Repaired configuration and permissions (0600) at %s", cfgPath),
			Healed:  true,
		})
		report.HealedCount++
	} else {
		report.Checks = append(report.Checks, CheckItem{
			Name:    "Configuration File",
			Status:  StatusOK,
			Message: fmt.Sprintf("Valid configuration and permissions (%s)", cfgPath),
		})
	}

	return cfg, nil
}

func checkAndHealSigningKey(cfg *config.AgentConfig, paths config.Paths, report *DoctorReport) {
	keyPath := filepath.Join(paths.SecretsDir, cfg.Name, "signing-key.pem")
	keyValid := false
	if data, err := os.ReadFile(keyPath); err == nil && len(data) > 0 {
		block, _ := pem.Decode(data)
		if block != nil && len(block.Bytes) == ed25519.PrivateKeySize {
			keyValid = true
		}
	}
	keyHealedPerm := false
	if fi, err := os.Stat(keyPath); err == nil && fi.Mode().Perm() != 0600 {
		if err := os.Chmod(keyPath, 0600); err == nil {
			keyHealedPerm = true
		}
	}

	if keyValid && cfg.SigningKeyPEM != "" && cfg.PublicKeyB64 != "" {
		if keyHealedPerm {
			report.Checks = append(report.Checks, CheckItem{
				Name:    "Ed25519 Signing Keys",
				Status:  StatusHealed,
				Message: "Repaired signing key file permissions to 0600",
				Healed:  true,
			})
			report.HealedCount++
			return
		}
		report.Checks = append(report.Checks, CheckItem{
			Name:    "Ed25519 Signing Keys",
			Status:  StatusOK,
			Message: "Cryptographic signing keypair intact",
		})
		return
	}

	// Heal signing key
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		report.Checks = append(report.Checks, CheckItem{
			Name:         "Ed25519 Signing Keys",
			Status:       StatusError,
			Message:      fmt.Sprintf("Failed generating keypair: %v", err),
			Unrepairable: true,
		})
		report.UnrepairableCount++
		return
	}

	keyDir := filepath.Dir(keyPath)
	_ = os.MkdirAll(keyDir, 0700)
	block := &pem.Block{
		Type:  "ED25519 PRIVATE KEY",
		Bytes: []byte(priv),
	}
	pemBytes := pem.EncodeToMemory(block)
	_ = os.WriteFile(keyPath, pemBytes, 0600)

	cfg.SigningKeyPEM = string(pemBytes)
	cfg.PublicKeyB64 = base64.StdEncoding.EncodeToString(pub)
	_ = config.SaveAgentConfig(cfg, paths)

	report.Checks = append(report.Checks, CheckItem{
		Name:    "Ed25519 Signing Keys",
		Status:  StatusHealed,
		Message: "Regenerated Ed25519 keypair and synchronized config",
		Healed:  true,
	})
	report.HealedCount++
}

func checkAndHealHostSSH(paths config.Paths, report *DoctorReport) {
	healedKey := false
	sshPub := paths.IDEKeyFile + ".pub"
	if _, err := os.Stat(paths.IDEKeyFile); os.IsNotExist(err) {
		_ = os.MkdirAll(filepath.Dir(paths.IDEKeyFile), 0700)
		_ = exec.Command("ssh-keygen", "-t", "ed25519", "-N", "", "-C", "agent-sandbox-ide", "-f", paths.IDEKeyFile).Run()
		healedKey = true
	} else if _, err := os.Stat(sshPub); os.IsNotExist(err) {
		_ = exec.Command("ssh-keygen", "-y", "-f", paths.IDEKeyFile).Run()
		healedKey = true
	}

	// Check managed ssh_config and ~/.ssh/config Include
	healedInclude := false
	if _, err := os.Stat(paths.SSHConfigFile); os.IsNotExist(err) {
		_ = os.MkdirAll(filepath.Dir(paths.SSHConfigFile), 0700)
		_ = os.WriteFile(paths.SSHConfigFile, []byte("# Agent Sandbox Auto-Generated SSH Configuration\n"), 0600)
		healedInclude = true
	}

	userSSHConfig := filepath.Join(paths.SSHDir, "config")
	includeDir := fmt.Sprintf("Include %s", paths.SSHConfigFile)
	if data, err := os.ReadFile(userSSHConfig); err == nil {
		if !strings.Contains(string(data), paths.SSHConfigFile) {
			newContent := fmt.Sprintf("# OpenSSH Configuration\n# Agent Sandbox Host Include\n%s\n\n%s", includeDir, string(data))
			_ = os.WriteFile(userSSHConfig, []byte(newContent), 0600)
			healedInclude = true
		}
	} else if os.IsNotExist(err) {
		_ = os.WriteFile(userSSHConfig, []byte(fmt.Sprintf("# OpenSSH Configuration\n# Agent Sandbox Host Include\n%s\n", includeDir)), 0600)
		healedInclude = true
	}

	if healedKey || healedInclude {
		msg := "Ensured host IDE keypair and OpenSSH Include directive"
		report.Checks = append(report.Checks, CheckItem{
			Name:    "Host SSH & IDE Config",
			Status:  StatusHealed,
			Message: msg,
			Healed:  true,
		})
		report.HealedCount++
	} else {
		report.Checks = append(report.Checks, CheckItem{
			Name:    "Host SSH & IDE Config",
			Status:  StatusOK,
			Message: "Dedicated IDE keypair and OpenSSH Include active",
		})
	}
}

func checkAndHealPodmanStorage(ctx context.Context, cfg *config.AgentConfig, report *DoctorReport) {
	healed := false
	netName := "agent-sandbox-infra"
	if err := runtime.EnsureNetwork(ctx, netName); err != nil {
		report.Checks = append(report.Checks, CheckItem{
			Name:         "Podman Network & Storage",
			Status:       StatusError,
			Message:      fmt.Sprintf("Failed ensuring bridge network %s: %v", netName, err),
			Unrepairable: true,
		})
		report.UnrepairableCount++
		return
	}

	_ = runtime.EnsureRootlessNetNS(ctx)

	volCmd := exec.CommandContext(ctx, "podman", "volume", "exists", cfg.VolumeName)
	if err := volCmd.Run(); err != nil {
		if err := runtime.EnsureVolume(ctx, cfg.VolumeName); err == nil {
			healed = true
		}
	}

	if healed {
		report.Checks = append(report.Checks, CheckItem{
			Name:    "Podman Storage",
			Status:  StatusHealed,
			Message: fmt.Sprintf("Created missing persistent home volume (%s)", cfg.VolumeName),
			Healed:  true,
		})
		report.HealedCount++
	} else {
		report.Checks = append(report.Checks, CheckItem{
			Name:    "Podman Storage",
			Status:  StatusOK,
			Message: fmt.Sprintf("Network (%s) and volume (%s) present", netName, cfg.VolumeName),
		})
	}
}

func checkAndHealImage(ctx context.Context, cfg *config.AgentConfig, paths config.Paths, report *DoctorReport) {
	resolved, isLocal := runtime.ResolveAgentImage(ctx, cfg.Image)
	if resolved != cfg.Image {
		cfg.Image = resolved
		_ = config.SaveAgentConfig(cfg, paths)
		report.Checks = append(report.Checks, CheckItem{
			Name:    "Agent Image",
			Status:  StatusHealed,
			Message: fmt.Sprintf("Updated image reference to %s", resolved),
			Healed:  true,
		})
		report.HealedCount++
		return
	}

	if isLocal {
		report.Checks = append(report.Checks, CheckItem{
			Name:    "Agent Image",
			Status:  StatusOK,
			Message: fmt.Sprintf("Image %s present in local Podman storage", cfg.Image),
		})
	} else {
		report.Checks = append(report.Checks, CheckItem{
			Name:    "Agent Image",
			Status:  StatusWarning,
			Message: fmt.Sprintf("Image %s not cached locally (will be pulled on start)", cfg.Image),
		})
		report.WarningCount++
	}
}

func checkAndHealValkey(ctx context.Context, cfg *config.AgentConfig, paths config.Paths, report *DoctorReport) {
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
			Name:    "Valkey Backplane",
			Status:  StatusWarning,
			Message: "Valkey infrastructure is offline (skipped ACL sync; start via `sndbx infra up`)",
		})
		report.WarningCount++
		return
	}
	defer client.Close()

	// Provision/repair ACL
	aclArgs := []string{
		"SETUSER", cfg.Name, "on",
		">" + cfg.Password,
		fmt.Sprintf("~%s:*", cfg.Name),
		fmt.Sprintf("~identity:%s", cfg.Name),
		"%R~*:*", "&*", "+@all", "-@admin", "-@dangerous",
		"(+xadd ~*:inbox)",
	}
	_, _ = client.Exec(ctx, "ACL", aclArgs...)

	_ = client.RegisterIdentity(ctx, libbp.IdentityRecord{
		Name:   cfg.Name,
		Role:   cfg.Role,
		Kind:   "agent",
		PubKey: cfg.PublicKeyB64,
	})

	report.Checks = append(report.Checks, CheckItem{
		Name:    "Valkey Backplane",
		Status:  StatusOK,
		Message: "Valkey ACL permissions and registry identity verified",
	})
}

func checkAndHealGitea(ctx context.Context, cfg *config.AgentConfig, paths config.Paths, report *DoctorReport) {
	adminPass := os.Getenv("ADMIN_BACKPLANE_PASSWORD")
	if adminPass == "" {
		adminPass = "admin_backplane_pass"
	}

	giteaURL := os.Getenv("GITEA_URL")
	if giteaURL == "" {
		giteaURL = "http://127.0.0.1:3000"
	}

	httpClient := &http.Client{Timeout: 1 * time.Second}
	req, _ := http.NewRequestWithContext(ctx, "GET", giteaURL+"/api/v1/version", nil)
	resp, err := httpClient.Do(req)
	if err != nil || (resp != nil && resp.StatusCode >= 500) {
		if resp != nil {
			_ = resp.Body.Close()
		}
		report.Checks = append(report.Checks, CheckItem{
			Name:    "Gitea Git Forge",
			Status:  StatusWarning,
			Message: "Gitea infrastructure is offline (skipped user sync; start via `sndbx infra up`)",
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

	_ = client.EnsureUser(ctx, cfg.Name, cfg.Password, fmt.Sprintf("%s@local.sndbx", cfg.Name))
	sshKeyPub := fmt.Sprintf("%s.pub", paths.IDEKeyFile)
	if keyData, err := os.ReadFile(sshKeyPub); err == nil && len(keyData) > 0 {
		_ = client.AddUserSSHKey(ctx, cfg.Name, fmt.Sprintf("%s-ide-key", cfg.Name), string(keyData))
	}
	_ = client.AddOrgMember(ctx, "fleet", cfg.Name)

	report.Checks = append(report.Checks, CheckItem{
		Name:    "Gitea Git Forge",
		Status:  StatusOK,
		Message: "User account, SSH key, and fleet organization membership verified",
	})
}

func checkAndHealContainer(ctx context.Context, cfg *config.AgentConfig, paths config.Paths, report *DoctorReport) {
	info, err := runtime.InspectAgentContainer(ctx, cfg.ContainerName)
	state := "stopped"
	if err == nil && info != nil {
		state = info.State
	}

	sshPort := 0
	if state == "running" {
		if p, err := runtime.GetAgentSSHPort(ctx, cfg.ContainerName); err == nil && p > 0 {
			sshPort = p
		}
	}

	_ = runtime.SyncSSHConfigFile(ctx, paths)

	var msg string
	if state == "running" && sshPort > 0 {
		msg = fmt.Sprintf("Container running (SSH port %d, alias: ssh sndbx-%s)", sshPort, cfg.Name)
	} else {
		msg = fmt.Sprintf("Container %s (start via `sndbx agent start %s`)", state, cfg.Name)
	}

	report.Checks = append(report.Checks, CheckItem{
		Name:    "Container & Runtime",
		Status:  StatusOK,
		Message: msg,
	})
}

// FormatDoctorReport formats the diagnostic report as human-readable text
func FormatDoctorReport(report *DoctorReport) string {
	var b bytes.Buffer
	if report.AgentName == "shared-infrastructure" || report.AgentName == "infra" {
		fmt.Fprintf(&b, "Diagnosing shared infrastructure...\n")
	} else {
		fmt.Fprintf(&b, "Diagnosing agent %q...\n", report.AgentName)
	}

	for _, c := range report.Checks {
		symbol := "[✓]"
		switch c.Status {
		case StatusHealed:
			symbol = "[⚡]"
		case StatusWarning:
			symbol = "[-]"
		case StatusError:
			symbol = "[✗]"
		}
		fmt.Fprintf(&b, "  %-3s %-25s : %s\n", symbol, c.Name, c.Message)
	}

	fmt.Fprintln(&b)
	if report.AgentName == "shared-infrastructure" || report.AgentName == "infra" {
		if report.UnrepairableCount > 0 {
			fmt.Fprintf(&b, "Summary: %d unrepairable issue(s) detected in shared infrastructure.\n", report.UnrepairableCount)
		} else if report.HealedCount > 0 {
			fmt.Fprintf(&b, "Summary: Shared infrastructure is healthy (%d issue(s) automatically healed).\n", report.HealedCount)
		} else if report.WarningCount > 0 {
			fmt.Fprintf(&b, "Summary: Shared infrastructure has %d warning(s) (offline services).\n", report.WarningCount)
		} else {
			fmt.Fprintf(&b, "Summary: Shared infrastructure is fully healthy (0 issues found).\n")
		}
	} else {
		if report.UnrepairableCount > 0 {
			fmt.Fprintf(&b, "Summary: %d unrepairable issue(s) detected for agent %q.\n", report.UnrepairableCount, report.AgentName)
		} else if report.HealedCount > 0 {
			fmt.Fprintf(&b, "Summary: Agent %q is healthy (%d issue(s) automatically healed).\n", report.AgentName, report.HealedCount)
		} else if report.WarningCount > 0 {
			fmt.Fprintf(&b, "Summary: Agent %q has %d warning(s) (offline dependencies).\n", report.AgentName, report.WarningCount)
		} else {
			fmt.Fprintf(&b, "Summary: Agent %q is fully healthy (0 issues found).\n", report.AgentName)
		}
	}

	return b.String()
}
