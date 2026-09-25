// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/boggycreek/sandbox/pkg/config"
	"github.com/boggycreek/sandbox/pkg/doctor"
	"github.com/boggycreek/sandbox/pkg/ide"
	"github.com/boggycreek/sandbox/pkg/libbp"
	"github.com/boggycreek/sandbox/pkg/lifecycle"
	"github.com/boggycreek/sandbox/pkg/runtime"
)

func handleAgent(ctx context.Context, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: sndbx agent <create|start|tmux|open|ssh|ssh-config|doctor|stop|list|clean|retire>")
		return 1
	}

	sub := strings.ToLower(args[0])
	subArgs := args[1:]

	switch sub {
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, `Usage: sndbx agent <command> [args...]

Commands:
  create <name> [as <type|oci>] [--image <type|oci>] [--role <role>] [--model-url <url>] [--model-name <name>] [--model-key <key>]
    Provision a new named agent with persistent configuration, keys, and volume.

  start <name>
    Start the agent's daemon container with Podman.

  tmux <name>
    Attach interactively to the agent container's tmux supervisor.

  open <name> [in <ide>] [--ide <ide>] [--no-launch]
    Launch desktop IDE remote development environment (VS Code, GoLand, CLion, WebStorm, PyCharm, etc.).

  ssh <name>
    Connect directly via SSH to the agent's unprivileged environment.

  ssh-config [name] [--all]
    Generate OpenSSH host configuration stanza(s) for IDE remote development.

  doctor <name>
    Diagnose configuration, cryptographic keys, storage, and infrastructure provisioning, and auto-heal defects.

  stop [name] [--all]
    Stop a running agent container (or all agents with --all).

  list [--json]
    List all configured agents, container states, images, and SSH endpoints.

  clean <name>
    Remove the agent container while preserving its home directory volume.

  retire <name> [--force]
    Fully decommission agent across the system (container, volume, local secrets, Valkey ACLs, and Gitea account).`)
		return 0
	case "tmux":
		return handleAgentTmux(ctx, paths, subArgs, stdout, stderr)
	case "connect":
		return handleAgentConnect(ctx, paths, subArgs, stdout, stderr)
	case "ssh":
		return handleAgentSSH(ctx, paths, subArgs, stdout, stderr)
	case "create", "start", "open", "ssh-config", "sshconfig", "doctor", "stop", "list", "clean", "retire":
		opCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
		defer cancel()
		switch sub {
		case "create":
			return handleAgentCreate(opCtx, paths, subArgs, stdout, stderr)
		case "start":
			return handleAgentStart(opCtx, paths, subArgs, stdout, stderr)
		case "open":
			return handleAgentOpen(opCtx, paths, subArgs, stdout, stderr)
		case "ssh-config", "sshconfig":
			return handleAgentSSHConfig(opCtx, paths, subArgs, stdout, stderr)
		case "doctor":
			return handleAgentDoctor(opCtx, paths, subArgs, stdout, stderr)
		case "stop":
			return handleAgentStop(opCtx, paths, subArgs, stdout, stderr)
		case "list":
			return handleAgentList(opCtx, paths, subArgs, stdout, stderr)
		case "clean":
			return handleAgentClean(opCtx, paths, subArgs, stdout, stderr)
		case "retire":
			return handleAgentRetire(opCtx, paths, subArgs, stdout, stderr)
		}
		return 1
	default:
		fmt.Fprintf(stderr, "sndbx agent: unknown command %q\n", sub)
		return 1
	}
}

