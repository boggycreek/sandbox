// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package runtime

import (
	"context"
	"errors"
	"fmt"
	"runtime/debug"
	"sync"
	"time"
)

var (
	// ErrDuplicateSubsystem is returned when registering a subsystem with an existing ID.
	ErrDuplicateSubsystem = errors.New("subsystem already registered")
	// ErrSupervisorRunning is returned when attempting to register a subsystem while running.
	ErrSupervisorRunning = errors.New("cannot register subsystem while supervisor is running")
)

// SubsystemStats tracks execution and restart metrics for a subsystem.
type SubsystemStats struct {
	ID           string    `json:"id"`
	RestartCount int       `json:"restart_count"`
	LastError    string    `json:"last_error,omitempty"`
	LastPanic    string    `json:"last_panic,omitempty"`
	StartedAt    time.Time `json:"started_at"`
	IsResilient  bool      `json:"is_resilient"`
}

// Supervisor manages concurrent Subsystem workers, panic recovery, and runtime lifecycle.
type Supervisor struct {
	mu           sync.RWMutex
	subsystems   []Subsystem
	subsystemMap map[string]Subsystem
	stats        map[string]*SubsystemStats
	bus          *EventBus
	state        *SharedState
	isRunning    bool
	cancelFn     context.CancelCauseFunc
	wg           sync.WaitGroup
	backoff      time.Duration
}

// NewSupervisor instantiates a Supervisor bound to an EventBus and SharedState.
func NewSupervisor(bus *EventBus, state *SharedState) *Supervisor {
	return &Supervisor{
		subsystemMap: make(map[string]Subsystem),
		stats:        make(map[string]*SubsystemStats),
		bus:          bus,
		state:        state,
		backoff:      50 * time.Millisecond,
	}
}

// SetRestartBackoff configures the pause duration before restarting a failed resilient subsystem.
func (s *Supervisor) SetRestartBackoff(d time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.backoff = d
}

// Register adds a new Subsystem to the supervisor before Start() is invoked.
func (s *Supervisor) Register(sub Subsystem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.isRunning {
		return ErrSupervisorRunning
	}

	id := sub.ID()
	if id == "" {
		return errors.New("subsystem ID cannot be empty")
	}
	if _, exists := s.subsystemMap[id]; exists {
		return fmt.Errorf("%w: %s", ErrDuplicateSubsystem, id)
	}

	isResilient := false
	if r, ok := sub.(ResilientSubsystem); ok {
		isResilient = r.IsResilient()
	}

	s.subsystems = append(s.subsystems, sub)
	s.subsystemMap[id] = sub
	s.stats[id] = &SubsystemStats{
		ID:          id,
		IsResilient: isResilient,
	}
	return nil
}

// Subsystems returns the list of registered subsystem IDs.
func (s *Supervisor) Subsystems() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ids := make([]string, len(s.subsystems))
	for i, sub := range s.subsystems {
		ids[i] = sub.ID()
	}
	return ids
}

// Stats returns a snapshot of execution metrics for all registered subsystems.
func (s *Supervisor) Stats() map[string]SubsystemStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]SubsystemStats, len(s.stats))
	for k, v := range s.stats {
		result[k] = *v
	}
	return result
}

// Start launches all registered subsystems and blocks until ctx is canceled or an unrecoverable failure occurs.
func (s *Supervisor) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.isRunning {
		s.mu.Unlock()
		return errors.New("supervisor is already running")
	}
	s.isRunning = true
	runCtx, cancel := context.WithCancelCause(ctx)
	s.cancelFn = cancel
	subs := make([]Subsystem, len(s.subsystems))
	copy(subs, s.subsystems)
	s.mu.Unlock()

	for _, sub := range subs {
		s.wg.Add(1)
		go s.runSubsystem(runCtx, sub)
	}

	// Wait for context cancellation or unrecoverable error
	<-runCtx.Done()

	// Wait for all workers to complete
	s.wg.Wait()

	s.mu.Lock()
	s.isRunning = false
	s.mu.Unlock()

	cause := context.Cause(runCtx)
	if errors.Is(cause, context.Canceled) {
		return nil
	}
	return cause
}

func (s *Supervisor) runSubsystem(ctx context.Context, sub Subsystem) {
	defer s.wg.Done()

	id := sub.ID()
	s.mu.Lock()
	stat := s.stats[id]
	stat.StartedAt = time.Now()
	isResilient := stat.IsResilient
	backoff := s.backoff
	s.mu.Unlock()

	for {
		if ctx.Err() != nil {
			return
		}

		err := s.executeWithRecovery(ctx, sub)
		if err == nil || errors.Is(err, context.Canceled) {
			return
		}

		// Check if canceled during execution
		if ctx.Err() != nil {
			return
		}

		// Handle error / panic
		s.mu.Lock()
		stat.RestartCount++
		stat.LastError = err.Error()
		s.mu.Unlock()

		_ = s.bus.Publish(Event{
			Priority: P2_StandardAsync,
			Source:   fmt.Sprintf("supervisor:%s", id),
			Target:   "*",
			Payload: map[string]any{
				"subsystem_id": id,
				"error":        err.Error(),
				"restart":      stat.RestartCount,
			},
		})

		if !isResilient {
			// Critical subsystem failed: trigger supervisor shutdown
			s.cancelFn(fmt.Errorf("critical subsystem %s failed: %w", id, err))
			return
		}

		// Resilient subsystem: backoff and restart
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
	}
}

func (s *Supervisor) executeWithRecovery(ctx context.Context, sub Subsystem) (err error) {
	defer func() {
		if r := recover(); r != nil {
			stack := string(debug.Stack())
			err = fmt.Errorf("subsystem %s panicked: %v\nstack:\n%s", sub.ID(), r, stack)

			s.mu.Lock()
			if stat, ok := s.stats[sub.ID()]; ok {
				stat.LastPanic = fmt.Sprintf("%v", r)
			}
			s.mu.Unlock()
		}
	}()

	return sub.Start(ctx, s.bus, s.state)
}

// Stop gracefully cancels all running subsystems and waits for termination.
func (s *Supervisor) Stop() {
	s.mu.RLock()
	cancel := s.cancelFn
	s.mu.RUnlock()

	if cancel != nil {
		cancel(context.Canceled)
	}
	s.wg.Wait()
}
