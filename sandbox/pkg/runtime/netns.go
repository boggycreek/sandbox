// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// netnsRegexPatterns defines structured regex signatures matching known
// Podman, slirp4netns, pasta, and netavark rootless network namespace failures.
var netnsRegexPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)failed to mount (?:runtime )?directory for rootless netns`),
	regexp.MustCompile(`(?i)rootless[- ]netns`),
	regexp.MustCompile(`(?i)runtime directory for rootless netns`),
	regexp.MustCompile(`(?i)(?:failed to (?:create|setup|set up|open|find|join)|error (?:configuring|setting up|creating)) (?:network namespace|netns)`),
	regexp.MustCompile(`(?i)(?:slirp4netns|pasta)(?::| failed|\.go:)`),
	regexp.MustCompile(`(?i)/run/user/\d+/netns`),
	regexp.MustCompile(`(?i)(?:^|\s)netns:\s*no such file or directory`),
	regexp.MustCompile(`(?i)failed to (?:open|find|join) netns`),
	regexp.MustCompile(`(?i)netavark.*(?:netns|network namespace)`),
	regexp.MustCompile(`(?i)cannot (?:open|mount|bind) (?:network namespace|netns)`),
}

// NetnsDirChecker tests whether the rootless network namespace directory is missing or inconsistent.
type NetnsDirChecker func() bool

var (
	netnsCheckerMu sync.RWMutex
	netnsChecker   NetnsDirChecker = defaultNetnsDirChecker
)

// SetNetnsDirCheckerForTesting overrides the netns directory check for unit tests and returns a restore function.
func SetNetnsDirCheckerForTesting(checker NetnsDirChecker) func() {
	netnsCheckerMu.Lock()
	orig := netnsChecker
	if checker == nil {
		netnsChecker = defaultNetnsDirChecker
	} else {
		netnsChecker = checker
	}
	netnsCheckerMu.Unlock()
	return func() {
		netnsCheckerMu.Lock()
		netnsChecker = orig
		netnsCheckerMu.Unlock()
	}
}

func checkNetnsDirInconsistent() bool {
	netnsCheckerMu.RLock()
	fn := netnsChecker
	netnsCheckerMu.RUnlock()
	if fn != nil {
		return fn()
	}
	return defaultNetnsDirChecker()
}

func defaultNetnsDirChecker() bool {
	uid := os.Getuid()
	dirs := []string{
		fmt.Sprintf("/run/user/%d/netns", uid),
	}
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		dirs = append(dirs, filepath.Join(xdg, "netns"))
	}
	for _, dir := range dirs {
		info, err := os.Stat(dir)
		if err != nil {
			return true
		}
		if !info.IsDir() {
			return true
		}
	}
	return false
}

// EnsureRootlessNetNS reconciles and initializes Podman's rootless network namespace
// runtime directory structure. In rootless mode, if the temporary runtime directory
// desynchronizes from the kernel netns mount, running 'podman unshare --rootless-netns true'
// forces Podman to recreate the internal mount namespace tree.
func EnsureRootlessNetNS(ctx context.Context) error {
	uid := os.Getuid()
	_ = os.MkdirAll(fmt.Sprintf("/run/user/%d/netns", uid), 0700)

	cmd := CommandContext(ctx, "podman", "unshare", "--rootless-netns", "true")
	return cmd.Run()
}

// IsRootlessNetnsError checks if a container start, run, or network error is due to a desynchronized
// rootless network namespace mount directory or rootless networking failure.
func IsRootlessNetnsError(err error, out []byte) bool {
	if err == nil && len(out) == 0 {
		return false
	}

	combined := string(out)
	if err != nil {
		if combined != "" {
			combined += " " + err.Error()
		} else {
			combined = err.Error()
		}
	}

	// 1. Structured signature pattern matching (case-insensitive)
	for _, pattern := range netnsRegexPatterns {
		if pattern.MatchString(combined) {
			return true
		}
	}

	lowerCombined := strings.ToLower(combined)
	hasNetKeyword := strings.Contains(lowerCombined, "network") ||
		strings.Contains(lowerCombined, "netns") ||
		strings.Contains(lowerCombined, "slirp") ||
		strings.Contains(lowerCombined, "pasta") ||
		strings.Contains(lowerCombined, "bridge") ||
		strings.Contains(lowerCombined, "port")

	// 2. Structured ExitError analysis (e.g. exit code 127 from slirp4netns/pasta failure or 125 from runtime setup)
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		code := exitErr.ExitCode()
		if code == 127 || code == 125 {
			if hasNetKeyword || checkNetnsDirInconsistent() {
				return true
			}
		}
	}

	// 3. Filesystem netns directory state check if error indicates network setup failure
	if hasNetKeyword && checkNetnsDirInconsistent() {
		return true
	}

	return false
}

// isRootlessNetnsError is an internal alias for IsRootlessNetnsError.
func isRootlessNetnsError(err error, out []byte) bool {
	return IsRootlessNetnsError(err, out)
}