func handleAgentCreate(ctx context.Context, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: sndbx agent create <name> [as <type|oci>] [--image <type|oci>] [--role <role>] [--model-url <url>] [--model-name <name>] [--model-key <key>]")
		return 1
	}

	agentName := args[0]
	var image string
	var role string
	var modelURL string
	var modelName string
	var modelKey string

	// Check natural language 'as <type>'
	if len(args) >= 3 && strings.ToLower(args[1]) == "as" {
		image = args[2]
	}

	// Flag parsing for overrides
	fs := flag.NewFlagSet("agent create", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.StringVar(&image, "image", image, "OCI image or preset (base, opencode, claude, agy)")
	fs.StringVar(&role, "role", "coding-agent", "Role metadata description")
	fs.StringVar(&modelURL, "model-url", os.Getenv("OPENAI_BASE_URL"), "OpenAI-compatible inference endpoint URL")
	fs.StringVar(&modelName, "model-name", os.Getenv("OPENAI_MODEL"), "Target model name")
	fs.StringVar(&modelKey, "model-key", os.Getenv("OPENAI_API_KEY"), "Model API key (optional)")
	fs.StringVar(&modelKey, "model-api-key", os.Getenv("OPENAI_API_KEY"), "Model API key (optional)")

	var flagArgs []string
	if len(args) >= 3 && strings.ToLower(args[1]) == "as" {
		flagArgs = args[3:]
	} else if len(args) > 1 {
		flagArgs = args[1:]
	}
	if err := fs.Parse(flagArgs); err != nil {
		return 1
	}

	if image == "" {
		image = "base"
	}

	resolvedImage, _ := runtime.ResolveAgentImage(ctx, image)

	cfg, err := config.NewAgentConfig(agentName, resolvedImage, role, modelURL, modelName, modelKey)
	if err != nil {
		fmt.Fprintf(stderr, "sndbx error: %v\n", err)
		return 1
	}

	if err := config.SaveAgentConfig(cfg, paths); err != nil {
		fmt.Fprintf(stderr, "sndbx error saving config: %v\n", err)
		return 1
	}

	// Register ACL with live Valkey instance if running
	registerValkeyACL(ctx, cfg, paths)

	// Register User, Key, and Org Membership with live Gitea instance if running
	registerGiteaUser(ctx, cfg, paths)

	// Register User and Analysis Token with live SonarQube instance if running
	registerSonarUser(ctx, cfg, paths)

	fmt.Fprintf(stdout, "Agent %q created successfully.\n", cfg.Name)
	fmt.Fprintf(stdout, "  Image:      %s\n", cfg.Image)
	fmt.Fprintf(stdout, "  Role:       %s\n", cfg.Role)
	if cfg.ModelURL != "" {
		fmt.Fprintf(stdout, "  Model URL:  %s\n", cfg.ModelURL)
	}
	if cfg.ModelName != "" {
		fmt.Fprintf(stdout, "  Model Name: %s\n", cfg.ModelName)
	}
	fmt.Fprintf(stdout, "  Container:  %s\n", cfg.ContainerName)
	fmt.Fprintf(stdout, "  Volume:     %s\n", cfg.VolumeName)
	fmt.Fprintf(stdout, "To start: sndbx agent start %s\n", cfg.Name)
	return 0
}

func registerValkeyACL(ctx context.Context, cfg *config.AgentConfig, paths config.Paths) {
	_ = lifecycle.RegisterValkeyACL(ctx, cfg, paths)
}

func registerGiteaUser(ctx context.Context, cfg *config.AgentConfig, paths config.Paths) {
	_ = lifecycle.RegisterGiteaUser(ctx, cfg, paths)
}

func registerSonarUser(ctx context.Context, cfg *config.AgentConfig, paths config.Paths) {
	_ = lifecycle.RegisterSonarUser(ctx, cfg, paths)
}

func handleAgentStart(ctx context.Context, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: sndbx agent start <name>")
		return 1
	}

	name := args[0]
	cfg, err := config.LoadAgentConfig(name, paths)
	if err != nil {
		fmt.Fprintf(stderr, "sndbx error: %v\n", err)
		return 1
	}

	bpCfg := libbp.LoadClientFromEnv()
	if err := runtime.StartAgentContainer(ctx, cfg, paths, bpCfg.Host, bpCfg.Port); err != nil {
		fmt.Fprintf(stderr, "sndbx error: %v\n", err)
		return 1
	}

	_ = runtime.SyncSSHConfigFile(ctx, paths)

	fmt.Fprintf(stdout, "Agent %q started (%s).\n", cfg.Name, cfg.ContainerName)
	fmt.Fprintf(stdout, "SSH:   sndbx agent ssh %s\n", cfg.Name)
	fmt.Fprintf(stdout, "IDE:   sndbx agent open %s\n", cfg.Name)
	fmt.Fprintf(stdout, "Tmux:  sndbx agent tmux %s\n", cfg.Name)
	return 0
}

