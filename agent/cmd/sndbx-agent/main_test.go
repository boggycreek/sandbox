// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/boggycreek/sandbox/agent/pkg/engine"
	"github.com/boggycreek/sandbox/agent/pkg/runtime"
)

func TestLoadConfigDefaults(t *testing.T) {
	cfg, err := LoadConfig([]string{})
	if err != nil {
		t.Fatalf("unexpected error loading defaults: %v", err)
	}

	if cfg.AgentName == "" {
		t.Error("expected non-empty default agent name")
	}
	if cfg.AgentRole != "General Software Engineer" {
		t.Errorf("unexpected default role: %s", cfg.AgentRole)
	}
	if cfg.LogFormat != "ndjson" {
		t.Errorf("unexpected default log format: %s", cfg.LogFormat)
	}
	if !strings.HasSuffix(cfg.StateFile, ".agent_state.json") {
		t.Errorf("unexpected default state file: %s", cfg.StateFile)
	}
}

func TestLoadConfigFlagsAndEnv(t *testing.T) {
	t.Setenv("AGENT_NAME", "env-agent")
	t.Setenv("OPENAI_MODEL", "env-model")

	flags := []string{
		"--name", "flag-agent",
		"--role", "Security Auditor",
		"--workspace", "/tmp/test-ws",
		"--state-file", "/tmp/test-ws/custom-state.json",
		"--model-url", "http://localhost:8000/v1",
		"--model-key", "secret-key",
		"--model-name", "flag-model",
		"--valkey-addr", "localhost:6379",
		"--valkey-password", "valkey-pass",
		"--mcp-binaries", "/usr/bin/tool1, /usr/bin/tool2",
		"--log-format", "text",
	}

	cfg, err := LoadConfig(flags)
	if err != nil {
		t.Fatalf("failed to parse flags: %v", err)
	}

	if cfg.AgentName != "flag-agent" {
		t.Errorf("expected flag override, got: %s", cfg.AgentName)
	}
	if cfg.AgentRole != "Security Auditor" {
		t.Errorf("unexpected role: %s", cfg.AgentRole)
	}
	if cfg.WorkspaceDir != "/tmp/test-ws" {
		t.Errorf("unexpected workspace: %s", cfg.WorkspaceDir)
	}
	if cfg.StateFile != "/tmp/test-ws/custom-state.json" {
		t.Errorf("unexpected state file: %s", cfg.StateFile)
	}
	if cfg.OpenAIBaseURL != "http://localhost:8000/v1" {
		t.Errorf("unexpected model url: %s", cfg.OpenAIBaseURL)
	}
	if cfg.OpenAIAPIKey != "secret-key" {
		t.Errorf("unexpected model key: %s", cfg.OpenAIAPIKey)
	}
	if cfg.OpenAIModel != "flag-model" {
		t.Errorf("unexpected model name: %s", cfg.OpenAIModel)
	}
	if cfg.ValkeyAddr != "localhost:6379" || cfg.ValkeyPassword != "valkey-pass" {
		t.Errorf("unexpected valkey settings: %s, %s", cfg.ValkeyAddr, cfg.ValkeyPassword)
	}
	if len(cfg.MCPBinaries) != 2 || cfg.MCPBinaries[0] != "/usr/bin/tool1" || cfg.MCPBinaries[1] != "/usr/bin/tool2" {
		t.Errorf("unexpected mcp binaries: %+v", cfg.MCPBinaries)
	}
	if cfg.LogFormat != "text" {
		t.Errorf("unexpected log format: %s", cfg.LogFormat)
	}
}

func TestLoggerOutput(t *testing.T) {
	// 1. NDJSON logger
	var jsonBuf bytes.Buffer
	jsonLogger := NewLogger(&jsonBuf, "ndjson")
	jsonLogger.Log("INFO", "Agent initialized", map[string]any{"step": 1})

	var entry LogEntry
	if err := json.Unmarshal(jsonBuf.Bytes(), &entry); err != nil {
		t.Fatalf("failed to parse ndjson log entry: %v", err)
	}
	if entry.Level != "INFO" || entry.Message != "Agent initialized" || entry.Fields["step"].(float64) != 1 {
		t.Errorf("unexpected log entry contents: %+v", entry)
	}

	// 2. Text logger
	var textBuf bytes.Buffer
	textLogger := NewLogger(&textBuf, "text")
	textLogger.Log("WARN", "Warning alert", map[string]any{"code": 404})
	textOut := textBuf.String()
	if !strings.Contains(textOut, "WARN: Warning alert") || !strings.Contains(textOut, `"code":404`) {
		t.Errorf("unexpected text log output: %s", textOut)
	}

	// 3. Text logger without fields
	textBuf.Reset()
	textLogger.Log("INFO", "Clean message", nil)
	if !strings.Contains(textBuf.String(), "INFO: Clean message\n") {
		t.Errorf("unexpected clean text log output: %s", textBuf.String())
	}
}

