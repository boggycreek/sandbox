// Copyright (c) 2026 Boggy Creek Software LLC
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package plugin

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boggycreek/sandbox/pkg/config"
)

func TestGenerateGatewayPluginXML(t *testing.T) {
	xml := GenerateGatewayPluginXML()
	if !strings.Contains(xml, JetBrainsPluginID) {
		t.Fatalf("expected XML to contain %s, got: %s", JetBrainsPluginID, xml)
	}
	if !strings.Contains(xml, "<gatewayConnector") {
		t.Fatalf("expected XML to declare gatewayConnector extension")
	}
	if !strings.Contains(xml, "<gatewayConnectionProvider") {
		t.Fatalf("expected XML to declare gatewayConnectionProvider extension")
	}
}

func TestGenerateGatewayPluginJAR(t *testing.T) {
	data, err := GenerateGatewayPluginJAR()
	if err != nil {
		t.Fatalf("unexpected error generating JAR: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("expected non-empty JAR bytes")
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("failed to open generated JAR as zip: %v", err)
	}

	var foundManifest, foundPluginXML bool
	for _, f := range zr.File {
		if f.Name == "META-INF/MANIFEST.MF" {
			foundManifest = true
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("failed to read manifest: %v", err)
			}
			content, _ := io.ReadAll(rc)
			_ = rc.Close()
			if !strings.Contains(string(content), "Manifest-Version:") {
				t.Errorf("manifest missing version: %s", string(content))
			}
		}
		if f.Name == "META-INF/plugin.xml" {
			foundPluginXML = true
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("failed to read plugin.xml: %v", err)
			}
			content, _ := io.ReadAll(rc)
			_ = rc.Close()
			if !strings.Contains(string(content), JetBrainsPluginID) {
				t.Errorf("plugin.xml missing ID: %s", string(content))
			}
		}
	}

	if !foundManifest {
		t.Errorf("expected META-INF/MANIFEST.MF in generated JAR")
	}
	if !foundPluginXML {
		t.Errorf("expected META-INF/plugin.xml in generated JAR")
	}
}

