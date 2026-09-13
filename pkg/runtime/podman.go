// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/boggycreek/agent-sandbox/pkg/config"
	"github.com/boggycreek/agent-sandbox/pkg/gitea"
)

// infraValkeyContainer and infraGiteaContainer are the expected shared infra container names.
// These are package-level vars so tests can override them with ephemeral container names.
var (
	infraValkeyContainer = "agent-sandbox-valkey"
	infraGiteaContainer  = "agent-sandbox-gitea"
)

// ContainerInfo describes a container's runtime state
type ContainerInfo struct {
	ID      string `json:"Id"`
	Names   []string `json:"Names"`
	State   string `json:"State"`
	Status  string `json:"Status"`
	Created int64  `json:"Created"`
	Ports   []PortMapping `json:"Ports"`
}

// PortMapping port mapping information
type PortMapping struct {
	HostPort      int `json:"hostPort"`
	ContainerPort int `json:"containerPort"`
	HostIP        string `json:"hostIP"`
}

// AgentStatus combines config and container state
type AgentStatus struct {
	Name          string `json:"name"`
	Role          string `json:"role"`
	Image         string `json:"image"`
	ContainerName string `json:"container_name"`
	ContainerState string `json:"container_state"`
	SSHPort       int    `json:"ssh_port"`
	IDEConnect    string `json:"ide_connect"`
}

// ResolveAgentImage resolves an image input according to ADR 00029:
// 1. Well-known presets (base, opencode, claude, agy)
// 2. Local Podman store images (matching local tags or localhost/ prefix)
// 3. Remote OCI image references
func ResolveAgentImage(ctx context.Context, input string) (image string, isLocal bool) {
	lower := strings.ToLower(strings.TrimSpace(input))
	if preset, ok := config.WellKnownImages[lower]; ok {
		// Check if the preset image exists locally
		checkCmd := exec.CommandContext(ctx, "podman", "image", "exists", preset)
		return preset, checkCmd.Run() == nil
	}
	if lower == "" {
		preset := config.WellKnownImages["base"]
		checkCmd := exec.CommandContext(ctx, "podman", "image", "exists", preset)
		return preset, checkCmd.Run() == nil
	}

	// Tier 2: Check local Podman storage
	candidates := []string{input}
	if !strings.Contains(input, ":") {
		candidates = append(candidates, input+":latest")
	}
	if !strings.HasPrefix(input, "localhost/") {
		candidates = append(candidates, "localhost/"+input)
		if !strings.Contains(input, ":") {
			candidates = append(candidates, "localhost/"+input+":latest")
		}
	}

	for _, cand := range candidates {
		checkCmd := exec.CommandContext(ctx, "podman", "image", "exists", cand)
		if err := checkCmd.Run(); err == nil {
			return cand, true
		}
	}

	// Tier 3: External OCI image reference
	resolved := input
	if !strings.Contains(resolved, ":") && !strings.Contains(resolved, "@") {
		resolved = resolved + ":latest"
	}
	checkCmd := exec.CommandContext(ctx, "podman", "image", "exists", resolved)
	return resolved, checkCmd.Run() == nil
}

// EnsureNetwork creates the bridge network if it does not already exist
func EnsureNetwork(ctx context.Context, netName string) error {
	cmd := exec.CommandContext(ctx, "podman", "network", "exists", netName)
	if err := cmd.Run(); err == nil {
		return nil
	}
	createCmd := exec.CommandContext(ctx, "podman", "network", "create", netName)
	return createCmd.Run()
}

// EnsureVolume creates a named volume if missing
func EnsureVolume(ctx context.Context, volName string) error {
	cmd := exec.CommandContext(ctx, "podman", "volume", "exists", volName)
	if err := cmd.Run(); err == nil {
		return nil
	}
	createCmd := exec.CommandContext(ctx, "podman", "volume", "create", volName)
	return createCmd.Run()
}

