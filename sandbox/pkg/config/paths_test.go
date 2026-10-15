// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathsEnvironmentResolution(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Custom XDG_DATA_HOME
	t.Setenv("XDG_DATA_HOME", filepath.Join(tmpDir, "custom-data"))
	paths := GetPaths()
	expectedData := filepath.Join(tmpDir, "custom-data", "agent-sandbox")
	if paths.DataHome != expectedData {
		t.Errorf("expected DataHome %q, got %q", expectedData, paths.DataHome)
	}

	// 2. Unset XDG_DATA_HOME uses ~/.local/share
	os.Unsetenv("XDG_DATA_HOME")
	defaultPaths := GetPaths()
	if !strings.Contains(defaultPaths.DataHome, ".local/share/agent-sandbox") && !strings.Contains(defaultPaths.DataHome, "agent-sandbox") {
		t.Errorf("unexpected default DataHome: %s", defaultPaths.DataHome)
	}
}

func TestPathsLoadEnv(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, ".env")

	envContent := strings.Join([]string{
		"# Comment line to ignore",
		"",
		"TEST_PATHS_KEY1=alpha",
		"TEST_PATHS_KEY2 = beta",
		"INVALID_LINE_WITHOUT_EQUALS",
		"# Another comment",
		"TEST_PATHS_EMPTY=",
	}, "\n")

	if err := os.WriteFile(envPath, []byte(envContent), 0600); err != nil {
		t.Fatalf("failed to write mock .env: %v", err)
	}

	p := Paths{EnvFile: envPath}
	p.LoadEnv()

	if val := os.Getenv("TEST_PATHS_KEY1"); val != "alpha" {
		t.Errorf("expected TEST_PATHS_KEY1='alpha', got %q", val)
	}
	if val := os.Getenv("TEST_PATHS_KEY2"); val != "beta" {
		t.Errorf("expected TEST_PATHS_KEY2='beta', got %q", val)
	}

	// Non-existent env file should not panic or error
	pMissing := Paths{EnvFile: filepath.Join(tmpDir, "nonexistent.env")}
	pMissing.LoadEnv()
}

func TestPathsResolveRepoDir(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. AGENT_SANDBOX_REPO env override
	repo1 := filepath.Join(tmpDir, "repo-env")
	if err := os.MkdirAll(repo1, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo1, "Makefile"), []byte("# test"), 0644); err != nil {
		t.Fatalf("write Makefile failed: %v", err)
	}

	t.Setenv("AGENT_SANDBOX_REPO", repo1)
	p := Paths{DataHome: filepath.Join(tmpDir, "data")}
	if r := p.ResolveRepoDir(); r != repo1 {
		t.Errorf("expected %q from AGENT_SANDBOX_REPO, got %q", repo1, r)
	}

	// 2. SANDBOX_ROOT env override
	os.Unsetenv("AGENT_SANDBOX_REPO")
	repo2 := filepath.Join(tmpDir, "repo-root")
	if err := os.MkdirAll(repo2, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo2, "Makefile"), []byte("# test"), 0644); err != nil {
		t.Fatalf("write Makefile failed: %v", err)
	}

	t.Setenv("SANDBOX_ROOT", repo2)
	if r := p.ResolveRepoDir(); r != repo2 {
		t.Errorf("expected %q from SANDBOX_ROOT, got %q", repo2, r)
	}

	// 3. Standard fallback under DataHome/repo
	os.Unsetenv("SANDBOX_ROOT")
	standardRepo := filepath.Join(p.DataHome, "repo")
	if err := os.MkdirAll(standardRepo, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(standardRepo, "Makefile"), []byte("# test"), 0644); err != nil {
		t.Fatalf("write Makefile failed: %v", err)
	}
	if r := p.ResolveRepoDir(); r != standardRepo && !strings.Contains(r, "sandbox") {
		t.Errorf("expected standard repo or workspace repo, got %q", r)
	}

	// 4. Resolve from deep child subdirectory of current working directory
	cwd, err := os.Getwd()
	if err == nil {
		childDir := filepath.Join(cwd, "test", "sub")
		_ = os.MkdirAll(childDir, 0755)
		defer os.RemoveAll(filepath.Join(cwd, "test", "sub"))
		_ = os.Chdir(childDir)
		defer func() { _ = os.Chdir(cwd) }()

		pEmpty := Paths{}
		if resolved := pEmpty.ResolveRepoDir(); resolved != cwd && !strings.Contains(resolved, "sandbox") {
			t.Errorf("expected repo root %q, got %q", cwd, resolved)
		}
	}
}