func TestGenerateToolboxPluginManifestAndJAR(t *testing.T) {
	extJson := GenerateToolboxExtensionJSON()
	if !strings.Contains(extJson, ToolboxPluginID) {
		t.Errorf("expected extension.json to contain %s", ToolboxPluginID)
	}
	if !strings.Contains(extJson, ToolboxPluginAPIVersion) {
		t.Errorf("expected extension.json to contain %s", ToolboxPluginAPIVersion)
	}

	iconSvg := GenerateToolboxPluginIconSVG()
	if !strings.Contains(iconSvg, "<svg") {
		t.Errorf("expected SVG icon")
	}

	data, err := GenerateToolboxPluginJAR()
	if err != nil {
		t.Fatalf("unexpected error generating toolbox JAR: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("failed to open generated toolbox JAR as zip: %v", err)
	}

	var foundExt, foundIcon, foundService bool
	for _, f := range zr.File {
		if f.Name == "extension.json" {
			foundExt = true
		}
		if f.Name == "pluginIcon.svg" {
			foundIcon = true
		}
		if f.Name == "META-INF/services/com.jetbrains.toolbox.api.remoteDev.RemoteDevExtension" {
			foundService = true
		}
	}
	if !foundExt || !foundIcon || !foundService {
		t.Errorf("toolbox JAR missing expected files (ext: %v, icon: %v, service: %v)", foundExt, foundIcon, foundService)
	}
}

func TestFindJetBrainsPluginDirs(t *testing.T) {
	dirs := FindJetBrainsPluginDirs()
	_ = dirs

	roots := GetDefaultJetBrainsSearchRoots()
	_ = roots

	_ = GetDefaultToolboxPluginDir()
	_ = GetDefaultToolboxSSHSettingsPath()

	tmpDir := t.TempDir()
	// Create mock IDE directories
	mockRoot := filepath.Join(tmpDir, "JetBrains")
	_ = os.MkdirAll(filepath.Join(mockRoot, "JetBrainsGateway2024.1"), 0755)
	_ = os.MkdirAll(filepath.Join(mockRoot, "WebStorm2026.2"), 0755)
	_ = os.MkdirAll(filepath.Join(mockRoot, "GoLand2026.2"), 0755)
	_ = os.MkdirAll(filepath.Join(mockRoot, "CLion2026.2"), 0755)
	_ = os.MkdirAll(filepath.Join(mockRoot, "PyCharm2026.2"), 0755)
	_ = os.MkdirAll(filepath.Join(mockRoot, "IntelliJIdea2026.2"), 0755)
	_ = os.MkdirAll(filepath.Join(mockRoot, "IdeaIC2026.2"), 0755)
	_ = os.MkdirAll(filepath.Join(mockRoot, "DataGrip2026.2"), 0755)
	// Create a non-matching directory and a file
	_ = os.MkdirAll(filepath.Join(mockRoot, "OtherApp"), 0755)
	_ = os.WriteFile(filepath.Join(mockRoot, "regular_file.txt"), []byte("hi"), 0644)

	found := FindJetBrainsPluginDirsFrom([]string{mockRoot, "/nonexistent/root"})
	if len(found) != 8 {
		t.Errorf("expected 8 matching directories, got %d: %v", len(found), found)
	}
}

func TestEnsureAndRemoveSSHConfigInclude(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	hostSSHConfig := filepath.Join(tmpDir, "config")
	targetSSHConfig := filepath.Join(tmpDir, "agent-sandbox", "ssh_config")

	// 1. Initial creation when hostSSHConfig does not exist
	linked, err := EnsureSSHConfigInclude(targetSSHConfig, hostSSHConfig)
	if err != nil {
		t.Fatalf("EnsureSSHConfigInclude failed: %v", err)
	}
	if !linked {
		t.Fatalf("expected linked=true on fresh create")
	}

	content, err := os.ReadFile(hostSSHConfig)
	if err != nil {
		t.Fatalf("reading host ssh config: %v", err)
	}
	if !strings.Contains(string(content), "Include "+targetSSHConfig) {
		t.Fatalf("expected Include directive in host ssh config: %s", string(content))
	}

	// 2. Second invocation should be idempotent (linked=false)
	linkedAgain, err := EnsureSSHConfigInclude(targetSSHConfig, hostSSHConfig)
	if err != nil {
		t.Fatalf("idempotent EnsureSSHConfigInclude failed: %v", err)
	}
	if linkedAgain {
		t.Fatalf("expected linked=false on subsequent invocation")
	}

	// 3. Unrelated include should not match and should be preserved
	matchHostConfig := filepath.Join(tmpDir, "config_match")
	_ = os.WriteFile(matchHostConfig, []byte("Host *\n  Include ~/.ssh/work/ssh_config\n"), 0600)
	linkedUnrelated, err := EnsureSSHConfigInclude(targetSSHConfig, matchHostConfig)
	if err != nil || !linkedUnrelated {
		t.Fatalf("expected linked=true when only unrelated include exists: %v", err)
	}
	matchBytes, _ := os.ReadFile(matchHostConfig)
	if !strings.Contains(string(matchBytes), "Include ~/.ssh/work/ssh_config") || !strings.Contains(string(matchBytes), "Include "+targetSSHConfig) {
		t.Fatalf("expected both unrelated and target includes: %s", string(matchBytes))
	}

	// 3b. Match with ~ expansion (tmpDir is HOME)
	tildeHostConfig := filepath.Join(tmpDir, "config_tilde")
	_ = os.WriteFile(tildeHostConfig, []byte("Host *\n  include ~/agent-sandbox/ssh_config\n"), 0600)
	linkedTilde, err := EnsureSSHConfigInclude(targetSSHConfig, tildeHostConfig)
	if err != nil || linkedTilde {
		t.Fatalf("expected linked=false when ~/agent-sandbox/ssh_config exists: %v", err)
	}

	// 4. Remove include directive
	unlinked, err := RemoveSSHConfigInclude(targetSSHConfig, hostSSHConfig)
	if err != nil {
		t.Fatalf("RemoveSSHConfigInclude failed: %v", err)
	}
	if !unlinked {
		t.Fatalf("expected unlinked=true")
	}
	contentAfter, _ := os.ReadFile(hostSSHConfig)
	if strings.Contains(string(contentAfter), "Include "+targetSSHConfig) {
		t.Errorf("expected Include directive to be removed: %s", string(contentAfter))
	}

	// 5. Remove again when not present
	unlinked2, err := RemoveSSHConfigInclude(targetSSHConfig, hostSSHConfig)
	if err != nil || unlinked2 {
		t.Fatalf("expected unlinked=false when already removed")
	}

	// 6. Remove on nonexistent file
	unlinkedNonexistent, err := RemoveSSHConfigInclude(targetSSHConfig, filepath.Join(tmpDir, "does-not-exist"))
	if err != nil || unlinkedNonexistent {
		t.Fatalf("expected no error and unlinked=false on nonexistent file")
	}

	// 7. Error creating directory (invalid path)
	_, err = EnsureSSHConfigInclude(targetSSHConfig, "/dev/null/impossible/config")
	if err == nil {
		t.Fatalf("expected error on invalid host config path")
	}

	// 8. Default hostSSHConfigFile resolution
	_, _ = EnsureSSHConfigInclude(targetSSHConfig, "")
	_, _ = RemoveSSHConfigInclude(targetSSHConfig, "")
}

func TestEnsureAndRemoveSSHConfigManagedHosts(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	hostSSHConfig := filepath.Join(tmpDir, "config")
	paths := config.Paths{
		DataHome:      filepath.Join(tmpDir, "data"),
		AgentsDir:     filepath.Join(tmpDir, "data", "agents"),
		SSHConfigFile: filepath.Join(tmpDir, "data", "ssh_config"),
		IDEKeyFile:    filepath.Join(tmpDir, "data", "id_ed25519"),
	}
	_ = paths.EnsureDirectories()

	ctx := context.Background()

	// 1. Initial creation
	err := EnsureSSHConfigManagedHosts(ctx, paths, hostSSHConfig)
	if err != nil {
		t.Fatalf("EnsureSSHConfigManagedHosts failed: %v", err)
	}
	data, _ := os.ReadFile(hostSSHConfig)
	if !strings.Contains(string(data), ManagedSSHConfigHeader) {
		t.Fatalf("expected header in config: %s", string(data))
	}

	// 2. Second invocation replaces existing block
	err = EnsureSSHConfigManagedHosts(ctx, paths, hostSSHConfig)
	if err != nil {
		t.Fatalf("EnsureSSHConfigManagedHosts second run failed: %v", err)
	}

	// 3. Remove managed hosts
	err = RemoveSSHConfigManagedHosts(hostSSHConfig)
	if err != nil {
		t.Fatalf("RemoveSSHConfigManagedHosts failed: %v", err)
	}
	dataAfter, _ := os.ReadFile(hostSSHConfig)
	if strings.Contains(string(dataAfter), ManagedSSHConfigHeader) {
		t.Fatalf("expected header to be removed: %s", string(dataAfter))
	}

	// 4. Remove on nonexistent file
	err = RemoveSSHConfigManagedHosts(filepath.Join(tmpDir, "not-found"))
	if err != nil {
		t.Fatalf("expected no error on nonexistent file: %v", err)
	}

	// 5. Default host config path resolution
	_ = EnsureSSHConfigManagedHosts(ctx, paths, "")
	_ = RemoveSSHConfigManagedHosts("")
}

func TestSyncAndRemoveToolboxSSHSettings(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	toolboxPluginsDir := filepath.Join(tmpDir, "Toolbox", "plugins")
	_ = os.MkdirAll(toolboxPluginsDir, 0755)
	settingsFile := filepath.Join(toolboxPluginsDir, "ssh", "settings.json")

	paths := config.Paths{
		DataHome:  filepath.Join(tmpDir, "data"),
		AgentsDir: filepath.Join(tmpDir, "data", "agents"),
	}
	_ = paths.EnsureDirectories()

	ctx := context.Background()

	// Initial pre-existing settings with another remote
	initSettings := map[string]any{
		"envsJson":                 `{"remotes":[{"host":"sidekick","port":0,"userName":"brian","shouldUseSystemSshAgent":true,"originalConnectionString":"brian@sidekick"}]}`,
		"CONNECTION_STRINGS_CACHE": `{"brian@sidekick":{"host":"sidekick","port":0,"userName":"brian","shouldUseSystemSshAgent":true,"originalConnectionString":"brian@sidekick"}}`,
	}
	initBytes, _ := json.MarshalIndent(initSettings, "", "    ")
	_ = os.MkdirAll(filepath.Dir(settingsFile), 0755)
	_ = os.WriteFile(settingsFile, initBytes, 0600)

	// Test Sync
	err := SyncToolboxSSHSettings(ctx, paths, settingsFile)
	if err != nil {
		t.Fatalf("SyncToolboxSSHSettings failed: %v", err)
	}

	content, _ := os.ReadFile(settingsFile)
	if !strings.Contains(string(content), "sidekick") {
		t.Fatalf("expected preserved non-sndbx remotes: %s", string(content))
	}

	// Test Remove
	err = RemoveToolboxSSHSettings(settingsFile)
	if err != nil {
		t.Fatalf("RemoveToolboxSSHSettings failed: %v", err)
	}

	// Test with nonexistent file
	_ = RemoveToolboxSSHSettings(filepath.Join(tmpDir, "nonexistent", "settings.json"))

	// Test default resolution
	_ = SyncToolboxSSHSettings(ctx, paths, "")
	_ = RemoveToolboxSSHSettings("")
}

func TestInstallAndRemoveGatewayPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	paths := config.Paths{
		DataHome:      filepath.Join(tmpDir, "data"),
		AgentsDir:     filepath.Join(tmpDir, "data", "agents"),
		SSHConfigFile: filepath.Join(tmpDir, "data", "ssh_config"),
	}
	_ = paths.EnsureDirectories()

	ctx := context.Background()
	var out bytes.Buffer

	res, err := InstallGatewayPlugin(ctx, paths, &out)
	if err != nil {
		t.Fatalf("InstallGatewayPlugin error: %v", err)
	}
	if res.PluginID != PluginIDGateway {
		t.Errorf("expected PluginID=%s, got %s", PluginIDGateway, res.PluginID)
	}
	if len(res.InstalledPaths) == 0 {
		t.Fatalf("expected at least 1 installed path")
	}

	centralJar := filepath.Join(paths.DataHome, "plugins", "jetbrains-gateway", "sndbx-gateway", "lib", "sndbx-gateway.jar")
	if _, err := os.Stat(centralJar); err != nil {
		t.Errorf("central gateway jar not found: %v", err)
	}

	remRes, err := RemoveGatewayPlugin(ctx, paths, &out)
	if err != nil {
		t.Fatalf("RemoveGatewayPlugin error: %v", err)
	}
	if remRes.PluginID != PluginIDGateway {
		t.Errorf("expected PluginID=%s, got %s", PluginIDGateway, remRes.PluginID)
	}
	if len(remRes.RemovedPaths) == 0 {
		t.Errorf("expected at least 1 removed path")
	}

	badPaths := config.Paths{
		DataHome: "/dev/null/impossible",
	}
	_, err = InstallGatewayPlugin(ctx, badPaths, &out)
	if err == nil {
		t.Errorf("expected error with bad DataHome path")
	}
}

