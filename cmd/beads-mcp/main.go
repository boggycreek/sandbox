// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package main implements the Beads Graph Issue Tracker Model Context Protocol (MCP) server.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// JSONRPCMessage represents a standard JSON-RPC 2.0 message
type JSONRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC error payload
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Tool represents an MCP tool definition
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

// InputSchema describes the tool parameters schema
type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
}

// Property describes a parameter attribute
type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// CallToolParams defines input for tools/call
type CallToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// ContentBlock represents a formatted MCP text output block
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// CallToolResult represents the response for tools/call
type CallToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// CommandRunner executes commands against the beads CLI
type CommandRunner interface {
	Run(ctx context.Context, dir string, args ...string) (stdout, stderr []byte, err error)
}

// OSCommandRunner is the production implementation executing the native bd binary
type OSCommandRunner struct {
	BinaryPath string
}

// Run executes the bd CLI command in the specified directory
func (r *OSCommandRunner) Run(ctx context.Context, dir string, args ...string) (stdout, stderr []byte, err error) {
	bin := r.BinaryPath
	if bin == "" {
		bin = os.Getenv("BEADS_BIN")
	}
	if bin == "" {
		bin = "bd"
	}

	// #nosec G204 - beads-mcp executes the configured beads binary with tool arguments
	cmd := exec.CommandContext(ctx, bin, args...)
	if dir != "" {
		cmd.Dir = dir
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	cmdErr := cmd.Run()
	return stdoutBuf.Bytes(), stderrBuf.Bytes(), cmdErr
}

// MCPServer manages the STDIO JSON-RPC lifecycle for Beads issue tracker integration
type MCPServer struct {
	runner     CommandRunner
	defaultDir string
	reader     *bufio.Reader
	writer     io.Writer
}

// NewMCPServer creates a new Beads MCP server instance
func NewMCPServer(runner CommandRunner, defaultDir string, r io.Reader, w io.Writer) *MCPServer {
	if defaultDir == "" {
		defaultDir = os.Getenv("FLEET_TASKS_DIR")
	}
	if defaultDir == "" {
		if _, err := os.Stat("/home/agent/tasks"); err == nil {
			defaultDir = "/home/agent/tasks"
		}
	}

	return &MCPServer{
		runner:     runner,
		defaultDir: defaultDir,
		reader:     bufio.NewReader(r),
		writer:     w,
	}
}

// Serve handles incoming JSON-RPC requests until context cancellation or EOF
func (s *MCPServer) Serve(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		line, err := s.reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}

		trimmed := strings.TrimSpace(string(line))
		if trimmed == "" {
			continue
		}

		var req JSONRPCMessage
		if err := json.Unmarshal([]byte(trimmed), &req); err != nil {
			s.sendError(nil, -32700, "Parse error")
			continue
		}

		s.handleMessage(ctx, &req)
	}
}

