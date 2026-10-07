// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/boggycreek/sandbox/pkg/config"
	"github.com/boggycreek/sandbox/pkg/libbp"
)

func main() {
	os.Exit(Run(os.Args[1:], os.Stdout, os.Stderr))
}

// readPayload reads message payload data from the specified file path or stdin ('-')
func readPayload(path string, stdin io.Reader) ([]byte, error) {
	if path == "-" {
		if stdin == nil {
			return nil, fmt.Errorf("stdin is nil")
		}
		return io.ReadAll(stdin)
	}
	return os.ReadFile(path)
}

const maxSummaryRunes = 256

// extractSummary finds the first non-empty line of the payload as a summary, capped at maxSummaryRunes
func extractSummary(data []byte) (string, error) {
	for _, line := range bytes.Split(data, []byte("\n")) {
		trimmed := strings.TrimSpace(string(line))
		if trimmed != "" {
			runes := []rune(trimmed)
			if len(runes) > maxSummaryRunes {
				return string(runes[:maxSummaryRunes-3]) + "...", nil
			}
			return trimmed, nil
		}
	}
	return "", errors.New("file is empty or contains only whitespace")
}

// Run executes the CLI command with the provided args and I/O streams
func Run(args []string, stdout, stderr io.Writer) int {
	return RunWithIO(args, os.Stdin, stdout, stderr)
}

// RunWithIO executes the CLI command with custom stdin, stdout, and stderr
func RunWithIO(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
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
		return handleSay(ctx, cfg, cmdArgs, stdin, stdout, stderr)

	case "post":
		return handlePost(ctx, cfg, cmdArgs, stdin, stdout, stderr)

	case "cat", "get":
		return handleCat(ctx, cfg, cmdArgs, stdout, stderr)

	case "tell":
		return handleTell(ctx, cfg, cmdArgs, stdin, stdout, stderr)

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

	case "state":
		return handleState(cmdArgs, stdout, stderr)

	default:
		fmt.Fprintf(stderr, "bp: unknown command %q (see 'bp help')\n", cmd)
		return 1
	}
}