// StartAgentContainer runs or starts an agent container with Podman
func StartAgentContainer(ctx context.Context, cfg *config.AgentConfig, paths config.Paths, bpHost string, bpPort int) error {
	netName := "agent-sandbox-infra"
	_ = EnsureNetwork(ctx, netName)
	_ = EnsureVolume(ctx, cfg.VolumeName)

	// Check if container already exists
	checkCmd := exec.CommandContext(ctx, "podman", "container", "exists", cfg.ContainerName)
	if err := checkCmd.Run(); err == nil {
		// Container exists, start if stopped
		startCmd := exec.CommandContext(ctx, "podman", "start", cfg.ContainerName)
		if out, err := startCmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed starting existing container %s: %v (%s)", cfg.ContainerName, err, string(out))
		}
		return nil
	}

	// Build run arguments
	sshKeyPub := fmt.Sprintf("%s.pub", paths.IDEKeyFile)
	var mounts []string
	if _, err := os.Stat(sshKeyPub); err == nil {
		mounts = append(mounts, "-v", fmt.Sprintf("%s:/tmp/host-keys/agent-sandbox.pub:ro", sshKeyPub))
	}
	mounts = append(mounts, "-v", fmt.Sprintf("%s:/home/agent:z", cfg.VolumeName))

	containerBPHost := bpHost
	if containerBPHost == "localhost" || containerBPHost == "127.0.0.1" || containerBPHost == "::1" || containerBPHost == "" {
		containerBPHost = infraValkeyContainer
	}

	args := []string{
		"run", "-d",
		"--name", cfg.ContainerName,
		"--hostname", fmt.Sprintf("%s-sandbox", cfg.Name),
		"--network", netName,
		"--add-host", "llm-gateway:host-gateway",
		"-p", "127.0.0.1::2222",
		"-e", fmt.Sprintf("AGENT_NAME=%s", cfg.Name),
		"-e", fmt.Sprintf("BP_AGENT=%s", cfg.Name),
		"-e", fmt.Sprintf("BP_PASSWORD=%s", cfg.Password),
		"-e", fmt.Sprintf("BP_HOST=%s", containerBPHost),
		"-e", fmt.Sprintf("BP_PORT=%d", bpPort),
		"-e", fmt.Sprintf("BP_SIGNING_KEY_PEM=%s", cfg.SigningKeyPEM),
	}

	if cfg.ModelURL != "" {
		// Translate localhost / 127.0.0.1 to llm-gateway for container-to-host bridge access
		containerModelURL := cfg.ModelURL
		containerModelURL = strings.ReplaceAll(containerModelURL, "://localhost", "://llm-gateway")
		containerModelURL = strings.ReplaceAll(containerModelURL, "://127.0.0.1", "://llm-gateway")

		args = append(args,
			"-e", fmt.Sprintf("OPENAI_BASE_URL=%s", containerModelURL),
			"-e", fmt.Sprintf("OPENAI_API_BASE=%s", containerModelURL),
			"-e", fmt.Sprintf("MODEL_URL=%s", containerModelURL),
		)
	}

	if cfg.ModelName != "" {
		args = append(args,
			"-e", fmt.Sprintf("MODEL_NAME=%s", cfg.ModelName),
			"-e", fmt.Sprintf("OPENAI_MODEL=%s", cfg.ModelName),
			"-e", fmt.Sprintf("LLM_MODEL=%s", cfg.ModelName),
		)
	}

	if cfg.ModelAPIKey != "" {
		args = append(args,
			"-e", fmt.Sprintf("OPENAI_API_KEY=%s", cfg.ModelAPIKey),
		)
	} else if cfg.ModelURL != "" {
		// Default dummy key for endpoints that require header presence (e.g. LiteLLM/vLLM)
		args = append(args, "-e", "OPENAI_API_KEY=local-openai-key")
	}

	args = append(args, mounts...)
	args = append(args, cfg.Image)

	cmd := exec.CommandContext(ctx, "podman", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed running podman container %s with image %s: %v (%s)", cfg.ContainerName, cfg.Image, err, string(out))
	}
	return nil
}

// StopAgentContainer stops a running container
func StopAgentContainer(ctx context.Context, containerName string) error {
	cmd := exec.CommandContext(ctx, "podman", "stop", containerName)
	return cmd.Run()
}

