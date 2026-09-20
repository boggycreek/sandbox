// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/boggycreek/sandbox/pkg/libbp"
	"github.com/boggycreek/sandbox/test/harness"
)

type mockBackplaneClient struct {
	mu           sync.Mutex
	recvMessages []*libbp.Message
	recvErr      error
	pollInterval int
	pollErr      error
	lastStatus   string
	statusErr    error
	recvCalled   int
	statusCalled int
}

func (m *mockBackplaneClient) Recv(ctx context.Context, blockSeconds int) ([]*libbp.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.recvCalled++
	if m.recvErr != nil {
		return nil, m.recvErr
	}
	msgs := m.recvMessages
	m.recvMessages = nil // Drain
	return msgs, nil
}

func (m *mockBackplaneClient) GetPollInterval(ctx context.Context) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.pollErr != nil {
		return 0, m.pollErr
	}
	if m.pollInterval == 0 {
		return 60, nil
	}
	return m.pollInterval, nil
}

func (m *mockBackplaneClient) SetStatus(ctx context.Context, statusText string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.statusCalled++
	m.lastStatus = statusText
	return m.statusErr
}

func (m *mockBackplaneClient) Close() error {
	return nil
}

func TestLoadConfigFromEnv(t *testing.T) {
	// 1. Missing AGENT_NAME
	os.Unsetenv("AGENT_NAME")
	if _, err := LoadConfigFromEnv(); err == nil {
		t.Fatalf("expected error when AGENT_NAME is not set")
	}

	// 2. Default values
	os.Setenv("AGENT_NAME", "coder-1")
	os.Unsetenv("BPD_STATE_DIR")
	os.Unsetenv("BPD_DEFAULT_INTERVAL_SECS")
	os.Unsetenv("BPD_CLAUDE_TIMEOUT_SECS")
	os.Unsetenv("BPD_MAX_CONSECUTIVE_FAILURES")
	os.Unsetenv("BPD_ATTACH_LOCK_FILE")
	os.Unsetenv("BPD_RUNNER_CMD")

	cfg, err := LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadConfigFromEnv failed: %v", err)
	}
	if cfg.AgentName != "coder-1" {
		t.Errorf("expected AgentName 'coder-1', got %q", cfg.AgentName)
	}
	if !strings.HasSuffix(cfg.StateDir, ".bpd") {
		t.Errorf("expected default StateDir ending in .bpd, got %q", cfg.StateDir)
	}
	if cfg.DefaultIntervalSecs != 60 {
		t.Errorf("expected DefaultIntervalSecs 60, got %d", cfg.DefaultIntervalSecs)
	}
	if cfg.ClaudeTimeoutSecs != 600 {
		t.Errorf("expected ClaudeTimeoutSecs 600, got %d", cfg.ClaudeTimeoutSecs)
	}
	if cfg.MaxConsecutiveFailures != 3 {
		t.Errorf("expected MaxConsecutiveFailures 3, got %d", cfg.MaxConsecutiveFailures)
	}
	if !strings.HasSuffix(cfg.AttachLockFile, ".bpd-attached") {
		t.Errorf("expected default AttachLockFile ending in .bpd-attached, got %q", cfg.AttachLockFile)
	}
	if cfg.RunnerCmd != "claude -p" {
		t.Errorf("expected RunnerCmd 'claude -p', got %q", cfg.RunnerCmd)
	}

	// 3. Custom values
	tmpDir := t.TempDir()
	os.Setenv("BPD_STATE_DIR", filepath.Join(tmpDir, "custom-state"))
	os.Setenv("BPD_DEFAULT_INTERVAL_SECS", "45")
	os.Setenv("BPD_CLAUDE_TIMEOUT_SECS", "300")
	os.Setenv("BPD_MAX_CONSECUTIVE_FAILURES", "5")
	os.Setenv("BPD_ATTACH_LOCK_FILE", filepath.Join(tmpDir, "custom-lock"))
	os.Setenv("BPD_RUNNER_CMD", "opencode -m test")

	cfg, err = LoadConfigFromEnv()
	if err != nil {
		t.Fatalf("LoadConfigFromEnv failed with custom env: %v", err)
	}
	if cfg.DefaultIntervalSecs != 45 {
		t.Errorf("expected DefaultIntervalSecs 45, got %d", cfg.DefaultIntervalSecs)
	}
	if cfg.ClaudeTimeoutSecs != 300 {
		t.Errorf("expected ClaudeTimeoutSecs 300, got %d", cfg.ClaudeTimeoutSecs)
	}
	if cfg.MaxConsecutiveFailures != 5 {
		t.Errorf("expected MaxConsecutiveFailures 5, got %d", cfg.MaxConsecutiveFailures)
	}
	if cfg.RunnerCmd != "opencode -m test" {
		t.Errorf("expected RunnerCmd 'opencode -m test', got %q", cfg.RunnerCmd)
	}
}

