// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package engine

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/boggycreek/sandbox/agent/pkg/runtime"
	"github.com/boggycreek/sandbox/agent/pkg/tools"
	"github.com/boggycreek/sandbox/backplane/pkg/libbp"
)

// --- LLMClient Tests ---

func TestLLMClientSuccess(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-api-key" {
			t.Errorf("unexpected authorization header: %s", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("unexpected content-type: %s", r.Header.Get("Content-Type"))
		}

		var req ChatCompletionRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("failed to decode request: %v", err)
		}

		resp := ChatCompletionResponse{
			ID:    "chatcmpl-test",
			Model: req.Model,
			Choices: []ChatChoice{
				{
					Index: 0,
					Message: ChatMessage{
						Role:    "assistant",
						Content: "Hello from mock model!",
					},
					FinishReason: "stop",
				},
			},
			Usage: UsageInfo{
				PromptTokens:     10,
				CompletionTokens: 5,
				TotalTokens:      15,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := NewLLMClient(ts.URL, "test-api-key", "test-model-4b", 5*time.Second)
	if client.Model() != "test-model-4b" {
		t.Errorf("unexpected model: %s", client.Model())
	}
	if !strings.HasSuffix(client.BaseURL(), "/chat/completions") {
		t.Errorf("expected URL to end with /chat/completions: %s", client.BaseURL())
	}

	ctx := context.Background()
	req := ChatCompletionRequest{
		Messages: []ChatMessage{
			{Role: "user", Content: "Hello"},
		},
	}
	resp, err := client.ChatCompletion(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Message.Content != "Hello from mock model!" {
		t.Fatalf("unexpected response choices: %+v", resp.Choices)
	}
}

func TestLLMClientErrors(t *testing.T) {
	ctx := context.Background()

	// 1. Empty model error
	emptyClient := NewLLMClient("http://localhost", "", "", 0)
	_, err := emptyClient.ChatCompletion(ctx, ChatCompletionRequest{})
	if err == nil || !strings.Contains(err.Error(), "model name cannot be empty") {
		t.Errorf("expected empty model error, got: %v", err)
	}

	// 2. HTTP 400 with OpenAI error JSON
	ts400 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(ChatCompletionResponse{
			Error: &OpenAIError{
				Message: "invalid parameter temperature",
				Type:    "invalid_request_error",
			},
		})
	}))
	defer ts400.Close()

	client400 := NewLLMClient(ts400.URL, "", "test-model", 2*time.Second)
	_, err = client400.ChatCompletion(ctx, ChatCompletionRequest{Model: "test-model"})
	if err == nil || !strings.Contains(err.Error(), "invalid parameter temperature") {
		t.Errorf("expected 400 error message, got: %v", err)
	}

	// 3. HTTP 500 plain text
	ts500 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal crash", http.StatusInternalServerError)
	}))
	defer ts500.Close()

	client500 := NewLLMClient(ts500.URL, "", "test-model", 2*time.Second)
	_, err = client500.ChatCompletion(ctx, ChatCompletionRequest{Model: "test-model"})
	if err == nil || !strings.Contains(err.Error(), "status 500") {
		t.Errorf("expected 500 status error, got: %v", err)
	}

	// 4. Invalid JSON body
	tsBadJSON := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("not valid json"))
	}))
	defer tsBadJSON.Close()

	clientBad := NewLLMClient(tsBadJSON.URL, "", "test-model", 2*time.Second)
	_, err = clientBad.ChatCompletion(ctx, ChatCompletionRequest{Model: "test-model"})
	if err == nil || !strings.Contains(err.Error(), "failed to decode completion response") {
		t.Errorf("expected decode error, got: %v", err)
	}

	// 5. Empty choices in 200 OK response
	tsEmptyChoices := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(ChatCompletionResponse{
			Choices: []ChatChoice{},
		})
	}))
	defer tsEmptyChoices.Close()

	clientEmpty := NewLLMClient(tsEmptyChoices.URL, "", "test-model", 2*time.Second)
	_, err = clientEmpty.ChatCompletion(ctx, ChatCompletionRequest{Model: "test-model"})
	if err == nil || !strings.Contains(err.Error(), "empty choices received") {
		t.Errorf("expected empty choices error, got: %v", err)
	}
}

