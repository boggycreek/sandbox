// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/boggycreek/sandbox/pkg/config"
)

var execCommandContext = exec.CommandContext

func main() {
	os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr))
}

// Run executes the sndbx command
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 1
	}

	domain := strings.ToLower(args[0])
	domainArgs := args[1:]

	paths := config.GetPaths()
	if err := paths.EnsureDirectories(); err != nil {
		fmt.Fprintf(stderr, "%s warning: failed to ensure directories: %v\n", binaryName(), err)
	}
	paths.LoadEnv()

	sigCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch domain {
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0

	case "agent":
		return handleAgent(sigCtx, paths, domainArgs, stdout, stderr)

	case "infra":
		ctx, cancel := context.WithTimeout(sigCtx, 120*time.Second)
		defer cancel()
		return handleInfra(ctx, paths, domainArgs, stdout, stderr)

	case "plugin":
		ctx, cancel := context.WithTimeout(sigCtx, 60*time.Second)
		defer cancel()
		return handlePlugin(ctx, paths, domainArgs, stdout, stderr)

	case "update":
		updateTimeout := 30 * time.Minute
		if envTimeout := os.Getenv("SNDBX_UPDATE_TIMEOUT"); envTimeout != "" {
			if d, err := time.ParseDuration(envTimeout); err == nil && d > 0 {
				updateTimeout = d
			}
		}
		updateCtx, updateCancel := context.WithTimeout(sigCtx, updateTimeout)
		defer updateCancel()
		return handleUpdate(updateCtx, paths, domainArgs, stdout, stderr)

	case "gui":
		return handleGUI(sigCtx, paths, domainArgs, stdout, stderr)

	default:
		bin := binaryName()
		fmt.Fprintf(stderr, "%s: unknown command %q (see '%s help')\n", bin, domain, bin)
		return 1
	}
}

// binaryName returns the invocation name of the CLI binary ("sb" or "sndbx").
func binaryName() string {
	if len(os.Args) > 0 {
		base := filepath.Base(os.Args[0])
		if base == "sb" {
			return "sb"
		}
	}
	return "sndbx"
}

func printUsage(out io.Writer) {
	bin := binaryName()
	fmt.Fprintf(out, `Agent Sandbox CLI (%s)

Usage:
  %s <domain|command> [args...]

Domains:
  agent       Manage agent instance provisioning and container lifecycles
  infra       Manage shared Valkey, Gitea, and PostgreSQL infrastructure
  plugin      Install and manage IDE plugins (JetBrains Gateway/Toolbox, VS Code)
  gui         Launch native desktop Backplane GUI client

Commands:
  update      Synchronize git repo, rebuild host CLIs, and compile all OCI images

Agent Commands:
  %s agent create <name> [as <type|oci>] [--image <type|oci>] [--role <role>] [--model-url <url>] [--model-name <name>] [--model-key <key>]
  %s agent start <name>
  %s agent tmux <name>
  %s agent open <name> [in <ide>] [--no-launch]
  %s agent ssh <name>
  %s agent ssh-config [name] [--all]
  %s agent doctor <name>
  %s agent stop [name] [--all]
  %s agent list [--json]
  %s agent clean <name>
  %s agent retire <name> [--force]

Infra Commands:
  %s infra up
  %s infra down
  %s infra list
  %s infra doctor

Plugin Commands:
  %s plugin add <gateway|toolbox|vscode>
  %s plugin remove <gateway|toolbox|vscode>
  %s plugin list

Update Command:
  %s update

Run '%s <domain> help' for more details on each command.
`, bin, bin, bin, bin, bin, bin, bin, bin, bin, bin, bin, bin, bin, bin, bin, bin, bin, bin, bin, bin, bin, bin)
}