func TestGenerateUUID(t *testing.T) {
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	for i := 0; i < 10; i++ {
		u, err := generateUUID()
		if err != nil {
			t.Fatalf("generateUUID failed: %v", err)
		}
		if !uuidRegex.MatchString(u) {
			t.Errorf("generated UUID %q does not match RFC 4122 v4 format", u)
		}
	}
}

func TestFormatMessage(t *testing.T) {
	m1 := &libbp.Message{
		Citation:  "msg-1",
		Sender:    "agent-1",
		Content:   "Hello fleet",
		Timestamp: 1600000000000,
	}
	f1 := formatMessage(m1)
	if !strings.Contains(f1, "[msg-1]: Hello fleet") {
		t.Errorf("unexpected format: %q", f1)
	}

	m2 := &libbp.Message{
		Citation:    "msg-2",
		Sender:      "agent-1",
		Destination: "agent-2",
		Content:     "Task assignment",
		Timestamp:   1600000000000,
		IsSigned:    true,
		IsVerified:  true,
		BlobPath:    "/tmp/payload.txt",
	}
	f2 := formatMessage(m2)
	if !strings.Contains(f2, "-> agent-2") || !strings.Contains(f2, "[verified]") || !strings.Contains(f2, "Payload attached: /tmp/payload.txt") {
		t.Errorf("unexpected format for verified signed with blob: %q", f2)
	}

	m3 := &libbp.Message{
		Citation:   "msg-3",
		Sender:     "agent-3",
		Content:    "Untrusted",
		Timestamp:  1600000000000,
		IsSigned:   true,
		IsVerified: false,
	}
	f3 := formatMessage(m3)
	if !strings.Contains(f3, "[UNVERIFIED]") {
		t.Errorf("unexpected format for unverified message: %q", f3)
	}
}

func TestNewDaemonValidation(t *testing.T) {
	if _, err := NewDaemon(nil, &mockBackplaneClient{}, nil, nil); err == nil {
		t.Errorf("expected error with nil config")
	}
	cfg := &Config{AgentName: "test", StateDir: t.TempDir()}
	if _, err := NewDaemon(cfg, nil, nil, nil); err == nil {
		t.Errorf("expected error with nil client")
	}

	// Nil stdout/stderr defaults
	d, err := NewDaemon(cfg, &mockBackplaneClient{}, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error with nil stdout/stderr: %v", err)
	}
	if d.stdout == nil || d.stderr == nil {
		t.Errorf("expected non-nil default stdout and stderr")
	}

	// Invalid state dir
	badCfg := &Config{AgentName: "test", StateDir: "/dev/null/forbidden"}
	if _, err := NewDaemon(badCfg, &mockBackplaneClient{}, nil, nil); err == nil {
		t.Errorf("expected error when StateDir cannot be created")
	}
}