func TestInstallAndRemoveToolboxPlugin(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	paths := config.Paths{
		DataHome:      filepath.Join(tmpDir, "data"),
		AgentsDir:     filepath.Join(tmpDir, "data", "agents"),
		SSHConfigFile: filepath.Join(tmpDir, "data", "ssh_config"),
	}
	_ = paths.EnsureDirectories()

	ctx := context.Background()
	var out bytes.Buffer

	res, err := InstallToolboxPlugin(ctx, paths, &out)
	if err != nil {
		t.Fatalf("InstallToolboxPlugin error: %v", err)
	}
	if res.PluginID != PluginIDToolbox {
		t.Errorf("expected PluginID=%s, got %s", PluginIDToolbox, res.PluginID)
	}
	if len(res.InstalledPaths) == 0 {
		t.Fatalf("expected at least 1 installed path")
	}

	centralExt := filepath.Join(paths.DataHome, "plugins", "jetbrains-toolbox", "sndbx", "extension.json")
	if _, err := os.Stat(centralExt); err != nil {
		t.Errorf("central toolbox extension.json not found: %v", err)
	}

	remRes, err := RemoveToolboxPlugin(ctx, paths, &out)
	if err != nil {
		t.Fatalf("RemoveToolboxPlugin error: %v", err)
	}
	if remRes.PluginID != PluginIDToolbox {
		t.Errorf("expected PluginID=%s, got %s", PluginIDToolbox, remRes.PluginID)
	}

	badPaths := config.Paths{
		DataHome: "/dev/null/impossible",
	}
	_, err = InstallToolboxPlugin(ctx, badPaths, &out)
	if err == nil {
		t.Errorf("expected error with bad DataHome path")
	}
}

