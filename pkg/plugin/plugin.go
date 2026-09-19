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
	"runtime"
	"strings"

	"github.com/boggycreek/agent-sandbox/pkg/config"
	sndbxRuntime "github.com/boggycreek/agent-sandbox/pkg/runtime"
)

// Plugin identifiers and constants.
const (
	PluginIDGateway = "gateway"
	PluginIDToolbox = "toolbox"
	PluginIDVSCode  = "vscode"

	JetBrainsPluginID      = "com.boggycreek.sndbx.gateway"
	JetBrainsPluginName    = "Agent Sandbox Gateway"
	JetBrainsPluginVersion = "0.1.0-alpha"
	JetBrainsPluginVendor  = "Boggy Creek Software LLC"

	ToolboxPluginID         = "com.boggycreek.sndbx.toolbox"
	ToolboxPluginName       = "Agent Sandbox Toolbox"
	ToolboxPluginVersion    = "0.1.0-alpha"
	ToolboxPluginAPIVersion = "1.13.87111"

	ManagedSSHConfigHeader = "# --- BEGIN AGENT SANDBOX MANAGED HOSTS ---"
	ManagedSSHConfigFooter = "# --- END AGENT SANDBOX MANAGED HOSTS ---"
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

// ToolboxSettingsPayload represents the inner structure of 'envsJson' in JetBrains Toolbox ssh/settings.json.
type ToolboxSettingsPayload struct {
	Remotes []ToolboxRemoteEntry `json:"remotes"`
}

// ToolboxRemoteEntry represents an individual remote host definition in JetBrains Toolbox.
type ToolboxRemoteEntry struct {
	Host                     string `json:"host"`
	Port                     int    `json:"port"`
	UserName                 string `json:"userName"`
	ShouldUseSystemSshAgent  bool   `json:"shouldUseSystemSshAgent"`
	OriginalConnectionString string `json:"originalConnectionString"`
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
      Direct JetBrains Gateway integration for Agent Sandbox.<br>
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
	mfWriter, err := zw.Create("META-INF/MANIFEST.MF")
	if err != nil {
		return nil, fmt.Errorf("creating manifest entry: %w", err)
	}
	if _, err := mfWriter.Write([]byte(manifestContent)); err != nil {
		return nil, fmt.Errorf("writing manifest entry: %w", err)
	}

	// 2. META-INF/plugin.xml
	pluginXmlContent := GenerateGatewayPluginXML()
	xmlWriter, err := zw.Create("META-INF/plugin.xml")
	if err != nil {
		return nil, fmt.Errorf("creating plugin.xml entry: %w", err)
	}
	if _, err := xmlWriter.Write([]byte(pluginXmlContent)); err != nil {
		return nil, fmt.Errorf("writing plugin.xml entry: %w", err)
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("closing zip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// GenerateToolboxExtensionJSON creates the manifest descriptor required by JetBrains Toolbox.
func GenerateToolboxExtensionJSON() string {
	return `{
  "id": "` + ToolboxPluginID + `",
  "version": "` + ToolboxPluginVersion + `",
  "apiVersion": "` + ToolboxPluginAPIVersion + `",
  "meta": {
    "readableName": "Agent Sandbox",
    "description": "Direct JetBrains Toolbox integration for local agent sandboxes",
    "vendor": "` + JetBrainsPluginVendor + `",
    "url": "https://github.com/boggycreek/agent-sandbox"
  }
}
`
}

// GenerateToolboxPluginIconSVG returns a standard SVG icon for JetBrains Toolbox.
func GenerateToolboxPluginIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32" width="32" height="32">
  <rect width="32" height="32" rx="6" fill="#1E1F22"/>
  <path d="M7 16L16 7l9 9-9 9-9-9z" fill="none" stroke="#3574F0" stroke-width="2.5" stroke-linejoin="round"/>
  <circle cx="16" cy="16" r="3.5" fill="#3574F0"/>
</svg>
`
}

// GenerateToolboxPluginJAR creates an in-memory valid JAR archive for the Toolbox plugin.
func GenerateToolboxPluginJAR() ([]byte, error) {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	// 1. META-INF/MANIFEST.MF
	manifestContent := "Manifest-Version: 1.0\r\nCreated-By: " + JetBrainsPluginVendor + "\r\n\r\n"
	mfWriter, err := zw.Create("META-INF/MANIFEST.MF")
	if err != nil {
		return nil, fmt.Errorf("creating manifest entry: %w", err)
	}
	if _, err := mfWriter.Write([]byte(manifestContent)); err != nil {
		return nil, fmt.Errorf("writing manifest entry: %w", err)
	}

	// 2. extension.json
	extWriter, err := zw.Create("extension.json")
	if err != nil {
		return nil, fmt.Errorf("creating extension.json entry: %w", err)
	}
	if _, err := extWriter.Write([]byte(GenerateToolboxExtensionJSON())); err != nil {
		return nil, fmt.Errorf("writing extension.json entry: %w", err)
	}

	// 3. pluginIcon.svg
	iconWriter, err := zw.Create("pluginIcon.svg")
	if err != nil {
		return nil, fmt.Errorf("creating pluginIcon.svg entry: %w", err)
	}
	if _, err := iconWriter.Write([]byte(GenerateToolboxPluginIconSVG())); err != nil {
		return nil, fmt.Errorf("writing pluginIcon.svg entry: %w", err)
	}

	// 4. META-INF/services/com.jetbrains.toolbox.api.remoteDev.RemoteDevExtension
	serviceWriter, err := zw.Create("META-INF/services/com.jetbrains.toolbox.api.remoteDev.RemoteDevExtension")
	if err != nil {
		return nil, fmt.Errorf("creating service entry: %w", err)
	}
	serviceContent := "com.boggycreek.sndbx.toolbox.SndbxRemoteDevExtension\n"
	if _, err := serviceWriter.Write([]byte(serviceContent)); err != nil {
		return nil, fmt.Errorf("writing service entry: %w", err)
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("closing zip writer: %w", err)
	}

	return buf.Bytes(), nil
}

// GetDefaultJetBrainsSearchRoots resolves standard root locations for JetBrains IDE installations.
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

// GetDefaultToolboxPluginDir returns the target plugin directory for JetBrains Toolbox App.
func GetDefaultToolboxPluginDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "JetBrains", "Toolbox", "plugins", "sndbx")
	case "windows":
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "JetBrains", "Toolbox", "plugins", "sndbx")
		}
		return filepath.Join(home, "AppData", "Local", "JetBrains", "Toolbox", "plugins", "sndbx")
	default:
		return filepath.Join(home, ".local", "share", "JetBrains", "Toolbox", "plugins", "sndbx")
	}
}

// GetDefaultToolboxSSHSettingsPath returns the location of JetBrains Toolbox SSH provider settings.
func GetDefaultToolboxSSHSettingsPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "JetBrains", "Toolbox", "plugins", "ssh", "settings.json")
	case "windows":
		if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
			return filepath.Join(localAppData, "JetBrains", "Toolbox", "plugins", "ssh", "settings.json")
		}
		return filepath.Join(home, "AppData", "Local", "JetBrains", "Toolbox", "plugins", "ssh", "settings.json")
	default:
		return filepath.Join(home, ".local", "share", "JetBrains", "Toolbox", "plugins", "ssh", "settings.json")
	}
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

	lines := strings.Split(string(existingContent), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.EqualFold(trimmed, includeDirective) ||
			(strings.HasPrefix(trimmed, "Include") && strings.Contains(trimmed, filepath.Base(targetSSHConfigFile))) {
			return false, nil // Already linked
		}
	}

	var newContent bytes.Buffer
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

// EnsureSSHConfigManagedHosts inlines host blocks directly into ~/.ssh/config for line-based importers.
func EnsureSSHConfigManagedHosts(ctx context.Context, paths config.Paths, hostSSHConfigFile string) error {
	if hostSSHConfigFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		hostSSHConfigFile = filepath.Join(home, ".ssh", "config")
	}

	configs, err := config.ListAgentConfigs(paths)
	if err != nil {
		return err
	}

	var blockBuf bytes.Buffer
	blockBuf.WriteString(ManagedSSHConfigHeader + "\n")
	blockBuf.WriteString("# Auto-generated by sndbx for tools (like JetBrains Toolbox) that do not support OpenSSH 'Include'.\n")

	for _, c := range configs {
		port, err := sndbxRuntime.GetAgentSSHPort(ctx, c.ContainerName)
		if err == nil && port > 0 {
			blockBuf.WriteString("\n")
			blockBuf.WriteString(sndbxRuntime.FormatSSHConfigBlock(c.Name, port, paths.IDEKeyFile))
			blockBuf.WriteString("\n")
		}
	}
	blockBuf.WriteString(ManagedSSHConfigFooter)
	blockContent := blockBuf.String()

	if err := os.MkdirAll(filepath.Dir(hostSSHConfigFile), 0700); err != nil {
		return fmt.Errorf("creating .ssh directory: %w", err)
	}

	content, err := os.ReadFile(hostSSHConfigFile)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("reading host ssh config: %w", err)
	}

	strContent := string(content)
	startIdx := strings.Index(strContent, ManagedSSHConfigHeader)
	endIdx := strings.Index(strContent, ManagedSSHConfigFooter)

	var updated string
	if startIdx != -1 && endIdx != -1 && endIdx >= startIdx {
		afterBlock := endIdx + len(ManagedSSHConfigFooter)
		if afterBlock < len(strContent) && strContent[afterBlock] == '\n' {
			afterBlock++
		}
		updated = strContent[:startIdx] + blockContent + "\n" + strContent[afterBlock:]
	} else {
		if len(strContent) > 0 && !strings.HasSuffix(strContent, "\n") {
			strContent += "\n"
		}
		updated = strContent + "\n" + blockContent + "\n"
	}

	return os.WriteFile(hostSSHConfigFile, []byte(strings.TrimSpace(updated)+"\n"), 0600)
}

// RemoveSSHConfigManagedHosts removes inlined host blocks from ~/.ssh/config.
func RemoveSSHConfigManagedHosts(hostSSHConfigFile string) error {
	if hostSSHConfigFile == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		hostSSHConfigFile = filepath.Join(home, ".ssh", "config")
	}

	content, err := os.ReadFile(hostSSHConfigFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading host ssh config: %w", err)
	}

	strContent := string(content)
	startIdx := strings.Index(strContent, ManagedSSHConfigHeader)
	endIdx := strings.Index(strContent, ManagedSSHConfigFooter)

	if startIdx == -1 || endIdx == -1 || endIdx < startIdx {
		return nil
	}

	afterBlock := endIdx + len(ManagedSSHConfigFooter)
	if afterBlock < len(strContent) && strContent[afterBlock] == '\n' {
		afterBlock++
	}

	updated := strContent[:startIdx] + strContent[afterBlock:]
	trimmed := strings.TrimSpace(updated)
	if len(trimmed) > 0 {
		trimmed += "\n"
	}
	return os.WriteFile(hostSSHConfigFile, []byte(trimmed), 0600)
}

// SyncToolboxSSHSettings updates ~/.local/share/JetBrains/Toolbox/plugins/ssh/settings.json with active sandboxes.
func SyncToolboxSSHSettings(ctx context.Context, paths config.Paths, settingsPath string) error {
	if settingsPath == "" {
		settingsPath = GetDefaultToolboxSSHSettingsPath()
	}
	if settingsPath == "" {
		return nil
	}

	// If Toolbox plugins directory doesn't exist, skip sync without error
	toolboxPluginsDir := filepath.Dir(filepath.Dir(settingsPath))
	if _, err := os.Stat(toolboxPluginsDir); os.IsNotExist(err) {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(settingsPath), 0755); err != nil {
		return fmt.Errorf("creating toolbox ssh directory: %w", err)
	}

	var root map[string]any
	if data, err := os.ReadFile(settingsPath); err == nil {
		_ = json.Unmarshal(data, &root)
	}
	if root == nil {
		root = make(map[string]any)
	}

	var payload ToolboxSettingsPayload
	if envsStr, ok := root["envsJson"].(string); ok && envsStr != "" {
		_ = json.Unmarshal([]byte(envsStr), &payload)
	}

	cacheMap := make(map[string]ToolboxRemoteEntry)
	if cacheStr, ok := root["CONNECTION_STRINGS_CACHE"].(string); ok && cacheStr != "" {
		_ = json.Unmarshal([]byte(cacheStr), &cacheMap)
	}

	// Filter out existing sndbx- entries
	var keptRemotes []ToolboxRemoteEntry
	for _, r := range payload.Remotes {
		if !strings.HasPrefix(r.Host, "sndbx-") {
			keptRemotes = append(keptRemotes, r)
		}
	}
	for k := range cacheMap {
		if strings.Contains(k, "sndbx-") {
			delete(cacheMap, k)
		}
	}
	for k := range root {
		if strings.HasPrefix(k, "TBX_DEPLOYMENT_TARGET_ENV") && strings.Contains(k, "sndbx-") {
			delete(root, k)
		}
		if strings.HasPrefix(k, "TBX_AUTO_CONNECT_ENV") && strings.Contains(k, "sndbx-") {
			delete(root, k)
		}
	}

	// Add running agents
	configs, _ := config.ListAgentConfigs(paths)
	for _, c := range configs {
		port, err := sndbxRuntime.GetAgentSSHPort(ctx, c.ContainerName)
		if err == nil && port > 0 {
			host := "sndbx-" + c.Name
			connStr := "agent@" + host
			entry := ToolboxRemoteEntry{
				Host:                     host,
				Port:                     0,
				UserName:                 "agent",
				ShouldUseSystemSshAgent:  true,
				OriginalConnectionString: connStr,
			}
			keptRemotes = append(keptRemotes, entry)
			cacheMap[connStr] = entry
			root["TBX_DEPLOYMENT_TARGET_ENV"+connStr] = "1"
			root["TBX_AUTO_CONNECT_ENV"+connStr] = "false"
		}
	}

	payload.Remotes = keptRemotes
	envsBytes, err := json.MarshalIndent(payload, "", "    ")
	if err == nil {
		root["envsJson"] = string(envsBytes)
	}
	cacheBytes, err := json.MarshalIndent(cacheMap, "", "    ")
	if err == nil {
		root["CONNECTION_STRINGS_CACHE"] = string(cacheBytes)
	}
	if _, ok := root["configPath"]; !ok {
		root["configPath"] = ""
	}
	if _, ok := root["REJECTED_HINTS"]; !ok {
		root["REJECTED_HINTS"] = "[]"
	}
	if _, ok := root["timeoutSeconds"]; !ok {
		root["timeoutSeconds"] = "10"
	}

	outBytes, err := json.MarshalIndent(root, "", "    ")
	if err != nil {
		return fmt.Errorf("marshaling toolbox ssh settings: %w", err)
	}

	return os.WriteFile(settingsPath, outBytes, 0600)
}

// RemoveToolboxSSHSettings removes sndbx remote entries from Toolbox ssh/settings.json.
func RemoveToolboxSSHSettings(settingsPath string) error {
	if settingsPath == "" {
		settingsPath = GetDefaultToolboxSSHSettingsPath()
	}
	if settingsPath == "" {
		return nil
	}

	data, err := os.ReadFile(settingsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading toolbox ssh settings: %w", err)
	}

	var root map[string]any
	if err := json.Unmarshal(data, &root); err != nil {
		return nil
	}

	var payload ToolboxSettingsPayload
	if envsStr, ok := root["envsJson"].(string); ok && envsStr != "" {
		_ = json.Unmarshal([]byte(envsStr), &payload)
	}

	cacheMap := make(map[string]ToolboxRemoteEntry)
	if cacheStr, ok := root["CONNECTION_STRINGS_CACHE"].(string); ok && cacheStr != "" {
		_ = json.Unmarshal([]byte(cacheStr), &cacheMap)
	}

	var keptRemotes []ToolboxRemoteEntry
	for _, r := range payload.Remotes {
		if !strings.HasPrefix(r.Host, "sndbx-") {
			keptRemotes = append(keptRemotes, r)
		}
	}
	for k := range cacheMap {
		if strings.Contains(k, "sndbx-") {
			delete(cacheMap, k)
		}
	}
	for k := range root {
		if strings.HasPrefix(k, "TBX_DEPLOYMENT_TARGET_ENV") && strings.Contains(k, "sndbx-") {
			delete(root, k)
		}
		if strings.HasPrefix(k, "TBX_AUTO_CONNECT_ENV") && strings.Contains(k, "sndbx-") {
			delete(root, k)
		}
	}

	payload.Remotes = keptRemotes
	if envsBytes, err := json.MarshalIndent(payload, "", "    "); err == nil {
		root["envsJson"] = string(envsBytes)
	}
	if cacheBytes, err := json.MarshalIndent(cacheMap, "", "    "); err == nil {
		root["CONNECTION_STRINGS_CACHE"] = string(cacheBytes)
	}

	outBytes, err := json.MarshalIndent(root, "", "    ")
	if err != nil {
		return fmt.Errorf("marshaling toolbox ssh settings: %w", err)
	}

	return os.WriteFile(settingsPath, outBytes, 0600)
}

// InstallGatewayPlugin packages and deploys the Agent Sandbox JetBrains Gateway IntelliJ plugin.
func InstallGatewayPlugin(ctx context.Context, paths config.Paths, stdout io.Writer) (*InstallResult, error) {
	jarBytes, err := GenerateGatewayPluginJAR()
	if err != nil {
		return nil, fmt.Errorf("generating gateway plugin jar: %w", err)
	}

	var installedPaths []string

	// 1. Central repository installation: ~/.local/share/agent-sandbox/plugins/jetbrains-gateway
	centralDir := filepath.Join(paths.DataHome, "plugins", "jetbrains-gateway", "sndbx-gateway", "lib")
	if err := os.MkdirAll(centralDir, 0755); err != nil {
		return nil, fmt.Errorf("creating central gateway plugin dir: %w", err)
	}
	centralJar := filepath.Join(centralDir, "sndbx-gateway.jar")
	if err := os.WriteFile(centralJar, jarBytes, 0644); err != nil {
		return nil, fmt.Errorf("writing central gateway plugin jar: %w", err)
	}
	installedPaths = append(installedPaths, centralJar)

	// 2. Discovered JetBrains IDE plugin directories
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

	return &InstallResult{
		PluginID:        PluginIDGateway,
		PluginName:      JetBrainsPluginName,
		Version:         JetBrainsPluginVersion,
		InstalledPaths:  installedPaths,
		SSHConfigLinked: linked,
		Message:         "JetBrains Gateway plugin successfully installed.",
	}, nil
}

// RemoveGatewayPlugin uninstalls the Agent Sandbox JetBrains Gateway plugin.
func RemoveGatewayPlugin(ctx context.Context, paths config.Paths, stdout io.Writer) (*RemoveResult, error) {
	var removedPaths []string

	centralDir := filepath.Join(paths.DataHome, "plugins", "jetbrains-gateway", "sndbx-gateway")
	if _, err := os.Stat(centralDir); err == nil {
		_ = os.RemoveAll(centralDir)
		removedPaths = append(removedPaths, centralDir)
	}

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
		PluginID:          PluginIDGateway,
		PluginName:        JetBrainsPluginName,
		RemovedPaths:      removedPaths,
		SSHConfigUnlinked: unlinked,
		Message:           "JetBrains Gateway plugin successfully removed.",
	}, nil
}

// InstallToolboxPlugin packages and deploys the Agent Sandbox JetBrains Toolbox App integration.
func InstallToolboxPlugin(ctx context.Context, paths config.Paths, stdout io.Writer) (*InstallResult, error) {
	var installedPaths []string

	// 1. Central repository directory: ~/.local/share/agent-sandbox/plugins/jetbrains-toolbox/sndbx
	centralDir := filepath.Join(paths.DataHome, "plugins", "jetbrains-toolbox", "sndbx")
	if err := os.MkdirAll(centralDir, 0755); err != nil {
		return nil, fmt.Errorf("creating central toolbox plugin dir: %w", err)
	}
	_ = os.WriteFile(filepath.Join(centralDir, "extension.json"), []byte(GenerateToolboxExtensionJSON()), 0644)
	_ = os.WriteFile(filepath.Join(centralDir, "pluginIcon.svg"), []byte(GenerateToolboxPluginIconSVG()), 0644)
	if jarBytes, err := GenerateToolboxPluginJAR(); err == nil {
		centralJar := filepath.Join(centralDir, "sndbx-toolbox.jar")
		_ = os.WriteFile(centralJar, jarBytes, 0644)
		installedPaths = append(installedPaths, centralJar)
	}

	// 2. Deploy directly to JetBrains Toolbox external plugins folder
	toolboxDir := GetDefaultToolboxPluginDir()
	if toolboxDir != "" {
		if err := os.MkdirAll(toolboxDir, 0755); err == nil {
			extFile := filepath.Join(toolboxDir, "extension.json")
			iconFile := filepath.Join(toolboxDir, "pluginIcon.svg")
			jarFile := filepath.Join(toolboxDir, "sndbx-toolbox.jar")
			_ = os.WriteFile(extFile, []byte(GenerateToolboxExtensionJSON()), 0644)
			_ = os.WriteFile(iconFile, []byte(GenerateToolboxPluginIconSVG()), 0644)
			if jarBytes, err := GenerateToolboxPluginJAR(); err == nil {
				_ = os.WriteFile(jarFile, jarBytes, 0644)
			}
			installedPaths = append(installedPaths, toolboxDir)
		}
	}

	// 3. Synchronize Toolbox SSH provider settings and inline host blocks
	_ = sndbxRuntime.SyncSSHConfigFile(ctx, paths)
	linked, _ := EnsureSSHConfigInclude(paths.SSHConfigFile, "")
	_ = EnsureSSHConfigManagedHosts(ctx, paths, "")
	_ = SyncToolboxSSHSettings(ctx, paths, "")

	return &InstallResult{
		PluginID:        PluginIDToolbox,
		PluginName:      ToolboxPluginName,
		Version:         ToolboxPluginVersion,
		InstalledPaths:  installedPaths,
		SSHConfigLinked: linked,
		Message:         "JetBrains Toolbox plugin and native SSH synchronization successfully installed.",
	}, nil
}

// RemoveToolboxPlugin uninstalls the Agent Sandbox JetBrains Toolbox App integration.
func RemoveToolboxPlugin(ctx context.Context, paths config.Paths, stdout io.Writer) (*RemoveResult, error) {
	var removedPaths []string

	centralDir := filepath.Join(paths.DataHome, "plugins", "jetbrains-toolbox", "sndbx")
	if _, err := os.Stat(centralDir); err == nil {
		_ = os.RemoveAll(centralDir)
		removedPaths = append(removedPaths, centralDir)
	}

	toolboxDir := GetDefaultToolboxPluginDir()
	if toolboxDir != "" {
		if _, err := os.Stat(toolboxDir); err == nil {
			_ = os.RemoveAll(toolboxDir)
			removedPaths = append(removedPaths, toolboxDir)
		}
	}

	_ = RemoveSSHConfigManagedHosts("")
	_ = RemoveToolboxSSHSettings("")
	unlinked, _ := RemoveSSHConfigInclude(paths.SSHConfigFile, "")

	return &RemoveResult{
		PluginID:          PluginIDToolbox,
		PluginName:        ToolboxPluginName,
		RemovedPaths:      removedPaths,
		SSHConfigUnlinked: unlinked,
		Message:           "JetBrains Toolbox plugin and native SSH synchronization successfully removed.",
	}, nil
}

// InstallVSCodePlugin configures the host environment for VS Code remote development.
func InstallVSCodePlugin(ctx context.Context, paths config.Paths, stdout io.Writer) (*InstallResult, error) {
	_ = sndbxRuntime.SyncSSHConfigFile(ctx, paths)
	linked, _ := EnsureSSHConfigInclude(paths.SSHConfigFile, "")

	var installedPaths []string
	codeCmd := ExecCommandContext(ctx, "code", "--version")
	hasCode := codeCmd.Run() == nil

	msg := "VS Code SSH integration configured successfully."
	if !hasCode {
		msg += " Note: 'code' CLI was not found in PATH, but SSH configuration was verified."
	} else {
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
	// Check Gateway plugin
	centralGatewayJar := filepath.Join(paths.DataHome, "plugins", "jetbrains-gateway", "sndbx-gateway", "lib", "sndbx-gateway.jar")
	gatewayInstalled := false
	var gatewayLocations []string
	if _, err := os.Stat(centralGatewayJar); err == nil {
		gatewayInstalled = true
		gatewayLocations = append(gatewayLocations, centralGatewayJar)
	}
	for _, dir := range FindJetBrainsPluginDirs() {
		jarPath := filepath.Join(dir, "sndbx-gateway", "lib", "sndbx-gateway.jar")
		if _, err := os.Stat(jarPath); err == nil {
			gatewayInstalled = true
			gatewayLocations = append(gatewayLocations, jarPath)
		}
	}

	// Check Toolbox plugin
	centralToolboxDir := filepath.Join(paths.DataHome, "plugins", "jetbrains-toolbox", "sndbx")
	toolboxInstalled := false
	var toolboxLocations []string
	if _, err := os.Stat(centralToolboxDir); err == nil {
		toolboxInstalled = true
		toolboxLocations = append(toolboxLocations, centralToolboxDir)
	}
	activeToolboxDir := GetDefaultToolboxPluginDir()
	if activeToolboxDir != "" {
		if _, err := os.Stat(activeToolboxDir); err == nil {
			toolboxInstalled = true
			toolboxLocations = append(toolboxLocations, activeToolboxDir)
		}
	}

	return []PluginInfo{
		{
			ID:          PluginIDGateway,
			Name:        JetBrainsPluginName,
			Target:      "JetBrains Gateway / IDEs",
			Description: "Direct IntelliJ Gateway connector for remote IDE development in container",
			Installed:   gatewayInstalled,
			Locations:   gatewayLocations,
		},
		{
			ID:          PluginIDToolbox,
			Name:        ToolboxPluginName,
			Target:      "JetBrains Toolbox",
			Description: "Direct JetBrains Toolbox provider and native SSH remote synchronization",
			Installed:   toolboxInstalled,
			Locations:   toolboxLocations,
		},
		{
			ID:          PluginIDVSCode,
			Name:        "VS Code Remote-SSH",
			Target:      "Visual Studio Code",
			Description: "One-shot workspace connection and Claude Code extension ensembling",
			Installed:   true,
			Locations:   []string{paths.SSHConfigFile},
		},
	}
}