func TestDaemonAttachLock(t *testing.T) {
	tmpDir := t.TempDir()
	lockFile := filepath.Join(tmpDir, ".bpd-attached")
	_ = os.WriteFile(lockFile, []byte("human attached\n"), 0644)

	cfg := &Config{
		AgentName:              "test-agent",
		StateDir:               filepath.Join(tmpDir, "state"),
		DefaultIntervalSecs:    60,
		ClaudeTimeoutSecs:      600,
		MaxConsecutiveFailures: 3,
		AttachLockFile:         lockFile,
		RunnerCmd:              "claude -p",
	}

	client := &mockBackplaneClient{
		recvMessages: []*libbp.Message{{Content: "Should not be drained"}},
	}
	var stdout, stderr bytes.Buffer
	daemon, err := NewDaemon(cfg, client, &stdout, &stderr)
	if err != nil {
		t.Fatalf("NewDaemon failed: %v", err)
	}

	runnerCalled := false
	daemon.SetRunner(func(ctx context.Context, cmdName string, cmdArgs []string, stdin io.Reader, stdout, stderr io.Writer) error {
		runnerCalled = true
		return nil
	})

	err = daemon.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick returned error: %v", err)
	}

	if runnerCalled {
		t.Errorf("runner should not be called when attach lock is present")
	}
	if client.recvCalled > 0 {
		t.Errorf("client.Recv should not be called when attach lock is present")
	}
	if !strings.Contains(stdout.String(), "Human is attached") {
		t.Errorf("expected attach lock log message, got: %s", stdout.String())
	}
}

func TestDaemonSessionIdLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, "state")

	cfg := &Config{
		AgentName:              "test-agent",
		StateDir:               stateDir,
		DefaultIntervalSecs:    60,
		ClaudeTimeoutSecs:      600,
		MaxConsecutiveFailures: 3,
		AttachLockFile:         filepath.Join(tmpDir, "nonexistent-lock"),
		RunnerCmd:              "claude -p",
	}

	client := &mockBackplaneClient{
		recvMessages: []*libbp.Message{{Citation: "1-0", Content: "Do work"}},
	}
	var stdout, stderr bytes.Buffer
	daemon, err := NewDaemon(cfg, client, &stdout, &stderr)
	if err != nil {
		t.Fatalf("NewDaemon failed: %v", err)
	}

	var passedArgs []string
	var passedStdin string
	daemon.SetRunner(func(ctx context.Context, cmdName string, cmdArgs []string, stdin io.Reader, out, errOut io.Writer) error {
		passedArgs = cmdArgs
		b, _ := io.ReadAll(stdin)
		passedStdin = string(b)
		return nil
	})

	// 1. First invocation: no session-id on disk
	err = daemon.Tick(context.Background())
	if err != nil {
		t.Fatalf("Tick failed: %v", err)
	}

	if len(passedArgs) < 3 || passedArgs[0] != "-p" || passedArgs[1] != "--session-id" {
		t.Fatalf("expected --session-id in runner args, got: %v", passedArgs)
	}
	sessionID := passedArgs[2]
	if sessionID == "" {
		t.Fatalf("expected non-empty session ID")
	}
	if !strings.Contains(passedStdin, "Do work") {
		t.Errorf("expected stdin to contain message content, got %q", passedStdin)
	}

	// Verify session-id is stored on disk
	savedID, err := daemon.getSessionID()
	if err != nil || savedID != sessionID {
		t.Fatalf("expected saved session ID %q, got %q (err: %v)", sessionID, savedID, err)
	}

	// Verify queue is cleared and status is OK
	if _, err := os.Stat(daemon.pendingBatchFile()); !os.IsNotExist(err) {
		t.Errorf("pending-batch.txt should be cleared after successful turn")
	}
	if !strings.HasPrefix(client.lastStatus, "ok (bpd, last processed") {
		t.Errorf("expected status 'ok (bpd, ...)', got: %q", client.lastStatus)
	}

	// 2. Subsequent invocation: session-id already on disk -> passes --resume
	client.recvMessages = []*libbp.Message{{Citation: "2-0", Content: "Follow up task"}}
	err = daemon.Tick(context.Background())
	if err != nil {
		t.Fatalf("second Tick failed: %v", err)
	}

	if len(passedArgs) < 3 || passedArgs[0] != "-p" || passedArgs[1] != "--resume" || passedArgs[2] != sessionID {
		t.Fatalf("expected --resume %s in runner args on subsequent invocation, got: %v", sessionID, passedArgs)
	}
}

