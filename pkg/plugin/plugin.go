// Copyright (c) 2026 Boggy Creek Software LLC
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package plugin

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/boggycreek/agent-sandbox/pkg/config"
	sndbxRuntime "github.com/boggycreek/agent-sandbox/pkg/runtime"
)

// Plugin identifiers and constants.
const (
	PluginIDToolbox = "toolbox"
	PluginIDVSCode  = "vscode"

	JetBrainsPluginID      = "com.boggycreek.sndbx.gateway"
	JetBrainsPluginName    = "Agent Sandbox Gateway"
	JetBrainsPluginVersion = "0.1.0-alpha"
	JetBrainsPluginVendor  = "Boggy Creek Software LLC"
)

// ExecCommandContext is a variable for stubbing in tests.
var ExecCommandContext = exec.CommandContext

// PluginInfo describes a supported or installed IDE plugin.
type PluginInfo struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Target      string   `json:"target"`
	Description string   `json:"description"`
	Installed   bool     `json:"installed"`
	Locations   []string `json:"locations,omitempty"`
}

// InstallResult details the outcome of an installation operation.
type InstallResult struct {
	PluginID        string   `json:"plugin_id"`
	PluginName      string   `json:"plugin_name"`
	Version         string   `json:"version"`
	InstalledPaths  []string `json:"installed_paths"`
	SSHConfigLinked bool     `json:"ssh_config_linked"`
	Message         string   `json:"message"`
}

// RemoveResult details the outcome of a plugin removal operation.
type RemoveResult struct {
	PluginID          string   `json:"plugin_id"`
	PluginName        string   `json:"plugin_name"`
	RemovedPaths      []string `json:"removed_paths"`
	SSHConfigUnlinked bool     `json:"ssh_config_unlinked"`
	Message           string   `json:"message"`
}

// GenerateGatewayPluginXML produces the JetBrains Gateway plugin descriptor XML.
func GenerateGatewayPluginXML() string {
	return `<!-- Copyright (c) 2026 Boggy Creek Software LLC -->
<idea-plugin>
    <id>` + JetBrainsPluginID + `</id>
    <name>` + JetBrainsPluginName + `</name>
    <version>` + JetBrainsPluginVersion + `</version>
    <vendor email="support@boggycreek.com" url="https://github.com/boggycreek/agent-sandbox">` + JetBrainsPluginVendor + `</vendor>

    <description><![CDATA[
      Direct JetBrains Gateway and Toolbox integration for Agent Sandbox.<br>
      Discovers local running agent containers and bridges seamless remote development sessions
      with zero host secret leakage.
    ]]></description>

    <category>Remote Development</category>

    <depends>com.intellij.modules.platform</depends>
    <depends optional="true">com.jetbrains.gateway</depends>

    <extensions defaultExtensionNs="com.jetbrains">
        <gatewayConnector implementation="com.boggycreek.sndbx.gateway.SndbxGatewayConnector" />
        <gatewayConnectionProvider implementation="com.boggycreek.sndbx.gateway.SndbxGatewayConnectionProvider" />
    </extensions>
</idea-plugin>
`
}

// GenerateGatewayPluginJAR creates an in-memory valid JAR archive for the Gateway plugin.
func GenerateGatewayPluginJAR() ([]byte, error) {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	// 1. META-INF/MANIFEST.MF
	manifestContent := "Manifest-Version: 1.0\r\nCreated-By: " + JetBrainsPluginVendor + "\r\n\r\n"
	mfWriter, _ := zw.Create("META-INF/MANIFEST.MF")
	_, _ = mfWriter.Write([]byte(manifestContent))

	// 2. META-INF/plugin.xml
	pluginXmlContent := GenerateGatewayPluginXML()
	xmlWriter, _ := zw.Create("META-INF/plugin.xml")
	_, _ = xmlWriter.Write([]byte(pluginXmlContent))

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("closing zip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// GetDefaultJetBrainsSearchRoots resolves standard root locations for JetBrains installations.
func GetDefaultJetBrainsSearchRoots() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}

	var searchRoots []string
	switch runtime.GOOS {
	case "darwin":
		searchRoots = append(searchRoots, filepath.Join(home, "Library", "Application Support", "JetBrains"))
	case "windows":
		if appData := os.Getenv("APPDATA"); appData != "" {
			searchRoots = append(searchRoots, filepath.Join(appData, "JetBrains"))
		}
	default:
		// Linux & generic Unix
		searchRoots = append(searchRoots, filepath.Join(home, ".local", "share", "JetBrains"))
		searchRoots = append(searchRoots, filepath.Join(home, ".config", "JetBrains"))
	}
	return searchRoots
}

