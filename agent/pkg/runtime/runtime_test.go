// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// --- Types Tests ---

func TestPriorityLevelString(t *testing.T) {
	tests := []struct {
		p        PriorityLevel
		expected string
	}{
		{P0_Control, "P0_Control"},
		{P1_HighPriority, "P1_HighPriority"},
		{P2_StandardAsync, "P2_StandardAsync"},
		{P3_TelemetryQuery, "P3_TelemetryQuery"},
		{PriorityLevel(99), "PriorityLevel(99)"},
	}

	for _, tt := range tests {
		if got := tt.p.String(); got != tt.expected {
			t.Errorf("PriorityLevel(%d).String() = %q, expected %q", tt.p, got, tt.expected)
		}
	}
}

func TestEventAndEnvelopeJSON(t *testing.T) {
	now := time.Now().UTC()
	evt := Event{
		ID:        "evt-123",
		Priority:  P1_HighPriority,
		Source:    "valkey:inbox",
		Target:    "repl-main",
		Payload:   map[string]string{"msg": "hello"},
		Timestamp: now,
	}

	data, err := json.Marshal(evt)
	if err != nil {
		t.Fatalf("failed to marshal Event: %v", err)
	}

	var decoded Event
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal Event: %v", err)
	}
	if decoded.ID != "evt-123" || decoded.Priority != P1_HighPriority || decoded.Source != "valkey:inbox" {
		t.Errorf("unexpected decoded event: %+v", decoded)
	}

	envelope := BackplaneEnvelope{
		ID:        "env-456",
		Sender:    "agent-alpha",
		Recipient: "agent-beta",
		Payload:   "execute test",
		Signature: "sig-xyz",
		Timestamp: now,
	}

	envData, err := json.Marshal(envelope)
	if err != nil {
		t.Fatalf("failed to marshal BackplaneEnvelope: %v", err)
	}

	var decodedEnv BackplaneEnvelope
	if err := json.Unmarshal(envData, &decodedEnv); err != nil {
		t.Fatalf("failed to unmarshal BackplaneEnvelope: %v", err)
	}
	if decodedEnv.ID != "env-456" || decodedEnv.Sender != "agent-alpha" {
		t.Errorf("unexpected decoded envelope: %+v", decodedEnv)
	}
}

// --- State Tests ---

func TestSharedStateLifecycle(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "sub", "state.json")

	state := NewSharedState(stateFile, "agent-test")
	initial := state.Read()
	if initial.AgentID != "agent-test" || initial.Status != "idle" {
		t.Fatalf("unexpected initial state: %+v", initial)
	}

	// Update state
	err := state.Update(func(s *AgentState) {
		s.Status = "working"
		s.CurrentTaskID = "task-42"
		s.CurrentActivity = "writing code"
	})
	if err != nil {
		t.Fatalf("state.Update failed: %v", err)
	}

	updated := state.Read()
	if updated.Status != "working" || updated.CurrentTaskID != "task-42" {
		t.Fatalf("updated state mismatch: %+v", updated)
	}

	// Verify file on disk
	if _, err := os.Stat(stateFile); err != nil {
		t.Fatalf("state file was not created: %v", err)
	}

	// Load into a new SharedState instance
	state2 := NewSharedState(stateFile, "agent-other")
	if err := state2.LoadFromDisk(); err != nil {
		t.Fatalf("LoadFromDisk failed: %v", err)
	}
	loaded := state2.Read()
	if loaded.AgentID != "agent-test" || loaded.Status != "working" || loaded.CurrentTaskID != "task-42" {
		t.Fatalf("loaded state mismatch: %+v", loaded)
	}

	// Empty path handling
	emptyState := NewSharedState("", "agent-noop")
	if err := emptyState.FlushToDisk(); err != nil {
		t.Errorf("FlushToDisk with empty path should succeed: %v", err)
	}
	if err := emptyState.LoadFromDisk(); err != nil {
		t.Errorf("LoadFromDisk with empty path should succeed: %v", err)
	}

	// Load non-existent file
	nonExistentState := NewSharedState(filepath.Join(tmpDir, "nonexistent.json"), "agent-none")
	if err := nonExistentState.LoadFromDisk(); err != nil {
		t.Errorf("LoadFromDisk non-existent file should succeed: %v", err)
	}

	// Corrupt file handling
	corruptFile := filepath.Join(tmpDir, "corrupt.json")
	if err := os.WriteFile(corruptFile, []byte("{invalid-json"), 0600); err != nil {
		t.Fatalf("failed to write corrupt file: %v", err)
	}
	corruptState := NewSharedState(corruptFile, "agent-corrupt")
	if err := corruptState.LoadFromDisk(); err == nil {
		t.Error("expected error loading corrupt JSON state")
	}
}

