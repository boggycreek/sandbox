// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/boggycreek/agent-sandbox/pkg/config"
)

func TestPluginCoverageBoost(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)
	t.Setenv("APPDATA", filepath.Join(tmpDir, "AppData", "Roaming"))
	t.Setenv("LOCALAPPDATA", filepath.Join(tmpDir, "AppData", "Local"))

	// 1. Path resolution helpers
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

	// 2. In-memory JAR generation
	tbJar, err := GenerateToolboxPluginJAR()
	if err != nil || len(tbJar) == 0 {
		t.Errorf("GenerateToolboxPluginJAR failed: %v", err)
	}

	gwJar, err := GenerateGatewayPluginJAR()
	if err != nil || len(gwJar) == 0 {
		t.Errorf("GenerateGatewayPluginJAR failed: %v", err)
	}

	// 3. ListPlugins with installed states
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

	// Create central toolbox dir
	centralToolboxDir := filepath.Join(paths.DataHome, "plugins", "jetbrains-toolbox", "sndbx")
	_ = os.MkdirAll(centralToolboxDir, 0755)

	// Create active toolbox dir
	_ = os.MkdirAll(tbDir, 0755)

	plugins := ListPlugins(paths)
	for _, p := range plugins {
		if (p.ID == PluginIDGateway || p.ID == PluginIDToolbox) && !p.Installed {
			t.Errorf("expected plugin %s to be marked installed", p.ID)
		}
	}

	// 4. EnsureSSHConfigInclude & RemoveSSHConfigInclude edge cases
	hostSSHConfig := filepath.Join(tmpDir, "ssh_config")
	targetInclude := filepath.Join(tmpDir, "target_config")

	// Missing host file -> creates
	added, err := EnsureSSHConfigInclude(targetInclude, hostSSHConfig)
	if err != nil || !added {
		t.Errorf("EnsureSSHConfigInclude failed to create host config: %v", err)
	}

	// Already present -> no-op
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

	// Remove on missing file
	removed, err = RemoveSSHConfigInclude(targetInclude, filepath.Join(tmpDir, "nonexistent"))
	if err != nil || removed {
		t.Errorf("RemoveSSHConfigInclude on nonexistent file should return false: %v", err)
	}

	// 5. EnsureSSHConfigManagedHosts & RemoveSSHConfigManagedHosts edge cases
	cfg := &config.AgentConfig{Name: "agent1", ContainerName: "sndbx-agent1"}
	_ = config.SaveAgentConfig(cfg, paths)

	managedHostFile := filepath.Join(tmpDir, "managed_ssh_config")
	ctx := context.Background()

	// Missing host config -> creates with header and block
	err = EnsureSSHConfigManagedHosts(ctx, paths, managedHostFile)
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

	// 6. SyncToolboxSSHSettings & RemoveToolboxSSHSettings edge cases
	toolboxPluginsDir := filepath.Join(tmpDir, "tb_plugins")
	settingsFile := filepath.Join(toolboxPluginsDir, "ssh", "settings.json")
	_ = os.MkdirAll(toolboxPluginsDir, 0755)

	// Settings file creates new JSON
	err = SyncToolboxSSHSettings(ctx, paths, settingsFile)
	if err != nil {
		t.Errorf("SyncToolboxSSHSettings failed to create settings: %v", err)
	}

	// Sync with existing entries to hit pruning logic
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

	// 7. Default empty path branches
	_, _ = EnsureSSHConfigInclude(targetInclude, "")
	_, _ = RemoveSSHConfigInclude(targetInclude, "")
	_ = EnsureSSHConfigManagedHosts(ctx, paths, "")
	_ = RemoveSSHConfigManagedHosts("")
	_ = SyncToolboxSSHSettings(ctx, paths, "")
	_ = RemoveToolboxSSHSettings("")
	_ = FindJetBrainsPluginDirsFrom([]string{filepath.Join(tmpDir, "nonexistent-dir")})
}