// --- Prompt & Context Tests ---

func TestPromptAssemblyAndFormatting(t *testing.T) {
	prompt := BuildSystemPrompt("agent-01", "Software Engineer", "/home/agent/workspace", "Focus on clean code.")
	if !strings.Contains(prompt, "You are agent-01") {
		t.Errorf("missing agent id in prompt: %s", prompt)
	}
	if !strings.Contains(prompt, "Software Engineer") {
		t.Errorf("missing role in prompt: %s", prompt)
	}
	if !strings.Contains(prompt, "Focus on clean code.") {
		t.Errorf("missing custom instructions: %s", prompt)
	}

	// Test event formatting
	p0 := FormatEventMessage(runtime.Event{
		Priority: runtime.P0_Control,
		Source:   "operator",
		Payload:  "Emergency stop",
	})
	if !strings.Contains(p0.Content, "[EMERGENCY CONTROL DIRECTIVE") {
		t.Errorf("unexpected p0 formatting: %s", p0.Content)
	}

	p1 := FormatEventMessage(runtime.Event{
		Priority: runtime.P1_HighPriority,
		Source:   "user",
		Payload:  "Refactor module X",
	})
	if !strings.Contains(p1.Content, "[URGENT DIRECTIVE") {
		t.Errorf("unexpected p1 formatting: %s", p1.Content)
	}

	p2 := FormatEventMessage(runtime.Event{
		Priority: runtime.P2_StandardAsync,
		Source:   "peer-02",
		Payload:  runtime.BackplaneEnvelope{Sender: "peer-02", Payload: "PR is ready"},
	})
	if !strings.Contains(p2.Content, "[NOTIFICATION") || !strings.Contains(p2.Content, "PR is ready") {
		t.Errorf("unexpected p2 formatting: %s", p2.Content)
	}

	pOther := FormatEventMessage(runtime.Event{
		Priority: runtime.PriorityLevel(99),
		Source:   "telemetry",
		Payload:  map[string]string{"ping": "pong"},
	})
	if !strings.Contains(pOther.Content, "[MESSAGE from telemetry]") {
		t.Errorf("unexpected fallback formatting: %s", pOther.Content)
	}

	emptyPayload := extractPayloadString(nil)
	if emptyPayload != "" {
		t.Errorf("expected empty string for nil payload, got %q", emptyPayload)
	}

	envelopePtr := extractPayloadString(&runtime.BackplaneEnvelope{Sender: "bob", Payload: "hi"})
	if !strings.Contains(envelopePtr, "From: bob") {
		t.Errorf("unexpected envelope pointer string: %s", envelopePtr)
	}

	var nilEnv *runtime.BackplaneEnvelope
	if extractPayloadString(nilEnv) != "" {
		t.Errorf("expected empty string for nil envelope pointer")
	}

	msgVal := extractPayloadString(libbp.Message{Sender: "alice", Content: "hello world"})
	if msgVal != "From: alice | Message: hello world" {
		t.Errorf("unexpected Message value string: %s", msgVal)
	}

	msgPtr := extractPayloadString(&libbp.Message{Sender: "alice", Content: "hello world"})
	if msgPtr != "From: alice | Message: hello world" {
		t.Errorf("unexpected Message pointer string: %s", msgPtr)
	}

	var nilMsg *libbp.Message
	if extractPayloadString(nilMsg) != "" {
		t.Errorf("expected empty string for nil Message pointer")
	}
}