func TestSharedStateWorkspaceLocking(t *testing.T) {
	state := NewSharedState("", "agent-lock")

	var counter int
	var wg sync.WaitGroup

	// Exclusive writers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			state.LockWorkspace()
			counter++
			time.Sleep(5 * time.Millisecond)
			state.UnlockWorkspace()
		}()
	}

	// Shared readers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			state.RLockWorkspace()
			_ = counter
			time.Sleep(2 * time.Millisecond)
			state.RUnlockWorkspace()
		}()
	}

	wg.Wait()
	if counter != 5 {
		t.Errorf("expected counter 5, got %d", counter)
	}
}

func TestSharedStateConcurrentDiskWrites(t *testing.T) {
	tmpDir := t.TempDir()
	stateFile := filepath.Join(tmpDir, "concurrent_state.json")
	state := NewSharedState(stateFile, "agent-concurrent")

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		idx := i
		go func() {
			defer wg.Done()
			_ = state.Update(func(s *AgentState) {
				s.CurrentActivity = fmt.Sprintf("Activity-%d", idx)
			})
		}()
	}
	wg.Wait()

	// Verify the final written file is valid JSON
	reloaded := NewSharedState(stateFile, "agent-concurrent")
	if err := reloaded.LoadFromDisk(); err != nil {
		t.Fatalf("failed to load state after concurrent writes: %v", err)
	}
	if !strings.HasPrefix(reloaded.Read().CurrentActivity, "Activity-") {
		t.Errorf("unexpected activity: %s", reloaded.Read().CurrentActivity)
	}
}

// --- EventBus Tests ---

func TestEventBusPriorityOrdering(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	// Publish in reverse priority order: P3, P2, P1, P0
	_ = bus.Publish(Event{Priority: P3_TelemetryQuery, Source: "p3"})
	_ = bus.Publish(Event{Priority: P2_StandardAsync, Source: "p2"})
	_ = bus.Publish(Event{Priority: P1_HighPriority, Source: "p1"})
	_ = bus.Publish(Event{Priority: P0_Control, Source: "p0"})

	depths := bus.QueueDepth()
	if depths[P0_Control] != 1 || depths[P1_HighPriority] != 1 || depths[P2_StandardAsync] != 1 || depths[P3_TelemetryQuery] != 1 {
		t.Fatalf("unexpected queue depths: %+v", depths)
	}
	if bus.TotalQueueDepth() != 4 {
		t.Fatalf("expected total queue depth 4, got %d", bus.TotalQueueDepth())
	}

	// Must poll in strict priority: P0, P1, P2, P3
	expectedOrder := []PriorityLevel{P0_Control, P1_HighPriority, P2_StandardAsync, P3_TelemetryQuery}
	for i, expected := range expectedOrder {
		evt, ok := bus.Poll()
		if !ok {
			t.Fatalf("step %d: expected event, got none", i)
		}
		if evt.Priority != expected {
			t.Errorf("step %d: expected priority %v, got %v", i, expected, evt.Priority)
		}
	}

	// Now empty
	if _, ok := bus.Poll(); ok {
		t.Error("expected poll on empty bus to return false")
	}
}

