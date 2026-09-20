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
	"strconv"
	"strings"
	"time"

	"github.com/boggycreek/sandbox/pkg/config"
	"github.com/boggycreek/sandbox/pkg/libbp"
)

func main() {
	os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr))
}

// Run executes the CLI command with the provided args and I/O streams
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printUsage(stdout)
		return 1
	}

	cmd := strings.ToLower(args[0])
	cmdArgs := args[1:]

	paths := config.GetPaths()
	paths.LoadEnv()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cfg := libbp.LoadClientFromEnv()

	switch cmd {
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0

	case "say":
		return handleSay(ctx, cfg, cmdArgs, stdout, stderr)

	case "tell":
		return handleTell(ctx, cfg, cmdArgs, stdout, stderr)

	case "reply":
		return handleReply(ctx, cfg, cmdArgs, stdout, stderr)

	case "recv":
		return handleRecv(ctx, cfg, cmdArgs, stdout, stderr)

	case "human":
		return handleHuman(ctx, cfg, cmdArgs, stdout, stderr)

	case "peers":
		return handlePeers(ctx, cfg, cmdArgs, stdout, stderr)

	case "status":
		return handleStatus(ctx, cfg, cmdArgs, stdout, stderr)

	case "finger":
		return handleFinger(ctx, cfg, cmdArgs, stdout, stderr)

	case "liaison":
		return handleLiaison(ctx, cfg, cmdArgs, stdout, stderr)

	case "interval":
		return handleInterval(ctx, cfg, cmdArgs, stdout, stderr)

	default:
		fmt.Fprintf(stderr, "bp: unknown command %q (see 'bp help')\n", cmd)
		return 1
	}
}

func printUsage(out io.Writer) {
	fmt.Fprintln(out, `Usage: bp <command> [args...]

Commands:
  say <message> [--file <path>]    Broadcast message to the fleet (<id>:out)
  tell <agent> <message>           Direct point-to-point message (<peer>:inbox)
  reply <msgid> <message>          Reply in-thread to a specific message ID
  recv [--block <sec>] [--json]    Read new messages since last cursor
  human [--count <n>] [--json]     Read authoritative human operator broadcast log
  peers [--json]                   Discover active agents across the fleet
  status set <message>             Set ephemeral 300s presence status
  finger [agent]                   Show profile and capability metadata
  liaison get                      Show current fleet liaison
  liaison set <agent>              Appoint an agent as fleet liaison (operator only)
  interval [get]                   Show current backplane poll interval in seconds
  interval set <seconds>           Set shared poll interval in seconds (5-3600)
  help                             Show this help message`)
}

func getClient(ctx context.Context, cfg libbp.ClientConfig, stderr io.Writer) (*libbp.Client, error) {
	client, err := libbp.Dial(ctx, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "bp error: %v\n", err)
		return nil, err
	}
	return client, nil
}

