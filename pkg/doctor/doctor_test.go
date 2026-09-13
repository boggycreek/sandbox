// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package doctor

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boggycreek/agent-sandbox/pkg/config"
	"github.com/boggycreek/agent-sandbox/test/harness"
)

func TestDoctorDiagnosticsAndHealing(t *testing.T) {
	valkey := harness.StartValkeyHarness(t)
	defer valkey.Teardown()

	tmpDir := t.TempDir()
	t.Cleanup(func() {
		_ = exec.Command("podman", "unshare", "rm", "-rf", tmpDir).Run()
	})

	os.Setenv("ADMIN_BACKPLANE_PASSWORD", valkey.AdminPass)
	os.Setenv("BP_HOST", "127.0.0.1")
	os.Setenv("BP_PORT", fmt.Sprintf("%d", valkey.Port))

	paths := config.Paths{
		DataHome:      filepath.Join(tmpDir, "data"),
		AgentsDir:     filepath.Join(tmpDir, "data", "agents"),
		SecretsDir:    filepath.Join(tmpDir, "data", "secrets"),
		BinDir:        filepath.Join(tmpDir, "bin"),
		SSHDir:        filepath.Join(tmpDir, "ssh"),
		IDEKeyFile:    filepath.Join(tmpDir, "ssh", "agent-sandbox"),
		SSHConfigFile: filepath.Join(tmpDir, "data", "ssh_config"),
	}
	_ = paths.EnsureDirectories()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. Empty agent name
	if _, err := DiagnoseAndHealAgent(ctx, "", paths); err == nil {
		t.Errorf("expected error on empty agent name")
	}

	// 2. Nonexistent agent
	report, err := DiagnoseAndHealAgent(ctx, "nonexistent", paths)
	if err != nil {
		t.Errorf("unexpected error on diagnosing nonexistent agent: %v", err)
	}
	if report.UnrepairableCount == 0 {
		t.Errorf("expected unrepairable issues for nonexistent agent")
	}
	formatted := FormatDoctorReport(report)
	if !strings.Contains(formatted, "unrepairable") {
		t.Errorf("formatted report unexpected: %s", formatted)
	}

	// 3. Corrupted agent JSON
	corrupted := filepath.Join(paths.AgentsDir, "corrupted.json")
	_ = os.WriteFile(corrupted, []byte("{broken-json"), 0600)
	report, _ = DiagnoseAndHealAgent(ctx, "corrupted", paths)
	if report.UnrepairableCount == 0 {
		t.Errorf("expected unrepairable issue for corrupted JSON")
	}

	// 4. Create an agent config with missing fields & missing signing key (to test auto-healing)
	cfg := &config.AgentConfig{
		Name: "heal-agent",
	}
	_ = config.SaveAgentConfig(cfg, paths)
	// Delete signing key to trigger regeneration
	_ = os.Remove(filepath.Join(paths.SecretsDir, cfg.Name, "signing-key.pem"))

	report, err = DiagnoseAndHealAgent(ctx, "heal-agent", paths)
	if err != nil {
		t.Fatalf("DiagnoseAndHealAgent failed on heal-agent: %v", err)
	}
	if report.HealedCount == 0 {
		t.Errorf("expected auto-healed issues for heal-agent")
	}
	outText := FormatDoctorReport(report)
	if !strings.Contains(outText, "heal-agent") {
		t.Errorf("unexpected doctor output: %s", outText)
	}

	// 5. Run doctor again on healthy agent
	healthyReport, err := DiagnoseAndHealAgent(ctx, "heal-agent", paths)
	if err != nil {
		t.Fatalf("DiagnoseAndHealAgent failed on second pass: %v", err)
	}
	if healthyReport.UnrepairableCount > 0 {
		t.Errorf("expected 0 unrepairable issues on healed agent")
	}
	healthyOut := FormatDoctorReport(healthyReport)
	if !strings.Contains(healthyOut, "healthy") {
		t.Errorf("expected healthy status on second pass: %s", healthyOut)
	}

	// 6. Test offline Valkey check branch
	os.Setenv("BP_PORT", "65500") // Unreachable port
	offlineReport, _ := DiagnoseAndHealAgent(ctx, "heal-agent", paths)
	if offlineReport.WarningCount == 0 {
		t.Errorf("expected warning count > 0 when Valkey is offline")
	}
	offlineOut := FormatDoctorReport(offlineReport)
	if !strings.Contains(offlineOut, "warning") {
		t.Errorf("expected warning in summary: %s", offlineOut)
	}

	// 7. Test SSH key healing (delete IDE key)
	_ = os.Remove(paths.IDEKeyFile)
	_ = os.Remove(paths.IDEKeyFile + ".pub")
	userSSHConfig := filepath.Join(paths.SSHDir, "config")
	_ = os.WriteFile(userSSHConfig, []byte("Host existing\n  HostName example.com\n"), 0600)
	_ = os.Remove(paths.SSHConfigFile)

	sshHealReport, _ := DiagnoseAndHealAgent(ctx, "heal-agent", paths)
	if sshHealReport.HealedCount == 0 {
		t.Errorf("expected healed count > 0 for SSH key and include healing")
	}

	// 8. Test FormatDoctorReport with all status types
	customReport := &DoctorReport{
		AgentName: "status-agent",
		Checks: []CheckItem{
			{Name: "Check 1", Status: StatusOK, Message: "All good"},
			{Name: "Check 2", Status: StatusHealed, Message: "Fixed issue", Healed: true},
			{Name: "Check 3", Status: StatusWarning, Message: "Warning item"},
			{Name: "Check 4", Status: StatusError, Message: "Fatal error", Unrepairable: true},
		},
		HealedCount:       1,
		WarningCount:      1,
		UnrepairableCount: 1,
	}
	formattedCustom := FormatDoctorReport(customReport)
	if !strings.Contains(formattedCustom, "[✓]") || !strings.Contains(formattedCustom, "[⚡]") || !strings.Contains(formattedCustom, "[-]") || !strings.Contains(formattedCustom, "[✗]") {
		t.Errorf("FormatDoctorReport missed status symbols: %s", formattedCustom)
	}

	// 9. Test Gitea online check branch with mock server
	giteaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/version":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"version":"1.22.0"}`))
		case "/api/v1/users/heal-agent":
			w.WriteHeader(http.StatusNotFound)
		case "/api/v1/admin/users":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"username":"heal-agent"}`))
		default:
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		}
	}))
	defer giteaServer.Close()

	os.Setenv("GITEA_URL", giteaServer.URL)
	_ = os.WriteFile(paths.IDEKeyFile+".pub", []byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA mock-ide-key"), 0644)
	reportGitea := &DoctorReport{AgentName: "heal-agent"}
	checkAndHealGitea(ctx, cfg, paths, reportGitea)
	if len(reportGitea.Checks) == 0 || reportGitea.Checks[0].Status != StatusOK {
		t.Errorf("expected Gitea check OK, got %+v", reportGitea.Checks)
	}

	// 10. Direct storage, container, and signing key tests
	reportDirect := &DoctorReport{AgentName: "heal-agent"}
	checkAndHealPodmanStorage(ctx, cfg, reportDirect)
	checkAndHealContainer(ctx, cfg, paths, reportDirect)
	if len(reportDirect.Checks) < 2 {
		t.Errorf("expected storage and container checks in report")
	}

	// 11. Signing key check when key file is corrupt
	keyPath := filepath.Join(paths.SecretsDir, cfg.Name, "signing-key.pem")
	_ = os.WriteFile(keyPath, []byte("bad-pem-data"), 0600)
	cfg.SigningKeyPEM = ""
	reportKey := &DoctorReport{AgentName: "heal-agent"}
	checkAndHealSigningKey(cfg, paths, reportKey)
	if reportKey.HealedCount == 0 {
		t.Errorf("expected key to be healed when corrupt")
	}

	// 12. Gitea server 500 error branch
	giteaErrServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer giteaErrServer.Close()
	os.Setenv("GITEA_URL", giteaErrServer.URL)
	reportErr := &DoctorReport{AgentName: "heal-agent"}
	checkAndHealGitea(ctx, cfg, paths, reportErr)
	if reportErr.WarningCount == 0 {
		t.Errorf("expected warning for 500 server error in Gitea check")
	}
}