func (s *MCPServer) handleMessage(ctx context.Context, req *JSONRPCMessage) {
	switch req.Method {
	case "initialize":
		res := map[string]any{
			"protocolVersion": "2024-11-05",
			"serverInfo": map[string]string{
				"name":    "beads-mcp",
				"version": "0.1.0",
			},
			"capabilities": map[string]any{
				"tools": map[string]bool{},
			},
		}
		s.sendResult(req.ID, res)

	case "notifications/initialized":
		// No response required for notifications

	case "tools/list":
		tools := []Tool{
			{
				Name:        "bd_ready",
				Description: "Surface unblocked tasks and issues ready to be worked on from the Beads dependency graph",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"dir": {
							Type:        "string",
							Description: "Task repository directory path (optional, defaults to ~/tasks)",
						},
						"json": {
							Type:        "boolean",
							Description: "Return ready items formatted as JSON objects",
						},
					},
				},
			},
			{
				Name:        "bd_list",
				Description: "List issues and tasks in the Beads dependency graph",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"dir": {
							Type:        "string",
							Description: "Task repository directory path (optional, defaults to ~/tasks)",
						},
						"all": {
							Type:        "boolean",
							Description: "Include closed and deferred issues",
						},
						"status": {
							Type:        "string",
							Description: "Filter by status: 'open', 'in_progress', 'closed', 'deferred', 'blocked'",
						},
						"priority": {
							Type:        "string",
							Description: "Filter by priority: 'P0', 'P1', 'P2', 'P3', 'P4'",
						},
						"json": {
							Type:        "boolean",
							Description: "Return list formatted as JSON objects",
						},
					},
				},
			},
			{
				Name:        "bd_show",
				Description: "Inspect detailed issue specifications, dependencies, acceptance criteria, and history",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"id": {
							Type:        "string",
							Description: "Issue or task identifier (e.g. task-12)",
						},
						"dir": {
							Type:        "string",
							Description: "Task repository directory path (optional, defaults to ~/tasks)",
						},
						"json": {
							Type:        "boolean",
							Description: "Return details formatted as JSON object",
						},
					},
					Required: []string{"id"},
				},
			},
			{
				Name:        "bd_create",
				Description: "Create a new task, feature, bug, or epic in the Beads issue graph",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"title": {
							Type:        "string",
							Description: "Issue title or summary",
						},
						"type": {
							Type:        "string",
							Description: "Issue type: 'task', 'feature', 'bug', 'chore', 'epic' (default: task)",
						},
						"priority": {
							Type:        "string",
							Description: "Priority tier: 'P0', 'P1', 'P2', 'P3', 'P4' (default: P2)",
						},
						"description": {
							Type:        "string",
							Description: "Detailed task description, acceptance criteria, or technical design",
						},
						"parent": {
							Type:        "string",
							Description: "Parent issue ID if creating a child subtask",
						},
						"dir": {
							Type:        "string",
							Description: "Task repository directory path (optional, defaults to ~/tasks)",
						},
					},
					Required: []string{"title"},
				},
			},
			{
				Name:        "bd_claim",
				Description: "Atomically claim an issue or task for the current agent",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"id": {
							Type:        "string",
							Description: "Issue or task identifier to claim (e.g. task-12)",
						},
						"dir": {
							Type:        "string",
							Description: "Task repository directory path (optional, defaults to ~/tasks)",
						},
					},
					Required: []string{"id"},
				},
			},
			{
				Name:        "bd_close",
				Description: "Close a completed task or issue with a completion reason",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"id": {
							Type:        "string",
							Description: "Issue or task identifier to close (e.g. task-12)",
						},
						"reason": {
							Type:        "string",
							Description: "Reason for closing the issue or commit reference",
						},
						"dir": {
							Type:        "string",
							Description: "Task repository directory path (optional, defaults to ~/tasks)",
						},
					},
					Required: []string{"id"},
				},
			},
			{
				Name:        "bd_sync",
				Description: "Synchronize local Dolt issue tracker commits with the upstream remote",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"dir": {
							Type:        "string",
							Description: "Task repository directory path (optional, defaults to ~/tasks)",
						},
					},
				},
			},
		}
		s.sendResult(req.ID, map[string]any{"tools": tools})

	case "tools/call":
		var params CallToolParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendError(req.ID, -32602, "Invalid params")
			return
		}

		result := s.executeTool(ctx, params)
		s.sendResult(req.ID, result)

	default:
		s.sendError(req.ID, -32601, fmt.Sprintf("Method not found: %s", req.Method))
	}
}

func (s *MCPServer) resolveDir(customDir string) string {
	if customDir != "" {
		return customDir
	}
	return s.defaultDir
}

