// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/boggycreek/sandbox/pkg/config"
	"github.com/boggycreek/sandbox/pkg/plugin"
)

func handlePlugin(ctx context.Context, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printPluginUsage(stderr)
		return 1
	}

	action := strings.ToLower(args[0])
	switch action {
	case "help", "-h", "--help":
		printPluginUsage(stdout)
		return 0

	case "list":
		plugins := plugin.ListPlugins(paths)
		w := tabwriter.NewWriter(stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "PLUGIN\tTARGET\tSTATUS\tDESCRIPTION")
		for _, p := range plugins {
			status := "Not Installed"
			if p.Installed {
				status = "Installed"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.ID, p.Target, status, p.Description)
		}
		_ = w.Flush()
		return 0

	case "add":
		if len(args) < 2 {
			fmt.Fprintln(stderr, "Usage: sndbx plugin add <gateway|toolbox|vscode>")
			return 1
		}
		return handlePluginAdd(ctx, paths, strings.ToLower(args[1]), stdout, stderr)

	case "remove":
		if len(args) < 2 {
			fmt.Fprintln(stderr, "Usage: sndbx plugin remove <gateway|toolbox|vscode>")
			return 1
		}
		return handlePluginRemove(ctx, paths, strings.ToLower(args[1]), stdout, stderr)

	default:
		fmt.Fprintf(stderr, "sndbx plugin: unknown command %q (see 'sndbx plugin help')\n", action)
		return 1
	}
}

