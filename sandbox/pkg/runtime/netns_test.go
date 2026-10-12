// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package runtime

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"testing"
)

func TestStructuredRootlessNetnsErrorDetection(t *testing.T) {
	// 1. Nil / Empty inputs
	if IsRootlessNetnsError(nil, nil) {
		t.Errorf("expected false for nil error and nil output")
	}
	if IsRootlessNetnsError(nil, []byte("")) {
		t.Errorf("expected false for nil error and empty output")
	}

	// 2. Exact and variant pattern matches (case-insensitive)
	patterns := []struct {
		name string
		err  error
		out  string
	}{
		{
			name: "classic lowercase runtime dir mount failure",
			err:  errors.New("exit status 127"),
			out:  "Error: failed to mount runtime directory for rootless netns: no such file or directory",
		},
		{
			name: "uppercase mount runtime failure",
			err:  errors.New("FAILED TO MOUNT RUNTIME DIRECTORY FOR ROOTLESS NETNS"),
			out:  "",
		},
		{
			name: "rootless-netns hyphenated",
			err:  nil,
			out:  "cannot setup rootless-netns bridge",
		},
		{
			name: "slirp4netns failed",
			err:  errors.New("slirp4netns failed with exit code 1"),
			out:  "",
		},
		{
			name: "slirp4netns prefix with colon",
			err:  nil,
			out:  "slirp4netns: failed to setup tap network device",
		},
		{
			name: "pasta failed",
			err:  errors.New("pasta failed"),
			out:  "",
		},
		{
			name: "pasta prefix with colon",
			err:  nil,
			out:  "pasta: cannot configure namespace interfaces",
		},
		{
			name: "path error in /run/user/<uid>/netns",
			err:  nil,
			out:  "stat /run/user/1000/netns: no such file or directory",
		},
		{
			name: "netns: no such file or directory standalone",
			err:  nil,
			out:  "podman: netns: no such file or directory",
		},
		{
			name: "failed to setup network namespace",
			err:  errors.New("failed to setup network namespace: permission denied"),
			out:  "",
		},
		{
			name: "error configuring network namespace",
			err:  errors.New("error configuring network namespace"),
			out:  "",
		},
		{
			name: "failed to open netns",
			err:  nil,
			out:  "failed to open netns: failed to join",
		},
		{
			name: "netavark failure",
			err:  nil,
			out:  "netavark: failed to configure netns for container",
		},
		{
			name: "cannot open network namespace",
			err:  nil,
			out:  "cannot open network namespace /proc/123/ns/net",
		},
	}

	for _, tc := range patterns {
		t.Run(tc.name, func(t *testing.T) {
			if !IsRootlessNetnsError(tc.err, []byte(tc.out)) {
				t.Errorf("expected IsRootlessNetnsError=true for tc %q (err=%v, out=%s)", tc.name, tc.err, tc.out)
			}
			if !isRootlessNetnsError(tc.err, []byte(tc.out)) {
				t.Errorf("expected isRootlessNetnsError=true for tc %q (err=%v, out=%s)", tc.name, tc.err, tc.out)
			}
		})
	}

	// 3. Negative tests: non-netns errors must return false
	negativeCases := []struct {
		name string
		err  error
		out  string
	}{
		{
			name: "image not found",
			err:  errors.New("exit status 125"),
			out:  "Error: short-name \"foo\" did not resolve to an alias and no search registries found",
		},
		{
			name: "port in use",
			err:  errors.New("exit status 125"),
			out:  "Error: address already in use: bind to 0.0.0.0:2222 failed",
		},
		{
			name: "generic io error",
			err:  errors.New("connection reset by peer"),
			out:  "stream closed unexpectedly",
		},
		{
			name: "unrelated command failure",
			err:  errors.New("exit status 1"),
			out:  "unknown flag: --invalid",
		},
	}

	for _, tc := range negativeCases {
		t.Run(tc.name, func(t *testing.T) {
			if IsRootlessNetnsError(tc.err, []byte(tc.out)) {
				t.Errorf("expected IsRootlessNetnsError=false for %q (err=%v, out=%s)", tc.name, tc.err, tc.out)
			}
		})
	}
}

func TestRootlessNetnsExitErrorAndDirectoryState(t *testing.T) {
	// Execute a real command to obtain an actual *exec.ExitError
	cmd127 := exec.Command("sh", "-c", "exit 127")
	err127 := cmd127.Run()
	if err127 == nil {
		t.Fatalf("expected error from exit 127")
	}

	cmd125 := exec.Command("sh", "-c", "exit 125")
	err125 := cmd125.Run()
	if err125 == nil {
		t.Fatalf("expected error from exit 125")
	}

	cmd1 := exec.Command("sh", "-c", "exit 1")
	err1 := cmd1.Run()
	if err1 == nil {
		t.Fatalf("expected error from exit 1")
	}

	// Exit code 127 with network keywords in output should be detected
	if !IsRootlessNetnsError(err127, []byte("container network bridge failed")) {
		t.Errorf("expected exit status 127 with network keyword to be detected")
	}

	// Exit code 125 with pasta keyword
	if !IsRootlessNetnsError(err125, []byte("pasta helper exited unexpectedly")) {
		t.Errorf("expected exit status 125 with pasta keyword to be detected")
	}

	// Exit code 1 without netns patterns or directory inconsistency should NOT be detected
	if IsRootlessNetnsError(err1, []byte("compilation error in main.go")) {
		t.Errorf("expected exit status 1 with unrelated text to return false")
	}

	// Test NetnsDirChecker override
	restoreChecker := SetNetnsDirCheckerForTesting(func() bool { return true }) // simulate inconsistent directory
	defer restoreChecker()

	// When directory is inconsistent and exit code is 127:
	if !IsRootlessNetnsError(err127, []byte("launch failed")) {
		t.Errorf("expected exit status 127 to be detected when netns directory is inconsistent")
	}

	// When directory is inconsistent and error mentions network:
	if !IsRootlessNetnsError(errors.New("failed to initialize network"), []byte("")) {
		t.Errorf("expected network failure to be detected when netns directory is inconsistent")
	}

	// Reset checker back to nil / default
	restoreChecker2 := SetNetnsDirCheckerForTesting(nil)
	defer restoreChecker2()

	// Default directory checker execution
	_ = defaultNetnsDirChecker()
}

func TestEnsureRootlessNetNSExecution(t *testing.T) {
	var executed []string
	restore := SetExecCommandContextForTesting(func(ctx context.Context, name string, args ...string) *exec.Cmd {
		executed = append(executed, fmt.Sprintf("%s %s", name, args))
		return exec.Command("true")
	})
	defer restore()

	if err := EnsureRootlessNetNS(context.Background()); err != nil {
		t.Errorf("EnsureRootlessNetNS returned error: %v", err)
	}

	if len(executed) == 0 {
		t.Fatalf("expected podman unshare command to be executed")
	}
}