func handleAgentTmux(ctx context.Context, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: sndbx agent tmux <name>")
		return 1
	}
	name := args[0]
	cfg, err := config.LoadAgentConfig(name, paths)
	if err != nil {
		fmt.Fprintf(stderr, "sndbx error: %v\n", err)
		return 1
	}

	cmd := execCommandContext(ctx, "podman", "exec", "-it", cfg.ContainerName, "tmux", "attach")
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(stderr, "sndbx tmux error: %v\n", err)
		return 1
	}
	return 0
}

func handleAgentConnect(ctx context.Context, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Notice: 'sndbx agent connect' is deprecated. Please use 'sndbx agent tmux <name>' instead.")
		fmt.Fprintln(stderr, "Usage: sndbx agent connect <name>")
		return 1
	}
	fmt.Fprintf(stderr, "Notice: 'sndbx agent connect' is deprecated. Please use 'sndbx agent tmux %s' instead.\n", args[0])
	return handleAgentTmux(ctx, paths, args, stdout, stderr)
}

func handleAgentOpen(ctx context.Context, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: sndbx agent open <name> [in <ide>] [--ide <ide>] [--no-launch]")
		return 1
	}

	name := args[0]
	ideName := "code" // sensible default; overridden if not installed
	noLaunch := false

	// Parse the optional positional "in <ide>" syntax before flag parsing.
	flagArgs := args[1:]
	if len(args) >= 3 && strings.ToLower(args[1]) == "in" {
		ideName = args[2]
		flagArgs = args[3:]
	} else if len(args) == 2 && strings.ToLower(args[1]) == "in" {
		fmt.Fprintln(stderr, "Usage: sndbx agent open <name> [in <ide>] [--ide <ide>] [--no-launch]")
		return 1
	}

	fs := flag.NewFlagSet("agent open", flag.ContinueOnError)
	fs.SetOutput(stderr)

	// Build a dynamic help string listing installed IDEs.
	installedNames := ide.NamesInstalled()
	ideHelp := "Target IDE"
	if len(installedNames) > 0 {
		ideHelp = "Target IDE (installed: " + strings.Join(installedNames, ", ") + ")"
	}
	fs.StringVar(&ideName, "ide", ideName, ideHelp)
	fs.BoolVar(&noLaunch, "no-launch", noLaunch, "Prepare SSH configuration without launching IDE")
	if err := fs.Parse(flagArgs); err != nil {
		return 1
	}

	// Resolve the requested IDE against the installed catalog.
	ideName = strings.ToLower(strings.TrimSpace(ideName))
	target, installed := ide.Lookup(ideName)
	if target.Name == "" {
		// Completely unknown — not even in the catalog.
		help := "(none detected)"
		if len(installedNames) > 0 {
			help = strings.Join(installedNames, ", ")
		}
		fmt.Fprintf(stderr, "sndbx error: unknown IDE %q. Installed IDEs: %s\n", ideName, help)
		return 1
	}
	if !installed {
		help := "(none detected)"
		if len(installedNames) > 0 {
			help = strings.Join(installedNames, ", ")
		}
		fmt.Fprintf(stderr, "sndbx error: %s (%s) is not installed on this host. Installed IDEs: %s\n",
			target.DisplayName, target.BinaryName, help)
		return 1
	}

	cfg, err := config.LoadAgentConfig(name, paths)
	if err != nil {
		fmt.Fprintf(stderr, "sndbx error: %v\n", err)
		return 1
	}

	port, err := runtime.GetAgentSSHPort(ctx, cfg.ContainerName)
	if err != nil || port <= 0 {
		bpCfg := libbp.LoadClientFromEnv()
		if startErr := runtime.StartAgentContainer(ctx, cfg, paths, bpCfg.Host, bpCfg.Port); startErr != nil {
			fmt.Fprintf(stderr, "sndbx error starting agent container %s: %v\n", cfg.ContainerName, startErr)
			return 1
		}
		port, err = runtime.GetAgentSSHPort(ctx, cfg.ContainerName)
		if err != nil || port <= 0 {
			fmt.Fprintf(stderr, "sndbx error discovering SSH port for %s: %v\n", cfg.Name, err)
			return 1
		}
	}

	if err := runtime.SyncSSHConfigFile(ctx, paths); err != nil {
		fmt.Fprintf(stderr, "sndbx error syncing SSH config: %v\n", err)
		return 1
	}

	hostAlias := fmt.Sprintf("sndbx-%s", name)

	// Build connection descriptors for each IDE family.
	vsCodeURI := fmt.Sprintf("vscode-remote://ssh-remote+%s/home/agent/workspace", hostAlias)
	jbSSHURI := fmt.Sprintf("ssh://agent@localhost:%d/home/agent/workspace", port)

	if noLaunch {
		fmt.Fprintf(stdout, "SSH host alias prepared: %s (Port: %d)\n", hostAlias, port)
		switch target.Family {
		case ide.FamilyVSCode:
			fmt.Fprintf(stdout, "%s Remote URI: %s\n", target.DisplayName, vsCodeURI)
			fmt.Fprintf(stdout, "Launch command: %s --file-uri %s\n", target.BinaryName, vsCodeURI)
		case ide.FamilyJetBrains:
			fmt.Fprintf(stdout, "Connect via %s with Host %q (Port: %d).\n", target.DisplayName, hostAlias, port)
			fmt.Fprintf(stdout, "Launch command: %s --remote-dev %q\n", target.BinaryName, jbSSHURI)
		}
		return 0
	}

	switch target.Family {
	case ide.FamilyVSCode:
		fmt.Fprintf(stdout, "Opening %s in %s (Host: %s)...\n", name, target.DisplayName, hostAlias)
		cmd := execCommandContext(ctx, target.Path, "--file-uri", vsCodeURI)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(stderr, "sndbx error launching %s: %v\n", target.DisplayName, err)
			return 1
		}
		// Best-effort: install the Claude Code extension if VS Code server is running.
		checkServer := execCommandContext(ctx, "podman", "exec", cfg.ContainerName, "sh", "-c", "test -d /home/agent/.vscode-server")
		if err := checkServer.Run(); err == nil {
			installCmd := execCommandContext(ctx, "podman", "exec", cfg.ContainerName, "sh", "-c",
				`find /home/agent/.vscode-server/bin -name "code-server" -o -name "code" 2>/dev/null | head -n 1 | xargs -r -I {} {} --install-extension anthropic.claude-code`)
			_ = installCmd.Run()
		}

	case ide.FamilyJetBrains:
		fmt.Fprintf(stdout, "Opening %s in %s via Remote Development...\n", name, target.DisplayName)
		cmd := execCommandContext(ctx, target.Path, "--remote-dev", jbSSHURI)
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(stderr, "sndbx error launching %s: %v\n", target.DisplayName, err)
			return 1
		}
	}

	return 0
}