func handleSay(ctx context.Context, cfg libbp.ClientConfig, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("say", flag.ContinueOnError)
	fs.SetOutput(stderr)
	filePath := fs.String("file", "", "Path to large payload file to park")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	remaining := fs.Args()
	if len(remaining) == 0 && *filePath == "" {
		fmt.Fprintln(stderr, "bp: say requires a message or --file")
		return 1
	}
	content := strings.Join(remaining, " ")

	client, err := getClient(ctx, cfg, stderr)
	if err != nil {
		return 1
	}
	defer client.Close()

	var blobKey string
	if *filePath != "" {
		data, err := os.ReadFile(*filePath)
		if err != nil {
			fmt.Fprintf(stderr, "bp error: failed reading file %s: %v\n", *filePath, err)
			return 1
		}
		bk, err := client.ParkBlob(ctx, data)
		if err != nil {
			fmt.Fprintf(stderr, "bp error: failed parking blob: %v\n", err)
			return 1
		}
		blobKey = bk
	}

	msg, err := client.Say(ctx, content, blobKey)
	if err != nil {
		fmt.Fprintf(stderr, "bp error: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "[%s] %s\n", msg.Citation, msg.Content)
	return 0
}

func handleTell(ctx context.Context, cfg libbp.ClientConfig, args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "bp: tell requires <agent> <message>")
		return 1
	}
	recipient := args[0]
	content := strings.Join(args[1:], " ")

	client, err := getClient(ctx, cfg, stderr)
	if err != nil {
		return 1
	}
	defer client.Close()

	msg, err := client.Tell(ctx, recipient, content)
	if err != nil {
		fmt.Fprintf(stderr, "bp error: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "[%s -> %s] %s\n", msg.Citation, recipient, msg.Content)
	return 0
}

func handleReply(ctx context.Context, cfg libbp.ClientConfig, args []string, stdout, stderr io.Writer) int {
	if len(args) < 3 {
		fmt.Fprintln(stderr, "bp: reply requires <msgid> <recipient> <message>")
		return 1
	}
	msgID := args[0]
	recipient := args[1]
	content := strings.Join(args[2:], " ")

	client, err := getClient(ctx, cfg, stderr)
	if err != nil {
		return 1
	}
	defer client.Close()

	msg, err := client.Reply(ctx, msgID, recipient, content)
	if err != nil {
		fmt.Fprintf(stderr, "bp error: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "[%s -> %s (re: %s)] %s\n", msg.Citation, recipient, msgID, msg.Content)
	return 0
}

func handleRecv(ctx context.Context, cfg libbp.ClientConfig, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("recv", flag.ContinueOnError)
	fs.SetOutput(stderr)
	blockSec := fs.Int("block", 0, "Block for N seconds waiting for messages")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	client, err := getClient(ctx, cfg, stderr)
	if err != nil {
		return 1
	}
	defer client.Close()

	messages, err := client.Recv(ctx, *blockSec)
	if err != nil {
		fmt.Fprintf(stderr, "bp error: %v\n", err)
		return 1
	}

	if *jsonOutput {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(messages)
		return 0
	}

	for _, m := range messages {
		sigStatus := ""
		if m.IsSigned {
			if m.IsVerified {
				sigStatus = " [verified]"
			} else {
				sigStatus = " [UNVERIFIED]"
			}
		}

		target := ""
		if m.Destination != "" {
			target = fmt.Sprintf(" -> %s", m.Destination)
		}

		fmt.Fprintf(stdout, "%s [%s%s%s]: %s\n",
			time.UnixMilli(m.Timestamp).Format("15:04:05"),
			m.Citation,
			target,
			sigStatus,
			m.Content,
		)
		if m.BlobPath != "" {
			fmt.Fprintf(stdout, "  └── Payload attached: %s\n", m.BlobPath)
		}
	}
	return 0
}

func handleHuman(ctx context.Context, cfg libbp.ClientConfig, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("human", flag.ContinueOnError)
	fs.SetOutput(stderr)
	count := fs.Int("count", 20, "Number of recent human messages to fetch")
	jsonOutput := fs.Bool("json", false, "Output in JSON format")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	client, err := getClient(ctx, cfg, stderr)
	if err != nil {
		return 1
	}
	defer client.Close()

	messages, err := client.Human(ctx, *count)
	if err != nil {
		fmt.Fprintf(stderr, "bp error: %v\n", err)
		return 1
	}

	if *jsonOutput {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(messages)
		return 0
	}

	for _, m := range messages {
		fmt.Fprintf(stdout, "%s [%s]: %s\n",
			time.UnixMilli(m.Timestamp).Format("15:04:05"),
			m.Citation,
			m.Content,
		)
	}
	return 0
}

func handlePeers(ctx context.Context, cfg libbp.ClientConfig, args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("peers", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOutput := fs.Bool("json", false, "Output in JSON format")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	client, err := getClient(ctx, cfg, stderr)
	if err != nil {
		return 1
	}
	defer client.Close()

	peers, err := client.Peers(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "bp error: %v\n", err)
		return 1
	}

	if *jsonOutput {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(peers)
		return 0
	}

	printfFmt := "%-18s %-12s %s\n"
	fmt.Fprintf(stdout, printfFmt, "AGENT_ID", "ROLE", "STATUS")
	for _, p := range peers {
		role := p.Role
		if role == "" {
			role = "agent"
		}
		if p.IsLiaison {
			role += " (liaison)"
		}
		status := p.Status
		if status == "" {
			status = "idle"
		}
		fmt.Fprintf(stdout, printfFmt, p.ID, role, status)
	}
	return 0
}

func handleStatus(ctx context.Context, cfg libbp.ClientConfig, args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 || strings.ToLower(args[0]) != "set" {
		fmt.Fprintln(stderr, "Usage: bp status set <message>")
		return 1
	}
	status := strings.Join(args[1:], " ")

	client, err := getClient(ctx, cfg, stderr)
	if err != nil {
		return 1
	}
	defer client.Close()

	if err := client.SetStatus(ctx, status); err != nil {
		fmt.Fprintf(stderr, "bp error: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "[%s] status set: %s\n", cfg.AgentID, status)
	return 0
}

func handleFinger(ctx context.Context, cfg libbp.ClientConfig, args []string, stdout, stderr io.Writer) int {
	targetAgent := cfg.AgentID
	if len(args) > 0 {
		targetAgent = args[0]
	}

	client, err := getClient(ctx, cfg, stderr)
	if err != nil {
		return 1
	}
	defer client.Close()

	finger, err := client.GetFinger(ctx, targetAgent)
	if err != nil || len(finger) == 0 {
		fmt.Fprintf(stdout, "[%s] No finger profile registered.\n", targetAgent)
		return 0
	}

	fmt.Fprintf(stdout, "--- Identity Profile: %s ---\n", targetAgent)
	for k, v := range finger {
		fmt.Fprintf(stdout, "  %-12s: %s\n", k, v)
	}
	return 0
}

func handleLiaison(ctx context.Context, cfg libbp.ClientConfig, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || strings.ToLower(args[0]) == "get" {
		client, err := getClient(ctx, cfg, stderr)
		if err != nil {
			return 1
		}
		defer client.Close()

		liaison, err := client.GetLiaison(ctx)
		if err != nil || liaison == "" {
			fmt.Fprintln(stdout, "No liaison currently appointed.")
			return 0
		}
		fmt.Fprintf(stdout, "Current fleet liaison: %s\n", liaison)
		return 0
	}

	if strings.ToLower(args[0]) == "set" {
		if len(args) < 2 {
			fmt.Fprintln(stderr, "Usage: bp liaison set <agent>")
			return 1
		}
		agentID := args[1]
		client, err := getClient(ctx, cfg, stderr)
		if err != nil {
			return 1
		}
		defer client.Close()

		if err := client.SetLiaison(ctx, agentID); err != nil {
			fmt.Fprintf(stderr, "bp error: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Liaison appointed: %s\n", agentID)
		return 0
	}

	fmt.Fprintln(stderr, "Usage: bp liaison <get|set <agent>>")
	return 1
}

func handleInterval(ctx context.Context, cfg libbp.ClientConfig, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || strings.ToLower(args[0]) == "get" {
		client, err := getClient(ctx, cfg, stderr)
		if err != nil {
			return 1
		}
		defer client.Close()

		interval, err := client.GetPollInterval(ctx)
		if err != nil {
			fmt.Fprintf(stderr, "bp error: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "%d\n", interval)
		return 0
	}

	if strings.ToLower(args[0]) == "set" {
		if len(args) < 2 {
			fmt.Fprintln(stderr, "Usage: bp interval set <seconds>")
			return 1
		}
		secs, err := strconv.Atoi(args[1])
		if err != nil || secs < 5 || secs > 3600 {
			fmt.Fprintln(stderr, "bp: interval must be an integer between 5 and 3600")
			return 1
		}

		client, err := getClient(ctx, cfg, stderr)
		if err != nil {
			return 1
		}
		defer client.Close()

		if err := client.SetPollInterval(ctx, secs); err != nil {
			fmt.Fprintf(stderr, "bp error: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "Poll interval set to %d seconds\n", secs)
		return 0
	}

	fmt.Fprintln(stderr, "Usage: bp interval [get|set <seconds>]")
	return 1
}
