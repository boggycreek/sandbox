// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package runtime

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/boggycreek/sandbox/pkg/config"
)

// TestStopInfraStack exercises StopInfraStack with ephemeral container names.
func TestStopInfraStack(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pid := os.Getpid()
	testValkeyName := fmt.Sprintf("unit-test-valkey-stop-%d", pid)
	testGiteaName := fmt.Sprintf("unit-test-gitea-stop-%d", pid)
	testPostgresName := fmt.Sprintf("unit-test-postgres-stop-%d", pid)
	testSonarName := fmt.Sprintf("unit-test-sonar-stop-%d", pid)

	origValkey := infraValkeyContainer
	origGitea := infraGiteaContainer
	origPostgres := infraPostgresContainer
	origSonar := infraSonarContainer
	infraValkeyContainer = testValkeyName
	infraGiteaContainer = testGiteaName
	infraPostgresContainer = testPostgresName
	infraSonarContainer = testSonarName
	defer func() {
		infraValkeyContainer = origValkey
		infraGiteaContainer = origGitea
		infraPostgresContainer = origPostgres
		infraSonarContainer = origSonar
	}()

	startOut, err := execCommandContext(ctx, "podman", "run", "-d", "--name", testValkeyName,
		"docker.io/valkey/valkey:8-alpine",
	).CombinedOutput()
	if err != nil {
		t.Skipf("Cannot start ephemeral valkey container for stop test: %v (%s)", err, string(startOut))
	}
	defer func() {
		_ = exec.Command("podman", "rm", "-f", testValkeyName).Run()
		_ = exec.Command("podman", "rm", "-f", testGiteaName).Run()
		_ = exec.Command("podman", "rm", "-f", testPostgresName).Run()
		_ = exec.Command("podman", "rm", "-f", testSonarName).Run()
	}()

	if err := StopInfraStack(ctx); err != nil {
		t.Errorf("StopInfraStack returned unexpected error: %v", err)
	}

	list, err := InspectInfraStack(ctx)
	if err != nil {
		t.Errorf("InspectInfraStack failed: %v", err)
	}
	if len(list) == 0 {
		t.Errorf("InspectInfraStack returned empty list")
	}
}