func TestStartZombieReaper(t *testing.T) {
	// 1. By default when not PID 1, StartZombieReaper exits immediately
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var logBuf bytes.Buffer
	logger := NewLogger(&logBuf, "ndjson")

	StartZombieReaper(ctx, logger)

	// 2. Forced mode activates the reaper
	t.Setenv("FORCE_ZOMBIE_REAPER", "1")
	StartZombieReaper(ctx, logger)

	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start true command: %v", err)
	}
	runtime.RegisterChildPID(cmd.Process.Pid)
	defer runtime.UnregisterChildPID(cmd.Process.Pid)

	if err := runtime.WaitManagedCmd(cmd); err != nil {
		t.Fatalf("expected clean exit from WaitManagedCmd: %v", err)
	}
}

func TestRunCleanShutdown(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := engine.ChatCompletionResponse{
			ID: "resp-1",
			Choices: []engine.ChatChoice{
				{
					Message: engine.ChatMessage{
						Role:    "assistant",
						Content: "Standing by.",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	cfg := &Config{
		AgentName:          "test-agent",
		AgentRole:          "Tester",
		WorkspaceDir:       tmpDir,
		StateFile:          filepath.Join(tmpDir, "state.json"),
		OpenAIBaseURL:      ts.URL,
		OpenAIModel:        "test-model",
		ValkeyAddr:         "", // No Valkey for this unit test
		MCPBinaries:        []string{"/non/existent/mcp-binary-xyz"},
		CustomInstructions: "Rule 1",
		LogFormat:          "ndjson",
	}

	ctx, cancel := context.WithCancel(context.Background())

	var outBuf bytes.Buffer
	errChan := make(chan error, 1)
	go func() {
		errChan <- Run(ctx, cfg, &outBuf)
	}()

	// Allow workers to start
	time.Sleep(50 * time.Millisecond)

	// Trigger shutdown
	cancel()

	select {
	case err := <-errChan:
		if err != nil {
			t.Errorf("unexpected error on clean shutdown: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not exit within timeout")
	}

	out := outBuf.String()
	if !strings.Contains(out, "Starting sndbx-agent runtime") || !strings.Contains(out, "shut down cleanly") {
		t.Errorf("expected startup and shutdown logs, got:\n%s", out)
	}
}

func TestRunMainAndEdgeCases(t *testing.T) {
	// 1. NewLogger with nil writer
	nilLogger := NewLogger(nil, "ndjson")
	if nilLogger.out == nil {
		t.Error("expected default stdout writer")
	}

	// 2. LoadConfig error with invalid flag
	_, err := LoadConfig([]string{"--unrecognized-flag-xyz"})
	if err == nil {
		t.Error("expected error for unrecognized flag")
	}

	// 3. runMain with invalid flag
	err = runMain(context.Background(), []string{"--unrecognized-flag-xyz"}, io.Discard)
	if err == nil {
		t.Error("expected error from runMain with invalid flag")
	}

	// 4. Mock MCP server script
	tmpDir := t.TempDir()
	mockScript := filepath.Join(tmpDir, "mock-mcp.sh")
	scriptContent := `#!/bin/sh
while read -r line; do
  case "$line" in
    *initialize*)
      echo '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{},"serverInfo":{"name":"mock","version":"1.0"}}}'
      ;;
    *tools/list*)
      echo '{"jsonrpc":"2.0","id":2,"result":{"tools":[{"name":"mock_tool","description":"mock","inputSchema":{"type":"object"}}]}}'
      ;;
  esac
done
`
	if err := os.WriteFile(mockScript, []byte(scriptContent), 0755); err != nil {
		t.Fatalf("failed to write mock mcp script: %v", err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := engine.ChatCompletionResponse{
			ID: "resp-mock",
			Choices: []engine.ChatChoice{
				{Message: engine.ChatMessage{Role: "assistant", Content: "Ready."}},
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	var outBuf bytes.Buffer
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	args := []string{
		"--name", "test-agent-mcp",
		"--workspace", tmpDir,
		"--model-url", ts.URL,
		"--mcp-binaries", mockScript,
		"--valkey-addr", "127.0.0.1:1", // unreachable port to exercise dial error path
	}

	err = runMain(ctx, args, &outBuf)
	if err != nil {
		t.Errorf("unexpected error running runMain with mock MCP: %v", err)
	}

	logs := outBuf.String()
	if !strings.Contains(logs, "Registered MCP tools") {
		t.Errorf("expected mock MCP registration log, got:\n%s", logs)
	}
	if !strings.Contains(logs, "Failed to connect to Valkey") {
		t.Errorf("expected Valkey dial failure log, got:\n%s", logs)
	}
}

func TestMainInvocation(t *testing.T) {
	origArgs := os.Args
	origExit := osExit
	defer func() {
		os.Args = origArgs
		osExit = origExit
	}()

	exitCalled := false
	var exitCode int
	osExit = func(code int) {
		exitCalled = true
		exitCode = code
	}

	os.Args = []string{"sndbx-agent", "--unrecognized-flag-xyz"}
	main()

	if !exitCalled || exitCode != 1 {
		t.Errorf("expected osExit(1) called, got called=%v code=%d", exitCalled, exitCode)
	}
}