func handleAgentSSH(ctx context.Context, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: sndbx agent ssh <name>")
		return 1
	}
	name := args[0]
	cfg, err := config.LoadAgentConfig(name, paths)
	if err != nil {
		fmt.Fprintf(stderr, "sndbx error: %v\n", err)
		return 1
	}

	port, err := runtime.GetAgentSSHPort(ctx, cfg.ContainerName)
	if err != nil {
		fmt.Fprintf(stderr, "sndbx error discovering SSH port for %s: %v\n", cfg.Name, err)
		return 1
	}

	sshArgs := []string{
		"-i", paths.IDEKeyFile,
		"-p", fmt.Sprintf("%d", port),
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"agent@localhost",
	}

	cmd := execCommandContext(ctx, "ssh", sshArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return 1
	}
	return 0
}

func handleAgentSSHConfig(ctx context.Context, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	var targetAgent string
	for _, a := range args {
		if a != "--all" && !strings.HasPrefix(a, "-") && targetAgent == "" {
			targetAgent = a
		}
	}

	if targetAgent != "" {
		cfg, err := config.LoadAgentConfig(targetAgent, paths)
		if err != nil {
			fmt.Fprintf(stderr, "sndbx error: %v\n", err)
			return 1
		}
		port, err := runtime.GetAgentSSHPort(ctx, cfg.ContainerName)
		if err != nil || port <= 0 {
			fmt.Fprintf(stderr, "sndbx error: agent %q is not running or SSH port is unavailable\n", cfg.Name)
			return 1
		}
		printSSHConfigBlock(stdout, cfg.Name, port, paths.IDEKeyFile)
		return 0
	}

	configs, err := config.ListAgentConfigs(paths)
	if err != nil {
		fmt.Fprintf(stderr, "sndbx error: %v\n", err)
		return 1
	}

	runningCount := 0
	for _, c := range configs {
		port, err := runtime.GetAgentSSHPort(ctx, c.ContainerName)
		if err == nil && port > 0 {
			if runningCount > 0 {
				fmt.Fprintln(stdout)
			}
			printSSHConfigBlock(stdout, c.Name, port, paths.IDEKeyFile)
			runningCount++
		}
	}

	return 0
}