func TestInstallAndRemoveVSCodePlugin(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	paths := config.Paths{
		DataHome:      filepath.Join(tmpDir, "data"),
		AgentsDir:     filepath.Join(tmpDir, "data", "agents"),
		SSHConfigFile: filepath.Join(tmpDir, "data", "ssh_config"),
	}
	_ = paths.EnsureDirectories()

	ctx := context.Background()
	var out bytes.Buffer

	origExec := ExecCommandContext
	defer func() { ExecCommandContext = origExec }()

	ExecCommandContext = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}

	res, err := InstallVSCodePlugin(ctx, paths, &out)
	if err != nil {
		t.Fatalf("InstallVSCodePlugin error: %v", err)
	}
	if res.PluginID != PluginIDVSCode {
		t.Errorf("expected PluginID=%s, got %s", PluginIDVSCode, res.PluginID)
	}

	ExecCommandContext = func(ctx context.Context, command string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	res2, err := InstallVSCodePlugin(ctx, paths, &out)
	if err != nil {
		t.Fatalf("InstallVSCodePlugin with missing code error: %v", err)
	}
	if !strings.Contains(res2.Message, "not found in PATH") {
		t.Errorf("expected message to note missing code CLI: %s", res2.Message)
	}

	remRes, err := RemoveVSCodePlugin(ctx, paths, &out)
	if err != nil {
		t.Fatalf("RemoveVSCodePlugin error: %v", err)
	}
	if remRes.PluginID != PluginIDVSCode {
		t.Errorf("expected PluginID=%s, got %s", PluginIDVSCode, remRes.PluginID)
	}
}

