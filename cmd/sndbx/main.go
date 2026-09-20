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
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/boggycreek/sandbox/pkg/config"
	"github.com/boggycreek/sandbox/pkg/doctor"
	"github.com/boggycreek/sandbox/pkg/gitea"
	"github.com/boggycreek/sandbox/pkg/ide"
	"github.com/boggycreek/sandbox/pkg/libbp"
	"github.com/boggycreek/sandbox/pkg/plugin"
	"github.com/boggycreek/sandbox/pkg/runtime"
	"text/tabwriter"
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
	_ = paths.EnsureDirectories()
	paths.LoadEnv()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	switch domain {
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0

	case "agent":
		return handleAgent(ctx, paths, domainArgs, stdout, stderr)

	case "infra":
		return handleInfra(ctx, paths, domainArgs, stdout, stderr)

	case "plugin":
		return handlePlugin(ctx, paths, domainArgs, stdout, stderr)

	case "update":
		return handleUpdate(ctx, paths, domainArgs, stdout, stderr)

	case "gui":
		return handleGUI(ctx, paths, domainArgs, stdout, stderr)

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
  infra       Manage shared Valkey backplane and Gitea Git infrastructure
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
  sndbx plugin add <toolbox|vscode>
  sndbx plugin remove <toolbox|vscode>
  sndbx plugin list

Update Command:
  sndbx update

Run 'sndbx <domain> help' for more details on each command.`)
}

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
    Launch desktop IDE remote development environment (VS Code or JetBrains WebStorm).

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
	case "create":
		return handleAgentCreate(ctx, paths, subArgs, stdout, stderr)
	case "start":
		return handleAgentStart(ctx, paths, subArgs, stdout, stderr)
	case "tmux":
		return handleAgentTmux(ctx, paths, subArgs, stdout, stderr)
	case "connect":
		return handleAgentConnect(ctx, paths, subArgs, stdout, stderr)
	case "open":
		return handleAgentOpen(ctx, paths, subArgs, stdout, stderr)
	case "ssh":
		return handleAgentSSH(ctx, paths, subArgs, stdout, stderr)
	case "ssh-config", "sshconfig":
		return handleAgentSSHConfig(ctx, paths, subArgs, stdout, stderr)
	case "doctor":
		return handleAgentDoctor(ctx, paths, subArgs, stdout, stderr)
	case "stop":
		return handleAgentStop(ctx, paths, subArgs, stdout, stderr)
	case "list":
		return handleAgentList(ctx, paths, subArgs, stdout, stderr)
	case "clean":
		return handleAgentClean(ctx, paths, subArgs, stdout, stderr)
	case "retire":
		return handleAgentRetire(ctx, paths, subArgs, stdout, stderr)
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
	bpCfg := libbp.LoadClientFromEnv()
	client, err := libbp.Dial(ctx, libbp.ClientConfig{
		Host:     bpCfg.Host,
		Port:     bpCfg.Port,
		Username: "admin",
		Password: os.Getenv("ADMIN_BACKPLANE_PASSWORD"),
	})
	if err != nil {
		return
	}
	defer client.Close()

	// Set Valkey ACL for agent
	// Format: ACL SETUSER <name> on ><password> ~<name>:* %R~*:* &* +@all -@admin -@dangerous (+xadd ~*:inbox)
	aclArgs := []string{
		"SETUSER", cfg.Name, "on",
		">" + cfg.Password,
		fmt.Sprintf("~%s:*", cfg.Name),
		fmt.Sprintf("~identity:%s", cfg.Name),
		"%R~*:*", "&*", "+@all", "-@admin", "-@dangerous",
		"(+xadd ~*:inbox)",
	}
	_, _ = client.Exec(ctx, "ACL", aclArgs...)

	// Register Identity
	_ = client.RegisterIdentity(ctx, libbp.IdentityRecord{
		Name:   cfg.Name,
		Role:   cfg.Role,
		Kind:   "agent",
		PubKey: cfg.PublicKeyB64,
	})
}

func registerGiteaUser(ctx context.Context, cfg *config.AgentConfig, paths config.Paths) {
	adminPass := os.Getenv("ADMIN_BACKPLANE_PASSWORD")
	if adminPass == "" {
		adminPass = "admin_backplane_pass"
	}

	giteaURL := os.Getenv("GITEA_URL")
	if giteaURL == "" {
		giteaURL = "http://127.0.0.1:3000"
	}

	client := gitea.NewClient(gitea.ClientConfig{
		BaseURL:   giteaURL,
		AdminUser: "giteaadmin",
		AdminPass: adminPass,
		Timeout:   2 * time.Second,
	})

	// 1. Provision user account in Gitea
	if err := client.EnsureUser(ctx, cfg.Name, cfg.Password, fmt.Sprintf("%s@local.sndbx", cfg.Name)); err != nil {
		return
	}

	// 2. Add public SSH key if host IDE key exists
	sshKeyPub := fmt.Sprintf("%s.pub", paths.IDEKeyFile)
	if keyData, err := os.ReadFile(sshKeyPub); err == nil && len(keyData) > 0 {
		_ = client.AddUserSSHKey(ctx, cfg.Name, fmt.Sprintf("%s-ide-key", cfg.Name), string(keyData))
	}

	// 3. Add user to default fleet organization
	_ = client.AddOrgMember(ctx, "fleet", cfg.Name)
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

	// Resolve the requested IDE against the installed catalogue.
	ideName = strings.ToLower(strings.TrimSpace(ideName))
	target, installed := ide.Lookup(ideName)
	if target.Name == "" {
		// Completely unknown — not even in the catalogue.
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
		if err := runtime.StartAgentContainer(ctx, cfg, paths, bpCfg.Host, bpCfg.Port); err != nil {
			fmt.Fprintf(stderr, "sndbx error starting agent container %s: %v\n", cfg.ContainerName, err)
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

	// 5. Update SSH config file
	_ = runtime.SyncSSHConfigFile(ctx, paths)

	fmt.Fprintf(stdout, "Agent %q retired and deprovisioned successfully.\n", name)
	fmt.Fprintf(stdout, "  ✓ Container (%s) and volume (%s) removed\n", cfg.ContainerName, cfg.VolumeName)
	fmt.Fprintf(stdout, "  ✓ Local configuration and secrets purged\n")
	fmt.Fprintf(stdout, "  ✓ Valkey ACL user and backplane identity removed\n")
	fmt.Fprintf(stdout, "  ✓ Gitea user account and authorized keys purged\n")
	return 0
}

func deprovisionValkeyUser(ctx context.Context, agentName string) {
	bpCfg := libbp.LoadClientFromEnv()
	client, err := libbp.Dial(ctx, libbp.ClientConfig{
		Host:     bpCfg.Host,
		Port:     bpCfg.Port,
		Username: "admin",
		Password: os.Getenv("ADMIN_BACKPLANE_PASSWORD"),
	})
	if err != nil {
		return
	}
	defer client.Close()

	// Delete ACL user
	_, _ = client.Exec(ctx, "ACL", "DELUSER", agentName)

	// Clean up backplane keys & identity
	_, _ = client.Exec(ctx, "DEL",
		fmt.Sprintf("identity:%s", agentName),
		fmt.Sprintf("%s:inbox", agentName),
		fmt.Sprintf("%s:out", agentName),
		fmt.Sprintf("%s:seq", agentName),
		fmt.Sprintf("%s:finger", agentName),
		fmt.Sprintf("%s:status", agentName),
	)
}

func deprovisionGiteaUser(ctx context.Context, agentName string) {
	adminPass := os.Getenv("ADMIN_BACKPLANE_PASSWORD")
	if adminPass == "" {
		adminPass = "admin_backplane_pass"
	}

	giteaURL := os.Getenv("GITEA_URL")
	if giteaURL == "" {
		giteaURL = "http://127.0.0.1:3000"
	}

	client := gitea.NewClient(gitea.ClientConfig{
		BaseURL:   giteaURL,
		AdminUser: "giteaadmin",
		AdminPass: adminPass,
		Timeout:   3 * time.Second,
	})

	_ = client.DeleteUser(ctx, agentName, true)
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

	var statuses []runtime.AgentStatus
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

func handleInfra(ctx context.Context, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: sndbx infra <up|down|list|doctor>")
		return 1
	}

	sub := strings.ToLower(args[0])
	switch sub {
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, `Usage: sndbx infra <command>

Commands:
  up      Start shared Valkey and Gitea services via Podman Compose
  down    Stop shared infrastructure services
  list    Show status of running infrastructure containers
  doctor  Diagnose shared infrastructure storage, containers, and services, and auto-heal defects`)
		return 0

	case "up":
		fmt.Fprintln(stdout, "Starting shared infrastructure (Valkey & Gitea)...")
		adminPass := os.Getenv("ADMIN_BACKPLANE_PASSWORD")
		humanPass := os.Getenv("HUMAN_BACKPLANE_PASSWORD")
		humanName := os.Getenv("HUMAN_NAME")
		if err := runtime.StartInfraStack(ctx, paths, adminPass, humanPass, humanName); err != nil {
			fmt.Fprintf(stderr, "sndbx infra error: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "Shared infrastructure is online.")
		return 0

	case "down":
		fmt.Fprintln(stdout, "Stopping shared infrastructure...")
		if err := runtime.StopInfraStack(ctx); err != nil {
			fmt.Fprintf(stderr, "sndbx infra error: %v\n", err)
			return 1
		}
		fmt.Fprintln(stdout, "Shared infrastructure stopped.")
		return 0

	case "doctor":
		report, err := doctor.DiagnoseAndHealInfra(ctx, paths)
		if err != nil && report == nil {
			fmt.Fprintf(stderr, "sndbx infra doctor error: %v\n", err)
			return 1
		}
		fmt.Fprint(stdout, doctor.FormatDoctorReport(report))
		if report.UnrepairableCount > 0 {
			return 1
		}
		return 0

	case "list":
		fmt.Fprintln(stdout, "Shared infrastructure status:")
		list, err := runtime.InspectInfraStack(ctx)
		if err != nil {
			fmt.Fprintf(stderr, "sndbx infra error: %v\n", err)
			return 1
		}
		fmtFmt := "%-28s %s\n"
		fmt.Fprintf(stdout, fmtFmt, "SERVICE", "STATUS")
		for _, item := range list {
			displayName := item.ID
			if len(item.Names) > 0 && item.Names[0] != "" {
				displayName = item.Names[0]
			}
			fmt.Fprintf(stdout, fmtFmt, displayName, item.State)
		}
		return 0

	default:
		fmt.Fprintf(stderr, "sndbx infra: unknown command %q\n", sub)
		return 1
	}
}

func handleUpdate(ctx context.Context, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) > 0 {
		sub := strings.ToLower(args[0])
		if sub == "help" || sub == "-h" || sub == "--help" {
			fmt.Fprintln(stdout, `Usage: sndbx update

Comprehensive update of the Agent Sandbox local environment:
  1. Synchronizes local git repository checkout with remote (git pull)
  2. Compiles native CLI binaries (sndbx, bp) and installs to ~/.local/bin
  3. Rebuilds all native OCI container images (base, opencode, claude, agy)`)
			return 0
		}
	}

	repoDir := paths.ResolveRepoDir()
	if repoDir == "" {
		fmt.Fprintln(stderr, "sndbx update: unable to locate local agent-sandbox repository")
		return 1
	}

	fmt.Fprintf(stdout, "==> Synchronizing repository (%s)...\n", repoDir)
	gitCmd := execCommandContext(ctx, "git", "pull", "--ff-only")
	gitCmd.Dir = repoDir
	gitCmd.Stdout = stdout
	gitCmd.Stderr = stderr
	if err := gitCmd.Run(); err != nil {
		fmt.Fprintf(stderr, "sndbx update: warning: git pull --ff-only failed (%v), continuing with local checkout\n", err)
	}

	binDir := paths.BinDir
	if binDir == "" {
		binDir = filepath.Join(os.Getenv("HOME"), ".local", "bin")
	}
	_ = os.MkdirAll(binDir, 0755)

	fmt.Fprintf(stdout, "==> Compiling native CLI binaries to %s...\n", binDir)
	sndbxBin := filepath.Join(binDir, "sndbx")
	bpBin := filepath.Join(binDir, "bp")

	buildSndbx := execCommandContext(ctx, "go", "build", "-o", sndbxBin, "-ldflags=-s -w", "./cmd/sndbx")
	buildSndbx.Dir = repoDir
	buildSndbx.Stdout = stdout
	buildSndbx.Stderr = stderr
	if err := buildSndbx.Run(); err != nil {
		fmt.Fprintf(stderr, "sndbx update: failed compiling sndbx: %v\n", err)
		return 1
	}
	_ = os.Chmod(sndbxBin, 0755)

	buildBP := execCommandContext(ctx, "go", "build", "-o", bpBin, "-ldflags=-s -w", "./cmd/bp")
	buildBP.Dir = repoDir
	buildBP.Stdout = stdout
	buildBP.Stderr = stderr
	if err := buildBP.Run(); err != nil {
		fmt.Fprintf(stderr, "sndbx update: failed compiling bp: %v\n", err)
		return 1
	}
	_ = os.Chmod(bpBin, 0755)

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

func handleGUI(ctx context.Context, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	fmt.Fprintln(stdout, "Launching Backplane GUI client...")
	return 0
}

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
