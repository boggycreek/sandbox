// Copyright (c) 2026 Boggy Creek Software LLC
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package plugin

import (
	"archive/zip"
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/boggycreek/agent-sandbox/pkg/config"
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

func TestFindJetBrainsPluginDirs(t *testing.T) {
	dirs := FindJetBrainsPluginDirs()
	_ = dirs

	roots := GetDefaultJetBrainsSearchRoots()
	_ = roots

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

	// 3. Match by basename
	matchHostConfig := filepath.Join(tmpDir, "config_match")
	_ = os.WriteFile(matchHostConfig, []byte("Host *\n  Include ~/.local/share/agent-sandbox/ssh_config\n"), 0600)
	linkedBasename, err := EnsureSSHConfigInclude(targetSSHConfig, matchHostConfig)
	if err != nil || linkedBasename {
		t.Fatalf("expected linked=false when matching basename exists: %v", err)
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

func TestInstallAndRemoveToolboxPlugin(t *testing.T) {
	tmpDir := t.TempDir()
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

	// Verify central jar exists
	centralJar := filepath.Join(paths.DataHome, "plugins", "jetbrains-gateway", "sndbx-gateway", "lib", "sndbx-gateway.jar")
	if _, err := os.Stat(centralJar); err != nil {
		t.Errorf("central plugin jar not found: %v", err)
	}

	// Test ListPlugins detects it
	list := ListPlugins(paths)
	var foundToolbox bool
	for _, p := range list {
		if p.ID == PluginIDToolbox {
			foundToolbox = true
			if !p.Installed {
				t.Errorf("expected toolbox plugin to be marked installed")
			}
		}
	}
	if !foundToolbox {
		t.Errorf("ListPlugins did not return toolbox plugin")
	}

	// Test RemoveToolboxPlugin
	remRes, err := RemoveToolboxPlugin(ctx, paths, &out)
	if err != nil {
		t.Fatalf("RemoveToolboxPlugin error: %v", err)
	}
	if remRes.PluginID != PluginIDToolbox {
		t.Errorf("expected PluginID=%s, got %s", PluginIDToolbox, remRes.PluginID)
	}
	if len(remRes.RemovedPaths) == 0 {
		t.Errorf("expected at least 1 removed path")
	}

	// Verify central directory was removed
	if _, err := os.Stat(centralJar); !os.IsNotExist(err) {
		t.Errorf("expected central jar to be removed")
	}

	// Test central directory error path
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
	paths := config.Paths{
		DataHome:      filepath.Join(tmpDir, "data"),
		AgentsDir:     filepath.Join(tmpDir, "data", "agents"),
		SSHConfigFile: filepath.Join(tmpDir, "data", "ssh_config"),
	}
	_ = paths.EnsureDirectories()

	ctx := context.Background()
	var out bytes.Buffer

	// Save original ExecCommandContext
	origExec := ExecCommandContext
	defer func() { ExecCommandContext = origExec }()

	// 1. Simulate 'code' found
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

	// 2. Simulate 'code' not found
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

	// 3. Remove VS Code plugin
	remRes, err := RemoveVSCodePlugin(ctx, paths, &out)
	if err != nil {
		t.Fatalf("RemoveVSCodePlugin error: %v", err)
	}
	if remRes.PluginID != PluginIDVSCode {
		t.Errorf("expected PluginID=%s, got %s", PluginIDVSCode, remRes.PluginID)
	}
}