func TestListPlugins(t *testing.T) {
	tmpDir := t.TempDir()
	paths := config.Paths{
		DataHome:      filepath.Join(tmpDir, "data"),
		AgentsDir:     filepath.Join(tmpDir, "data", "agents"),
		SSHConfigFile: filepath.Join(tmpDir, "data", "ssh_config"),
	}
	_ = paths.EnsureDirectories()

	plugins := ListPlugins(paths)
	if len(plugins) != 3 {
		t.Fatalf("expected 3 plugins, got %d", len(plugins))
	}
	found := make(map[string]bool)
	for _, p := range plugins {
		found[p.ID] = true
	}
	if !found[PluginIDGateway] || !found[PluginIDToolbox] || !found[PluginIDVSCode] {
		t.Fatalf("missing expected plugins: %+v", found)
	}
}

func TestPluginPathResolutionAndJARGeneration(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("APPDATA", filepath.Join(tmpDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(tmpDir, "AppData", "Local"))

	roots := GetDefaultJetBrainsSearchRoots()
	if len(roots) == 0 {
		t.Errorf("expected search roots, got empty")
	}

	tbDir := GetDefaultToolboxPluginDir()
	if tbDir == "" {
		t.Errorf("expected toolbox plugin dir, got empty")
	}

	sshPath := GetDefaultToolboxSSHSettingsPath()
	if sshPath == "" {
		t.Errorf("expected toolbox ssh path, got empty")
	}

	tbJar, err := GenerateToolboxPluginJAR()
	if err != nil || len(tbJar) == 0 {
		t.Errorf("GenerateToolboxPluginJAR failed: %v", err)
	}

	gwJar, err := GenerateGatewayPluginJAR()
	if err != nil || len(gwJar) == 0 {
		t.Errorf("GenerateGatewayPluginJAR failed: %v", err)
	}

	dirs := FindJetBrainsPluginDirsFrom([]string{filepath.Join(tmpDir, "nonexistent-dir")})
	if len(dirs) != 0 {
		t.Errorf("expected empty dirs for nonexistent root, got %v", dirs)
	}
}

func TestListPluginsInstalledState(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	tbDir := GetDefaultToolboxPluginDir()

	paths := config.Paths{
		DataHome:      filepath.Join(tmpDir, "data"),
		AgentsDir:     filepath.Join(tmpDir, "data", "agents"),
		SecretsDir:    filepath.Join(tmpDir, "data", "secrets"),
		SSHConfigFile: filepath.Join(tmpDir, "data", "ssh_config"),
	}
	_ = paths.EnsureDirectories()

	// Create central gateway jar
	centralGatewayJar := filepath.Join(paths.DataHome, "plugins", "jetbrains-gateway", "sndbx-gateway", "lib", "sndbx-gateway.jar")
	_ = os.MkdirAll(filepath.Dir(centralGatewayJar), 0755)
	_ = os.WriteFile(centralGatewayJar, []byte("fake-jar"), 0644)

	// Create central toolbox dir and active toolbox dir
	centralToolboxDir := filepath.Join(paths.DataHome, "plugins", "jetbrains-toolbox", "sndbx")
	_ = os.MkdirAll(centralToolboxDir, 0755)
	_ = os.MkdirAll(tbDir, 0755)

	plugins := ListPlugins(paths)
	for _, p := range plugins {
		if (p.ID == PluginIDGateway || p.ID == PluginIDToolbox) && !p.Installed {
			t.Errorf("expected plugin %s to be marked installed", p.ID)
		}
	}
}

func TestSSHConfigIncludeLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	hostSSHConfig := filepath.Join(tmpDir, "ssh_config")
	targetInclude := filepath.Join(tmpDir, "target_config")

	// Missing host file -> creates with Include directive
	added, err := EnsureSSHConfigInclude(targetInclude, hostSSHConfig)
	if err != nil || !added {
		t.Errorf("EnsureSSHConfigInclude failed to create host config: %v", err)
	}

	// Already present -> idempotent no-op
	added, err = EnsureSSHConfigInclude(targetInclude, hostSSHConfig)
	if err != nil || added {
		t.Errorf("EnsureSSHConfigInclude should not re-add existing include: %v", err)
	}

	// Remove include
	removed, err := RemoveSSHConfigInclude(targetInclude, hostSSHConfig)
	if err != nil || !removed {
		t.Errorf("RemoveSSHConfigInclude failed: %v", err)
	}

	// Remove when not present
	removed, err = RemoveSSHConfigInclude(targetInclude, hostSSHConfig)
	if err != nil || removed {
		t.Errorf("RemoveSSHConfigInclude should return false when not present: %v", err)
	}

	// Remove on nonexistent file
	removed, err = RemoveSSHConfigInclude(targetInclude, filepath.Join(tmpDir, "nonexistent"))
	if err != nil || removed {
		t.Errorf("RemoveSSHConfigInclude on nonexistent file should return false: %v", err)
	}
}

func TestSSHConfigManagedHostsLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	paths := config.Paths{
		DataHome:      filepath.Join(tmpDir, "data"),
		AgentsDir:     filepath.Join(tmpDir, "data", "agents"),
		SecretsDir:    filepath.Join(tmpDir, "data", "secrets"),
		SSHConfigFile: filepath.Join(tmpDir, "data", "ssh_config"),
	}
	_ = paths.EnsureDirectories()

	cfg := &config.AgentConfig{Name: "agent1", ContainerName: "sndbx-agent1"}
	_ = config.SaveAgentConfig(cfg, paths)

	managedHostFile := filepath.Join(tmpDir, "managed_ssh_config")
	ctx := context.Background()

	// Missing host config -> creates with header and block
	err := EnsureSSHConfigManagedHosts(ctx, paths, managedHostFile)
	if err != nil {
		t.Errorf("EnsureSSHConfigManagedHosts failed to create file: %v", err)
	}

	// Update existing block
	err = EnsureSSHConfigManagedHosts(ctx, paths, managedHostFile)
	if err != nil {
		t.Errorf("EnsureSSHConfigManagedHosts failed to update: %v", err)
	}

	// Remove managed hosts
	err = RemoveSSHConfigManagedHosts(managedHostFile)
	if err != nil {
		t.Errorf("RemoveSSHConfigManagedHosts failed: %v", err)
	}

	// Remove when already removed
	err = RemoveSSHConfigManagedHosts(managedHostFile)
	if err != nil {
		t.Errorf("RemoveSSHConfigManagedHosts should return nil when block absent: %v", err)
	}

	// Remove on missing file
	err = RemoveSSHConfigManagedHosts(filepath.Join(tmpDir, "nonexistent"))
	if err != nil {
		t.Errorf("RemoveSSHConfigManagedHosts on nonexistent file should return nil: %v", err)
	}
}

func TestToolboxSSHSettingsLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	paths := config.Paths{
		DataHome:      filepath.Join(tmpDir, "data"),
		AgentsDir:     filepath.Join(tmpDir, "data", "agents"),
		SecretsDir:    filepath.Join(tmpDir, "data", "secrets"),
		SSHConfigFile: filepath.Join(tmpDir, "data", "ssh_config"),
	}
	_ = paths.EnsureDirectories()

	toolboxPluginsDir := filepath.Join(tmpDir, "tb_plugins")
	settingsFile := filepath.Join(toolboxPluginsDir, "ssh", "settings.json")
	_ = os.MkdirAll(toolboxPluginsDir, 0755)
	ctx := context.Background()

	// Settings file creates new JSON
	err := SyncToolboxSSHSettings(ctx, paths, settingsFile)
	if err != nil {
		t.Errorf("SyncToolboxSSHSettings failed to create settings: %v", err)
	}

	// Re-sync with existing entries
	err = SyncToolboxSSHSettings(ctx, paths, settingsFile)
	if err != nil {
		t.Errorf("SyncToolboxSSHSettings re-sync failed: %v", err)
	}

	// Sync with invalid JSON file should reset and recreate
	_ = os.WriteFile(settingsFile, []byte("{invalid-json"), 0644)
	err = SyncToolboxSSHSettings(ctx, paths, settingsFile)
	if err != nil {
		t.Errorf("SyncToolboxSSHSettings failed on invalid json: %v", err)
	}

	// RemoveToolboxSSHSettings on valid file
	err = RemoveToolboxSSHSettings(settingsFile)
	if err != nil {
		t.Errorf("RemoveToolboxSSHSettings failed: %v", err)
	}

	// Remove on nonexistent file
	err = RemoveToolboxSSHSettings(filepath.Join(tmpDir, "nonexistent.json"))
	if err != nil {
		t.Errorf("RemoveToolboxSSHSettings on nonexistent should return nil: %v", err)
	}

	// Remove on invalid JSON file
	_ = os.WriteFile(settingsFile, []byte("{bad-json"), 0644)
	err = RemoveToolboxSSHSettings(settingsFile)
	if err != nil {
		t.Errorf("RemoveToolboxSSHSettings on invalid json returned error: %v", err)
	}
}