func TestTruncateAndCompactHistory(t *testing.T) {
	// 1. TruncateToolOutput
	shortText := "short output"
	if TruncateToolOutput(shortText, 50) != shortText {
		t.Errorf("expected short text untouched")
	}
	longText := strings.Repeat("A", 100)
	truncated := TruncateToolOutput(longText, 20)
	if !strings.Contains(truncated, "[content truncated to fit context budget]") {
		t.Errorf("expected truncation marker in %s", truncated)
	}

	// 2. CompactHistory within limit
	msgs := []ChatMessage{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "u1"},
		{Role: "assistant", Content: "a1"},
	}
	compacted := CompactHistory(msgs, 5)
	if len(compacted) != 3 {
		t.Errorf("expected all 3 messages preserved, got %d", len(compacted))
	}

	// 3. CompactHistory exceeding limit: preserves system prompt at index 0
	manyMsgs := []ChatMessage{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "u1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "u2"},
		{Role: "assistant", Content: "a2"},
		{Role: "tool", Content: "t2"},
		{Role: "user", Content: "u3"},
	}
	compactedPruned := CompactHistory(manyMsgs, 4)
	if compactedPruned[0].Role != "system" {
		t.Errorf("expected system prompt at index 0, got %s", compactedPruned[0].Role)
	}
	// Tool message pairing guard check: ensure we didn't start with orphan tool message
	for i, m := range compactedPruned {
		if m.Role == "tool" && i == 1 {
			t.Errorf("orphan tool message retained at start of history")
		}
	}

	// 4. CompactHistory without system prompt
	noSys := []ChatMessage{
		{Role: "user", Content: "u1"},
		{Role: "assistant", Content: "a1"},
		{Role: "user", Content: "u2"},
	}
	cNoSys := CompactHistory(noSys, 2)
	if len(cNoSys) != 2 || cNoSys[0].Content != "a1" {
		t.Errorf("unexpected pruning without system prompt: %+v", cNoSys)
	}
}

// --- REPLEngine Tests ---

func TestREPLEngineConfiguration(t *testing.T) {
	llm := NewLLMClient("http://mock", "key", "model", 0)
	reg := tools.NewRegistry()
	eng := NewREPLEngine(llm, reg, "/tmp/ws", "coder")

	if eng.ID() != "repl-main" {
		t.Errorf("unexpected ID: %s", eng.ID())
	}

	eng.SetPollInterval(10 * time.Millisecond)
	eng.SetMaxTurns(50)
	eng.SetMaxToolIterations(10)
	eng.SetCustomInstructions("custom rule")

	hist := []ChatMessage{{Role: "user", Content: "hi"}}
	eng.SetHistory(hist)
	got := eng.GetHistory()
	if len(got) != 1 || got[0].Content != "hi" {
		t.Errorf("unexpected history: %+v", got)
	}

	// Start validation with nil bus or state
	if err := eng.Start(context.Background(), nil, nil); err == nil {
		t.Error("expected error with nil bus/state")
	}
}

