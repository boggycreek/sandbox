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
	"strings"
	"time"

	"github.com/boggycreek/agent-sandbox/pkg/config"
	"github.com/boggycreek/agent-sandbox/pkg/gitea"
	"github.com/boggycreek/agent-sandbox/pkg/libbp"
	"github.com/boggycreek/agent-sandbox/pkg/runtime"
)

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

	case "repo":
		return handleRepo(ctx, paths, domainArgs, stdout, stderr)

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
  sndbx <domain> <command> [args...]

Domains:
  agent       Manage agent instance provisioning and container lifecycles
  infra       Manage shared Valkey backplane and Gitea Git infrastructure
  repo        Manage monorepo builds, image compilation, and upgrades
  gui         Launch native desktop Backplane GUI client

Agent Commands:
  sndbx agent create <name> [as <type|oci>] [--image <type|oci>] [--role <role>] [--model-url <url>] [--model-name <name>] [--model-key <key>]
  sndbx agent start <name>
  sndbx agent connect <name>
  sndbx agent ssh <name>
  sndbx agent stop [name] [--all]
  sndbx agent list [--json]
  sndbx agent clean <name>
  sndbx agent destroy <name> [--force]

Infra Commands:
  sndbx infra up
  sndbx infra down
  sndbx infra list

Run 'sndbx <domain> help' for more details on each command.`)
}

func handleAgent(ctx context.Context, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: sndbx agent <create|start|connect|ssh|stop|list|clean|destroy>")
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

  connect <name>
    Attach interactively to the agent container's tmux supervisor.

  ssh <name>
    Connect directly via SSH to the agent's unprivileged environment.

  stop [name] [--all]
    Stop a running agent container (or all agents with --all).

  list [--json]
    List all configured agents, container states, images, and SSH endpoints.

  clean <name>
    Remove the agent container while preserving its home directory volume.

  destroy <name> [--force]
    Permanently purge the agent container, home volume, and secrets.`)
		return 0
	case "create":
		return handleAgentCreate(ctx, paths, subArgs, stdout, stderr)
	case "start":
		return handleAgentStart(ctx, paths, subArgs, stdout, stderr)
	case "connect":
		return handleAgentConnect(ctx, paths, subArgs, stdout, stderr)
	case "ssh":
		return handleAgentSSH(ctx, paths, subArgs, stdout, stderr)
	case "stop":
		return handleAgentStop(ctx, paths, subArgs, stdout, stderr)
	case "list":
		return handleAgentList(ctx, paths, subArgs, stdout, stderr)
	case "clean":
		return handleAgentClean(ctx, paths, subArgs, stdout, stderr)
	case "destroy":
		return handleAgentDestroy(ctx, paths, subArgs, stdout, stderr)
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

	cfg, err := config.NewAgentConfig(agentName, image, role, modelURL, modelName, modelKey)
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

	client := gitea.NewClient(gitea.ClientConfig{
		BaseURL:   "http://127.0.0.1:3000",
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

	fmt.Fprintf(stdout, "Agent %q started (%s).\n", cfg.Name, cfg.ContainerName)
	fmt.Fprintf(stdout, "Connect via: sndbx agent connect %s\n", cfg.Name)
	return 0
}

func handleAgentConnect(ctx context.Context, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: sndbx agent connect <name>")
		return 1
	}
	name := args[0]
	cfg, err := config.LoadAgentConfig(name, paths)
	if err != nil {
		fmt.Fprintf(stderr, "sndbx error: %v\n", err)
		return 1
	}

	cmd := exec.Command("podman", "exec", "-it", cfg.ContainerName, "tmux", "attach")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(stderr, "sndbx connect error: %v\n", err)
		return 1
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

	cmd := exec.Command("ssh", sshArgs...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
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
	fmt.Fprintf(stdout, "Agent container %q removed (home volume preserved).\n", cfg.ContainerName)
	return 0
}

func handleAgentDestroy(ctx context.Context, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: sndbx agent destroy <name>")
		return 1
	}
	name := args[0]
	cfg, err := config.LoadAgentConfig(name, paths)
	if err != nil {
		fmt.Fprintf(stderr, "sndbx error: %v\n", err)
		return 1
	}

	if err := runtime.DestroyAgentContainer(ctx, cfg.ContainerName, cfg.VolumeName); err != nil {
		fmt.Fprintf(stderr, "sndbx error destroying %s: %v\n", cfg.Name, err)
		return 1
	}
	_ = config.DeleteAgentConfig(name, paths)

	fmt.Fprintf(stdout, "Agent %q and volume %q destroyed completely.\n", cfg.Name, cfg.VolumeName)
	return 0
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
		fmt.Fprintln(stderr, "Usage: sndbx infra <up|down|list>")
		return 1
	}

	sub := strings.ToLower(args[0])
	switch sub {
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, `Usage: sndbx infra <command>

Commands:
  up      Start shared Valkey and Gitea services via Podman Compose
  down    Stop shared infrastructure services
  list    Show status of running infrastructure containers`)
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

func handleRepo(ctx context.Context, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "Usage: sndbx repo <build|build-images|path>")
		return 1
	}
	sub := strings.ToLower(args[0])
	switch sub {
	case "help", "-h", "--help":
		fmt.Fprintln(stdout, `Usage: sndbx repo <command>

Commands:
  build         Compile native CLI binaries (sndbx, bp) into bin/
  build-images  Build all OCI container images (base, opencode, claude, agy)
  path          Print absolute path to the local repository checkout`)
		return 0
	case "path":
		cwd, _ := os.Getwd()
		fmt.Fprintln(stdout, cwd)
		return 0
	case "build":
		cmd := exec.CommandContext(ctx, "make", "build-cli")
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			return 1
		}
		return 0
	case "build-images":
		cmd := exec.CommandContext(ctx, "make", "build-images")
		cmd.Stdout = stdout
		cmd.Stderr = stderr
		if err := cmd.Run(); err != nil {
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "sndbx repo: unknown command %q\n", sub)
		return 1
	}
}

func handleGUI(ctx context.Context, paths config.Paths, args []string, stdout, stderr io.Writer) int {
	fmt.Fprintln(stdout, "Launching Backplane GUI client...")
	return 0
}
