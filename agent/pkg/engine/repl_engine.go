// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package engine

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/boggycreek/sandbox/agent/pkg/runtime"
	"github.com/boggycreek/sandbox/agent/pkg/tools"
)

const (
	// DefaultPollInterval is the default idle wait interval before polling.
	DefaultPollInterval = 200 * time.Millisecond

	// DefaultMaxToolIterations is the maximum consecutive tool execution turns before forced conclusion.
	DefaultMaxToolIterations = 25
)

// REPLEngine drives the multi-turn cognitive agent loop.
type REPLEngine struct {
	mu                 sync.RWMutex
	llm                *LLMClient
	registry           *tools.Registry
	workspaceDir       string
	role               string
	customInstructions string
	pollInterval       time.Duration
	maxTurns           int
	maxToolIterations  int
	history            []ChatMessage
	inboundQueue       []runtime.Event
}

// NewREPLEngine constructs a new REPLEngine instance.
func NewREPLEngine(llm *LLMClient, registry *tools.Registry, workspaceDir, role string) *REPLEngine {
	return &REPLEngine{
		llm:               llm,
		registry:          registry,
		workspaceDir:      workspaceDir,
		role:              role,
		pollInterval:      DefaultPollInterval,
		maxTurns:          DefaultMaxTurns,
		maxToolIterations: DefaultMaxToolIterations,
		history:           make([]ChatMessage, 0),
		inboundQueue:      make([]runtime.Event, 0),
	}
}

func (e *REPLEngine) enqueueInbound(evt runtime.Event) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.inboundQueue = append(e.inboundQueue, evt)
}

func (e *REPLEngine) drainInboundQueue() []runtime.Event {
	e.mu.Lock()
	defer e.mu.Unlock()
	if len(e.inboundQueue) == 0 {
		return nil
	}
	events := make([]runtime.Event, len(e.inboundQueue))
	copy(events, e.inboundQueue)
	e.inboundQueue = e.inboundQueue[:0]
	return events
}

// ID returns the unique subsystem identifier.
func (e *REPLEngine) ID() string {
	return "repl-main"
}

// SetPollInterval sets the idle event wait interval.
func (e *REPLEngine) SetPollInterval(d time.Duration) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if d > 0 {
		e.pollInterval = d
	}
}

// SetMaxTurns configures the history compaction threshold.
func (e *REPLEngine) SetMaxTurns(n int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if n > 0 {
		e.maxTurns = n
	}
}

// SetMaxToolIterations configures the maximum tool calls per cognitive turn.
func (e *REPLEngine) SetMaxToolIterations(n int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if n > 0 {
		e.maxToolIterations = n
	}
}

// SetCustomInstructions configures custom system instructions.
func (e *REPLEngine) SetCustomInstructions(instructions string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.customInstructions = instructions
}

// GetHistory returns a snapshot copy of the conversation history.
func (e *REPLEngine) GetHistory() []ChatMessage {
	e.mu.RLock()
	defer e.mu.RUnlock()
	copied := make([]ChatMessage, len(e.history))
	copy(copied, e.history)
	return copied
}

// SetHistory overwrites conversation history.
func (e *REPLEngine) SetHistory(history []ChatMessage) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.history = make([]ChatMessage, len(history))
	copy(e.history, history)
}

// AppendMessage appends a message to conversation history.
func (e *REPLEngine) AppendMessage(msg ChatMessage) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.history = append(e.history, msg)
}

// hasPendingUserTurn checks if the conversation ends with an unfulfilled user or tool message.
func (e *REPLEngine) hasPendingUserTurn() bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if len(e.history) == 0 {
		return false
	}
	lastRole := e.history[len(e.history)-1].Role
	return lastRole == "user" || lastRole == "tool"
}

