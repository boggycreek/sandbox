// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	stdRuntime "runtime"
	"strings"
	"time"

	"github.com/boggycreek/sandbox/pkg/config"
)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

var httpClientForDownload HTTPClient = &http.Client{Timeout: 10 * time.Second}

func handleUpdate(ctx context.Context, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		sub := strings.ToLower(args[0])
		if sub == "help" || sub == "-h" || sub == "--help" {
			fmt.Fprintln(stdout, `Usage: sndbx update

Comprehensive update of the Agent Sandbox local environment:
  1. Synchronizes local git repository checkout with remote (git pull)
  2. Updates native CLI binaries (sndbx, bp, bpd, sonar-mcp) from GitHub Releases or source
  3. Rebuilds all native OCI container images (base, opencode, claude, agy)`)
			return 0
		}
	}

	repoDir := paths.ResolveRepoDir()
	if repoDir == "" {
		repoDir = filepath.Join(paths.DataHome, "repo")
	}

	if _, err := os.Stat(filepath.Join(repoDir, ".git")); os.IsNotExist(err) {
		fmt.Fprintf(stdout, "==> Cloning repository into %s...\n", repoDir)
		_ = os.MkdirAll(filepath.Dir(repoDir), 0755)
		cloneCmd := execCommandContext(ctx, "git", "clone", "--branch", "main", "https://github.com/boggycreek/sandbox.git", repoDir)
		cloneCmd.Stdout = stdout
		cloneCmd.Stderr = stderr
		if err := cloneCmd.Run(); err != nil {
			fmt.Fprintf(stderr, "sndbx update: warning: git clone failed (%v), continuing with local fallback\n", err)
		}
	} else {
		fmt.Fprintf(stdout, "==> Synchronizing repository (%s)...\n", repoDir)
		fetchCmd := execCommandContext(ctx, "git", "fetch", "origin", "main")
		fetchCmd.Dir = repoDir
		fetchCmd.Stdout = stdout
		fetchCmd.Stderr = stderr
		_ = fetchCmd.Run()

		gitCmd := execCommandContext(ctx, "git", "pull", "--ff-only", "origin", "main")
		gitCmd.Dir = repoDir
		gitCmd.Stdout = stdout
		gitCmd.Stderr = stderr
		if err := gitCmd.Run(); err != nil {
			fmt.Fprintf(stderr, "sndbx update: warning: git pull failed (%v), continuing with local checkout\n", err)
		}
	}

	binDir := paths.BinDir
	if binDir == "" {
		binDir = filepath.Join(os.Getenv("HOME"), ".local", "bin")
	}
	_ = os.MkdirAll(binDir, 0755)

	if err := installOrBuildBinaries(ctx, repoDir, binDir, stdout, stderr); err != nil {
		fmt.Fprintf(stderr, "sndbx update: failed installing binaries: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, "==> Building all native OCI container images...")
	imgCmd := execCommandContext(ctx, "make", "build-images")
	imgCmd.Dir = repoDir
	imgCmd.Stdout = stdout
	imgCmd.Stderr = stderr
	if err := imgCmd.Run(); err != nil {
		fmt.Fprintf(stderr, "sndbx update: failed building OCI container images: %v\n", err)
		return 1
	}

	fmt.Fprintln(stdout, "==> Update complete.")
	return 0
}

func installOrBuildBinaries(ctx context.Context, repoDir, binDir string, stdout, stderr io.Writer) error {
	binaries := []string{"sndbx", "bp", "bpd", "sonar-mcp"}
	goos := stdRuntime.GOOS
	goarch := stdRuntime.GOARCH
	releaseURLBase := "https://github.com/boggycreek/sandbox/releases/latest/download"

	downloadedAll := true

	for _, bin := range binaries {
		targetFile := filepath.Join(binDir, bin)
		downloadURL := fmt.Sprintf("%s/%s-%s-%s", releaseURLBase, bin, goos, goarch)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, http.NoBody)
		if err != nil {
			downloadedAll = false
			break
		}
		resp, err := httpClientForDownload.Do(req)
		if err != nil || resp.StatusCode != http.StatusOK {
			if resp != nil {
				_ = resp.Body.Close()
			}
			downloadedAll = false
			break
		}
		tmpFile := targetFile + ".tmp"
		out, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			_ = resp.Body.Close()
			downloadedAll = false
			break
		}
		_, copyErr := io.Copy(out, resp.Body)
		_ = out.Close()
		_ = resp.Body.Close()
		if copyErr != nil {
			_ = os.Remove(tmpFile)
			downloadedAll = false
			break
		}
		_ = os.Rename(tmpFile, targetFile)
		_ = os.Chmod(targetFile, 0755)
	}

	if downloadedAll {
		fmt.Fprintf(stdout, "==> Installed prebuilt native CLI binaries from GitHub Releases to %s\n", binDir)
		return nil
	}

	fmt.Fprintf(stdout, "==> Prebuilt release binaries unavailable; compiling from source via Go toolchain...\n")
	cleanEnv := sanitizeGoEnv(os.Environ())

	for _, bin := range binaries {
		cmdDir := fmt.Sprintf("./cmd/%s", bin)
		if _, err := os.Stat(filepath.Join(repoDir, "cmd", bin)); err != nil {
			continue
		}
		targetFile := filepath.Join(binDir, bin)
		buildCmd := execCommandContext(ctx, "go", "build", "-trimpath", "-ldflags=-s -w", "-o", targetFile, cmdDir)
		buildCmd.Dir = repoDir
		buildCmd.Env = cleanEnv
		buildCmd.Stdout = stdout
		buildCmd.Stderr = stderr
		if err := buildCmd.Run(); err != nil {
			return fmt.Errorf("failed compiling %s: %w", bin, err)
		}
		_ = os.Chmod(targetFile, 0755)
	}
	fmt.Fprintf(stdout, "==> Compiled native CLI binaries to %s\n", binDir)
	return nil
}

func sanitizeGoEnv(env []string) []string {
	var clean []string
	for _, e := range env {
		if !strings.HasPrefix(e, "GOROOT=") {
			clean = append(clean, e)
		}
	}
	return clean
}
