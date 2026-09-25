// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package doctor

import (
	"context"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boggycreek/sandbox/pkg/config"
	"github.com/boggycreek/sandbox/pkg/libbp"
	"github.com/boggycreek/sandbox/pkg/runtime"
	"github.com/boggycreek/sandbox/test/harness"
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

	// 4. Create an agent config with missing fields & bad key permissions (to test auto-healing)
	cfg, err := config.NewAgentConfig("heal-agent", "base", "coder")
	if err != nil {
		t.Fatalf("failed creating agent config: %v", err)
	}
	cfg.Password = ""
	_ = config.SaveAgentConfig(cfg, paths)
	// Set signing key permissions to 0644 to test permission auto-healing
	_ = os.Chmod(filepath.Join(paths.SecretsDir, cfg.Name, "signing-key.pem"), 0644)

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
	if !strings.Contains(healthyOut, "heal-agent") {
		t.Errorf("expected heal-agent in summary: %s", healthyOut)
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

	// 10. Direct storage, container, host SSH pubkey heal, and signing key tests
	_ = os.Remove(paths.IDEKeyFile + ".pub")
	reportSSHPub := &DoctorReport{AgentName: "heal-agent"}
	checkAndHealHostSSH(paths, reportSSHPub)

	_ = exec.Command("podman", "volume", "rm", "-f", cfg.VolumeName).Run()
	reportDirect := &DoctorReport{AgentName: "heal-agent"}
	checkAndHealPodmanStorage(ctx, cfg, reportDirect)
	// Second run when volume already exists (healed == false branch)
	checkAndHealPodmanStorage(ctx, cfg, reportDirect)
	checkAndHealImage(ctx, cfg, paths, reportDirect)
	// Test image update heal when untagged
	cfgUntagged := &config.AgentConfig{Name: "test-untagged", Image: "docker.io/library/alpine"}
	checkAndHealImage(ctx, cfgUntagged, paths, reportDirect)
	checkAndHealContainer(ctx, cfg, paths, reportDirect)
	if len(reportDirect.Checks) < 4 {
		t.Errorf("expected storage, image, and container checks in report")
	}

	// 11. Signing key check when key file is corrupt
	keyPath := filepath.Join(paths.SecretsDir, cfg.Name, "signing-key.pem")
	_ = os.WriteFile(keyPath, []byte("bad-pem-data"), 0600)
	cfg.SigningKeyPEM = ""
	reportKey := &DoctorReport{AgentName: "heal-agent"}
	checkAndHealSigningKey(cfg, paths, reportKey)
	if reportKey.UnrepairableCount == 0 || len(reportKey.Checks) == 0 || reportKey.Checks[0].Status != StatusError || !reportKey.Checks[0].Unrepairable {
		t.Errorf("expected corrupt key to be reported as unrepairable error")
	}
	// Verify corrupt key was NOT overwritten
	content, _ := os.ReadFile(keyPath)
	if string(content) != "bad-pem-data" {
		t.Errorf("expected corrupt key file to be preserved without overwrite, got: %s", string(content))
	}

	// Test missing signing key
	_ = os.Remove(keyPath)
	reportMissing := &DoctorReport{AgentName: "heal-agent"}
	checkAndHealSigningKey(cfg, paths, reportMissing)
	if reportMissing.UnrepairableCount == 0 || len(reportMissing.Checks) == 0 || reportMissing.Checks[0].Status != StatusError || !reportMissing.Checks[0].Unrepairable {
		t.Errorf("expected missing key to be reported as unrepairable error")
	}

	// Test empty signing key
	_ = os.WriteFile(keyPath, []byte(""), 0600)
	reportEmpty := &DoctorReport{AgentName: "heal-agent"}
	checkAndHealSigningKey(cfg, paths, reportEmpty)
	if reportEmpty.UnrepairableCount == 0 || len(reportEmpty.Checks) == 0 || reportEmpty.Checks[0].Status != StatusError || !reportEmpty.Checks[0].Unrepairable {
		t.Errorf("expected empty key to be reported as unrepairable error")
	}

	// Restore valid signing key
	priv, pub, err := libbp.GenerateKeypair()
	if err != nil {
		t.Fatalf("failed generating keypair: %v", err)
	}
	pemStr, _ := libbp.EncodePrivateKeyPEM(priv)
	_ = os.WriteFile(keyPath, []byte(pemStr), 0600)
	cfg.SigningKeyPEM = pemStr
	cfg.PublicKeyB64 = libbp.EncodePublicKeyBase64(pub)

	// Run again now that key is valid
	reportKeyIntact := &DoctorReport{AgentName: "heal-agent"}
	checkAndHealSigningKey(cfg, paths, reportKeyIntact)
	if len(reportKeyIntact.Checks) == 0 || reportKeyIntact.Checks[0].Status != StatusOK {
		t.Errorf("expected intact signing key to return StatusOK")
	}

	// Test healing of permissions when key permissions are 0644
	_ = os.Chmod(keyPath, 0644)
	reportKeyPerm := &DoctorReport{AgentName: "heal-agent"}
	checkAndHealSigningKey(cfg, paths, reportKeyPerm)
	if reportKeyPerm.HealedCount == 0 || reportKeyPerm.Checks[0].Status != StatusHealed {
		t.Errorf("expected permission auto-healing for signing key")
	}
	if fi, err := os.Stat(keyPath); err != nil || fi.Mode().Perm() != 0600 {
		t.Errorf("expected key permissions to be healed to 0600, got %o", fi.Mode().Perm())
	}

	// Test config synchronization when config is missing key fields but file is valid
	cfg.SigningKeyPEM = ""
	cfg.PublicKeyB64 = ""
	reportKeySync := &DoctorReport{AgentName: "heal-agent"}
	checkAndHealSigningKey(cfg, paths, reportKeySync)
	if reportKeySync.HealedCount == 0 || cfg.SigningKeyPEM == "" || cfg.PublicKeyB64 == "" {
		t.Errorf("expected config synchronization from valid key file")
	}

	// Test combined permission heal and config sync
	_ = os.Chmod(keyPath, 0644)
	cfg.SigningKeyPEM = ""
	cfg.PublicKeyB64 = ""
	reportBoth := &DoctorReport{AgentName: "heal-agent"}
	checkAndHealSigningKey(cfg, paths, reportBoth)
	if reportBoth.HealedCount == 0 || !strings.Contains(reportBoth.Checks[0].Message, "and synchronized config") {
		t.Errorf("expected combined permission heal and config sync")
	}

	// Test parseEd25519PrivateKey variations
	// 1. Raw 64-byte private key PEM
	raw64 := make([]byte, 64)
	pem64 := pem.EncodeToMemory(&pem.Block{Type: "ED25519 PRIVATE KEY", Bytes: raw64})
	if k, err := parseEd25519PrivateKey(pem64); err != nil || len(k) != 64 {
		t.Errorf("expected successful decode of raw 64-byte key: %v", err)
	}

	// 2. Raw 32-byte seed PEM
	raw32 := make([]byte, 32)
	pem32 := pem.EncodeToMemory(&pem.Block{Type: "ED25519 SEED", Bytes: raw32})
	if k, err := parseEd25519PrivateKey(pem32); err != nil || len(k) != 64 {
		t.Errorf("expected successful decode of 32-byte seed: %v", err)
	}

	// 3. Invalid PEM block
	if _, err := parseEd25519PrivateKey([]byte("not-pem-data")); err == nil {
		t.Errorf("expected error decoding non-PEM data")
	}

	// 4. Invalid length block
	badBlock := pem.EncodeToMemory(&pem.Block{Type: "UNKNOWN", Bytes: []byte("short")})
	if _, err := parseEd25519PrivateKey(badBlock); err == nil {
		t.Errorf("expected error decoding invalid length block")
	}

	// Test checkAndHealImage when image is not local
	reportImgWarn := &DoctorReport{AgentName: "heal-agent"}
	cfgRemoteImg := &config.AgentConfig{Name: "remote-img-agent", Image: "docker.io/library/busybox:latest"}
	checkAndHealImage(ctx, cfgRemoteImg, paths, reportImgWarn)
	if reportImgWarn.WarningCount == 0 {
		t.Errorf("expected warning for non-local image")
	}

	// Test healing of agent config permissions when 0644
	agentCfgPath := filepath.Join(paths.AgentsDir, fmt.Sprintf("%s.json", cfg.Name))
	_ = os.Chmod(agentCfgPath, 0644)
	reportCfgPerm := &DoctorReport{AgentName: "heal-agent"}
	_, _ = checkAndHealConfig(paths, cfg.Name, reportCfgPerm)
	if reportCfgPerm.HealedCount == 0 {
		t.Errorf("expected permission auto-healing for agent config JSON")
	}

	// Test healing of missing containerName, volumeName, and image in config
	cfgMissingFields := &config.AgentConfig{Name: "missing-fields-agent"}
	cfgMissingPath := filepath.Join(paths.AgentsDir, "missing-fields-agent.json")
	dataMissing, _ := json.Marshal(cfgMissingFields)
	_ = os.WriteFile(cfgMissingPath, dataMissing, 0600)
	reportMissingFields := &DoctorReport{AgentName: "missing-fields-agent"}
	repairedCfg, _ := checkAndHealConfig(paths, "missing-fields-agent", reportMissingFields)
	if repairedCfg == nil || repairedCfg.ContainerName == "" || repairedCfg.VolumeName == "" || repairedCfg.Image == "" {
		t.Errorf("expected missing container/volume/image fields to be healed")
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

	// 13. SonarQube agent check & heal tests
	sonarMockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/system/status":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"status":"UP"}`))
		case "/api/users/create":
			w.WriteHeader(http.StatusOK)
		case "/api/user_tokens/generate":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"token":"sqa_test_healed_token"}`))
		case "/api/user_tokens/revoke":
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer sonarMockServer.Close()
	os.Setenv("SONAR_HOST_URL", sonarMockServer.URL)

	cfg.SonarToken = ""
	reportSonarHeal := &DoctorReport{AgentName: "heal-agent"}
	checkAndHealSonar(ctx, cfg, paths, reportSonarHeal)
	if reportSonarHeal.HealedCount == 0 || cfg.SonarToken != "sqa_test_healed_token" {
		t.Errorf("expected SonarQube token to be auto-generated and healed")
	}

	// Run again now that token exists
	reportSonarOK := &DoctorReport{AgentName: "heal-agent"}
	checkAndHealSonar(ctx, cfg, paths, reportSonarOK)
	if len(reportSonarOK.Checks) == 0 || reportSonarOK.Checks[0].Status != StatusOK {
		t.Errorf("expected StatusOK for valid SonarQube user check")
	}

	// Offline SonarQube branch
	os.Setenv("SONAR_HOST_URL", "http://127.0.0.1:65505")
	reportSonarOffline := &DoctorReport{AgentName: "heal-agent"}
	checkAndHealSonar(ctx, cfg, paths, reportSonarOffline)
	if reportSonarOffline.WarningCount == 0 {
		t.Errorf("expected warning on offline SonarQube")
	}
}

func TestInfraDoctorDiagnosticsAndHealing(t *testing.T) {
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
		EnvFile:       filepath.Join(tmpDir, "data", ".env"),
		SSHDir:        filepath.Join(tmpDir, "ssh"),
		IDEKeyFile:    filepath.Join(tmpDir, "ssh", "agent-sandbox"),
		SSHConfigFile: filepath.Join(tmpDir, "data", "ssh_config"),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 1. First run: heals missing .env and creates directories
	report1, err := DiagnoseAndHealInfra(ctx, paths)
	if err != nil {
		t.Fatalf("DiagnoseAndHealInfra failed: %v", err)
	}
	if report1.HealedCount == 0 {
		t.Errorf("expected healed items for initial infra run")
	}
	formatted := FormatDoctorReport(report1)
	if !strings.Contains(formatted, "shared infrastructure") {
		t.Errorf("expected shared infrastructure in report output: %s", formatted)
	}

	// 2. Second run: .env is valid
	report2, err := DiagnoseAndHealInfra(ctx, paths)
	if err != nil {
		t.Fatalf("DiagnoseAndHealInfra second pass failed: %v", err)
	}
	if report2.UnrepairableCount > 0 {
		t.Errorf("expected 0 unrepairable items on second pass")
	}

	// Test healing of .env permissions when 0644
	_ = os.Chmod(paths.EnvFile, 0644)
	reportEnvPerm := &DoctorReport{AgentName: "shared-infrastructure"}
	checkAndHealInfraEnv(paths, reportEnvPerm)
	if reportEnvPerm.HealedCount == 0 {
		t.Errorf("expected permission auto-healing for .env file")
	}

	// 3. Direct checks
	checkAndHealInfraStorage(ctx, report2)
	checkAndHealValkeyContainer(ctx, paths, report2)
	checkAndHealGiteaContainer(ctx, paths, report2)

	// 4. Test storage auto-heal (remove a volume)
	_ = exec.Command("podman", "volume", "rm", "-f", "agent-sandbox-valkey-data").Run()
	checkAndHealInfraStorage(ctx, report2)

	// 5. Test Gitea mock server for infra check
	giteaServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"version":"1.22.0"}`))
	}))
	defer giteaServer.Close()
	os.Setenv("GITEA_URL", giteaServer.URL)
	checkAndHealGiteaContainer(ctx, paths, report2)

	// 6. Test unreachable Valkey in infra check
	os.Setenv("BP_PORT", "65501")
	reportOffline := &DoctorReport{AgentName: "shared-infrastructure"}
	checkAndHealValkeyContainer(ctx, paths, reportOffline)
	if reportOffline.WarningCount == 0 {
		t.Errorf("expected warning on offline Valkey in infra check")
	}

	// 7. Test unreachable Gitea in infra check
	os.Setenv("GITEA_URL", "http://127.0.0.1:65502")
	checkAndHealGiteaContainer(ctx, paths, reportOffline)

	// 7b. Test SonarQube mock server for infra check
	sonarServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"UP"}`))
	}))
	defer sonarServer.Close()
	os.Setenv("SONAR_HOST_URL", sonarServer.URL)
	checkAndHealSonarContainer(ctx, report2)

	// 7c. Test unreachable SonarQube in infra check
	os.Setenv("SONAR_HOST_URL", "http://127.0.0.1:65503")
	checkAndHealSonarContainer(ctx, reportOffline)

	// 7d. Test PostgreSQL TCP listener for infra check
	pgLn, pgErr := net.Listen("tcp", "127.0.0.1:0")
	if pgErr == nil {
		defer pgLn.Close()
		_, pgPortStr, _ := net.SplitHostPort(pgLn.Addr().String())
		os.Setenv("POSTGRES_HOST", "127.0.0.1")
		os.Setenv("POSTGRES_PORT", pgPortStr)
		checkAndHealPostgresContainer(ctx, report2)
	}

	// 7e. Test unreachable PostgreSQL in infra check
	os.Setenv("POSTGRES_PORT", "65504")
	checkAndHealPostgresContainer(ctx, reportOffline)

	// 8. Test FormatDoctorReport with unrepairable counts and warning counts
	reportUnrep := &DoctorReport{
		AgentName:         "unrep-agent",
		UnrepairableCount: 2,
		Checks: []CheckItem{
			{Name: "Check 1", Status: StatusError, Message: "error", Unrepairable: true},
		},
	}
	outUnrep := FormatDoctorReport(reportUnrep)
	if !strings.Contains(outUnrep, "unrepairable issue(s)") {
		t.Errorf("expected unrepairable issues summary: %s", outUnrep)
	}

	reportInfraUnrep := &DoctorReport{
		AgentName:         "shared-infrastructure",
		UnrepairableCount: 1,
		Checks: []CheckItem{
			{Name: "Infra Check", Status: StatusError, Message: "error", Unrepairable: true},
		},
	}
	outInfraUnrep := FormatDoctorReport(reportInfraUnrep)
	if !strings.Contains(outInfraUnrep, "unrepairable issue(s) detected in shared infrastructure") {
		t.Errorf("expected shared infra unrepairable summary: %s", outInfraUnrep)
	}

	reportInfraWarn := &DoctorReport{
		AgentName:    "shared-infrastructure",
		WarningCount: 2,
		Checks: []CheckItem{
			{Name: "Warning Check", Status: StatusWarning, Message: "warn"},
		},
	}
	outInfraWarn := FormatDoctorReport(reportInfraWarn)
	if !strings.Contains(outInfraWarn, "warning(s) (offline services)") {
		t.Errorf("expected shared infra warning summary: %s", outInfraWarn)
	}

	reportInfraClean := &DoctorReport{
		AgentName: "shared-infrastructure",
		Checks: []CheckItem{
			{Name: "Clean Check", Status: StatusOK, Message: "ok"},
		},
	}
	outInfraClean := FormatDoctorReport(reportInfraClean)
	if !strings.Contains(outInfraClean, "fully healthy (0 issues found)") {
		t.Errorf("expected shared infra clean summary: %s", outInfraClean)
	}

	reportAgentClean := &DoctorReport{
		AgentName: "clean-agent",
		Checks: []CheckItem{
			{Name: "Clean Check", Status: StatusOK, Message: "ok"},
		},
	}
	outAgentClean := FormatDoctorReport(reportAgentClean)
	if !strings.Contains(outAgentClean, "fully healthy (0 issues found)") {
		t.Errorf("expected agent clean summary: %s", outAgentClean)
	}
}

// TestCheckAndHealValkeyHealPath exercises the auto-heal code path in checkAndHealValkeyContainer
// by overriding the infraValkeyContainer package-level var to point at an ephemeral test container.
func TestCheckAndHealValkeyHealPath(t *testing.T) {
	tmpDir := t.TempDir()
	t.Cleanup(func() {
		_ = exec.Command("podman", "unshare", "rm", "-rf", tmpDir).Run()
	})

	// Start an ephemeral Valkey container with a "correct" password that our check will fail against
	// (we'll intentionally set the wrong admin password so auth fails while state is "running")
	harnessCtr := fmt.Sprintf("test-heal-valkey-%d", os.Getpid())
	aclPath := filepath.Join(tmpDir, "valkey-users.acl")
	realAdminPass := "real_admin_pass_heal_test"
	aclContent := fmt.Sprintf("user default off\nuser admin on >%s ~* &* +@all\n", realAdminPass)
	_ = os.WriteFile(aclPath, []byte(aclContent), 0644)

	port, err := getFreeTestPort()
	if err != nil {
		t.Skip("Cannot find free port for heal test")
	}

	startCmd := exec.Command("podman", "run", "-d", "--name", harnessCtr,
		"-p", fmt.Sprintf("127.0.0.1:%d:6379", port),
		"-v", fmt.Sprintf("%s:/etc/valkey/users.acl:ro", aclPath),
		"docker.io/valkey/valkey:8-alpine",
		"valkey-server", "--aclfile", "/etc/valkey/users.acl",
	)
	if out, err := startCmd.CombinedOutput(); err != nil {
		t.Skipf("Cannot start ephemeral Valkey container for heal test: %v (%s)", err, string(out))
	}
	t.Cleanup(func() {
		_ = exec.Command("podman", "stop", harnessCtr).Run()
		_ = exec.Command("podman", "rm", "-f", harnessCtr).Run()
		// Restore live infra container if StartInfraStack recreated it with tmpDir paths
		realPaths := config.GetPaths()
		_ = runtime.StartInfraStack(context.Background(), realPaths, "", "", "")
	})

	// Wait for container to be ready
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", port), 100*time.Millisecond)
		if err == nil {
			conn.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	// Override the package-level var so doctor targets our ephemeral container
	origContainer := infraValkeyContainer
	infraValkeyContainer = harnessCtr
	defer func() { infraValkeyContainer = origContainer }()

	paths := config.Paths{
		DataHome: filepath.Join(tmpDir, "data"),
	}
	_ = os.MkdirAll(paths.DataHome, 0755)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Case 1: Container running but wrong password → triggers heal attempt
	os.Setenv("BP_HOST", "127.0.0.1")
	os.Setenv("BP_PORT", fmt.Sprintf("%d", port))
	os.Setenv("ADMIN_BACKPLANE_PASSWORD", "wrong_password_for_heal_test")

	report1 := &DoctorReport{AgentName: "shared-infrastructure"}
	checkAndHealValkeyContainer(ctx, paths, report1)
	// Either healed or warning — both are valid since StartInfraStack will try to fix the ACL
	if len(report1.Checks) == 0 {
		t.Errorf("expected at least one check item in heal path test")
	}

	// Case 2: Container running with correct password → exercises the OK path
	os.Setenv("ADMIN_BACKPLANE_PASSWORD", realAdminPass)
	// Wait a brief moment for Valkey to be ready post-start
	time.Sleep(500 * time.Millisecond)
	report2 := &DoctorReport{AgentName: "shared-infrastructure"}
	checkAndHealValkeyContainer(ctx, paths, report2)
	if len(report2.Checks) == 0 {
		t.Errorf("expected at least one check item in correct-password test")
	}
}

func getFreeTestPort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}