// Start runs the REPL cognitive loop until ctx is canceled or emergency stop is received.
func (e *REPLEngine) Start(ctx context.Context, bus *runtime.EventBus, state *runtime.SharedState) error {
	if bus == nil || state == nil {
		return errors.New("event bus and shared state cannot be nil")
	}

	agentID := state.Read().AgentID
	if agentID == "" {
		agentID = "sndbx-agent"
	}

	// Initialize history with foundational system prompt if missing
	e.mu.Lock()
	if len(e.history) == 0 || e.history[0].Role != "system" {
		sysPrompt := BuildSystemPrompt(agentID, e.role, e.workspaceDir, e.customInstructions)
		e.history = append([]ChatMessage{{
			Role:    "system",
			Content: sysPrompt,
		}}, e.history...)
	}
	e.mu.Unlock()

	_ = state.Update(func(s *runtime.AgentState) {
		s.Status = "idle"
		s.CurrentActivity = "Ready"
	})

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		// 1. Drain pending events
		hasNewDirective := false
		for _, evt := range e.drainInboundQueue() {
			e.AppendMessage(FormatEventMessage(evt))
			hasNewDirective = true
		}
		for {
			evt, ok := bus.Poll()
			if !ok {
				break
			}
			if evt.Priority == runtime.P0_Control {
				_ = state.Update(func(s *runtime.AgentState) {
					s.Status = "cancelled"
					s.CurrentActivity = "Emergency stop received"
				})
				return nil
			}
			if evt.Priority == runtime.P1_HighPriority || evt.Priority == runtime.P2_StandardAsync {
				e.AppendMessage(FormatEventMessage(evt))
				hasNewDirective = true
			}
		}

		// 2. If we have work to do, execute cognitive turn
		if hasNewDirective || e.hasPendingUserTurn() {
			if err := e.executeTurn(ctx, bus, state); err != nil {
				if ctx.Err() != nil {
					return nil
				}
				// Sleep brief backoff before retry on transient error
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(50 * time.Millisecond):
				}
			}
			continue
		}

		// 3. Idle wait for incoming events
		waitInterval := e.getPollInterval()
		waitCtx, cancel := context.WithTimeout(ctx, waitInterval)
		evt, err := bus.Next(waitCtx)
		cancel()

		if err != nil {
			// Timeout or cancellation; loop around
			continue
		}

		// Process arrived event
		if evt.Priority == runtime.P0_Control {
			_ = state.Update(func(s *runtime.AgentState) {
				s.Status = "cancelled"
				s.CurrentActivity = "Emergency stop received"
			})
			return nil
		}
		if evt.Priority == runtime.P1_HighPriority || evt.Priority == runtime.P2_StandardAsync {
			e.AppendMessage(FormatEventMessage(evt))
			_ = e.executeTurn(ctx, bus, state)
		}
	}
}

func (e *REPLEngine) getPollInterval() time.Duration {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.pollInterval
}

// executeTurn executes a multi-turn LLM reasoning and tool execution sequence.
func (e *REPLEngine) executeTurn(ctx context.Context, bus *runtime.EventBus, state *runtime.SharedState) error {
	_ = state.Update(func(s *runtime.AgentState) {
		s.Status = "thinking"
		s.CurrentActivity = "Reasoning..."
	})

	e.mu.RLock()
	maxIterations := e.maxToolIterations
	maxTurns := e.maxTurns
	e.mu.RUnlock()

	for iter := 0; iter < maxIterations; iter++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Check for emergency events interrupting current tool loop
		for {
			evt, ok := bus.Poll()
			if !ok {
				break
			}
			if evt.Priority == runtime.P0_Control {
				_ = state.Update(func(s *runtime.AgentState) {
					s.Status = "cancelled"
					s.CurrentActivity = "Emergency stop received during turn"
				})
				return nil
			}
			if evt.Priority == runtime.P1_HighPriority {
				// Inject high-priority directive immediately into history
				e.AppendMessage(FormatEventMessage(evt))
			} else if evt.Priority == runtime.P2_StandardAsync {
				// Buffer P2 events into inbound queue without dropping
				e.enqueueInbound(evt)
			}
		}

		// Prepare LLM request
		msgs := CompactHistory(e.GetHistory(), maxTurns)
		var openAITools []map[string]any
		if e.registry != nil {
			openAITools = e.registry.ToOpenAITools()
		}

		req := ChatCompletionRequest{
			Model:    e.llm.Model(),
			Messages: msgs,
			Tools:    openAITools,
		}

		resp, err := e.llm.ChatCompletion(ctx, req)
		if err != nil {
			_ = state.Update(func(s *runtime.AgentState) {
				s.Status = "error"
				s.CurrentActivity = fmt.Sprintf("LLM error: %v", err)
			})
			return err
		}

		if len(resp.Choices) == 0 {
			return errors.New("empty choices from llm")
		}

		assistantMsg := resp.Choices[0].Message
		assistantMsg.Role = "assistant"
		if len(assistantMsg.ToolCalls) == 0 {
			if fallbackCalls := parseFallbackToolCalls(assistantMsg.Content); len(fallbackCalls) > 0 {
				assistantMsg.ToolCalls = fallbackCalls
			}
		}
		e.AppendMessage(assistantMsg)

		// If no tools were invoked, turn execution is finished
		if len(assistantMsg.ToolCalls) == 0 {
			for _, queuedEvt := range e.drainInboundQueue() {
				e.AppendMessage(FormatEventMessage(queuedEvt))
			}
			_ = state.Update(func(s *runtime.AgentState) {
				s.Status = "idle"
				s.CurrentActivity = "Ready"
			})
			return nil
		}

		// Execute tool calls sequentially
		for _, toolCall := range assistantMsg.ToolCalls {
			if ctx.Err() != nil {
				return ctx.Err()
			}

			toolName := toolCall.Function.Name
			_ = state.Update(func(s *runtime.AgentState) {
				s.Status = "executing_tool"
				s.CurrentActivity = fmt.Sprintf("Executing tool: %s", toolName)
			})

			// Single-writer lock workspace during tool execution
			state.LockWorkspace()
			output, execErr := "", error(nil)
			if e.registry != nil {
				output, execErr = e.registry.Execute(ctx, toolName, toolCall.Function.Arguments)
			} else {
				execErr = fmt.Errorf("tool registry is nil")
			}
			state.UnlockWorkspace()

			if execErr != nil {
				output = fmt.Sprintf("Error: %v", execErr)
			}

			e.AppendMessage(ChatMessage{
				Role:       "tool",
				ToolCallID: toolCall.ID,
				Name:       toolName,
				Content:    output,
			})
		}
	}

	// Tool iteration limit reached: append assistant message so hasPendingUserTurn is false
	e.AppendMessage(ChatMessage{
		Role:    "assistant",
		Content: "Tool execution limit reached for this turn. Summary: max tool iterations reached. Execution paused until further instructions.",
	})

	for _, queuedEvt := range e.drainInboundQueue() {
		e.AppendMessage(FormatEventMessage(queuedEvt))
	}

	_ = state.Update(func(s *runtime.AgentState) {
		s.Status = "idle"
		s.CurrentActivity = "Iteration limit reached; ready"
	})

	return nil
}