// CleanAgentContainer removes a container while keeping the named volume
func CleanAgentContainer(ctx context.Context, containerName string) error {
	_ = StopAgentContainer(ctx, containerName)
	cmd := exec.CommandContext(ctx, "podman", "rm", "-f", containerName)
	return cmd.Run()
}

// DestroyAgentContainer removes container and persistent volume
func DestroyAgentContainer(ctx context.Context, containerName, volumeName string) error {
	_ = CleanAgentContainer(ctx, containerName)
	cmd := exec.CommandContext(ctx, "podman", "volume", "rm", "-f", volumeName)
	return cmd.Run()
}

// GetAgentSSHPort discovers the dynamically assigned host port mapping for port 2222
func GetAgentSSHPort(ctx context.Context, containerName string) (int, error) {
	cmd := exec.CommandContext(ctx, "podman", "port", containerName, "2222")
	out, err := cmd.Output()
	if err != nil {
		return 0, err
	}
	// Output format typically: 127.0.0.1:41235 or :::41235
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 || lines[0] == "" {
		return 0, fmt.Errorf("no port mapping found")
	}
	parts := strings.Split(lines[0], ":")
	portStr := parts[len(parts)-1]
	return strconv.Atoi(portStr)
}

// InspectAgentContainer returns state summary for a container
func InspectAgentContainer(ctx context.Context, containerName string) (*ContainerInfo, error) {
	cmd := exec.CommandContext(ctx, "podman", "inspect", "--type", "container", containerName)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var list []struct {
		ID    string   `json:"Id"`
		Name  string   `json:"Name"`
		Names []string `json:"Names"`
		State struct {
			Status  string `json:"Status"`
			Running bool   `json:"Running"`
		} `json:"State"`
	}
	if err := json.Unmarshal(out.Bytes(), &list); err != nil || len(list) == 0 {
		return nil, fmt.Errorf("failed inspecting container")
	}

	state := list[0].State.Status
	if state == "" {
		if list[0].State.Running {
			state = "running"
		} else {
			state = "stopped"
		}
	}

	names := list[0].Names
	if len(names) == 0 && list[0].Name != "" {
		names = []string{strings.TrimPrefix(list[0].Name, "/")}
	}

	return &ContainerInfo{
		ID:    list[0].ID,
		Names: names,
		State: state,
	}, nil
}