func TestDaemonFailureAndDegradedStatus(t *testing.T) {
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, "state")

	cfg := &Config{
		AgentName:              "test-agent",
		StateDir:               stateDir,
		DefaultIntervalSecs:    60,
		ClaudeTimeoutSecs:      600,
		MaxConsecutiveFailures: 3,
		AttachLockFile:         filepath.Join(tmpDir, "nonexistent-lock"),
		RunnerCmd:              "claude -p",
	}

	client := &mockBackplaneClient{
		recvMessages: []*libbp.Message{{Citation: "1-0", Content: "Failing task"}},
	}
	var stdout, stderr bytes.Buffer
	daemon, err := NewDaemon(cfg, client, &stdout, &stderr)
	if err != nil {
		t.Fatalf("NewDaemon failed: %v", err)
	}

	runnerErr := errors.New("claude exited with code 1")
	daemon.SetRunner(func(ctx context.Context, cmdName string, cmdArgs []string, stdin io.Reader, out, errOut io.Writer) error {
		return runnerErr
	})

	// Tick 1: Failure 1
	_ = daemon.Tick(context.Background())
	if daemon.getConsecutiveFailures() != 1 {
		t.Errorf("expected 1 consecutive failure, got %d", daemon.getConsecutiveFailures())
	}
	if client.lastStatus != "" {
		t.Errorf("status should not be set on failure 1, got %q", client.lastStatus)
	}
	// Verify queue is NOT clobbered
	batch, _ := daemon.readPendingBatch()
	if !strings.Contains(batch, "Failing task") {
		t.Errorf("pending-batch.txt must be preserved on failure")
	}

	// Tick 2: Failure 2
	_ = daemon.Tick(context.Background())
	if daemon.getConsecutiveFailures() != 2 {
		t.Errorf("expected 2 consecutive failures, got %d", daemon.getConsecutiveFailures())
	}
	if client.lastStatus != "" {
		t.Errorf("status should not be set on failure 2, got %q", client.lastStatus)
	}

	// Tick 3: Failure 3 (reaches threshold of 3) -> sets DEGRADED status
	_ = daemon.Tick(context.Background())
	if daemon.getConsecutiveFailures() != 3 {
		t.Errorf("expected 3 consecutive failures, got %d", daemon.getConsecutiveFailures())
	}
	if !strings.HasPrefix(client.lastStatus, "DEGRADED: bpd has failed 3 consecutive") {
		t.Errorf("expected DEGRADED status after 3 failures, got: %q", client.lastStatus)
	}

	// Next tick succeeds: consecutive failures resets to 0 and status transitions to ok
	daemon.SetRunner(func(ctx context.Context, cmdName string, cmdArgs []string, stdin io.Reader, out, errOut io.Writer) error {
		return nil
	})
	_ = daemon.Tick(context.Background())
	if daemon.getConsecutiveFailures() != 0 {
		t.Errorf("expected 0 consecutive failures after success, got %d", daemon.getConsecutiveFailures())
	}
	if !strings.HasPrefix(client.lastStatus, "ok (bpd, last processed") {
		t.Errorf("expected ok status after recovery, got: %q", client.lastStatus)
	}
}

