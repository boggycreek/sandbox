// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package tools

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
)

var (
	// ErrToolNotFound is returned when invoking an unregistered tool.
	ErrToolNotFound = errors.New("tool not found")
	// ErrToolAlreadyRegistered is returned when registering duplicate tool names.
	ErrToolAlreadyRegistered = errors.New("tool already registered")
)

// Tool defines an executable agent function.
type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]any
	Execute(ctx context.Context, args map[string]any) (string, error)
}

// Registry stores and routes tool invocations.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry creates a new tool registry.
func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Register adds a tool to the registry.
func (r *Registry) Register(tool Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := tool.Name()
	if name == "" {
		return errors.New("tool name cannot be empty")
	}
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("%w: %s", ErrToolAlreadyRegistered, name)
	}
	r.tools[name] = tool
	return nil
}

// Get retrieves a tool by name.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tool, exists := r.tools[name]
	return tool, exists
}

// List returns all registered tools.
func (r *Registry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Tool, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, tool)
	}
	return result
}

// ToOpenAITools formats all registered tools into OpenAI function definitions.
func (r *Registry) ToOpenAITools() []map[string]any {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]map[string]any, 0, len(r.tools))
	for _, tool := range r.tools {
		tools = append(tools, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        tool.Name(),
				"description": tool.Description(),
				"parameters":  tool.Parameters(),
			},
		})
	}
	return tools
}

// Execute parses raw JSON arguments and invokes the named tool.
func (r *Registry) Execute(ctx context.Context, name string, rawArgs string) (string, error) {
	tool, exists := r.Get(name)
	if !exists {
		return "", fmt.Errorf("%w: %s", ErrToolNotFound, name)
	}

	var args map[string]any
	rawArgs = strings.TrimSpace(rawArgs)
	if rawArgs != "" && rawArgs != "{}" {
		if err := json.Unmarshal([]byte(rawArgs), &args); err != nil {
			return "", fmt.Errorf("invalid json arguments: %w", err)
		}
	}
	if args == nil {
		args = make(map[string]any)
	}

	return tool.Execute(ctx, args)
}