// StartInfraStack launches the shared Valkey and Gitea containers using Podman directly
func StartInfraStack(ctx context.Context, paths config.Paths, adminPass, humanPass, humanName string) error {
	netName := "agent-sandbox-infra"
	if err := EnsureNetwork(ctx, netName); err != nil {
		return fmt.Errorf("failed ensuring network %s: %w", netName, err)
	}

	// Ensure volumes
	_ = EnsureVolume(ctx, "agent-sandbox-valkey-data")
	_ = EnsureVolume(ctx, "agent-sandbox-valkey-config")
	_ = EnsureVolume(ctx, "agent-sandbox-gitea-data")

	// Render Valkey ACL in config directory
	valkeyConfigDir := filepath.Join(paths.DataHome, "valkey")
	_ = os.MkdirAll(valkeyConfigDir, 0755)
	aclFile := filepath.Join(valkeyConfigDir, "valkey-users.acl")

	if adminPass == "" {
		adminPass = "admin_backplane_pass"
	}
	if humanPass == "" {
		humanPass = "human_backplane_pass"
	}
	if humanName == "" {
		humanName = "operator"
	}

	aclContent := fmt.Sprintf(`user default off
user admin on >%s ~* &* +@all
user %s on >%s ~%s:* ~human:name ~liaison:current ~identity:* %%R~*:* &* +@all (+xadd ~*:inbox)
`, adminPass, humanName, humanPass, humanName)
	_ = os.WriteFile(aclFile, []byte(aclContent), 0644)
	_ = os.Chmod(aclFile, 0644)
	_ = os.Chmod(valkeyConfigDir, 0755)

	// 1. Start Valkey container if not already running
	valkeyContainer := infraValkeyContainer
	valkeyArgs := []string{
		"run", "-d",
		"--name", valkeyContainer,
		"--hostname", "valkey",
		"--network", netName,
		"-p", "127.0.0.1:6379:6379",
		"-v", fmt.Sprintf("%s:/etc/valkey:z", valkeyConfigDir),
		"-v", "agent-sandbox-valkey-data:/data:z",
		"docker.io/valkey/valkey:8.0-alpine",
		"valkey-server", "--aclfile", "/etc/valkey/valkey-users.acl", "--appendonly", "yes", "--port", "6379",
	}

	checkValkey := exec.CommandContext(ctx, "podman", "container", "exists", valkeyContainer)
	if err := checkValkey.Run(); err != nil {
		cmd := exec.CommandContext(ctx, "podman", valkeyArgs...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed starting valkey container: %v (%s)", err, string(out))
		}
	} else {
		startCmd := exec.CommandContext(ctx, "podman", "start", valkeyContainer)
		if err := startCmd.Run(); err != nil {
			_ = exec.CommandContext(ctx, "podman", "rm", "-f", valkeyContainer).Run()
			cmd := exec.CommandContext(ctx, "podman", valkeyArgs...)
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed restarting valkey container: %v (%s)", err, string(out))
			}
		}
	}

	// 2. Start Gitea container if not already running
	giteaContainer := infraGiteaContainer
	giteaArgs := []string{
		"run", "-d",
		"--name", giteaContainer,
		"--hostname", "gitea",
		"--network", netName,
		"-p", "127.0.0.1:3000:3000",
		"-p", "127.0.0.1:2223:2222",
		"-e", "GITEA__server__DOMAIN=gitea",
		"-e", "GITEA__server__HTTP_PORT=3000",
		"-e", "GITEA__server__ROOT_URL=http://gitea:3000/",
		"-e", "GITEA__server__SSH_PORT=2222",
		"-e", "GITEA__server__SSH_LISTEN_PORT=2222",
		"-e", "GITEA__database__DB_TYPE=sqlite3",
		"-e", "GITEA__database__PATH=/var/lib/gitea/data/gitea.db",
		"-e", "GITEA__service__DISABLE_REGISTRATION=false",
		"-e", "GITEA__service__REQUIRE_SIGNIN_VIEW=false",
		"-e", "GITEA__security__INSTALL_LOCK=true",
		"-v", "agent-sandbox-gitea-data:/var/lib/gitea:z",
		"docker.io/gitea/gitea:1.22-rootless",
	}

	checkGitea := exec.CommandContext(ctx, "podman", "container", "exists", giteaContainer)
	if err := checkGitea.Run(); err != nil {
		cmd := exec.CommandContext(ctx, "podman", giteaArgs...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed starting gitea container: %v (%s)", err, string(out))
		}
	} else {
		startCmd := exec.CommandContext(ctx, "podman", "start", giteaContainer)
		if err := startCmd.Run(); err != nil {
			_ = exec.CommandContext(ctx, "podman", "rm", "-f", giteaContainer).Run()
			cmd := exec.CommandContext(ctx, "podman", giteaArgs...)
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed restarting gitea container: %v (%s)", err, string(out))
			}
		}
	}

	// 3. Bootstrap Gitea admin user and default fleet organization
	_ = BootstrapGitea(ctx, adminPass)

	return nil
}

