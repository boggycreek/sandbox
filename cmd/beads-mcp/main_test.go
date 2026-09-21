// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

type mockRunner struct {
	runFunc func(ctx context.Context, dir string, args ...string) ([]byte, []byte, error)
	calls   [][]string
}

func (m *mockRunner) Run(ctx context.Context, dir string, args ...string) (stdout, stderr []byte, err error) {
	m.calls = append(m.calls, args)
	if m.runFunc != nil {
		return m.runFunc(ctx, dir, args...)
	}
	return []byte("mock output for: " + strings.Join(args, " ")), nil, nil
}

func TestMCPServerLifecycle(t *testing.T) {
	mock := &mockRunner{}

	requests := []string{
		`{"jsonrpc":"2.0","id":1,"method":"initialize"}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"bd_ready","arguments":{"json":true,"dir":"/custom/tasks"}}}`,
		`{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"bd_list","arguments":{"all":true,"status":"open","priority":"P1","json":true}}}`,
		`{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{"name":"bd_show","arguments":{"id":"task-12","json":true}}}`,
		`{"jsonrpc":"2.0","id":6,"method":"tools/call","params":{"name":"bd_create","arguments":{"title":"New Task","type":"task","priority":"P1","description":"desc","parent":"task-10"}}}`,
		`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"bd_claim","arguments":{"id":"task-12"}}}`,
		`{"jsonrpc":"2.0","id":8,"method":"tools/call","params":{"name":"bd_close","arguments":{"id":"task-12","reason":"done"}}}`,
		`{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"bd_sync","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":10,"method":"non_existent_method"}`,
		`{"jsonrpc":"2.0","id":11,"method":"tools/call","params":{"name":"unknown_tool"}}`,
		`{"jsonrpc":"2.0","id":12,"method":"tools/call","params":{"name":"bd_show","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":13,"method":"tools/call","params":{"name":"bd_create","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":14,"method":"tools/call","params":{"name":"bd_claim","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":15,"method":"tools/call","params":{"name":"bd_close","arguments":{}}}`,
		`{invalid json`,
		``,
	}

	input := strings.Join(requests, "\n") + "\n"
	inBuf := bytes.NewBufferString(input)
	outBuf := &bytes.Buffer{}

	server := NewMCPServer(mock, "/home/agent/tasks", inBuf, outBuf)
	err := server.Serve(context.Background())
	if err != nil {
		t.Fatalf("unexpected Serve error: %v", err)
	}

	output := outBuf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) < 12 {
		t.Fatalf("expected at least 12 responses, got %d: %s", len(lines), output)
	}

	// Verify initialize response
	var initResp JSONRPCMessage
	if err := json.Unmarshal([]byte(lines[0]), &initResp); err != nil {
		t.Fatalf("failed unmarshaling init response: %v", err)
	}
	if initResp.ID != float64(1) {
		t.Errorf("expected ID 1, got %v", initResp.ID)
	}

	// Verify tools/list response
	var toolsResp JSONRPCMessage
	if err := json.Unmarshal([]byte(lines[1]), &toolsResp); err != nil {
		t.Fatalf("failed unmarshaling tools/list response: %v", err)
	}
	toolsMap, ok := toolsResp.Result.(map[string]any)
	if !ok || len(toolsMap["tools"].([]any)) != 7 {
		t.Errorf("expected 7 tools in tools/list, got: %+v", toolsMap)
	}
}

func TestMCPServerRunnerErrors(t *testing.T) {
	errRunner := &mockRunner{
		runFunc: func(_ context.Context, _ string, _ ...string) ([]byte, []byte, error) {
			return nil, []byte("fatal: dolt merge error"), errors.New("exit status 1")
		},
	}

	requests := []string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"bd_ready","arguments":{}}}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":"invalid-params"}`,
	}

	input := strings.Join(requests, "\n") + "\n"
	inBuf := bytes.NewBufferString(input)
	outBuf := &bytes.Buffer{}

	server := NewMCPServer(errRunner, "", inBuf, outBuf)
	_ = server.Serve(context.Background())

	output := outBuf.String()
	if !strings.Contains(output, "dolt merge error") {
		t.Errorf("expected stderr in tool error response, got: %s", output)
	}
	if !strings.Contains(output, "Invalid params") {
		t.Errorf("expected Invalid params error, got: %s", output)
	}
}

func TestMCPServerRunnerEmptyStderr(t *testing.T) {
	errRunner := &mockRunner{
		runFunc: func(_ context.Context, _ string, _ ...string) ([]byte, []byte, error) {
			return nil, nil, errors.New("command not found")
		},
	}

	requests := []string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"bd_ready","arguments":{}}}`,
	}

	input := strings.Join(requests, "\n") + "\n"
	inBuf := bytes.NewBufferString(input)
	outBuf := &bytes.Buffer{}

	server := NewMCPServer(errRunner, "", inBuf, outBuf)
	_ = server.Serve(context.Background())

	output := outBuf.String()
	if !strings.Contains(output, "command not found") {
		t.Errorf("expected err.Error() in output, got: %s", output)
	}
}

func TestMCPServerEmptyOutput(t *testing.T) {
	emptyRunner := &mockRunner{
		runFunc: func(_ context.Context, _ string, _ ...string) ([]byte, []byte, error) {
			return []byte(""), nil, nil
		},
	}

	requests := []string{
		`{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"bd_sync","arguments":{}}}`,
	}

	input := strings.Join(requests, "\n") + "\n"
	inBuf := bytes.NewBufferString(input)
	outBuf := &bytes.Buffer{}

	server := NewMCPServer(emptyRunner, "", inBuf, outBuf)
	_ = server.Serve(context.Background())

	output := outBuf.String()
	if !strings.Contains(output, "OK") {
		t.Errorf("expected OK in output, got: %s", output)
	}
}

func TestMCPServerContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	mock := &mockRunner{}
	server := NewMCPServer(mock, "", bytes.NewBuffer(nil), &bytes.Buffer{})
	err := server.Serve(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestOSCommandRunner(t *testing.T) {
	runner := &OSCommandRunner{BinaryPath: "echo"}
	stdout, stderr, err := runner.Run(context.Background(), t.TempDir(), "hello", "beads")
	if err != nil {
		t.Fatalf("expected echo to succeed, got %v (stderr: %s)", err, string(stderr))
	}
	if !strings.Contains(string(stdout), "hello beads") {
		t.Errorf("unexpected stdout: %s", string(stdout))
	}
}

func TestRunFunction(t *testing.T) {
	inBuf := bytes.NewBufferString("{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"initialize\"}\n")
	outBuf := &bytes.Buffer{}

	err := Run(context.Background(), inBuf, outBuf)
	if err != nil {
		t.Fatalf("unexpected error running Run: %v", err)
	}
	if !strings.Contains(outBuf.String(), "beads-mcp") {
		t.Errorf("expected beads-mcp in output, got: %s", outBuf.String())
	}
}
