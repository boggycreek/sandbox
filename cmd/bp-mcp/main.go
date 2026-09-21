// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package main implements the Fleet Backplane Model Context Protocol (MCP) server.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/boggycreek/sandbox/pkg/libbp"
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

// BPClient defines the interface required by the Backplane MCP server
type BPClient interface {
	Say(ctx context.Context, content string, blobPath string) (*libbp.Message, error)
	Tell(ctx context.Context, recipient string, content string) (*libbp.Message, error)
	Reply(ctx context.Context, replyToMsgID string, recipient string, content string) (*libbp.Message, error)
	Recv(ctx context.Context, blockSeconds int) ([]*libbp.Message, error)
	Peers(ctx context.Context) ([]*libbp.Peer, error)
	SetStatus(ctx context.Context, statusText string) error
	Close() error
}

// MCPServer manages the STDIO JSON-RPC lifecycle for Fleet Backplane messaging
type MCPServer struct {
	bpClient BPClient
	reader   *bufio.Reader
	writer   io.Writer
}

// NewMCPServer creates a new Backplane MCP server instance
func NewMCPServer(client BPClient, r io.Reader, w io.Writer) *MCPServer {
	return &MCPServer{
		bpClient: client,
		reader:   bufio.NewReader(r),
		writer:   w,
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
			if err == io.EOF {
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
				"name":    "bp-mcp",
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
				Name:        "fleet_send_message",
				Description: "Send a direct, signed point-to-point message or threaded reply to another agent or human operator inbox",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"recipient": {
							Type:        "string",
							Description: "Target agent name or operator ID (e.g. opencode-1, operator)",
						},
						"message": {
							Type:        "string",
							Description: "Content of the message",
						},
						"reply_to": {
							Type:        "string",
							Description: "Optional citation ID or message ID being replied to (e.g. agent-1#42 or stream ID)",
						},
						"file_path": {
							Type:        "string",
							Description: "Optional local file path to attach as blob",
						},
					},
					Required: []string{"recipient", "message"},
				},
			},
			{
				Name:        "fleet_broadcast",
				Description: "Broadcast a public signed message to the entire fleet public feed",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"message": {
							Type:        "string",
							Description: "Content of the broadcast announcement",
						},
						"file_path": {
							Type:        "string",
							Description: "Optional local file path to attach as a public blob",
						},
					},
					Required: []string{"message"},
				},
			},
			{
				Name:        "fleet_read_inbox",
				Description: "Read new pending messages from agent inbox and peer broadcast feeds",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"block_seconds": {
							Type:        "integer",
							Description: "Optional blocking timeout in seconds (0 for non-blocking)",
						},
						"count": {
							Type:        "integer",
							Description: "Maximum number of messages to return (default: all pending)",
						},
					},
				},
			},
			{
				Name:        "fleet_list_peers",
				Description: "List all known and active peer agents in the fleet, their status, roles, and liaison status",
				InputSchema: InputSchema{
					Type: "object",
				},
			},
			{
				Name:        "fleet_set_status",
				Description: "Update current agent operational status broadcasted to the fleet",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"status": {
							Type:        "string",
							Description: "Agent status text (e.g. idle, working: task-123, blocked: awaiting human input)",
						},
					},
					Required: []string{"status"},
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

func (s *MCPServer) executeTool(ctx context.Context, params CallToolParams) CallToolResult {
	var args map[string]any
	if len(params.Arguments) > 0 {
		_ = json.Unmarshal(params.Arguments, &args)
	}
	if args == nil {
		args = make(map[string]any)
	}

	switch params.Name {
	case "fleet_send_message":
		recipient, _ := args["recipient"].(string)
		message, _ := args["message"].(string)
		replyTo, _ := args["reply_to"].(string)

		if recipient == "" || message == "" {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: "Missing required arguments 'recipient' and 'message'"}},
				IsError: true,
			}
		}

		var sentMsg *libbp.Message
		var err error

		if replyTo != "" {
			sentMsg, err = s.bpClient.Reply(ctx, replyTo, recipient, message)
		} else {
			sentMsg, err = s.bpClient.Tell(ctx, recipient, message)
		}

		if err != nil {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Error sending fleet message: %v", err)}},
				IsError: true,
			}
		}

		data, _ := json.MarshalIndent(sentMsg, "", "  ")
		return CallToolResult{Content: []ContentBlock{{Type: "text", Text: string(data)}}}

	case "fleet_broadcast":
		message, _ := args["message"].(string)
		filePath, _ := args["file_path"].(string)

		if message == "" {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: "Missing required argument 'message'"}},
				IsError: true,
			}
		}

		sentMsg, err := s.bpClient.Say(ctx, message, filePath)
		if err != nil {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Error broadcasting fleet message: %v", err)}},
				IsError: true,
			}
		}

		data, _ := json.MarshalIndent(sentMsg, "", "  ")
		return CallToolResult{Content: []ContentBlock{{Type: "text", Text: string(data)}}}

	case "fleet_read_inbox":
		var blockSeconds int
		if bsVal, ok := args["block_seconds"].(float64); ok {
			blockSeconds = int(bsVal)
		}
		var maxCount int
		if mcVal, ok := args["count"].(float64); ok {
			maxCount = int(mcVal)
		}

		msgs, err := s.bpClient.Recv(ctx, blockSeconds)
		if err != nil {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Error reading fleet inbox: %v", err)}},
				IsError: true,
			}
		}

		if maxCount > 0 && len(msgs) > maxCount {
			msgs = msgs[:maxCount]
		}

		data, _ := json.MarshalIndent(msgs, "", "  ")
		return CallToolResult{Content: []ContentBlock{{Type: "text", Text: string(data)}}}

	case "fleet_list_peers":
		peers, err := s.bpClient.Peers(ctx)
		if err != nil {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Error listing fleet peers: %v", err)}},
				IsError: true,
			}
		}

		data, _ := json.MarshalIndent(peers, "", "  ")
		return CallToolResult{Content: []ContentBlock{{Type: "text", Text: string(data)}}}

	case "fleet_set_status":
		status, _ := args["status"].(string)
		if status == "" {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: "Missing required argument 'status'"}},
				IsError: true,
			}
		}

		if err := s.bpClient.SetStatus(ctx, status); err != nil {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Error updating fleet status: %v", err)}},
				IsError: true,
			}
		}

		return CallToolResult{Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Status updated to %q", status)}}}

	default:
		return CallToolResult{
			Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Unknown tool: %s", params.Name)}},
			IsError: true,
		}
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
	cfg := libbp.LoadClientFromEnv()
	client, err := libbp.Dial(ctx, cfg)
	if err != nil {
		return fmt.Errorf("failed connecting to backplane: %w", err)
	}
	defer func() {
		_ = client.Close()
	}()

	server := NewMCPServer(client, r, w)
	return server.Serve(ctx)
}

func main() {
	if err := Run(context.Background(), os.Stdin, os.Stdout); err != nil && !errors.Is(err, io.EOF) {
		fmt.Fprintf(os.Stderr, "bp-mcp server error: %v\n", err)
		os.Exit(1)
	}
}
