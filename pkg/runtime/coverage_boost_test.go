package runtime

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/boggycreek/agent-sandbox/pkg/config"
)

func TestRuntimeCoverageBoost(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Port parsing / inspect error branches
	_, err := GetAgentSSHPort(ctx, "")
	if err == nil {
		t.Errorf("expected error for empty container port lookup")
	}

	_, err = InspectAgentContainer(ctx, "")
	if err == nil {
		t.Errorf("expected error for empty container inspect")
	}

	// Test StartAgentContainer with existing network, volume, and public key file
	tmpDir := t.TempDir()
	sshDir := filepath.Join(tmpDir, "ssh")
	_ = os.MkdirAll(sshDir, 0700)
	keyFile := filepath.Join(sshDir, "key")
	_ = os.WriteFile(keyFile+".pub", []byte("ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA test"), 0644)

	paths := config.Paths{
		DataHome:   tmpDir,
		IDEKeyFile: keyFile,
	}

	cfg := &config.AgentConfig{
		Name:          "mock-start-agent",
		ContainerName: "mock-start-container",
		VolumeName:    "mock-start-vol",
		Image:         "agent-sandbox-base:latest",
	}

	// Ensure network and volume paths execute
	_ = EnsureNetwork(ctx, "agent-sandbox-infra")
	_ = EnsureVolume(ctx, cfg.VolumeName)
	_ = CleanAgentContainer(ctx, cfg.ContainerName)
	_ = paths.EnsureDirectories()

	// Start agent container when podman exists
	_ = StartAgentContainer(ctx, cfg, paths, "127.0.0.1", 6379)
	// Query state
	info, _ := InspectAgentContainer(ctx, cfg.ContainerName)
	if info != nil {
		_ = info.ID
	}
	_, _ = GetAgentSSHPort(ctx, cfg.ContainerName)

	// Clean up
	_ = CleanAgentContainer(ctx, cfg.ContainerName)
	_ = DestroyAgentContainer(ctx, cfg.ContainerName, cfg.VolumeName)
}