func printSSHConfigBlock(w io.Writer, name string, port int, keyFile string) {
	fmt.Fprint(w, runtime.FormatSSHConfigBlock(name, port, keyFile))
}

func handleAgentDoctor(ctx context.Context, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: sndbx agent doctor <name>")
		return 1
	}

	name := args[0]
	report, err := doctor.DiagnoseAndHealAgent(ctx, name, paths)
	if err != nil && report == nil {
		fmt.Fprintf(stderr, "sndbx doctor error: %v\n", err)
		return 1
	}

	fmt.Fprint(stdout, doctor.FormatDoctorReport(report))
	if report.UnrepairableCount > 0 {
		return 1
	}
	return 0
}

func handleAgentStop(ctx context.Context, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: sndbx agent stop <name> [--all]")
		return 1
	}

	if args[0] == "--all" {
		configs, _ := config.ListAgentConfigs(paths)
		for _, c := range configs {
			_ = runtime.StopAgentContainer(ctx, c.ContainerName)
			fmt.Fprintf(stdout, "Stopped %s\n", c.Name)
		}
		_ = runtime.SyncSSHConfigFile(ctx, paths)
		return 0
	}

	name := args[0]
	cfg, err := config.LoadAgentConfig(name, paths)
	if err != nil {
		fmt.Fprintf(stderr, "sndbx error: %v\n", err)
		return 1
	}

	if err := runtime.StopAgentContainer(ctx, cfg.ContainerName); err != nil {
		fmt.Fprintf(stderr, "sndbx error stopping %s: %v\n", cfg.Name, err)
		return 1
	}
	_ = runtime.SyncSSHConfigFile(ctx, paths)
	fmt.Fprintf(stdout, "Agent %q stopped.\n", cfg.Name)
	return 0
}

func handleAgentClean(ctx context.Context, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: sndbx agent clean <name>")
		return 1
	}
	name := args[0]
	cfg, err := config.LoadAgentConfig(name, paths)
	if err != nil {
		fmt.Fprintf(stderr, "sndbx error: %v\n", err)
		return 1
	}

	if err := runtime.CleanAgentContainer(ctx, cfg.ContainerName); err != nil {
		fmt.Fprintf(stderr, "sndbx error cleaning %s: %v\n", cfg.Name, err)
		return 1
	}
	_ = runtime.SyncSSHConfigFile(ctx, paths)
	fmt.Fprintf(stdout, "Agent container %q removed (home volume preserved).\n", cfg.ContainerName)
	return 0
}