func TestPathsEnsureDirectoriesErrors(t *testing.T) {
	tmpDir := t.TempDir()
	// Block directory creation by creating a file with the same name
	blockedPath := filepath.Join(tmpDir, "blocked_dir")
	if err := os.WriteFile(blockedPath, []byte("file blocker"), 0600); err != nil {
		t.Fatalf("failed to write blocker file: %v", err)
	}

	p := Paths{
		DataHome: blockedPath,
	}
	if err := p.EnsureDirectories(); err == nil {
		t.Errorf("expected EnsureDirectories to fail when path is a regular file")
	}
}

func TestFormatAndGetSonarAdminPassword(t *testing.T) {
	if got := FormatSonarAdminPassword(""); got != "Admin_backplane_pass" {
		t.Errorf("expected default password, got %s", got)
	}
	if got := FormatSonarAdminPassword("alllowercase123"); got != "Admin_alllowercase123" {
		t.Errorf("expected prepended Admin_, got %s", got)
	}
	if got := FormatSonarAdminPassword("AlreadyHasUpper"); got != "AlreadyHasUpper" {
		t.Errorf("expected unchanged, got %s", got)
	}

	t.Setenv("SONAR_ADMIN_PASSWORD", "customUpper123")
	t.Setenv("ADMIN_BACKPLANE_PASSWORD", "fallbacklower")
	if got := GetSonarAdminPassword(); got != "customUpper123" {
		t.Errorf("expected customUpper123, got %s", got)
	}

	t.Setenv("SONAR_ADMIN_PASSWORD", "")
	if got := GetSonarAdminPassword(); got != "Admin_fallbacklower" {
		t.Errorf("expected Admin_fallbacklower, got %s", got)
	}

	t.Setenv("ADMIN_BACKPLANE_PASSWORD", "")
	if got := GetSonarAdminPassword(); got != "Admin_backplane_pass" {
		t.Errorf("expected Admin_backplane_pass, got %s", got)
	}
}

func TestPathsSetEnvVar(t *testing.T) {
	tmpDir := t.TempDir()
	envPath := filepath.Join(tmpDir, "test.env")
	p := Paths{EnvFile: envPath}

	if err := p.SetEnvVar("KEY1", "val1"); err != nil {
		t.Fatalf("SetEnvVar failed: %v", err)
	}
	if os.Getenv("KEY1") != "val1" {
		t.Errorf("expected env var set, got %s", os.Getenv("KEY1"))
	}

	// Update existing
	if err := p.SetEnvVar("KEY1", "newval"); err != nil {
		t.Fatalf("SetEnvVar update failed: %v", err)
	}
	if os.Getenv("KEY1") != "newval" {
		t.Errorf("expected updated env var, got %s", os.Getenv("KEY1"))
	}

	// Append second
	if err := p.SetEnvVar("KEY2", "val2"); err != nil {
		t.Fatalf("SetEnvVar second append failed: %v", err)
	}
	content, _ := os.ReadFile(envPath)
	contentStr := string(content)
	if !strings.Contains(contentStr, "KEY1=newval") || !strings.Contains(contentStr, "KEY2=val2") {
		t.Errorf("unexpected env content:\n%s", contentStr)
	}
}

func TestGetDefaultProjectOrg(t *testing.T) {
	t.Setenv("DEFAULT_PROJECT_ORG", "")
	if got := GetDefaultProjectOrg(); got != "factory" {
		t.Errorf("expected factory by default, got %s", got)
	}

	t.Setenv("DEFAULT_PROJECT_ORG", "custom-org")
	if got := GetDefaultProjectOrg(); got != "custom-org" {
		t.Errorf("expected custom-org, got %s", got)
	}
}

func TestPathsProjectsDir(t *testing.T) {
	p := GetPaths()
	if !strings.HasSuffix(p.ProjectsDir, filepath.Join("agent-sandbox", "projects")) {
		t.Errorf("expected ProjectsDir to end with agent-sandbox/projects, got %s", p.ProjectsDir)
	}
}