// parseFallbackToolCalls extracts function calls from raw content text when models
// (e.g., local LLMs under llama.cpp or Ollama) format tool calls as JSON in content
// rather than populating the OpenAI tool_calls structure.
func parseFallbackToolCalls(content string) []ToolCall {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil
	}

	// Strip markdown code fences if present: ```json ... ``` or ``` ... ```
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		if len(lines) >= 2 {
			lines = lines[1:]
			if len(lines) > 0 && strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
				lines = lines[:len(lines)-1]
			}
			trimmed = strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}

	// Helper to extract a single ToolCall from a map
	extractCall := func(m map[string]any, idx int) (ToolCall, bool) {
		name := ""
		if n, ok := m["name"].(string); ok {
			name = n
		} else if n, ok := m["tool"].(string); ok {
			name = n
		} else if n, ok := m["action"].(string); ok {
			name = n
		} else if fn, ok := m["function"].(map[string]any); ok {
			if n, ok := fn["name"].(string); ok {
				name = n
			}
		}
		if name == "" {
			return ToolCall{}, false
		}

		var argsStr string
		var rawArgs any
		if a, ok := m["arguments"]; ok {
			rawArgs = a
		} else if p, ok := m["parameters"]; ok {
			rawArgs = p
		} else if in, ok := m["input"]; ok {
			rawArgs = in
		} else if fn, ok := m["function"].(map[string]any); ok {
			if a, ok := fn["arguments"]; ok {
				rawArgs = a
			}
		}

		switch v := rawArgs.(type) {
		case string:
			argsStr = v
		case map[string]any, []any:
			if b, err := json.Marshal(v); err == nil {
				argsStr = string(b)
			}
		default:
			if rawArgs != nil {
				if b, err := json.Marshal(rawArgs); err == nil {
					argsStr = string(b)
				}
			} else {
				argsStr = "{}"
			}
		}

		return ToolCall{
			ID:   fmt.Sprintf("call-%d-%d", time.Now().UnixNano(), idx),
			Type: "function",
			Function: ToolFunctionCall{
				Name:      name,
				Arguments: argsStr,
			},
		}, true
	}

	// 1. Try parsing direct JSON object
	var obj map[string]any
	if err := json.Unmarshal([]byte(trimmed), &obj); err == nil {
		if tc, ok := extractCall(obj, 0); ok {
			return []ToolCall{tc}
		}
	}

	// 2. Try parsing direct JSON array
	var arr []map[string]any
	if err := json.Unmarshal([]byte(trimmed), &arr); err == nil && len(arr) > 0 {
		var calls []ToolCall
		for i, item := range arr {
			if tc, ok := extractCall(item, i); ok {
				calls = append(calls, tc)
			}
		}
		if len(calls) > 0 {
			return calls
		}
	}

	// 3. Scan for embedded JSON object in text (e.g., surrounding reasoning)
	start := strings.Index(trimmed, "{")
	end := strings.LastIndex(trimmed, "}")
	if start >= 0 && end > start {
		sub := trimmed[start : end+1]
		var subObj map[string]any
		if err := json.Unmarshal([]byte(sub), &subObj); err == nil {
			if tc, ok := extractCall(subObj, 0); ok {
				return []ToolCall{tc}
			}
		}
	}

	return nil
}