func TestDaemonEdgeCasesAndErrors(t *testing.T) {
	tmpDir := t.TempDir()
	stateDir := filepath.Join(tmpDir, "state")

	cfg := &Config{
		AgentName:              "test-agent",
		StateDir:               stateDir,
		DefaultIntervalSecs:    60,
		ClaudeTimeoutSecs:      600,
		MaxConsecutiveFailures: 3,
		AttachLockFile:         filepath.Join(tmpDir, "nonexistent-lock"),
		RunnerCmd:              "claude -p",
	}

	// 1. Client Recv error does not crash Tick
	client := &mockBackplaneClient{
		recvErr: errors.New("valkey timeout"),
	}
	var stdout, stderr bytes.Buffer
	daemon, _ := NewDaemon(cfg, client, &stdout, &stderr)
	err := daemon.Tick(context.Background())
	if err != nil {
		t.Errorf("Tick with recvErr should not fail if batch is empty: %v", err)
	}
	if !strings.Contains(stderr.String(), "Error draining messages") {
		t.Errorf("expected draining error logged to stderr: %s", stderr.String())
	}

	// 2. Empty runner command
	cfg.RunnerCmd = "   "
	daemon.appendPendingBatch("Some pending message\n")
	err = daemon.Tick(context.Background())
	if err == nil || !strings.Contains(err.Error(), "empty runner command") {
		t.Errorf("expected empty runner command error, got: %v", err)
	}

	// 3. Status update error on success
	cfg.RunnerCmd = "claude -p"
	client.statusErr = errors.New("redis read-only replica")
	daemon.SetRunner(func(ctx context.Context, cmdName string, cmdArgs []string, stdin io.Reader, out, errOut io.Writer) error {
		return nil
	})
	stderr.Reset()
	err = daemon.Tick(context.Background())
	if err != nil {
		t.Errorf("Tick should return nil on runner success even if status fails: %v", err)
	}
	if !strings.Contains(stderr.String(), "Failed updating backplane status") {
		t.Errorf("expected status failure logged: %s", stderr.String())
	}

	// 4. Status update error on degraded
	client.statusErr = errors.New("network down")
	daemon.setConsecutiveFailures(2)
	daemon.appendPendingBatch("retry message\n")
	daemon.SetRunner(func(ctx context.Context, cmdName string, cmdArgs []string, stdin io.Reader, out, errOut io.Writer) error {
		return errors.New("command failed")
	})
	stderr.Reset()
	_ = daemon.Tick(context.Background())
	if !strings.Contains(stderr.String(), "Failed setting degraded status") {
		t.Errorf("expected degraded status failure logged: %s", stderr.String())
	}

	// 5. Corrupted consecutive-failures file returns 0
	_ = os.WriteFile(daemon.consecutiveFailuresFile(), []byte("not-a-number"), 0600)
	if n := daemon.getConsecutiveFailures(); n != 0 {
		t.Errorf("expected 0 for unparseable consecutive failures, got %d", n)
	}
}

func TestDaemonDynamicPollIntervalAndRun(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &Config{
		AgentName:              "test-agent",
		StateDir:               filepath.Join(tmpDir, "state"),
		DefaultIntervalSecs:    60,
		ClaudeTimeoutSecs:      600,
		MaxConsecutiveFailures: 3,
		AttachLockFile:         filepath.Join(tmpDir, "lock"),
		RunnerCmd:              "claude -p",
	}

	client := &mockBackplaneClient{
		pollInterval: 120,
	}

	var stdout, stderr bytes.Buffer
	daemon, err := NewDaemon(cfg, client, &stdout, &stderr)
	if err != nil {
		t.Fatalf("NewDaemon failed: %v", err)
	}

	var sleptDurations []time.Duration
	ctx, cancel := context.WithCancel(context.Background())
	daemon.SetSleeper(func(sCtx context.Context, d time.Duration) error {
		sleptDurations = append(sleptDurations, d)
		cancel() // Stop daemon after first sleep
		return context.Canceled
	})

	err = daemon.Run(ctx)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(sleptDurations) == 0 || sleptDurations[0] != 120*time.Second {
		t.Errorf("expected sleeper to sleep 120s from backplane poll-interval, got: %v", sleptDurations)
	}

	// Test interval fallback when out of range (< 5 or > 3600)
	sleptDurations = nil
	client.pollInterval = 2 // Below MinPollInterval
	ctx2, cancel2 := context.WithCancel(context.Background())
	daemon.SetSleeper(func(sCtx context.Context, d time.Duration) error {
		sleptDurations = append(sleptDurations, d)
		cancel2()
		return context.Canceled
	})
	_ = daemon.Run(ctx2)
	if len(sleptDurations) == 0 || sleptDurations[0] != 60*time.Second {
		t.Errorf("expected fallback to 60s when poll-interval is < 5, got: %v", sleptDurations)
	}

	// Test interval fallback when error
	sleptDurations = nil
	client.pollErr = errors.New("redis timeout")
	ctx3, cancel3 := context.WithCancel(context.Background())
	daemon.SetSleeper(func(sCtx context.Context, d time.Duration) error {
		sleptDurations = append(sleptDurations, d)
		cancel3()
		return context.Canceled
	})
	_ = daemon.Run(ctx3)
	if len(sleptDurations) == 0 || sleptDurations[0] != 60*time.Second {
		t.Errorf("expected fallback to 60s when poll error occurs, got: %v", sleptDurations)
	}

	// Test sleeper error propagation
	daemon.SetSleeper(func(sCtx context.Context, d time.Duration) error {
		return errors.New("custom sleep error")
	})
	err = daemon.Run(context.Background())
	if err == nil || err.Error() != "custom sleep error" {
		t.Errorf("expected custom sleep error returned from Run, got: %v", err)
	}

	// Test pre-canceled context stops immediately
	canceledCtx, cancelNow := context.WithCancel(context.Background())
	cancelNow()
	if err := daemon.Run(canceledCtx); err != nil {
		t.Errorf("expected nil from Run with canceled context, got: %v", err)
	}
}

