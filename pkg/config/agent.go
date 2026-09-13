// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package config

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/boggycreek/agent-sandbox/pkg/libbp"
)

// Well-known image presets
var WellKnownImages = map[string]string{
	"base":     "agent-sandbox-base:latest",
	"opencode": "agent-sandbox-opencode:latest",
	"claude":   "agent-sandbox-claude:latest",
	"agy":      "agent-sandbox-agy:latest",
}

// AgentConfig holds persistent configuration for a single named agent
type AgentConfig struct {
	Name          string    `json:"name"`
	Role          string    `json:"role"`
	Image         string    `json:"image"`
	ContainerName string    `json:"container_name"`
	VolumeName    string    `json:"volume_name"`
	Password      string    `json:"password"`
	SigningKeyPEM string    `json:"signing_key_pem"`
	PublicKeyB64  string    `json:"public_key_b64"`
	ModelURL      string    `json:"model_url,omitempty"`
	ModelName     string    `json:"model_name,omitempty"`
	ModelAPIKey   string    `json:"model_api_key,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// ResolveImage returns the canonical OCI image name for a given input or preset
func ResolveImage(input string) string {
	lower := strings.ToLower(strings.TrimSpace(input))
	if img, found := WellKnownImages[lower]; found {
		return img
	}
	if lower == "" {
		return WellKnownImages["base"]
	}
	return input
}

// NewAgentConfig creates a newly initialized AgentConfig with random credentials
func NewAgentConfig(name, imageInput, role string, modelOpts ...string) (*AgentConfig, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return nil, fmt.Errorf("agent name cannot be empty")
	}

	if role == "" {
		role = "coding-agent"
	}

	var modelURL, modelName, modelAPIKey string
	if len(modelOpts) > 0 {
		modelURL = strings.TrimSpace(modelOpts[0])
	}
	if len(modelOpts) > 1 {
		modelName = strings.TrimSpace(modelOpts[1])
	}
	if len(modelOpts) > 2 {
		modelAPIKey = strings.TrimSpace(modelOpts[2])
	}

	// Generate random 32-character hex password
	pwBytes := make([]byte, 16)
	if _, err := rand.Read(pwBytes); err != nil {
		return nil, err
	}
	password := hex.EncodeToString(pwBytes)

	// Generate Ed25519 signing keypair
	priv, pub, err := libbp.GenerateKeypair()
	if err != nil {
		return nil, err
	}
	pemStr, err := libbp.EncodePrivateKeyPEM(priv)
	if err != nil {
		return nil, err
	}
	pubB64 := libbp.EncodePublicKeyBase64(pub)

	return &AgentConfig{
		Name:          name,
		Role:          role,
		Image:         ResolveImage(imageInput),
		ContainerName: fmt.Sprintf("sndbx-agent-%s", name),
		VolumeName:    fmt.Sprintf("sndbx-agent-%s-home", name),
		Password:      password,
		SigningKeyPEM: pemStr,
		PublicKeyB64:  pubB64,
		ModelURL:      modelURL,
		ModelName:     modelName,
		ModelAPIKey:   modelAPIKey,
		CreatedAt:     time.Now().UTC(),
	}, nil
}

// SaveAgentConfig writes the agent descriptor and secret key files
func SaveAgentConfig(cfg *AgentConfig, paths Paths) error {
	if err := paths.EnsureDirectories(); err != nil {
		return err
	}

	// Save agent JSON
	jsonPath := filepath.Join(paths.AgentsDir, fmt.Sprintf("%s.json", cfg.Name))
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(jsonPath, data, 0600); err != nil {
		return err
	}

	// Save dedicated secret directory
	agentSecretDir := filepath.Join(paths.SecretsDir, cfg.Name)
	if err := os.MkdirAll(agentSecretDir, 0700); err != nil {
		return err
	}

	keyPath := filepath.Join(agentSecretDir, "signing-key.pem")
	if err := os.WriteFile(keyPath, []byte(cfg.SigningKeyPEM), 0600); err != nil {
		return err
	}

	return nil
}

// LoadAgentConfig reads an existing agent configuration by name
func LoadAgentConfig(name string, paths Paths) (*AgentConfig, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	jsonPath := filepath.Join(paths.AgentsDir, fmt.Sprintf("%s.json", name))
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, fmt.Errorf("agent %q not found (no config at %s)", name, jsonPath)
	}

	var cfg AgentConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("corrupted agent config for %q: %w", name, err)
	}
	return &cfg, nil
}

// ListAgentConfigs discovers all configured agents in the repository
func ListAgentConfigs(paths Paths) ([]*AgentConfig, error) {
	if err := paths.EnsureDirectories(); err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(paths.AgentsDir)
	if err != nil {
		return nil, err
	}

	var configs []*AgentConfig
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			name := strings.TrimSuffix(e.Name(), ".json")
			cfg, err := LoadAgentConfig(name, paths)
			if err == nil && cfg != nil {
				configs = append(configs, cfg)
			}
		}
	}
	return configs, nil
}

// DeleteAgentConfig removes the agent config and secrets
func DeleteAgentConfig(name string, paths Paths) error {
	name = strings.ToLower(strings.TrimSpace(name))
	jsonPath := filepath.Join(paths.AgentsDir, fmt.Sprintf("%s.json", name))
	_ = os.Remove(jsonPath)

	agentSecretDir := filepath.Join(paths.SecretsDir, name)
	_ = os.RemoveAll(agentSecretDir)
	return nil
}
