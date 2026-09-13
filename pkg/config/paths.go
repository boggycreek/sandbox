// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package config

import (
	"os"
	"path/filepath"
)

// Paths holds XDG-compliant filesystem paths for the sandbox
type Paths struct {
	DataHome   string
	AgentsDir  string
	SecretsDir string
	BinDir     string
	EnvFile    string
	SSHDir     string
	IDEKeyFile string
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
		DataHome:   dataHome,
		AgentsDir:  filepath.Join(dataHome, "agents"),
		SecretsDir: filepath.Join(dataHome, "secrets"),
		BinDir:     filepath.Join(home, ".local", "bin"),
		EnvFile:    filepath.Join(dataHome, ".env"),
		SSHDir:     sshDir,
		IDEKeyFile: filepath.Join(sshDir, "agent-sandbox"),
	}
}

// EnsureDirectories creates required data and secrets directories
func (p Paths) EnsureDirectories() error {
	for _, dir := range []string{p.DataHome, p.AgentsDir, p.SecretsDir, p.BinDir, p.SSHDir} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	return nil
}