func TestSSHConfigIncludeNonDestructiveAndAtomic(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	hostSSHConfig := filepath.Join(tmpDir, "config")
	targetSSHConfig := filepath.Join(tmpDir, "agent-sandbox", "ssh_config")

	// 1. Host config contains custom user formatting, comments, and host block
	userContent := `# OpenSSH Configuration
# Custom user comment

Host legacy-host
    HostName 10.0.0.1
    User admin

# Another custom comment
`
	if err := os.WriteFile(hostSSHConfig, []byte(userContent), 0640); err != nil {
		t.Fatalf("failed writing initial config: %v", err)
	}

	linked, err := EnsureSSHConfigInclude(targetSSHConfig, hostSSHConfig)
	if err != nil {
		t.Fatalf("EnsureSSHConfigInclude failed: %v", err)
	}
	if !linked {
		t.Fatalf("expected linked=true when directive newly added")
	}

	// Verify backup file was created with original content
	bakPath := hostSSHConfig + ".bak"
	bakContent, err := os.ReadFile(bakPath)
	if err != nil {
		t.Fatalf("expected backup file %s to exist: %v", bakPath, err)
	}
	if string(bakContent) != userContent {
		t.Fatalf("expected backup file to match original user content")
	}

	// Verify file mode was preserved
	fi, err := os.Stat(hostSSHConfig)
	if err != nil {
		t.Fatalf("failed to stat host config: %v", err)
	}
	if fi.Mode().Perm() != 0640 {
		t.Errorf("expected permissions 0640 preserved, got %o", fi.Mode().Perm())
	}

	content, err := os.ReadFile(hostSSHConfig)
	if err != nil {
		t.Fatalf("failed reading host config: %v", err)
	}
	contentStr := string(content)

	// Verify header and include exist at top, and user content is preserved verbatim
	expectedPrefix := fmt.Sprintf("%s\nInclude %s\n\n", sshConfigCommentHeader, targetSSHConfig)
	if !strings.HasPrefix(contentStr, expectedPrefix) {
		t.Fatalf("expected config to begin with include block, got:\n%s", contentStr)
	}
	remainder := strings.TrimPrefix(contentStr, expectedPrefix)
	if remainder != userContent {
		t.Fatalf("expected user content to remain untouched, got:\n%s\nexpected:\n%s", remainder, userContent)
	}

	// 2. Idempotent call when already linked
	linkedAgain, err := EnsureSSHConfigInclude(targetSSHConfig, hostSSHConfig)
	if err != nil || linkedAgain {
		t.Fatalf("expected linked=false on second invocation, got %v (err: %v)", linkedAgain, err)
	}
	contentAgain, _ := os.ReadFile(hostSSHConfig)
	if string(contentAgain) != contentStr {
		t.Errorf("expected identical content after idempotent EnsureSSHConfigInclude")
	}

	// 3. Remove include directive cleans up header and include, leaving user content untouched
	unlinked, err := RemoveSSHConfigInclude(targetSSHConfig, hostSSHConfig)
	if err != nil || !unlinked {
		t.Fatalf("RemoveSSHConfigInclude failed: unlinked=%v, err=%v", unlinked, err)
	}

	contentAfterRemove, _ := os.ReadFile(hostSSHConfig)
	afterStr := string(contentAfterRemove)

	if strings.Contains(afterStr, sshConfigCommentHeader) {
		t.Errorf("expected comment header to be removed, found in:\n%s", afterStr)
	}
	if strings.Contains(afterStr, targetSSHConfig) {
		t.Errorf("expected include directive to be removed in:\n%s", afterStr)
	}
	if afterStr != userContent {
		t.Errorf("expected user content to be preserved exactly after removal, got:\n%s\nexpected:\n%s", afterStr, userContent)
	}

	// 4. Removing when only the include block was present leaves zero residue (empty file)
	soloConfig := filepath.Join(tmpDir, "solo_config")
	if _, err := EnsureSSHConfigInclude(targetSSHConfig, soloConfig); err != nil {
		t.Fatalf("failed creating solo config: %v", err)
	}
	if _, err := RemoveSSHConfigInclude(targetSSHConfig, soloConfig); err != nil {
		t.Fatalf("failed removing solo config: %v", err)
	}
	soloAfter, _ := os.ReadFile(soloConfig)
	if len(strings.TrimSpace(string(soloAfter))) != 0 {
		t.Errorf("expected solo config to be empty after remove, got:\n%s", string(soloAfter))
	}
}