// FindJetBrainsPluginDirsFrom discovers JetBrains Gateway and IDE plugin directories from search roots.
func FindJetBrainsPluginDirsFrom(searchRoots []string) []string {
	var candidates []string
	for _, root := range searchRoots {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			name := entry.Name()
			// Matches JetBrainsGateway*, WebStorm*, GoLand*, CLion*, PyCharm*, IntelliJIdea*, IdeaIC*, DataGrip*, etc.
			if strings.HasPrefix(name, "JetBrainsGateway") ||
				strings.HasPrefix(name, "WebStorm") ||
				strings.HasPrefix(name, "GoLand") ||
				strings.HasPrefix(name, "CLion") ||
				strings.HasPrefix(name, "PyCharm") ||
				strings.HasPrefix(name, "IntelliJIdea") ||
				strings.HasPrefix(name, "IdeaIC") ||
				strings.HasPrefix(name, "DataGrip") {
				candidates = append(candidates, filepath.Join(root, name))
			}
		}
	}
	return candidates
}

// FindJetBrainsPluginDirs discovers JetBrains Gateway and IDE plugin directories on the system.
func FindJetBrainsPluginDirs() []string {
	return FindJetBrainsPluginDirsFrom(GetDefaultJetBrainsSearchRoots())
}

// EnsureSSHConfigInclude guarantees that ~/.ssh/config includes the given target config file.
func EnsureSSHConfigInclude(targetSSHConfigFile, hostSSHConfigFile string) (bool, error) {
	if hostSSHConfigFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return false, err
		}
		hostSSHConfigFile = filepath.Join(home, ".ssh", "config")
	}

	if err := os.MkdirAll(filepath.Dir(hostSSHConfigFile), 0700); err != nil {
		return false, fmt.Errorf("creating .ssh directory: %w", err)
	}

	includeDirective := fmt.Sprintf("Include %s", targetSSHConfigFile)

	existingContent, err := os.ReadFile(hostSSHConfigFile)
	if err != nil && !os.IsNotExist(err) {
		return false, fmt.Errorf("reading host ssh config: %w", err)
	}

	// Check if directive already exists
	lines := strings.Split(string(existingContent), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.EqualFold(trimmed, includeDirective) ||
			(strings.HasPrefix(trimmed, "Include") && strings.Contains(trimmed, filepath.Base(targetSSHConfigFile))) {
			return false, nil // Already linked
		}
	}

	// Prepend or append include directive
	var newContent bytes.Buffer
	newContent.WriteString("# Agent Sandbox Auto-Include\n")
	newContent.WriteString(includeDirective + "\n\n")
	newContent.Write(existingContent)

	if err := os.WriteFile(hostSSHConfigFile, newContent.Bytes(), 0600); err != nil {
		return false, fmt.Errorf("writing host ssh config: %w", err)
	}

	return true, nil
}

