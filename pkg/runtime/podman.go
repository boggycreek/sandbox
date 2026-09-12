package runtime

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/boggycreek/agent-sandbox/pkg/config"
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

	args := []string{
		"run", "-d",
		"--name", cfg.ContainerName,
		"--hostname", fmt.Sprintf("%s-sandbox", cfg.Name),
		"--network", netName,
		"-p", "127.0.0.1::2222",
		"-e", fmt.Sprintf("AGENT_NAME=%s", cfg.Name),
		"-e", fmt.Sprintf("BP_AGENT=%s", cfg.Name),
		"-e", fmt.Sprintf("BP_PASSWORD=%s", cfg.Password),
		"-e", fmt.Sprintf("BP_HOST=%s", bpHost),
		"-e", fmt.Sprintf("BP_PORT=%d", bpPort),
		"-e", fmt.Sprintf("BP_SIGNING_KEY_PEM=%s", cfg.SigningKeyPEM),
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
		ID    string `json:"Id"`
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

	return &ContainerInfo{
		ID:    list[0].ID,
		State: state,
	}, nil
}