// BootstrapGitea initializes the admin user, default organization, and standard repositories in Gitea
func BootstrapGitea(ctx context.Context, adminPass string) error {
	if adminPass == "" {
		adminPass = "admin_backplane_pass"
	}

	giteaURL := os.Getenv("GITEA_URL")
	if giteaURL == "" {
		giteaURL = "http://127.0.0.1:3000"
	}

	// Wait up to 10 seconds for Gitea HTTP service to become responsive
	httpClient := &http.Client{Timeout: 1 * time.Second}
	deadline := time.Now().Add(10 * time.Second)
	ready := false
	for time.Now().Before(deadline) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, giteaURL+"/api/v1/version", nil)
		resp, err := httpClient.Do(req)
		if err == nil && resp.StatusCode == http.StatusOK {
			_ = resp.Body.Close()
			ready = true
			break
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
	if !ready {
		return fmt.Errorf("timed out waiting for gitea service")
	}

	// Ensure admin user via container CLI
	createAdminCmd := exec.CommandContext(ctx, "podman", "exec", "agent-sandbox-gitea",
		"gitea", "admin", "user", "create",
		"--admin",
		"--username", "giteaadmin",
		"--password", adminPass,
		"--email", "giteaadmin@local.sndbx",
		"--must-change-password=false",
	)
	_ = createAdminCmd.Run()

	// Ensure password is sync'd in case user already existed with different password
	changePassCmd := exec.CommandContext(ctx, "podman", "exec", "agent-sandbox-gitea",
		"gitea", "admin", "user", "change-password",
		"--username", "giteaadmin",
		"--password", adminPass,
		"--must-change-password=false",
	)
	_ = changePassCmd.Run()

	// Ensure fleet organization and repos via API
	client := gitea.NewClient(gitea.ClientConfig{
		BaseURL:   giteaURL,
		AdminUser: "giteaadmin",
		AdminPass: adminPass,
		Timeout:   3 * time.Second,
	})

	_ = client.EnsureOrg(ctx, "fleet")
	_ = client.EnsureRepo(ctx, "fleet", "tools", "Fleet shared utility repositories", true)
	_ = client.EnsureRepo(ctx, "fleet", "tasks", "Fleet task tracking backlog", true)

	return nil
}

// StopInfraStack halts shared infrastructure containers
func StopInfraStack(ctx context.Context) error {
	_ = exec.CommandContext(ctx, "podman", "stop", infraValkeyContainer).Run()
	_ = exec.CommandContext(ctx, "podman", "stop", infraGiteaContainer).Run()
	return nil
}

// InspectInfraStack returns the runtime state of Valkey and Gitea containers
func InspectInfraStack(ctx context.Context) ([]ContainerInfo, error) {
	containers := []string{infraValkeyContainer, infraGiteaContainer}
	var results []ContainerInfo
	for _, c := range containers {
		info, err := InspectAgentContainer(ctx, c)
		if err == nil && info != nil {
			results = append(results, *info)
		} else {
			results = append(results, ContainerInfo{
				ID:    c,
				State: "stopped",
			})
		}
	}
	return results, nil
}

// FormatSSHConfigBlock returns a standard OpenSSH host block string.
func FormatSSHConfigBlock(name string, port int, keyFile string) string {
	return fmt.Sprintf("Host sndbx-%s\n    HostName 127.0.0.1\n    Port %d\n    User agent\n    IdentityFile %s\n    StrictHostKeyChecking no\n    UserKnownHostsFile /dev/null\n", name, port, keyFile)
}

// SyncSSHConfigFile updates the managed SSH config file with Host blocks for all currently running agents.
func SyncSSHConfigFile(ctx context.Context, paths config.Paths) error {
	configs, err := config.ListAgentConfigs(paths)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	buf.WriteString("# Agent Sandbox Auto-Generated SSH Configuration\n")
	buf.WriteString("# Managed automatically by sndbx. Do not edit manually.\n\n")

	runningCount := 0
	for _, c := range configs {
		port, err := GetAgentSSHPort(ctx, c.ContainerName)
		if err == nil && port > 0 {
			if runningCount > 0 {
				buf.WriteString("\n")
			}
			buf.WriteString(FormatSSHConfigBlock(c.Name, port, paths.IDEKeyFile))
			runningCount++
		}
	}

	if paths.SSHConfigFile == "" {
		paths.SSHConfigFile = filepath.Join(paths.DataHome, "ssh_config")
	}
	if dir := filepath.Dir(paths.SSHConfigFile); dir != "" {
		_ = os.MkdirAll(dir, 0700)
	}
	return os.WriteFile(paths.SSHConfigFile, buf.Bytes(), 0600)
}
