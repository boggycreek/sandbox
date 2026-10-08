// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package engine

import (
	"context"
	"errors"
	"fmt"
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
	}
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

	_, unsub1 := bus.Subscribe("repl-main", 64)
	defer unsub1()
	_, unsub2 := bus.Subscribe("*", 64)
	defer unsub2()

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
		e.AppendMessage(assistantMsg)

		// If no tools were invoked, turn execution is finished
		if len(assistantMsg.ToolCalls) == 0 {
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

	// Tool iteration limit reached
	e.AppendMessage(ChatMessage{
		Role:    "user",
		Content: "Tool execution limit reached for this turn. Please summarize your progress and next steps.",
	})

	_ = state.Update(func(s *runtime.AgentState) {
		s.Status = "idle"
		s.CurrentActivity = "Iteration limit reached; ready"
	})

	return nil
}