func printUsage(out io.Writer) {
	fmt.Fprintln(out, `Usage: bp <command> [args...]

Commands:
  say <message> [--file <path>]    Broadcast message to the fleet (<id>:out)
  post <file> [to <agent>]         Publish long-form message with first line as summary
  cat <blob-key>                   Output parked payload blob to stdout (alias: get)
  tell <agent> [<msg>] [--file <path>] Direct point-to-point message (<peer>:inbox)
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
  state get [<key>]                Read field from shared synaptic state (ADR 00040)
  state set <key> <val>            Update field in shared synaptic state (ADR 00040)
  state show                       Display entire shared synaptic state JSON (ADR 00040)
  state queue <push|pop|list>      Manage shared inter-lobe directive queue (ADR 00040)
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

// parseFileFlag extracts the --file argument from args, returning filePath, remaining args, and success flag
func parseFileFlag(command string, args []string, stderr io.Writer) (string, []string, bool) {
	var filePath string
	var remaining []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			remaining = append(remaining, args[i+1:]...)
			break
		}
		if arg == "--file" || arg == "-file" {
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "%s: flag needs an argument: %s\n", command, arg)
				return "", nil, false
			}
			filePath = args[i+1]
			i++
		} else if strings.HasPrefix(arg, "--file=") {
			filePath = strings.TrimPrefix(arg, "--file=")
		} else if strings.HasPrefix(arg, "-file=") {
			filePath = strings.TrimPrefix(arg, "-file=")
		} else if strings.HasPrefix(arg, "-") && arg != "-" {
			fmt.Fprintf(stderr, "%s: flag provided but not defined: %s\n", command, arg)
			return "", nil, false
		} else {
			remaining = append(remaining, arg)
		}
	}
	return filePath, remaining, true
}

func handleSay(ctx context.Context, cfg libbp.ClientConfig, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	filePath, remaining, ok := parseFileFlag("say", args, stderr)
	if !ok {
		return 1
	}

	if len(remaining) == 0 && filePath == "" {
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
	if filePath != "" {
		data, err := readPayload(filePath, stdin)
		if err != nil {
			fmt.Fprintf(stderr, "bp error: failed reading file %s: %v\n", filePath, err)
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

func handlePost(ctx context.Context, cfg libbp.ClientConfig, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "bp: post requires <file> [to <agent>]")
		return 1
	}

	var filePath string
	var remaining []string

	if args[0] == "--file" {
		if len(args) < 2 {
			fmt.Fprintln(stderr, "bp: post flag --file needs an argument")
			return 1
		}
		filePath = args[1]
		remaining = args[2:]
	} else if strings.HasPrefix(args[0], "--file=") {
		filePath = strings.TrimPrefix(args[0], "--file=")
		remaining = args[1:]
	} else {
		filePath = args[0]
		remaining = args[1:]
	}

	var recipient string
	if len(remaining) > 0 {
		if remaining[0] == "to" {
			if len(remaining) != 2 {
				fmt.Fprintln(stderr, "bp: post to requires exactly <agent>")
				return 1
			}
			recipient = remaining[1]
		} else if len(remaining) == 1 {
			recipient = remaining[0]
		} else {
			fmt.Fprintln(stderr, "bp: post unexpected arguments after <file>")
			return 1
		}
	}

	data, err := readPayload(filePath, stdin)
	if err != nil {
		fmt.Fprintf(stderr, "bp error: failed reading file %s: %v\n", filePath, err)
		return 1
	}

	summary, err := extractSummary(data)
	if err != nil {
		fmt.Fprintf(stderr, "bp error: %v\n", err)
		return 1
	}

	client, err := getClient(ctx, cfg, stderr)
	if err != nil {
		return 1
	}
	defer client.Close()

	msg, err := client.Post(ctx, recipient, summary, data)
	if err != nil {
		fmt.Fprintf(stderr, "bp error: %v\n", err)
		return 1
	}

	target := "fleet"
	if recipient != "" {
		target = recipient
	}
	fmt.Fprintf(stdout, "[%s -> %s] %s\n", msg.Citation, target, msg.Content)
	if msg.BlobPath != "" {
		fmt.Fprintf(stdout, "  └── Payload attached: %s\n", msg.BlobPath)
	}
	return 0
}

func handleCat(ctx context.Context, cfg libbp.ClientConfig, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "bp: cat requires <blob-key>")
		return 1
	}

	client, err := getClient(ctx, cfg, stderr)
	if err != nil {
		return 1
	}
	defer client.Close()

	data, err := client.GetBlob(ctx, args[0])
	if err != nil {
		fmt.Fprintf(stderr, "bp error: %v\n", err)
		return 1
	}

	_, _ = stdout.Write(data)
	return 0
}

func handleTell(ctx context.Context, cfg libbp.ClientConfig, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	filePath, remaining, ok := parseFileFlag("tell", args, stderr)
	if !ok {
		return 1
	}

	if len(remaining) == 0 || (len(remaining) < 2 && filePath == "") {
		fmt.Fprintln(stderr, "bp: tell requires <agent> <message>")
		return 1
	}
	recipient := remaining[0]
	var content string
	if len(remaining) > 1 {
		content = strings.Join(remaining[1:], " ")
	}

	client, err := getClient(ctx, cfg, stderr)
	if err != nil {
		return 1
	}
	defer client.Close()

	var blobKey string
	if filePath != "" {
		data, err := readPayload(filePath, stdin)
		if err != nil {
			fmt.Fprintf(stderr, "bp error: failed reading file %s: %v\n", filePath, err)
			return 1
		}
		bk, err := client.ParkBlob(ctx, data)
		if err != nil {
			fmt.Fprintf(stderr, "bp error: failed parking blob: %v\n", err)
			return 1
		}
		blobKey = bk
	}

	msg, err := client.TellWithBlob(ctx, recipient, content, blobKey)
	if err != nil {
		fmt.Fprintf(stderr, "bp error: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "[%s -> %s] %s\n", msg.Citation, recipient, msg.Content)
	if msg.BlobPath != "" {
		fmt.Fprintf(stdout, "  └── Payload attached: %s\n", msg.BlobPath)
	}
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

func agentStatePath() string {
	if p := os.Getenv("AGENT_STATE_FILE"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/tmp"
	}
	return filepath.Join(home, ".agent", "state.json")
}

func agentQueuePath() string {
	if p := os.Getenv("AGENT_QUEUE_FILE"); p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		home = "/tmp"
	}
	return filepath.Join(home, ".agent", "queue.jsonl")
}

func handleState(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "bp: state requires subcommand (get, set, show, queue)")
		return 1
	}

	sub := strings.ToLower(args[0])
	stateFile := agentStatePath()
	queueFile := agentQueuePath()

	switch sub {
	case "get":
		data, err := os.ReadFile(stateFile)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintln(stderr, "bp: state file does not exist")
				return 1
			}
			fmt.Fprintf(stderr, "bp error: %v\n", err)
			return 1
		}
		if len(args) > 1 {
			var m map[string]interface{}
			if err := json.Unmarshal(data, &m); err != nil {
				fmt.Fprintf(stderr, "bp error: invalid state JSON: %v\n", err)
				return 1
			}
			key := args[1]
			if val, ok := m[key]; ok {
				fmt.Fprintf(stdout, "%v\n", val)
				return 0
			}
			fmt.Fprintf(stderr, "bp: state field %q not found\n", key)
			return 1
		}
		_, _ = stdout.Write(data)
		return 0

	case "set":
		if len(args) < 3 {
			fmt.Fprintln(stderr, "bp: state set requires <key> <value>")
			return 1
		}
		key := args[1]
		val := strings.Join(args[2:], " ")

		var m map[string]interface{}
		data, err := os.ReadFile(stateFile)
		if err == nil {
			_ = json.Unmarshal(data, &m)
		}
		if m == nil {
			m = make(map[string]interface{})
		}
		m[key] = val
		m["updated_at"] = time.Now().UTC().Format(time.RFC3339)

		if err := os.MkdirAll(filepath.Dir(stateFile), 0700); err != nil {
			fmt.Fprintf(stderr, "bp error: %v\n", err)
			return 1
		}
		outData, err := json.MarshalIndent(m, "", "  ")
		if err != nil {
			fmt.Fprintf(stderr, "bp error: %v\n", err)
			return 1
		}
		if err := os.WriteFile(stateFile, append(outData, '\n'), 0600); err != nil {
			fmt.Fprintf(stderr, "bp error: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "state.%s = %s\n", key, val)
		return 0

	case "show":
		data, err := os.ReadFile(stateFile)
		if err != nil {
			if os.IsNotExist(err) {
				fmt.Fprintln(stderr, "bp: state file does not exist")
				return 1
			}
			fmt.Fprintf(stderr, "bp error: %v\n", err)
			return 1
		}
		_, _ = stdout.Write(data)
		return 0

	case "queue":
		if len(args) < 2 {
			fmt.Fprintln(stderr, "bp: state queue requires subcommand (push, pop, list, clear)")
			return 1
		}
		qSub := strings.ToLower(args[1])
		switch qSub {
		case "push":
			if len(args) < 3 {
				fmt.Fprintln(stderr, "bp: state queue push requires <directive>")
				return 1
			}
			directive := strings.Join(args[2:], " ")
			if err := os.MkdirAll(filepath.Dir(queueFile), 0700); err != nil {
				fmt.Fprintf(stderr, "bp error: %v\n", err)
				return 1
			}
			f, err := os.OpenFile(queueFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
			if err != nil {
				fmt.Fprintf(stderr, "bp error: %v\n", err)
				return 1
			}
			defer f.Close()
			if _, err := f.WriteString(directive + "\n"); err != nil {
				fmt.Fprintf(stderr, "bp error: %v\n", err)
				return 1
			}
			fmt.Fprintln(stdout, "queued directive")
			return 0

		case "pop":
			data, err := os.ReadFile(queueFile)
			if err != nil || len(bytes.TrimSpace(data)) == 0 {
				return 1
			}
			lines := bytes.Split(data, []byte("\n"))
			var firstLine []byte
			var remaining [][]byte
			for _, l := range lines {
				if len(bytes.TrimSpace(l)) > 0 {
					if firstLine == nil {
						firstLine = l
					} else {
						remaining = append(remaining, l)
					}
				}
			}
			if firstLine == nil {
				return 1
			}
			var newContent bytes.Buffer
			for _, r := range remaining {
				newContent.Write(r)
				newContent.WriteByte('\n')
			}
			if err := os.WriteFile(queueFile, newContent.Bytes(), 0600); err != nil {
				fmt.Fprintf(stderr, "bp error: %v\n", err)
				return 1
			}
			fmt.Fprintf(stdout, "%s\n", string(firstLine))
			return 0

		case "list":
			data, err := os.ReadFile(queueFile)
			if err != nil {
				return 0
			}
			_, _ = stdout.Write(data)
			return 0

		case "clear":
			_ = os.Remove(queueFile)
			fmt.Fprintln(stdout, "queue cleared")
			return 0

		default:
			fmt.Fprintf(stderr, "bp: unknown queue subcommand %q\n", qSub)
			return 1
		}

	default:
		fmt.Fprintf(stderr, "bp: unknown state subcommand %q\n", sub)
		return 1
	}
}
