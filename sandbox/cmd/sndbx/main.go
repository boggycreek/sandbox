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
		fmt.Fprintf(stderr, "sndbx warning: failed to ensure directories: %v\n", err)
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
		fmt.Fprintf(stderr, "sndbx: unknown command %q (see 'sndbx help')\n", domain)
		return 1
	}
}

func printUsage(out io.Writer) {
	fmt.Fprintln(out, `Agent Sandbox CLI (sndbx)

Usage:
  sndbx <domain|command> [args...]

Domains:
  agent       Manage agent instance provisioning and container lifecycles
  infra       Manage shared Valkey, Gitea, and PostgreSQL infrastructure
  plugin      Install and manage IDE plugins (JetBrains Gateway/Toolbox, VS Code)
  gui         Launch native desktop Backplane GUI client

Commands:
  update      Synchronize git repo, rebuild host CLIs, and compile all OCI images

Agent Commands:
  sndbx agent create <name> [as <type|oci>] [--image <type|oci>] [--role <role>] [--model-url <url>] [--model-name <name>] [--model-key <key>]
  sndbx agent start <name>
  sndbx agent tmux <name>
  sndbx agent open <name> [in <ide>] [--ide <ide>] [--no-launch]
  sndbx agent ssh <name>
  sndbx agent ssh-config [name] [--all]
  sndbx agent doctor <name>
  sndbx agent stop [name] [--all]
  sndbx agent list [--json]
  sndbx agent clean <name>
  sndbx agent retire <name> [--force]

Infra Commands:
  sndbx infra up
  sndbx infra down
  sndbx infra list
  sndbx infra doctor

Plugin Commands:
  sndbx plugin add <gateway|toolbox|vscode>
  sndbx plugin remove <gateway|toolbox|vscode>
  sndbx plugin list

Update Command:
  sndbx update

Run 'sndbx <domain> help' for more details on each command.`)
}