func TestEventBusNextBlocking(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	done := make(chan Event, 1)
	go func() {
		evt, err := bus.Next(ctx)
		if err == nil {
			done <- evt
		}
	}()

	time.Sleep(20 * time.Millisecond)
	_ = bus.Publish(Event{Priority: P1_HighPriority, Target: "repl-main", Payload: "start-task"})

	select {
	case evt := <-done:
		if evt.Priority != P1_HighPriority || evt.Target != "repl-main" {
			t.Errorf("unexpected event received: %+v", evt)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for Next() to return")
	}
}

func TestEventBusNextCancellationAndClose(t *testing.T) {
	bus := NewEventBus()

	// Context cancel
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := bus.Next(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	// Close bus while blocked
	ctx2 := context.Background()
	done := make(chan error, 1)
	go func() {
		_, err := bus.Next(ctx2)
		done <- err
	}()

	time.Sleep(20 * time.Millisecond)
	bus.Close()

	select {
	case err := <-done:
		if !errors.Is(err, ErrBusClosed) {
			t.Errorf("expected ErrBusClosed, got %v", err)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for bus.Close() to unblock Next()")
	}

	// Subsequent publish on closed bus
	if err := bus.Publish(Event{Priority: P0_Control}); !errors.Is(err, ErrBusClosed) {
		t.Errorf("expected ErrBusClosed on publish, got %v", err)
	}

	// Close again is idempotent
	bus.Close()
}

func TestEventBusSubscription(t *testing.T) {
	bus := NewEventBus()
	defer bus.Close()

	replCh, unsubscribeRepl := bus.Subscribe("repl-main", 10)
	defer unsubscribeRepl()

	allCh, unsubscribeAll := bus.Subscribe("*", 10)
	defer unsubscribeAll()

	_ = bus.Publish(Event{Priority: P1_HighPriority, Target: "repl-main", Payload: "for-repl"})
	_ = bus.Publish(Event{Priority: P2_StandardAsync, Target: "gateway", Payload: "for-gateway"})

	// replCh should receive only "for-repl"
	select {
	case e := <-replCh:
		if e.Target != "repl-main" {
			t.Errorf("unexpected event on replCh: %+v", e)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("replCh did not receive expected event")
	}

	// allCh should receive both
	receivedCount := 0
	timeout := time.After(200 * time.Millisecond)
loop:
	for {
		select {
		case <-allCh:
			receivedCount++
			if receivedCount == 2 {
				break loop
			}
		case <-timeout:
			break loop
		}
	}
	if receivedCount != 2 {
		t.Errorf("expected 2 events on wildcard allCh, got %d", receivedCount)
	}

	// Unsubscribe cleanup
	unsubscribeRepl()
	_ = bus.Publish(Event{Priority: P1_HighPriority, Target: "repl-main"})
}

// --- Supervisor Tests ---

type mockSubsystem struct {
	id          string
	isResilient bool
	startFunc   func(ctx context.Context, bus *EventBus, state *SharedState) error
}

func (m *mockSubsystem) ID() string        { return m.id }
func (m *mockSubsystem) IsResilient() bool { return m.isResilient }
func (m *mockSubsystem) Start(ctx context.Context, bus *EventBus, state *SharedState) error {
	if m.startFunc != nil {
		return m.startFunc(ctx, bus, state)
	}
	<-ctx.Done()
	return ctx.Err()
}

func TestSupervisorRegistration(t *testing.T) {
	bus := NewEventBus()
	state := NewSharedState("", "agent-reg")
	sup := NewSupervisor(bus, state)

	if err := sup.Register(&mockSubsystem{id: ""}); err == nil {
		t.Error("expected error registering empty subsystem ID")
	}

	sub1 := &mockSubsystem{id: "worker-1", isResilient: true}
	if err := sup.Register(sub1); err != nil {
		t.Fatalf("failed to register sub1: %v", err)
	}

	// Duplicate
	if err := sup.Register(sub1); !errors.Is(err, ErrDuplicateSubsystem) {
		t.Errorf("expected ErrDuplicateSubsystem, got %v", err)
	}

	ids := sup.Subsystems()
	if len(ids) != 1 || ids[0] != "worker-1" {
		t.Errorf("unexpected subsystems list: %v", ids)
	}

	stats := sup.Stats()
	if s, ok := stats["worker-1"]; !ok || !s.IsResilient {
		t.Errorf("unexpected stats: %+v", stats)
	}
}

func TestSupervisorGracefulLifecycle(t *testing.T) {
	bus := NewEventBus()
	state := NewSharedState("", "agent-life")
	sup := NewSupervisor(bus, state)

	var started atomic.Bool
	sub := &mockSubsystem{
		id: "worker-normal",
		startFunc: func(ctx context.Context, bus *EventBus, state *SharedState) error {
			started.Store(true)
			<-ctx.Done()
			return nil
		},
	}
	_ = sup.Register(sub)

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- sup.Start(ctx)
	}()

	time.Sleep(20 * time.Millisecond)
	if !started.Load() {
		t.Fatal("subsystem was not started")
	}

	// Registering while running should fail
	if err := sup.Register(&mockSubsystem{id: "late"}); !errors.Is(err, ErrSupervisorRunning) {
		t.Errorf("expected ErrSupervisorRunning, got %v", err)
	}

	// Calling Start again while running should fail
	if err := sup.Start(context.Background()); err == nil {
		t.Error("expected error starting already running supervisor")
	}

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("expected nil error on clean cancel, got %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("supervisor did not stop within timeout")
	}
}

func TestSupervisorCriticalSubsystemFailure(t *testing.T) {
	bus := NewEventBus()
	state := NewSharedState("", "agent-crit")
	sup := NewSupervisor(bus, state)

	expectedErr := errors.New("unrecoverable fatal database failure")
	sub := &mockSubsystem{
		id:          "critical-worker",
		isResilient: false,
		startFunc: func(ctx context.Context, bus *EventBus, state *SharedState) error {
			return expectedErr
		},
	}
	_ = sup.Register(sub)

	err := sup.Start(context.Background())
	if err == nil || !errors.Is(err, expectedErr) {
		t.Fatalf("expected error containing %v, got %v", expectedErr, err)
	}
}

func TestSupervisorResilientSubsystemPanicRecovery(t *testing.T) {
	bus := NewEventBus()
	state := NewSharedState("", "agent-resilient")
	sup := NewSupervisor(bus, state)
	sup.SetRestartBackoff(10 * time.Millisecond)

	var invocations atomic.Int32
	sub := &mockSubsystem{
		id:          "resilient-worker",
		isResilient: true,
		startFunc: func(ctx context.Context, bus *EventBus, state *SharedState) error {
			count := invocations.Add(1)
			if count == 1 {
				panic("simulated transient panic in background watcher")
			}
			if count == 2 {
				return errors.New("simulated transient network error")
			}
			// Third run stays alive
			<-ctx.Done()
			return nil
		},
	}
	_ = sup.Register(sub)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- sup.Start(ctx)
	}()

	// Wait for 3 invocations
	for i := 0; i < 30; i++ {
		if invocations.Load() >= 3 {
			break
		}
		time.Sleep(15 * time.Millisecond)
	}

	if invocations.Load() < 3 {
		t.Fatalf("expected at least 3 invocations, got %d", invocations.Load())
	}

	stats := sup.Stats()
	wStat := stats["resilient-worker"]
	if wStat.RestartCount < 2 {
		t.Errorf("expected at least 2 restarts, got %d", wStat.RestartCount)
	}
	if wStat.LastPanic == "" {
		t.Error("expected LastPanic to be recorded in stats")
	}

	// Stop supervisor gracefully
	sup.Stop()
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("expected clean stop, got %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("timed out waiting for sup.Stop()")
	}
}

func TestProcessTrackingAndManagedWait(t *testing.T) {
	// 1. Invalid / edge cases
	RegisterChildPID(0)
	RegisterChildPID(-1)
	UnregisterChildPID(0)
	UnregisterChildPID(-1)

	if IsManagedChildPID(999999) {
		t.Error("unexpected managed status for unmanaged PID")
	}

	if _, ok := PopReapedExit(999999); ok {
		t.Error("unexpected reaped status for unknown PID")
	}

	if err := WaitManagedCmd(nil); err == nil {
		t.Error("expected error waiting on nil cmd")
	}
	if err := WaitManagedCmd(&exec.Cmd{}); err == nil {
		t.Error("expected error waiting on cmd with nil Process")
	}

	// 2. Normal execution
	cmd := exec.Command("true")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start cmd: %v", err)
	}
	pid := cmd.Process.Pid
	RegisterChildPID(pid)
	if !IsManagedChildPID(pid) {
		t.Errorf("expected PID %d to be managed", pid)
	}

	if err := WaitManagedCmd(cmd); err != nil {
		t.Fatalf("expected successful WaitManagedCmd: %v", err)
	}
	UnregisterChildPID(pid)
	if IsManagedChildPID(pid) {
		t.Errorf("expected PID %d to be unmanaged after unregister", pid)
	}

	// 3. Simulated reaped exit with code 0 and non-zero code
	RecordReapedExit(12345, 0)
	if code, ok := PopReapedExit(12345); !ok || code != 0 {
		t.Errorf("expected code 0, got %d, %v", code, ok)
	}
	// Second pop should return false
	if _, ok := PopReapedExit(12345); ok {
		t.Error("expected second pop to return false")
	}

	RecordReapedExit(12346, 42)
	if code, ok := PopReapedExit(12346); !ok || code != 42 {
		t.Errorf("expected code 42, got %d, %v", code, ok)
	}
}

