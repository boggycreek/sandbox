// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package mcp provides a shared STDIO JSON-RPC 2.0 Model Context Protocol (MCP) server implementation.
package mcp

import "encoding/json"

// ProtocolVersion specifies the supported Model Context Protocol revision.
const ProtocolVersion = "2024-11-05"

// JSON-RPC 2.0 standard error codes.
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// JSONRPCMessage represents a standard JSON-RPC 2.0 message.
type JSONRPCMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *JSONRPCError   `json:"error,omitempty"`
}

// JSONRPCError represents a JSON-RPC error payload.
type JSONRPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Tool represents an MCP tool definition.
type Tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

// InputSchema describes the tool parameters schema.
type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties,omitempty"`
	Required   []string            `json:"required,omitempty"`
}

// Property describes a parameter attribute.
type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// CallToolParams defines input for tools/call.
type CallToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// ParseArguments unmarshals arguments into a map[string]any.
func (p CallToolParams) ParseArguments() map[string]any {
	var args map[string]any
	if len(p.Arguments) > 0 {
		_ = json.Unmarshal(p.Arguments, &args)
	}
	if args == nil {
		args = make(map[string]any)
	}
	return args
}

// ContentBlock represents a formatted MCP text output block.
type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// CallToolResult represents the response for tools/call.
type CallToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// TextResult formats a successful single-text content block response.
func TextResult(text string) CallToolResult {
	return CallToolResult{
		Content: []ContentBlock{
			{
				Type: "text",
				Text: text,
			},
		},
	}
}

// ErrorResult formats an error single-text content block response.
func ErrorResult(text string) CallToolResult {
	return CallToolResult{
		Content: []ContentBlock{
			{
				Type: "text",
				Text: text,
			},
		},
		IsError: true,
	}
}
