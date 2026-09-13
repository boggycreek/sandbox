// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package config

import (
	"os"
	"path/filepath"
	"strings"
)

// Paths holds XDG-compliant filesystem paths for the sandbox
type Paths struct {
	DataHome      string
	AgentsDir     string
	SecretsDir    string
	BinDir        string
	EnvFile       string
	SSHDir        string
	IDEKeyFile    string
	SSHConfigFile string
}

// GetPaths returns standard paths resolved against XDG environment variables
func GetPaths() Paths {
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/tmp"
	}

	xdgData := os.Getenv("XDG_DATA_HOME")
	if xdgData == "" {
		xdgData = filepath.Join(home, ".local", "share")
	}

	dataHome := filepath.Join(xdgData, "agent-sandbox")
	sshDir := filepath.Join(home, ".ssh")

	return Paths{
		DataHome:      dataHome,
		AgentsDir:     filepath.Join(dataHome, "agents"),
		SecretsDir:    filepath.Join(dataHome, "secrets"),
		BinDir:        filepath.Join(home, ".local", "bin"),
		EnvFile:       filepath.Join(dataHome, ".env"),
		SSHDir:        sshDir,
		IDEKeyFile:    filepath.Join(sshDir, "agent-sandbox"),
		SSHConfigFile: filepath.Join(dataHome, "ssh_config"),
	}
}

// EnsureDirectories creates required data and secrets directories
func (p Paths) EnsureDirectories() error {
	for _, dir := range []string{p.DataHome, p.AgentsDir, p.SecretsDir, p.BinDir, p.SSHDir} {
		if dir == "" {
			continue
		}
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	return nil
}

// LoadEnv loads environment variables from .env if present and not already set
func (p Paths) LoadEnv() {
	data, err := os.ReadFile(p.EnvFile)
	if err != nil {
		return
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.TrimSpace(parts[1])
			if os.Getenv(k) == "" && v != "" {
				_ = os.Setenv(k, v)
			}
		}
	}
}

// ResolveRepoDir returns the absolute path to the local repository checkout.
// It checks AGENT_SANDBOX_REPO, SANDBOX_ROOT, the parent of the binary,
// the current working directory, and the standard ~/.local/share/agent-sandbox/repo fallback.
func (p Paths) ResolveRepoDir() string {
	// 1. Explicit environment variable override
	if repo := os.Getenv("AGENT_SANDBOX_REPO"); repo != "" {
		if fi, err := os.Stat(filepath.Join(repo, "Makefile")); err == nil && !fi.IsDir() {
			return repo
		}
	}
	if repo := os.Getenv("SANDBOX_ROOT"); repo != "" {
		if fi, err := os.Stat(filepath.Join(repo, "Makefile")); err == nil && !fi.IsDir() {
			return repo
		}
	}

	// 2. Current working directory
	if cwd, err := os.Getwd(); err == nil {
		if fi, err := os.Stat(filepath.Join(cwd, "Makefile")); err == nil && !fi.IsDir() {
			return cwd
		}
	}

	// 3. Relative to the executing binary (e.g. repo/bin/sndbx -> repo)
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		parent := filepath.Dir(exeDir)
		if fi, err := os.Stat(filepath.Join(parent, "Makefile")); err == nil && !fi.IsDir() {
			return parent
		}
	}

	// 4. Default standard XDG repository location
	standardRepo := filepath.Join(p.DataHome, "repo")
	if fi, err := os.Stat(filepath.Join(standardRepo, "Makefile")); err == nil && !fi.IsDir() {
		return standardRepo
	}

	// Fallback to current working directory if none found
	cwd, _ := os.Getwd()
	return cwd
}