func TestRunCLIHelpAndValidation(t *testing.T) {
	var stdout, stderr bytes.Buffer

	// 1. Help
	code := RunCLI([]string{"--help"}, &stdout, &stderr)
	if code != 0 || !strings.Contains(stdout.String(), "Usage: bpd") {
		t.Errorf("expected code 0 and usage output for --help")
	}

	// 2. Missing AGENT_NAME
	os.Unsetenv("AGENT_NAME")
	stderr.Reset()
	code = RunCLI([]string{}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "AGENT_NAME environment variable is required") {
		t.Errorf("expected code 1 when AGENT_NAME is missing, got %d (err: %s)", code, stderr.String())
	}

	// 3. Connection error
	os.Setenv("AGENT_NAME", "agent-1")
	os.Setenv("BP_HOST", "127.0.0.1")
	os.Setenv("BP_PORT", "64999") // Unused port
	stderr.Reset()
	code = RunCLI([]string{}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "bpd connection error:") {
		t.Errorf("expected connection error code 1, got %d (err: %s)", code, stderr.String())
	}
}

func TestRunCLIWithLiveHarness(t *testing.T) {
	valkey := harness.StartValkeyHarness(t)
	defer valkey.Teardown()

	tmpDir := t.TempDir()
	os.Setenv("AGENT_NAME", "agent-1")
	os.Setenv("BP_HOST", "127.0.0.1")
	os.Setenv("BP_PORT", fmt.Sprintf("%d", valkey.Port))
	os.Setenv("BP_AGENT", "agent-1")
	os.Setenv("BP_PASSWORD", valkey.Agent1Pass)
	os.Setenv("BPD_STATE_DIR", filepath.Join(tmpDir, "state"))
	os.Setenv("BPD_ATTACH_LOCK_FILE", filepath.Join(tmpDir, "lock"))

	var stdout, stderr bytes.Buffer
	// Test defaultRunner with echo
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err := defaultRunner(ctx, "echo", []string{"runner test"}, strings.NewReader("hello"), &stdout, &stderr)
	if err != nil {
		t.Fatalf("defaultRunner failed: %v", err)
	}

	// Test defaultSleeper
	sleepCtx, sleepCancel := context.WithCancel(context.Background())
	sleepCancel()
	if err := defaultSleeper(sleepCtx, 5*time.Second); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled from defaultSleeper, got %v", err)
	}

	// Test defaultSleeper normal expiration
	if err := defaultSleeper(context.Background(), 1*time.Millisecond); err != nil {
		t.Errorf("expected nil from defaultSleeper on normal duration, got: %v", err)
	}

	// Test RunCLI initialization error with invalid state dir
	os.Setenv("BPD_STATE_DIR", "/dev/null/forbidden/state")
	stderr.Reset()
	code := RunCLI([]string{}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), "bpd initialization error:") {
		t.Errorf("expected code 1 on init error, got %d (err: %s)", code, stderr.String())
	}
}

func TestMainFunction(t *testing.T) {
	if os.Getenv("TEST_BPD_MAIN") == "1" {
		os.Args = []string{"bpd", "help"}
		main()
		return
	}
	cmd := exec.Command(os.Args[0], "-test.run=TestMainFunction")
	cmd.Env = append(os.Environ(), "TEST_BPD_MAIN=1")
	_ = cmd.Run()
}