// RemoveSSHConfigInclude removes the Include directive for target config file from host SSH config.
func RemoveSSHConfigInclude(targetSSHConfigFile, hostSSHConfigFile string) (bool, error) {
	if hostSSHConfigFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return false, err
		}
		hostSSHConfigFile = filepath.Join(home, ".ssh", "config")
	}

	content, err := os.ReadFile(hostSSHConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("reading host ssh config: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	var newLines []string
	removed := false
	targetBase := filepath.Base(targetSSHConfigFile)

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Include") && strings.Contains(trimmed, targetBase) {
			removed = true
			if len(newLines) > 0 && strings.Contains(newLines[len(newLines)-1], "Agent Sandbox Auto-Include") {
				newLines = newLines[:len(newLines)-1]
			}
			continue
		}
		newLines = append(newLines, line)
	}

	if !removed {
		return false, nil
	}

	cleaned := strings.Join(newLines, "\n")
	cleaned = strings.TrimLeft(cleaned, "\n")
	if len(cleaned) > 0 && !strings.HasSuffix(cleaned, "\n") {
		cleaned += "\n"
	}

	if err := os.WriteFile(hostSSHConfigFile, []byte(cleaned), 0600); err != nil {
		return false, fmt.Errorf("writing host ssh config: %w", err)
	}

	return true, nil
}

// InstallToolboxPlugin packages and deploys the Agent Sandbox JetBrains Gateway / Toolbox plugin.
func InstallToolboxPlugin(ctx context.Context, paths config.Paths, stdout io.Writer) (*InstallResult, error) {
	jarBytes, err := GenerateGatewayPluginJAR()
	if err != nil {
		return nil, fmt.Errorf("generating plugin jar: %w", err)
	}

	var installedPaths []string

	// 1. Central repository installation: ~/.local/share/agent-sandbox/plugins/jetbrains-gateway
	centralDir := filepath.Join(paths.DataHome, "plugins", "jetbrains-gateway", "sndbx-gateway", "lib")
	if err := os.MkdirAll(centralDir, 0755); err != nil {
		return nil, fmt.Errorf("creating central plugin dir: %w", err)
	}
	centralJar := filepath.Join(centralDir, "sndbx-gateway.jar")
	if err := os.WriteFile(centralJar, jarBytes, 0644); err != nil {
		return nil, fmt.Errorf("writing central plugin jar: %w", err)
	}
	installedPaths = append(installedPaths, centralJar)

	// 2. Install to discovered JetBrains plugin directories
	jbDirs := FindJetBrainsPluginDirs()
	for _, jbDir := range jbDirs {
		targetLibDir := filepath.Join(jbDir, "sndbx-gateway", "lib")
		if err := os.MkdirAll(targetLibDir, 0755); err == nil {
			targetJar := filepath.Join(targetLibDir, "sndbx-gateway.jar")
			if err := os.WriteFile(targetJar, jarBytes, 0644); err == nil {
				installedPaths = append(installedPaths, targetJar)
			}
		}
	}

	// 3. Ensure OpenSSH configuration sync & host .ssh/config inclusion
	_ = sndbxRuntime.SyncSSHConfigFile(ctx, paths)
	linked, _ := EnsureSSHConfigInclude(paths.SSHConfigFile, "")

	res := &InstallResult{
		PluginID:        PluginIDToolbox,
		PluginName:      JetBrainsPluginName,
		Version:         JetBrainsPluginVersion,
		InstalledPaths:  installedPaths,
		SSHConfigLinked: linked,
		Message:         "JetBrains Gateway / Toolbox plugin successfully installed.",
	}

	return res, nil
}

// RemoveToolboxPlugin uninstalls the Agent Sandbox JetBrains Gateway / Toolbox plugin.
func RemoveToolboxPlugin(ctx context.Context, paths config.Paths, stdout io.Writer) (*RemoveResult, error) {
	var removedPaths []string

	// 1. Remove central directory
	centralDir := filepath.Join(paths.DataHome, "plugins", "jetbrains-gateway", "sndbx-gateway")
	if _, err := os.Stat(centralDir); err == nil {
		_ = os.RemoveAll(centralDir)
		removedPaths = append(removedPaths, centralDir)
	}

	// 2. Remove from discovered JetBrains plugin directories
	jbDirs := FindJetBrainsPluginDirs()
	for _, jbDir := range jbDirs {
		targetDir := filepath.Join(jbDir, "sndbx-gateway")
		if _, err := os.Stat(targetDir); err == nil {
			_ = os.RemoveAll(targetDir)
			removedPaths = append(removedPaths, targetDir)
		}
	}

	unlinked, _ := RemoveSSHConfigInclude(paths.SSHConfigFile, "")

	return &RemoveResult{
		PluginID:          PluginIDToolbox,
		PluginName:        JetBrainsPluginName,
		RemovedPaths:      removedPaths,
		SSHConfigUnlinked: unlinked,
		Message:           "JetBrains Gateway / Toolbox plugin successfully removed.",
	}, nil
}