// TestInfraStackOrchestrationScenarios tests failure and recovery branches of StartInfraStack.
func TestInfraStackOrchestrationScenarios(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()
	paths := config.Paths{DataHome: tmpDir}

	// 1. EnsureNetwork error
	mockFailNet := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "network" {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	restore1 := SetExecCommandContextForTesting(mockFailNet)
	err := StartInfraStack(ctx, paths, "p1", "p2", "u1")
	restore1()
	if err == nil {
		t.Errorf("expected error when network fails")
	}

	// 2. Valkey doesn't exist, run fails
	mockValkeyRunFail := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "container" && args[1] == "exists" {
			return exec.Command("false")
		}
		if name == "podman" && len(args) > 0 && args[0] == "run" && strings.Contains(args[3], "valkey") {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	restore2 := SetExecCommandContextForTesting(mockValkeyRunFail)
	err = StartInfraStack(ctx, paths, "p1", "p2", "u1")
	restore2()
	if err == nil {
		t.Errorf("expected error when valkey run fails")
	}

	// 3. Valkey exists, start fails, restart fails
	mockValkeyRestartFail := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "container" && args[1] == "exists" {
			return exec.Command("true")
		}
		if name == "podman" && len(args) > 0 && args[0] == "start" {
			return exec.Command("false")
		}
		if name == "podman" && len(args) > 0 && args[0] == "run" {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	restore3 := SetExecCommandContextForTesting(mockValkeyRestartFail)
	err = StartInfraStack(ctx, paths, "p1", "p2", "u1")
	restore3()
	if err == nil {
		t.Errorf("expected error when valkey restart fails")
	}

	// 4. Gitea doesn't exist, run fails
	mockGiteaRunFail := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "container" && args[1] == "exists" {
			if strings.Contains(args[2], "gitea") {
				return exec.Command("false")
			}
			return exec.Command("true")
		}
		if name == "podman" && len(args) > 0 && args[0] == "run" && strings.Contains(args[3], "gitea") {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	restore4 := SetExecCommandContextForTesting(mockGiteaRunFail)
	err = StartInfraStack(ctx, paths, "p1", "p2", "u1")
	restore4()
	if err == nil {
		t.Errorf("expected error when gitea run fails")
	}

	// 5. Gitea exists, start fails, restart fails
	mockGiteaRestartFail := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "container" && args[1] == "exists" {
			return exec.Command("true")
		}
		if name == "podman" && len(args) > 1 && args[0] == "start" && strings.Contains(args[1], "gitea") {
			return exec.Command("false")
		}
		if name == "podman" && len(args) > 0 && args[0] == "run" && strings.Contains(args[3], "gitea") {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	restore5 := SetExecCommandContextForTesting(mockGiteaRestartFail)
	err = StartInfraStack(ctx, paths, "p1", "p2", "u1")
	restore5()
	if err == nil {
		t.Errorf("expected error when gitea restart fails")
	}

	// 6. Postgres doesn't exist, run fails
	mockPostgresRunFail := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "container" && args[1] == "exists" {
			if strings.Contains(args[2], "postgres") {
				return exec.Command("false")
			}
			return exec.Command("true")
		}
		if name == "podman" && len(args) > 0 && args[0] == "run" && strings.Contains(args[3], "postgres") {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	restore5b := SetExecCommandContextForTesting(mockPostgresRunFail)
	err = StartInfraStack(ctx, paths, "p1", "p2", "u1")
	restore5b()
	if err == nil {
		t.Errorf("expected error when postgres run fails")
	}

	// 7. Postgres exists, start fails, restart fails
	mockPostgresRestartFail := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "container" && args[1] == "exists" {
			return exec.Command("true")
		}
		if name == "podman" && len(args) > 1 && args[0] == "start" && strings.Contains(args[1], "postgres") {
			return exec.Command("false")
		}
		if name == "podman" && len(args) > 0 && args[0] == "run" && strings.Contains(args[3], "postgres") {
			return exec.Command("false")
		}
		return exec.Command("true")
	}
	restore5c := SetExecCommandContextForTesting(mockPostgresRestartFail)
	err = StartInfraStack(ctx, paths, "p1", "p2", "u1")
	restore5c()
	if err == nil {
		t.Errorf("expected error when postgres restart fails")
	}

	// 8. Sonarqube image exists, container doesn't exist, run succeeds
	mockSonarRunSuccess := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 1 && args[0] == "image" && args[1] == "exists" {
			return exec.Command("true")
		}
		if name == "podman" && len(args) > 1 && args[0] == "container" && args[1] == "exists" {
			if strings.Contains(args[2], "sonar") {
				return exec.Command("false")
			}
			return exec.Command("true")
		}
		return exec.Command("true")
	}
	restore6 := SetExecCommandContextForTesting(mockSonarRunSuccess)
	err = StartInfraStack(ctx, paths, "p1", "p2", "u1")
	restore6()
	if err != nil {
		t.Errorf("unexpected error in sonar run success: %v", err)
	}

	// 9. Rootless NetNS error branches on Valkey, Gitea, Postgres, Sonar
	mockNetnsRetry := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		if name == "podman" && len(args) > 0 && args[0] == "run" {
			return exec.Command("sh", "-c", "echo 'failed to mount runtime directory for rootless netns' >&2; exit 127")
		}
		if name == "podman" && len(args) > 1 && args[0] == "container" && args[1] == "exists" {
			return exec.Command("false")
		}
		if name == "podman" && len(args) > 1 && args[0] == "image" && args[1] == "exists" {
			return exec.Command("true")
		}
		return exec.Command("true")
	}
	restore9 := SetExecCommandContextForTesting(mockNetnsRetry)
	_ = StartInfraStack(ctx, paths, "p1", "p2", "u1")
	restore9()
}

func TestBootstrapGiteaEdgeCases(t *testing.T) {
	// 1. Canceled context returns context error
	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	err := BootstrapGitea(canceledCtx, "pass")
	if err == nil {
		t.Errorf("expected canceled context error from BootstrapGitea")
	}

	// 2. StartInfraStack with empty credentials (exercises defaults fallback)
	mockRun := func(ctx context.Context, name string, args ...string) *exec.Cmd {
		return exec.Command("true")
	}
	restore := SetExecCommandContextForTesting(mockRun)
	defer restore()

	paths := config.Paths{DataHome: t.TempDir()}
	if err := StartInfraStack(context.Background(), paths, "", "", ""); err != nil {
		t.Errorf("expected StartInfraStack with empty credentials to succeed with mocks: %v", err)
	}
}