func TestREPLEngineSimpleTurn(t *testing.T) {
	var requestCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&requestCount, 1)
		resp := ChatCompletionResponse{
			ID:    "resp-1",
			Model: "test-model",
			Choices: []ChatChoice{
				{
					Index: 0,
					Message: ChatMessage{
						Role:    "assistant",
						Content: "Task completed successfully.",
					},
					FinishReason: "stop",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	llm := NewLLMClient(ts.URL, "key", "test-model", 5*time.Second)
	reg := tools.NewRegistry()
	eng := NewREPLEngine(llm, reg, tmpDir, "coder")
	eng.SetPollInterval(10 * time.Millisecond)

	bus := runtime.NewEventBus()
	state := runtime.NewSharedState(filepath.Join(tmpDir, "state.json"), "agent-01")

	// Append a user task
	eng.AppendMessage(ChatMessage{Role: "user", Content: "Do something."})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- eng.Start(ctx, bus, state)
	}()

	// Wait until turn finishes and state is idle
	var finalState runtime.AgentState
	for i := 0; i < 50; i++ {
		time.Sleep(20 * time.Millisecond)
		finalState = state.Read()
		if finalState.Status == "idle" && atomic.LoadInt32(&requestCount) > 0 {
			break
		}
	}

	cancel()
	_ = <-errChan

	if finalState.Status != "idle" {
		t.Errorf("expected final status idle, got %s", finalState.Status)
	}

	history := eng.GetHistory()
	if len(history) < 2 {
		t.Fatalf("expected at least system, user, assistant messages, got %d", len(history))
	}
	lastMsg := history[len(history)-1]
	if lastMsg.Role != "assistant" || lastMsg.Content != "Task completed successfully." {
		t.Errorf("unexpected last message: %+v", lastMsg)
	}
}

func TestREPLEngineToolExecution(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "sample.txt")
	_ = os.WriteFile(testFile, []byte("Content of sample file"), 0644)

	var callStep int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		step := atomic.AddInt32(&callStep, 1)
		var resp ChatCompletionResponse

		if step == 1 {
			// First call: request read_file tool call
			resp = ChatCompletionResponse{
				ID:    "resp-call-tool",
				Model: "test-model",
				Choices: []ChatChoice{
					{
						Index: 0,
						Message: ChatMessage{
							Role: "assistant",
							ToolCalls: []ToolCall{
								{
									ID:   "call_123",
									Type: "function",
									Function: ToolFunctionCall{
										Name:      "read_file",
										Arguments: `{"path": "sample.txt"}`,
									},
								},
							},
						},
						FinishReason: "tool_calls",
					},
				},
			}
		} else {
			// Second call: return final completion after seeing tool output
			resp = ChatCompletionResponse{
				ID:    "resp-final",
				Model: "test-model",
				Choices: []ChatChoice{
					{
						Index: 0,
						Message: ChatMessage{
							Role:    "assistant",
							Content: "File read successfully.",
						},
						FinishReason: "stop",
					},
				},
			}
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	llm := NewLLMClient(ts.URL, "key", "test-model", 5*time.Second)
	reg := tools.NewRegistry()
	_ = tools.RegisterBuiltinTools(reg, tmpDir)

	eng := NewREPLEngine(llm, reg, tmpDir, "coder")
	eng.SetPollInterval(10 * time.Millisecond)

	bus := runtime.NewEventBus()
	state := runtime.NewSharedState(filepath.Join(tmpDir, "state.json"), "agent-01")

	eng.AppendMessage(ChatMessage{Role: "user", Content: "Read sample.txt please."})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- eng.Start(ctx, bus, state)
	}()

	// Wait for tool execution to finish
	for i := 0; i < 50; i++ {
		time.Sleep(20 * time.Millisecond)
		if atomic.LoadInt32(&callStep) >= 2 && state.Read().Status == "idle" {
			break
		}
	}

	cancel()
	_ = <-errChan

	history := eng.GetHistory()
	// History should contain: system, user, assistant(tool_call), tool(result), assistant(final)
	hasToolResult := false
	for _, m := range history {
		if m.Role == "tool" && strings.Contains(m.Content, "Content of sample file") {
			hasToolResult = true
			break
		}
	}
	if !hasToolResult {
		t.Errorf("tool output not found in history: %+v", history)
	}
}