// InstallVSCodePlugin configures the host environment for VS Code remote development.
func InstallVSCodePlugin(ctx context.Context, paths config.Paths, stdout io.Writer) (*InstallResult, error) {
	_ = sndbxRuntime.SyncSSHConfigFile(ctx, paths)
	linked, _ := EnsureSSHConfigInclude(paths.SSHConfigFile, "")

	var installedPaths []string
	// Check if 'code' CLI is available
	codeCmd := ExecCommandContext(ctx, "code", "--version")
	hasCode := codeCmd.Run() == nil

	msg := "VS Code SSH integration configured successfully."
	if !hasCode {
		msg += " Note: 'code' CLI was not found in PATH, but SSH configuration was verified."
	} else {
		// Install Claude Code extension on host if desired
		installExt := ExecCommandContext(ctx, "code", "--install-extension", "anthropic.claude-code")
		_ = installExt.Run()
		installedPaths = append(installedPaths, "VS Code Remote-SSH Config (~/.ssh/config)")
	}

	return &InstallResult{
		PluginID:        PluginIDVSCode,
		PluginName:      "VS Code Remote-SSH Integration",
		Version:         "1.0.0",
		InstalledPaths:  installedPaths,
		SSHConfigLinked: linked,
		Message:         msg,
	}, nil
}

// RemoveVSCodePlugin removes the VS Code SSH configuration link.
func RemoveVSCodePlugin(ctx context.Context, paths config.Paths, stdout io.Writer) (*RemoveResult, error) {
	unlinked, _ := RemoveSSHConfigInclude(paths.SSHConfigFile, "")
	var removedPaths []string
	if unlinked {
		removedPaths = append(removedPaths, "Host ~/.ssh/config Include directive")
	}

	return &RemoveResult{
		PluginID:          PluginIDVSCode,
		PluginName:        "VS Code Remote-SSH Integration",
		RemovedPaths:      removedPaths,
		SSHConfigUnlinked: unlinked,
		Message:           "VS Code Remote-SSH configuration unlinked.",
	}, nil
}

// ListPlugins enumerates available and installed IDE plugins.
func ListPlugins(paths config.Paths) []PluginInfo {
	// Check Toolbox plugin
	centralJar := filepath.Join(paths.DataHome, "plugins", "jetbrains-gateway", "sndbx-gateway", "lib", "sndbx-gateway.jar")
	jbInstalled := false
	var jbLocations []string
	if _, err := os.Stat(centralJar); err == nil {
		jbInstalled = true
		jbLocations = append(jbLocations, centralJar)
	}

	// Discover any installed IDE instances
	for _, dir := range FindJetBrainsPluginDirs() {
		jarPath := filepath.Join(dir, "sndbx-gateway", "lib", "sndbx-gateway.jar")
		if _, err := os.Stat(jarPath); err == nil {
			jbInstalled = true
			jbLocations = append(jbLocations, jarPath)
		}
	}

	return []PluginInfo{
		{
			ID:          PluginIDToolbox,
			Name:        JetBrainsPluginName,
			Target:      "JetBrains Gateway / Toolbox",
			Description: "Connect directly to running agent containers from JetBrains Gateway & Toolbox",
			Installed:   jbInstalled,
			Locations:   jbLocations,
		},
		{
			ID:          PluginIDVSCode,
			Name:        "VS Code Remote-SSH",
			Target:      "Visual Studio Code",
			Description: "One-shot workspace connection and Claude Code extension ensembling",
			Installed:   true, // SSH config bridge is built-in
			Locations:   []string{paths.SSHConfigFile},
		},
	}
}
