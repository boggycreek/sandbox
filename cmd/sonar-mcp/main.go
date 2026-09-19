// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/boggycreek/agent-sandbox/pkg/sonar"
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

// MCPServer manages the STDIO JSON-RPC lifecycle for SonarQube integration
type MCPServer struct {
	sonarClient *sonar.Client
	reader      *bufio.Reader
	writer      io.Writer
}

// NewMCPServer creates a new SonarQube MCP server instance
func NewMCPServer(client *sonar.Client, r io.Reader, w io.Writer) *MCPServer {
	return &MCPServer{
		sonarClient: client,
		reader:      bufio.NewReader(r),
		writer:      w,
	}
}

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
				"name":    "sonar-mcp",
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
				Name:        "sonar_status",
				Description: "Check SonarQube server operational status and version",
				InputSchema: InputSchema{
					Type: "object",
				},
			},
			{
				Name:        "sonar_quality_gate",
				Description: "Retrieve Quality Gate pass/fail status and condition metrics for a project",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"project_key": {
							Type:        "string",
							Description: "The unique key of the SonarQube project",
						},
					},
					Required: []string{"project_key"},
				},
			},
			{
				Name:        "sonar_issues",
				Description: "List mechanical code issues, bugs, vulnerabilities, and code smells",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"project_key": {
							Type:        "string",
							Description: "The unique key of the project",
						},
						"severity": {
							Type:        "string",
							Description: "Filter by severity (BLOCKER, CRITICAL, MAJOR, MINOR, INFO)",
						},
						"issue_type": {
							Type:        "string",
							Description: "Filter by type (BUG, VULNERABILITY, CODE_SMELL, SECURITY_HOTSPOT)",
						},
					},
					Required: []string{"project_key"},
				},
			},
			{
				Name:        "sonar_measures",
				Description: "Retrieve project code metrics such as test coverage, duplicated lines, and complexity",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"project_key": {
							Type:        "string",
							Description: "The unique key of the project",
						},
						"metric_keys": {
							Type:        "string",
							Description: "Comma-separated metric keys (e.g. coverage,duplicated_lines_density,cognitive_complexity)",
						},
					},
					Required: []string{"project_key"},
				},
			},
			{
				Name:        "sonar_create_project",
				Description: "Create or register a project space in SonarQube",
				InputSchema: InputSchema{
					Type: "object",
					Properties: map[string]Property{
						"project_key": {
							Type:        "string",
							Description: "The unique key for the project",
						},
						"name": {
							Type:        "string",
							Description: "Human-readable project display name",
						},
					},
					Required: []string{"project_key", "name"},
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
	case "sonar_status":
		status, err := s.sonarClient.GetSystemStatus(ctx)
		if err != nil {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Error checking SonarQube status: %v", err)}},
				IsError: true,
			}
		}
		data, _ := json.MarshalIndent(status, "", "  ")
		return CallToolResult{Content: []ContentBlock{{Type: "text", Text: string(data)}}}

	case "sonar_quality_gate":
		projectKey, _ := args["project_key"].(string)
		if projectKey == "" {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: "Missing required argument 'project_key'"}},
				IsError: true,
			}
		}
		qg, err := s.sonarClient.GetProjectQualityGate(ctx, projectKey)
		if err != nil {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Error retrieving quality gate: %v", err)}},
				IsError: true,
			}
		}
		data, _ := json.MarshalIndent(qg, "", "  ")
		return CallToolResult{Content: []ContentBlock{{Type: "text", Text: string(data)}}}

	case "sonar_issues":
		projectKey, _ := args["project_key"].(string)
		if projectKey == "" {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: "Missing required argument 'project_key'"}},
				IsError: true,
			}
		}
		severity, _ := args["severity"].(string)
		issueType, _ := args["issue_type"].(string)
		issues, err := s.sonarClient.GetIssues(ctx, projectKey, severity, issueType)
		if err != nil {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Error querying issues: %v", err)}},
				IsError: true,
			}
		}
		data, _ := json.MarshalIndent(issues, "", "  ")
		return CallToolResult{Content: []ContentBlock{{Type: "text", Text: string(data)}}}

	case "sonar_measures":
		projectKey, _ := args["project_key"].(string)
		if projectKey == "" {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: "Missing required argument 'project_key'"}},
				IsError: true,
			}
		}
		metricKeysStr, _ := args["metric_keys"].(string)
		var metricKeys []string
		if metricKeysStr != "" {
			for _, m := range strings.Split(metricKeysStr, ",") {
				if t := strings.TrimSpace(m); t != "" {
					metricKeys = append(metricKeys, t)
				}
			}
		}
		if len(metricKeys) == 0 {
			metricKeys = []string{"coverage", "duplicated_lines_density", "cognitive_complexity", "bugs", "vulnerabilities", "code_smells"}
		}
		measures, err := s.sonarClient.GetComponentMeasures(ctx, projectKey, metricKeys)
		if err != nil {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Error querying measures: %v", err)}},
				IsError: true,
			}
		}
		data, _ := json.MarshalIndent(measures, "", "  ")
		return CallToolResult{Content: []ContentBlock{{Type: "text", Text: string(data)}}}

	case "sonar_create_project":
		projectKey, _ := args["project_key"].(string)
		name, _ := args["name"].(string)
		if projectKey == "" || name == "" {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: "Missing required arguments 'project_key' and 'name'"}},
				IsError: true,
			}
		}
		if err := s.sonarClient.CreateProject(ctx, projectKey, name); err != nil {
			return CallToolResult{
				Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Error creating project: %v", err)}},
				IsError: true,
			}
		}
		return CallToolResult{Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Project %s (%s) successfully ensured", projectKey, name)}}}

	default:
		return CallToolResult{
			Content: []ContentBlock{{Type: "text", Text: fmt.Sprintf("Unknown tool: %s", params.Name)}},
			IsError: true,
		}
	}
}

func (s *MCPServer) sendResult(id any, result any) {
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
	sonarURL := os.Getenv("SONAR_HOST_URL")
	if sonarURL == "" {
		sonarURL = os.Getenv("SONARQUBE_URL")
	}
	if sonarURL == "" {
		sonarURL = "http://127.0.0.1:9000"
	}
	sonarToken := os.Getenv("SONAR_TOKEN")

	client := sonar.NewClient(sonar.ClientConfig{
		BaseURL: sonarURL,
		Token:   sonarToken,
	})

	server := NewMCPServer(client, r, w)
	return server.Serve(ctx)
}

func main() {
	if err := Run(context.Background(), os.Stdin, os.Stdout); err != nil && err != io.EOF {
		fmt.Fprintf(os.Stderr, "sonar-mcp server error: %v\n", err)
		os.Exit(1)
	}
}