func TestREPLEngineEventInteractions(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := ChatCompletionResponse{
			ID:    "resp-evt",
			Model: "test-model",
			Choices: []ChatChoice{
				{
					Index: 0,
					Message: ChatMessage{
						Role:    "assistant",
						Content: "Processed directive.",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	llm := NewLLMClient(ts.URL, "key", "test-model", 5*time.Second)
	reg := tools.NewRegistry()
	eng := NewREPLEngine(llm, reg, tmpDir, "coder")
	eng.SetPollInterval(10 * time.Millisecond)

	bus := runtime.NewEventBus()
	state := runtime.NewSharedState(filepath.Join(tmpDir, "state.json"), "agent-01")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- eng.Start(ctx, bus, state)
	}()

	time.Sleep(30 * time.Millisecond)

	// 1. Send P1 Urgent Directive
	_ = bus.Publish(runtime.Event{
		Priority:  runtime.P1_HighPriority,
		Source:    "commander",
		Target:    "repl-main",
		Payload:   "Urgent: switch branch",
		Timestamp: time.Now(),
	})

	// Wait for processing
	for i := 0; i < 50; i++ {
		time.Sleep(20 * time.Millisecond)
		if state.Read().Status == "idle" {
			break
		}
	}

	// 2. Send P0 Emergency stop
	_ = bus.Publish(runtime.Event{
		Priority:  runtime.P0_Control,
		Source:    "operator",
		Target:    "repl-main",
		Payload:   "EMERGENCY HALT",
		Timestamp: time.Now(),
	})

	select {
	case err := <-errChan:
		if err != nil {
			t.Errorf("unexpected error on emergency stop: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("engine did not stop on P0 event")
	}

	if state.Read().Status != "cancelled" {
		t.Errorf("expected state cancelled, got %s", state.Read().Status)
	}
}

func TestREPLEngineIterationLimit(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always return a tool call to trigger iteration limit
		resp := ChatCompletionResponse{
			ID:    "resp-loop",
			Model: "test-model",
			Choices: []ChatChoice{
				{
					Index: 0,
					Message: ChatMessage{
						Role: "assistant",
						ToolCalls: []ToolCall{
							{
								ID:   "call_loop",
								Type: "function",
								Function: ToolFunctionCall{
									Name:      "test_loop",
									Arguments: "{}",
								},
							},
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	llm := NewLLMClient(ts.URL, "key", "test-model", 5*time.Second)
	reg := tools.NewRegistry()
	eng := NewREPLEngine(llm, reg, tmpDir, "coder")
	eng.SetPollInterval(10 * time.Millisecond)
	eng.SetMaxToolIterations(2) // low threshold for test

	bus := runtime.NewEventBus()
	state := runtime.NewSharedState(filepath.Join(tmpDir, "state.json"), "agent-01")

	eng.AppendMessage(ChatMessage{Role: "user", Content: "Start infinite loop"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- eng.Start(ctx, bus, state)
	}()

	// Wait for iteration limit to trigger and return to idle
	for i := 0; i < 50; i++ {
		time.Sleep(20 * time.Millisecond)
		if state.Read().Status == "idle" {
			break
		}
	}

	cancel()
	_ = <-errChan

	// Verify limit message was appended to history
	hasLimitMsg := false
	for _, m := range eng.GetHistory() {
		if strings.Contains(m.Content, "Tool execution limit reached") {
			hasLimitMsg = true
			break
		}
	}
	if !hasLimitMsg {
		t.Error("expected iteration limit notification in history")
	}
}

func TestREPLEngineP2BufferingDuringTurn(t *testing.T) {
	bus := runtime.NewEventBus()
	defer bus.Close()

	var turnCount int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&turnCount, 1)
		if count == 1 {
			// First turn: invoke a dummy tool and concurrently publish P2 event to bus
			_ = bus.Publish(runtime.Event{
				Priority:  runtime.P2_StandardAsync,
				Source:    "peer-agent",
				Target:    "*",
				Payload:   "async notification while tools run",
				Timestamp: time.Now(),
			})
			resp := ChatCompletionResponse{
				ID: "resp-turn-1",
				Choices: []ChatChoice{
					{
						Message: ChatMessage{
							Role: "assistant",
							ToolCalls: []ToolCall{
								{
									ID:       "tc-1",
									Type:     "function",
									Function: ToolFunctionCall{Name: "read_file", Arguments: `{"path":"sample.txt"}`},
								},
							},
						},
					},
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// Second turn: conclude turn
		resp := ChatCompletionResponse{
			ID: "resp-turn-2",
			Choices: []ChatChoice{
				{
					Message: ChatMessage{
						Role:    "assistant",
						Content: "Finished tool run.",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmpDir, "sample.txt"), []byte("sample content"), 0644)
	llm := NewLLMClient(ts.URL, "key", "test-model", 5*time.Second)
	reg := tools.NewRegistry()
	_ = tools.RegisterBuiltinTools(reg, tmpDir)
	eng := NewREPLEngine(llm, reg, tmpDir, "coder")
	eng.SetPollInterval(10 * time.Millisecond)

	state := runtime.NewSharedState(filepath.Join(tmpDir, "state.json"), "agent-01")
	eng.AppendMessage(ChatMessage{Role: "user", Content: "Start turn"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errChan := make(chan error, 1)
	go func() {
		errChan <- eng.Start(ctx, bus, state)
	}()

	// Wait for processing to complete
	for i := 0; i < 50; i++ {
		time.Sleep(20 * time.Millisecond)
		if atomic.LoadInt32(&turnCount) >= 2 && state.Read().Status == "idle" {
			break
		}
	}
	cancel()
	_ = <-errChan

	// Verify that the P2 async notification was NOT dropped and exists in history
	hasP2Msg := false
	for _, m := range eng.GetHistory() {
		if strings.Contains(m.Content, "async notification while tools run") {
			hasP2Msg = true
			break
		}
	}
	if !hasP2Msg {
		t.Errorf("expected P2 buffered event in history, got history: %+v", eng.GetHistory())
	}
}

func TestEngineEdgeCasesAndErrors(t *testing.T) {
	// TruncateToolOutput default length
	longText := strings.Repeat("B", 10000)
	truncDef := TruncateToolOutput(longText, 0)
	if !strings.Contains(truncDef, "[content truncated") {
		t.Error("expected default truncation")
	}

	// CompactHistory default turns
	msgs := []ChatMessage{
		{Role: "system", Content: "sys"},
		{Role: "user", Content: "u"},
	}
	compactDef := CompactHistory(msgs, 0)
	if len(compactDef) != 2 {
		t.Errorf("expected 2 messages with default turns, got %d", len(compactDef))
	}

	// LLM error during executeTurn
	tsErr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "server boom", http.StatusInternalServerError)
	}))
	defer tsErr.Close()

	tmpDir := t.TempDir()
	llmErr := NewLLMClient(tsErr.URL, "k", "m", 5*time.Second)
	engErr := NewREPLEngine(llmErr, nil, tmpDir, "coder")
	engErr.SetPollInterval(10 * time.Millisecond)

	bus := runtime.NewEventBus()
	state := runtime.NewSharedState(filepath.Join(tmpDir, "state.json"), "agent-err")
	engErr.AppendMessage(ChatMessage{Role: "user", Content: "cause error"})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		_ = engErr.Start(ctx, bus, state)
	}()

	for i := 0; i < 50; i++ {
		time.Sleep(20 * time.Millisecond)
		if state.Read().Status == "error" {
			break
		}
	}
	if state.Read().Status != "error" {
		t.Errorf("expected status error, got %s", state.Read().Status)
	}

	// Tool execution failure handled
	var callStep int32
	tsToolErr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		step := atomic.AddInt32(&callStep, 1)
		var resp ChatCompletionResponse
		if step == 1 {
			resp = ChatCompletionResponse{
				Choices: []ChatChoice{
					{
						Message: ChatMessage{
							Role: "assistant",
							ToolCalls: []ToolCall{
								{
									ID:   "call_bad",
									Type: "function",
									Function: ToolFunctionCall{
										Name:      "non_existent_tool",
										Arguments: "{}",
									},
								},
							},
						},
					},
				},
			}
		} else {
			resp = ChatCompletionResponse{
				Choices: []ChatChoice{
					{
						Message: ChatMessage{
							Role:    "assistant",
							Content: "Recovered from tool error.",
						},
					},
				},
			}
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer tsToolErr.Close()

	llmTool := NewLLMClient(tsToolErr.URL, "k", "m", 5*time.Second)
	reg := tools.NewRegistry()
	engTool := NewREPLEngine(llmTool, reg, tmpDir, "coder")
	engTool.SetPollInterval(10 * time.Millisecond)
	stateTool := runtime.NewSharedState(filepath.Join(tmpDir, "state2.json"), "agent-tool")
	engTool.AppendMessage(ChatMessage{Role: "user", Content: "run bad tool"})

	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	go func() {
		_ = engTool.Start(ctx2, bus, stateTool)
	}()

	for i := 0; i < 50; i++ {
		time.Sleep(20 * time.Millisecond)
		if atomic.LoadInt32(&callStep) >= 2 && stateTool.Read().Status == "idle" {
			break
		}
	}

	// Verify tool error message is in history
	hasErr := false
	for _, m := range engTool.GetHistory() {
		if m.Role == "tool" && strings.Contains(m.Content, "Error:") {
			hasErr = true
			break
		}
	}
	if !hasErr {
		t.Error("expected tool error recorded in history")
	}
}

func TestParseFallbackToolCalls(t *testing.T) {
	// 1. Empty string
	if calls := parseFallbackToolCalls(""); len(calls) != 0 {
		t.Errorf("expected 0 calls for empty content, got %d", len(calls))
	}

	// 2. Direct JSON object with map arguments
	obj1 := `{"name": "bash", "arguments": {"command": "bp tell brian hi"}}`
	calls1 := parseFallbackToolCalls(obj1)
	if len(calls1) != 1 || calls1[0].Function.Name != "bash" || !strings.Contains(calls1[0].Function.Arguments, "bp tell brian hi") {
		t.Errorf("unexpected parse result for obj1: %+v", calls1)
	}

	// 3. Markdown fenced JSON with parameters
	fenced := "```json\n{\"tool\": \"read_file\", \"parameters\": {\"path\": \"README.md\"}}\n```"
	calls2 := parseFallbackToolCalls(fenced)
	if len(calls2) != 1 || calls2[0].Function.Name != "read_file" || !strings.Contains(calls2[0].Function.Arguments, "README.md") {
		t.Errorf("unexpected parse result for fenced: %+v", calls2)
	}

	// 4. Action / input format
	actionFmt := `{"action": "write_file", "input": {"path": "test.txt", "content": "data"}}`
	calls3 := parseFallbackToolCalls(actionFmt)
	if len(calls3) != 1 || calls3[0].Function.Name != "write_file" || !strings.Contains(calls3[0].Function.Arguments, "test.txt") {
		t.Errorf("unexpected parse result for actionFmt: %+v", calls3)
	}

	// 5. Function wrapper format
	fnFmt := `{"function": {"name": "edit_file", "arguments": "{\"path\": \"main.go\"}"}}`
	calls4 := parseFallbackToolCalls(fnFmt)
	if len(calls4) != 1 || calls4[0].Function.Name != "edit_file" || calls4[0].Function.Arguments != `{"path": "main.go"}` {
		t.Errorf("unexpected parse result for fnFmt: %+v", calls4)
	}

	// 6. JSON array format
	arrFmt := `[{"name": "bash", "arguments": {"command": "ls"}}, {"name": "bash", "arguments": {"command": "pwd"}}]`
	calls5 := parseFallbackToolCalls(arrFmt)
	if len(calls5) != 2 || calls5[0].Function.Name != "bash" || calls5[1].Function.Name != "bash" {
		t.Errorf("unexpected parse result for arrFmt: %+v", calls5)
	}

	// 7. Embedded in conversational text
	embedded := "I will list the directory.\n{\"name\": \"bash\", \"arguments\": {\"command\": \"ls -la\"}}\nDone."
	calls6 := parseFallbackToolCalls(embedded)
	if len(calls6) != 1 || calls6[0].Function.Name != "bash" || !strings.Contains(calls6[0].Function.Arguments, "ls -la") {
		t.Errorf("unexpected parse result for embedded: %+v", calls6)
	}

	// 8. Plain conversational text (no tool call)
	plain := "Hello! How can I assist you with the Agent Sandbox today?"
	calls7 := parseFallbackToolCalls(plain)
	if len(calls7) != 0 {
		t.Errorf("expected 0 calls for plain text, got %d", len(calls7))
	}
}