func handleAgentRetire(ctx context.Context, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: sndbx agent retire <name> [--force]")
		return 1
	}

	var name string
	force := false
	for _, arg := range args {
		if arg == "--force" || arg == "-f" {
			force = true
		} else if name == "" {
			name = strings.ToLower(strings.TrimSpace(arg))
		}
	}

	if name == "" {
		fmt.Fprintln(stderr, "Usage: sndbx agent retire <name> [--force]")
		return 1
	}
	_ = force // Flag parsed and supported for scripts/automation

	cfg, err := config.LoadAgentConfig(name, paths)
	if err != nil {
		fmt.Fprintf(stderr, "sndbx error: %v\n", err)
		return 1
	}

	// 1. Destroy container and persistent volume
	_ = runtime.DestroyAgentContainer(ctx, cfg.ContainerName, cfg.VolumeName)

	// 2. Delete local config and secrets
	_ = config.DeleteAgentConfig(name, paths)

	// 2b. Clear per-agent known hosts file
	runtime.ClearAgentKnownHosts(name, paths)

	// 3. Deprovision Valkey ACL user & streams
	deprovisionValkeyUser(ctx, name)

	// 4. Deprovision Gitea user & keys
	deprovisionGiteaUser(ctx, name)

	// 4b. Deprovision SonarQube user & analysis tokens
	deprovisionSonarUser(ctx, name)

	// 5. Update SSH config file
	_ = runtime.SyncSSHConfigFile(ctx, paths)

	fmt.Fprintf(stdout, "Agent %q retired and deprovisioned successfully.\n", name)
	fmt.Fprintf(stdout, "  ✓ Container (%s) and volume (%s) removed\n", cfg.ContainerName, cfg.VolumeName)
	fmt.Fprintf(stdout, "  ✓ Local configuration and secrets purged\n")
	fmt.Fprintf(stdout, "  ✓ Valkey ACL user and backplane identity removed\n")
	fmt.Fprintf(stdout, "  ✓ Gitea user account and authorized keys purged\n")
	fmt.Fprintf(stdout, "  ✓ SonarQube user account and analysis tokens purged\n")
	return 0
}

func deprovisionValkeyUser(ctx context.Context, agentName string) {
	_ = lifecycle.DeprovisionValkeyUser(ctx, agentName)
}

func deprovisionGiteaUser(ctx context.Context, agentName string) {
	_ = lifecycle.DeprovisionGiteaUser(ctx, agentName)
}

func deprovisionSonarUser(ctx context.Context, agentName string) {
	_ = lifecycle.DeprovisionSonarUser(ctx, agentName)
}

func handleAgentList(ctx context.Context, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	jsonOutput := false
	for _, a := range args {
		if a == "--json" {
			jsonOutput = true
		}
	}

	configs, err := config.ListAgentConfigs(paths)
	if err != nil {
		fmt.Fprintf(stderr, "sndbx error: %v\n", err)
		return 1
	}

	statuses := make([]runtime.AgentStatus, 0, len(configs))
	for _, c := range configs {
		state := "stopped"
		info, err := runtime.InspectAgentContainer(ctx, c.ContainerName)
		if err == nil && info != nil {
			state = info.State
		}

		sshPort := 0
		ideConnect := ""
		if state == "running" {
			if p, err := runtime.GetAgentSSHPort(ctx, c.ContainerName); err == nil && p > 0 {
				sshPort = p
				ideConnect = fmt.Sprintf("agent@localhost:%d", p)
			}
		}

		statuses = append(statuses, runtime.AgentStatus{
			Name:           c.Name,
			Role:           c.Role,
			Image:          c.Image,
			ContainerName:  c.ContainerName,
			ContainerState: state,
			SSHPort:        sshPort,
			IDEConnect:     ideConnect,
		})
	}

	if jsonOutput {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(statuses)
		return 0
	}

	fmtFmt := "%-16s %-14s %-12s %-26s %s\n"
	fmt.Fprintf(stdout, fmtFmt, "AGENT_NAME", "ROLE", "STATUS", "IMAGE", "IDE_SSH")
	for _, s := range statuses {
		ide := s.IDEConnect
		if ide == "" {
			ide = "-"
		}
		fmt.Fprintf(stdout, fmtFmt, s.Name, s.Role, s.ContainerState, s.Image, ide)
	}
	return 0
}
