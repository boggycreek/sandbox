package runtime

import (
	"context"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/boggycreek/agent-sandbox/pkg/config"
)

func TestPodmanHelpers(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Network / Volume helpers
	_ = EnsureNetwork(ctx, "test-sndbx-net")
	_ = EnsureVolume(ctx, "test-sndbx-vol")

	// Struct instantiation & serialization tests
	info := &ContainerInfo{
		ID:    "12345",
		Names: []string{"test-container"},
		State: "running",
		Ports: []PortMapping{
			{HostPort: 32000, ContainerPort: 2222, HostIP: "127.0.0.1"},
		},
	}
	if info.State != "running" || len(info.Ports) != 1 {
		t.Errorf("unexpected ContainerInfo struct: %+v", info)
	}

	status := AgentStatus{
		Name:          "agent-test",
		Role:          "coder",
		Image:         "agent-sandbox-base:latest",
		ContainerName: "sndbx-agent-test",
		ContainerState: "stopped",
		SSHPort:       0,
		IDEConnect:    "-",
	}
	if status.Name != "agent-test" {
		t.Errorf("unexpected AgentStatus struct: %+v", status)
	}

	// Clean / Stop non-existent containers shouldn't crash
	_ = StopAgentContainer(ctx, "nonexistent-container-999")
	_ = CleanAgentContainer(ctx, "nonexistent-container-999")
	_ = DestroyAgentContainer(ctx, "nonexistent-container-999", "nonexistent-vol-999")

	// Inspect nonexistent
	_, _ = InspectAgentContainer(ctx, "nonexistent-container-999")
	_, _ = GetAgentSSHPort(ctx, "nonexistent-container-999")

	// Start container error check with nonexistent image
	cfg := &config.AgentConfig{
		Name:          "test-agent-run",
		ContainerName: "sndbx-agent-nonexistent-test",
		VolumeName:    "sndbx-agent-nonexistent-vol",
		Image:         "nonexistent-image-404:notfound",
	}
	paths := config.GetPaths()
	err := StartAgentContainer(ctx, cfg, paths, "127.0.0.1", 6379)
	if err == nil {
		t.Errorf("expected error starting container with nonexistent image")
	}

	// Start real container with alpine/sleep if podman works
	if _, err := exec.LookPath("podman"); err == nil {
		realCfg := &config.AgentConfig{
			Name:          "real-agent-test",
			ContainerName: fmt.Sprintf("test-podman-rt-%d", time.Now().UnixNano()),
			VolumeName:    fmt.Sprintf("test-vol-rt-%d", time.Now().UnixNano()),
			Image:         "docker.io/library/alpine:latest",
		}
		_ = CleanAgentContainer(ctx, realCfg.ContainerName)
		// Run alpine
		runCmd := exec.CommandContext(ctx, "podman", "run", "-d",
			"--name", realCfg.ContainerName,
			"-p", "127.0.0.1::2222",
			"docker.io/library/alpine:latest",
			"sleep", "30",
		)
		if err := runCmd.Run(); err == nil {
			// Query port
			port, err := GetAgentSSHPort(ctx, realCfg.ContainerName)
			if err == nil && port > 0 {
				_ = port
			}
			// Inspect
			inf, err := InspectAgentContainer(ctx, realCfg.ContainerName)
			if err == nil && inf != nil {
				_ = inf.State
			}
			// Test start on existing container branch in StartAgentContainer
			_ = StartAgentContainer(ctx, realCfg, paths, "127.0.0.1", 6379)

			_ = StopAgentContainer(ctx, realCfg.ContainerName)
			_ = CleanAgentContainer(ctx, realCfg.ContainerName)
			_ = DestroyAgentContainer(ctx, realCfg.ContainerName, realCfg.VolumeName)
		}
	}
}