func (s *MCPServer) executeTool(ctx context.Context, params CallToolParams) CallToolResult {
	var args map[string]any
	if len(params.Arguments) > 0 {
		_ = json.Unmarshal(params.Arguments, &args)
	}
	if args == nil {
		args = make(map[string]any)
	}

	customDir, _ := args["dir"].(string)
	dir := s.resolveDir(customDir)

	switch params.Name {
	case "bd_ready":
		cmdArgs := []string{"ready"}
		if asJSON, ok := args["json"].(bool); ok && asJSON {
			cmdArgs = append(cmdArgs, "--json")
		}
		return s.runBd(ctx, dir, cmdArgs...)

	case "bd_list":
		cmdArgs := []string{"list"}
		if showAll, ok := args["all"].(bool); ok && showAll {
			cmdArgs = append(cmdArgs, "--all")
		}
		if status, ok := args["status"].(string); ok && status != "" {
			cmdArgs = append(cmdArgs, "--status", status)
		}
		if priority, ok := args["priority"].(string); ok && priority != "" {
			cmdArgs = append(cmdArgs, "--priority", priority)
		}
		if asJSON, ok := args["json"].(bool); ok && asJSON {
			cmdArgs = append(cmdArgs, "--json")
		}
		return s.runBd(ctx, dir, cmdArgs...)

	case "bd_show":
		id, _ := args["id"].(string)
		if strings.TrimSpace(id) == "" {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: "Missing required argument 'id'"}},
				IsError: true,
			}
		}
		cmdArgs := []string{"show", id}
		if asJSON, ok := args["json"].(bool); ok && asJSON {
			cmdArgs = append(cmdArgs, "--json")
		}
		return s.runBd(ctx, dir, cmdArgs...)

	case "bd_create":
		title, _ := args["title"].(string)
		if strings.TrimSpace(title) == "" {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: "Missing required argument 'title'"}},
				IsError: true,
			}
		}
		cmdArgs := []string{"create", title}
		if issueType, ok := args["type"].(string); ok && issueType != "" {
			cmdArgs = append(cmdArgs, "-t", issueType)
		}
		if priority, ok := args["priority"].(string); ok && priority != "" {
			cmdArgs = append(cmdArgs, "-p", priority)
		}
		if description, ok := args["description"].(string); ok && description != "" {
			cmdArgs = append(cmdArgs, "-d", description)
		}
		if parent, ok := args["parent"].(string); ok && parent != "" {
			cmdArgs = append(cmdArgs, "--parent", parent)
		}
		return s.runBd(ctx, dir, cmdArgs...)

	case "bd_claim":
		id, _ := args["id"].(string)
		if strings.TrimSpace(id) == "" {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: "Missing required argument 'id'"}},
				IsError: true,
			}
		}
		return s.runBd(ctx, dir, "update", id, "--claim")

	case "bd_close":
		id, _ := args["id"].(string)
		if strings.TrimSpace(id) == "" {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: "Missing required argument 'id'"}},
				IsError: true,
			}
		}
		cmdArgs := []string{"close", id}
		if reason, ok := args["reason"].(string); ok && reason != "" {
			cmdArgs = append(cmdArgs, "--reason", reason)
		}
		return s.runBd(ctx, dir, cmdArgs...)

	case "bd_sync":
		return s.runBd(ctx, dir, "sync")

	default:
		return CallToolResult{
			Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Unknown tool: %s", params.Name)}},
			IsError: true,
		}
	}
}

func (s *MCPServer) runBd(ctx context.Context, dir string, args ...string) CallToolResult {
	stdout, stderr, err := s.runner.Run(ctx, dir, args...)
	if err != nil {
		errText := strings.TrimSpace(string(stderr))
		if errText == "" {
			errText = err.Error()
		}
		return CallToolResult{
			Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Error executing 'bd %s': %s", strings.Join(args, " "), errText)}},
			IsError: true,
		}
	}

	outText := strings.TrimSpace(string(stdout))
	if outText == "" {
		outText = "OK"
	}
	return CallToolResult{
		Content: []ContentBlock{{Type: "text", Text: outText}},
	}
}

func (s *MCPServer) sendResult(id, result any) {
	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	data, _ := json.Marshal(msg)
	_, _ = s.writer.Write(append(data, '\n'))
}

func (s *MCPServer) sendError(id any, code int, message string) {
	msg := JSONRPCMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: &JSONRPCError{
			Code:    code,
			Message: message,
		},
	}
	data, _ := json.Marshal(msg)
	_, _ = s.writer.Write(append(data, '\n'))
}

// Run executes the MCP server reading from r and writing to w
func Run(ctx context.Context, r io.Reader, w io.Writer) error {
	runner := &OSCommandRunner{}
	defaultDir := os.Getenv("FLEET_TASKS_DIR")
	server := NewMCPServer(runner, defaultDir, r, w)
	return server.Serve(ctx)
}

func main() {
	if err := Run(context.Background(), os.Stdin, os.Stdout); err != nil && !errors.Is(err, io.EOF) {
		fmt.Fprintf(os.Stderr, "beads-mcp server error: %v\n", err)
		os.Exit(1)
	}
}
