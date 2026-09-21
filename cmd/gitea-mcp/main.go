// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package main implements the Gitea Fleet Forge Model Context Protocol (MCP) server.
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

	"github.com/boggycreek/sandbox/pkg/gitea"
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

// GiteaClient defines the interface required by the Gitea Forge MCP server
type GiteaClient interface {
	ListIssues(ctx context.Context, owner, repo, state string, page, limit int) ([]*gitea.Issue, error)
	CreateIssue(ctx context.Context, owner, repo, title, body string, labels, assignees []string) (*gitea.Issue, error)
	CreatePullRequest(ctx context.Context, owner, repo, title, body, head, base string) (*gitea.PullRequest, error)
	GetPullRequest(ctx context.Context, owner, repo string, index int64) (*gitea.PullRequest, error)
	ReviewPullRequest(ctx context.Context, owner, repo string, index int64, event, body string) (*gitea.PullReview, error)
	GetFile(ctx context.Context, owner, repo, filePath, ref string) (*gitea.FileContent, error)
}

// MCPServer manages the STDIO JSON-RPC lifecycle for Gitea Fleet Forge integration
type MCPServer struct {
	giteaClient GiteaClient
	reader      *bufio.Reader
	writer      io.Writer
}

// NewMCPServer creates a new Gitea Forge MCP server instance
func NewMCPServer(client GiteaClient, r io.Reader, w io.Writer) *MCPServer {
	return &MCPServer{
		giteaClient: client,
		reader:      bufio.NewReader(r),
		writer:      w,
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
				"name":    "gitea-mcp",
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
				Name:        "forge_list_tasks",
				Description: "List tasks, issues, and backlog items from a Gitea fleet forge repository",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"owner": {
							Type:        "string",
							Description: "Repository owner or organization (default: fleet)",
						},
						"repo": {
							Type:        "string",
							Description: "Repository name (default: tasks)",
						},
						"state": {
							Type:        "string",
							Description: "Task state filter: 'open', 'closed', or 'all' (default: open)",
						},
						"page": {
							Type:        "integer",
							Description: "Page number for pagination (default: 1)",
						},
						"limit": {
							Type:        "integer",
							Description: "Maximum items per page (default: 20)",
						},
					},
				},
			},
			{
				Name:        "forge_create_task",
				Description: "Create a new task, defect, or backlog issue in a Gitea fleet forge repository",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"owner": {
							Type:        "string",
							Description: "Repository owner or organization (default: fleet)",
						},
						"repo": {
							Type:        "string",
							Description: "Repository name (default: tasks)",
						},
						"title": {
							Type:        "string",
							Description: "Task summary title",
						},
						"body": {
							Type:        "string",
							Description: "Detailed task specifications or acceptance criteria",
						},
					},
					Required: []string{"title"},
				},
			},
			{
				Name:        "forge_create_pull_request",
				Description: "Create a new pull request for code review across fleet branches",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"owner": {
							Type:        "string",
							Description: "Repository owner or organization (default: fleet)",
						},
						"repo": {
							Type:        "string",
							Description: "Repository name",
						},
						"title": {
							Type:        "string",
							Description: "Pull request title",
						},
						"head": {
							Type:        "string",
							Description: "Source branch containing new changes",
						},
						"base": {
							Type:        "string",
							Description: "Target branch to merge into (default: main)",
						},
						"body": {
							Type:        "string",
							Description: "Pull request description, summary of changes, and test instructions",
						},
					},
					Required: []string{"repo", "title", "head"},
				},
			},
			{
				Name:        "forge_review_pull_request",
				Description: "Submit a code review approval, change request, or comment on a pull request",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"owner": {
							Type:        "string",
							Description: "Repository owner or organization (default: fleet)",
						},
						"repo": {
							Type:        "string",
							Description: "Repository name",
						},
						"index": {
							Type:        "integer",
							Description: "Pull request number / index",
						},
						"event": {
							Type:        "string",
							Description: "Review action: 'APPROVE', 'REQUEST_CHANGES', or 'COMMENT' (default: COMMENT)",
						},
						"body": {
							Type:        "string",
							Description: "Review feedback comments or critique",
						},
					},
					Required: []string{"repo", "index"},
				},
			},
			{
				Name:        "forge_read_file",
				Description: "Read raw or decoded source file contents from a repository at a specific branch or commit",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"owner": {
							Type:        "string",
							Description: "Repository owner or organization (default: fleet)",
						},
						"repo": {
							Type:        "string",
							Description: "Repository name",
						},
						"file_path": {
							Type:        "string",
							Description: "Relative file path inside the repository",
						},
						"ref": {
							Type:        "string",
							Description: "Branch name, tag, or commit SHA (default: main)",
						},
					},
					Required: []string{"repo", "file_path"},
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

	owner, _ := args["owner"].(string)
	if owner == "" {
		owner = "fleet"
	}
	repo, _ := args["repo"].(string)

	switch params.Name {
	case "forge_list_tasks":
		if repo == "" {
			repo = "tasks"
		}
		state, _ := args["state"].(string)
		var page, limit int
		if pVal, ok := args["page"].(float64); ok {
			page = int(pVal)
		}
		if lVal, ok := args["limit"].(float64); ok {
			limit = int(lVal)
		}

		issues, err := s.giteaClient.ListIssues(ctx, owner, repo, state, page, limit)
		if err != nil {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Error listing forge tasks: %v", err)}},
				IsError: true,
			}
		}

		data, _ := json.MarshalIndent(issues, "", "  ")
		return CallToolResult{Content: []ContentBlock{{Type: "text", Text: string(data)}}}

	case "forge_create_task":
		if repo == "" {
			repo = "tasks"
		}
		title, _ := args["title"].(string)
		body, _ := args["body"].(string)
		if strings.TrimSpace(title) == "" {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: "Missing required argument 'title'"}},
				IsError: true,
			}
		}

		var labels []string
		if rawLabels, ok := args["labels"].([]any); ok {
			for _, l := range rawLabels {
				if ls, ok := l.(string); ok && ls != "" {
					labels = append(labels, ls)
				}
			}
		}

		var assignees []string
		if rawAssignees, ok := args["assignees"].([]any); ok {
			for _, a := range rawAssignees {
				if as, ok := a.(string); ok && as != "" {
					assignees = append(assignees, as)
				}
			}
		}

		issue, err := s.giteaClient.CreateIssue(ctx, owner, repo, title, body, labels, assignees)
		if err != nil {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Error creating forge task: %v", err)}},
				IsError: true,
			}
		}

		data, _ := json.MarshalIndent(issue, "", "  ")
		return CallToolResult{Content: []ContentBlock{{Type: "text", Text: string(data)}}}

	case "forge_create_pull_request":
		title, _ := args["title"].(string)
		head, _ := args["head"].(string)
		base, _ := args["base"].(string)
		body, _ := args["body"].(string)

		if repo == "" || strings.TrimSpace(title) == "" || strings.TrimSpace(head) == "" {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: "Missing required arguments ('repo', 'title', 'head')"}},
				IsError: true,
			}
		}

		pr, err := s.giteaClient.CreatePullRequest(ctx, owner, repo, title, body, head, base)
		if err != nil {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Error creating pull request: %v", err)}},
				IsError: true,
			}
		}

		data, _ := json.MarshalIndent(pr, "", "  ")
		return CallToolResult{Content: []ContentBlock{{Type: "text", Text: string(data)}}}

	case "forge_review_pull_request":
		var index int64
		if idxVal, ok := args["index"].(float64); ok {
			index = int64(idxVal)
		}
		if repo == "" || index <= 0 {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: "Missing required arguments ('repo', 'index')"}},
				IsError: true,
			}
		}

		event, _ := args["event"].(string)
		body, _ := args["body"].(string)

		review, err := s.giteaClient.ReviewPullRequest(ctx, owner, repo, index, event, body)
		if err != nil {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Error reviewing pull request #%d: %v", index, err)}},
				IsError: true,
			}
		}

		data, _ := json.MarshalIndent(review, "", "  ")
		return CallToolResult{Content: []ContentBlock{{Type: "text", Text: string(data)}}}

	case "forge_read_file":
		filePath, _ := args["file_path"].(string)
		ref, _ := args["ref"].(string)

		if repo == "" || filePath == "" {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: "Missing required arguments ('repo', 'file_path')"}},
				IsError: true,
			}
		}

		file, err := s.giteaClient.GetFile(ctx, owner, repo, filePath, ref)
		if err != nil {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Error reading file %s: %v", filePath, err)}},
				IsError: true,
			}
		}

		data, _ := json.MarshalIndent(file, "", "  ")
		return CallToolResult{Content: []ContentBlock{{Type: "text", Text: string(data)}}}

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
	baseURL := os.Getenv("GITEA_URL")
	if baseURL == "" {
		baseURL = os.Getenv("GITEA_HOST_URL")
	}
	if baseURL == "" {
		baseURL = "http://gitea:3000"
	}

	token := os.Getenv("GITEA_TOKEN")
	adminUser := os.Getenv("GITEA_ADMIN_USER")
	adminPass := os.Getenv("GITEA_ADMIN_PASSWORD")
	if adminPass == "" {
		adminPass = os.Getenv("ADMIN_BACKPLANE_PASSWORD")
	}

	client := gitea.NewClient(gitea.ClientConfig{
		BaseURL:   baseURL,
		Token:     token,
		AdminUser: adminUser,
		AdminPass: adminPass,
	})

	server := NewMCPServer(client, r, w)
	return server.Serve(ctx)
}

func main() {
	if err := Run(context.Background(), os.Stdin, os.Stdout); err != nil && !errors.Is(err, io.EOF) {
		fmt.Fprintf(os.Stderr, "gitea-mcp server error: %v\n", err)
		os.Exit(1)
	}
}