func handlePluginAdd(ctx context.Context, paths config.Paths, target string, stdout, stderr io.Writer) int {
	switch target {
	case "gateway":
		res, err := plugin.InstallGatewayPlugin(ctx, paths, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "sndbx error installing gateway plugin: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "Agent Sandbox JetBrains Gateway Plugin")
		fmt.Fprintln(stdout, "=====================================")
		fmt.Fprintf(stdout, "Plugin:     %s (v%s)\n", res.PluginName, res.Version)
		fmt.Fprintf(stdout, "Status:     %s\n", res.Message)
		if res.SSHConfigLinked {
			fmt.Fprintln(stdout, "SSH Config: Linked managed config into ~/.ssh/config")
		} else {
			fmt.Fprintln(stdout, "SSH Config: Already linked in ~/.ssh/config")
		}
		fmt.Fprintln(stdout, "\nInstalled Locations:")
		for _, loc := range res.InstalledPaths {
			fmt.Fprintf(stdout, "  - %s\n", loc)
		}
		fmt.Fprintln(stdout, "\nNext Steps:")
		fmt.Fprintln(stdout, "  1. Launch JetBrains Gateway or your JetBrains IDE (GoLand, CLion, WebStorm, PyCharm).")
		fmt.Fprintln(stdout, "  2. Connect to your running agents via the 'Agent Sandbox' provider")
		fmt.Fprintln(stdout, "     or select any 'sndbx-<name>' host under SSH Connections.")
		return 0

	case "toolbox":
		res, err := plugin.InstallToolboxPlugin(ctx, paths, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "sndbx error installing toolbox plugin: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "Agent Sandbox JetBrains Toolbox Plugin")
		fmt.Fprintln(stdout, "=====================================")
		fmt.Fprintf(stdout, "Plugin:     %s (v%s)\n", res.PluginName, res.Version)
		fmt.Fprintf(stdout, "Status:     %s\n", res.Message)
		if res.SSHConfigLinked {
			fmt.Fprintln(stdout, "SSH Config: Linked managed config into ~/.ssh/config")
		} else {
			fmt.Fprintln(stdout, "SSH Config: Already linked in ~/.ssh/config")
		}
		fmt.Fprintln(stdout, "\nInstalled Locations:")
		for _, loc := range res.InstalledPaths {
			fmt.Fprintf(stdout, "  - %s\n", loc)
		}
		fmt.Fprintln(stdout, "\nNext Steps:")
		fmt.Fprintln(stdout, "  1. Restart JetBrains Toolbox.")
		fmt.Fprintln(stdout, "  2. Connect to running agents under 'SSH Connections' or the 'Agent Sandbox' provider.")
		return 0

	case "vscode":
		res, err := plugin.InstallVSCodePlugin(ctx, paths, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "sndbx error configuring VS Code integration: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "Agent Sandbox VS Code Integration")
		fmt.Fprintln(stdout, "==================================")
		fmt.Fprintf(stdout, "Status:     %s\n", res.Message)
		if res.SSHConfigLinked {
			fmt.Fprintln(stdout, "SSH Config: Linked managed config into ~/.ssh/config")
		} else {
			fmt.Fprintln(stdout, "SSH Config: Already linked in ~/.ssh/config")
		}
		fmt.Fprintln(stdout, "\nNext Steps:")
		fmt.Fprintln(stdout, "  Run 'sndbx agent open <name>' to launch an agent in VS Code.")
		return 0

	default:
		fmt.Fprintf(stderr, "sndbx plugin: unknown plugin %q (see 'sndbx plugin help')\n", target)
		return 1
	}
}

func handlePluginRemove(ctx context.Context, paths config.Paths, target string, stdout, stderr io.Writer) int {
	switch target {
	case "gateway":
		res, err := plugin.RemoveGatewayPlugin(ctx, paths, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "sndbx error removing gateway plugin: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "Agent Sandbox JetBrains Gateway Plugin")
		fmt.Fprintln(stdout, "=====================================")
		fmt.Fprintf(stdout, "Plugin:     %s\n", res.PluginName)
		fmt.Fprintf(stdout, "Status:     %s\n", res.Message)
		if res.SSHConfigUnlinked {
			fmt.Fprintln(stdout, "SSH Config: Unlinked managed config from ~/.ssh/config")
		}
		if len(res.RemovedPaths) > 0 {
			fmt.Fprintln(stdout, "\nRemoved Locations:")
			for _, loc := range res.RemovedPaths {
				fmt.Fprintf(stdout, "  - %s\n", loc)
			}
		}
		return 0

	case "toolbox":
		res, err := plugin.RemoveToolboxPlugin(ctx, paths, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "sndbx error removing toolbox plugin: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "Agent Sandbox JetBrains Toolbox Plugin")
		fmt.Fprintln(stdout, "=====================================")
		fmt.Fprintf(stdout, "Plugin:     %s\n", res.PluginName)
		fmt.Fprintf(stdout, "Status:     %s\n", res.Message)
		if res.SSHConfigUnlinked {
			fmt.Fprintln(stdout, "SSH Config: Unlinked managed config from ~/.ssh/config")
		}
		if len(res.RemovedPaths) > 0 {
			fmt.Fprintln(stdout, "\nRemoved Locations:")
			for _, loc := range res.RemovedPaths {
				fmt.Fprintf(stdout, "  - %s\n", loc)
			}
		}
		return 0

	case "vscode":
		res, err := plugin.RemoveVSCodePlugin(ctx, paths, stdout)
		if err != nil {
			fmt.Fprintf(stderr, "sndbx error removing VS Code integration: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "Agent Sandbox VS Code Integration")
		fmt.Fprintln(stdout, "==================================")
		fmt.Fprintf(stdout, "Status:     %s\n", res.Message)
		if res.SSHConfigUnlinked {
			fmt.Fprintln(stdout, "SSH Config: Unlinked managed config from ~/.ssh/config")
		}
		return 0

	default:
		fmt.Fprintf(stderr, "sndbx plugin: unknown plugin %q (see 'sndbx plugin help')\n", target)
		return 1
	}
}

func printPluginUsage(out io.Writer) {
	fmt.Fprintln(out, `Agent Sandbox Plugin Manager

Usage:
  sndbx plugin <command> [target]

Commands:
  add <plugin>       Install and configure the specified IDE plugin
  remove <plugin>    Uninstall and remove the specified IDE plugin
  list               List supported and installed IDE plugins
  help               Show this help message

Supported Targets:
  gateway            JetBrains Gateway and IntelliJ IDEs (GoLand, CLion, WebStorm, PyCharm)
  toolbox            JetBrains Toolbox desktop app provider & native SSH sync
  vscode             VS Code Remote-SSH and Claude Code integration`)
}
