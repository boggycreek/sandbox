// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package runtime

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AgentState represents in-memory runtime telemetry and task execution progress.
type AgentState struct {
	AgentID         string    `json:"agent_id"`
	Status          string    `json:"status"` // "idle", "working", "interrupted", "error"
	CurrentTaskID   string    `json:"current_task_id"`
	TaskDescription string    `json:"task_description"`
	ActiveBranch    string    `json:"active_branch"`
	CurrentActivity string    `json:"current_activity"`
	LastTestResult  string    `json:"last_test_result"`
	QueueDepth      int       `json:"queue_depth"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// SharedState provides thread-safe access to agent state and workspace mutations.
type SharedState struct {
	mu            sync.RWMutex
	workspaceMu   sync.RWMutex
	fileMu        sync.Mutex
	data          AgentState
	stateFilePath string
}

// NewSharedState creates a new SharedState instance initialized with agentID.
func NewSharedState(filePath string, agentID string) *SharedState {
	return &SharedState{
		data: AgentState{
			AgentID:   agentID,
			Status:    "idle",
			UpdatedAt: time.Now(),
		},
		stateFilePath: filePath,
	}
}

// Read returns an atomic snapshot copy of the agent state.
func (s *SharedState) Read() AgentState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data
}

// Update mutates the agent state under an exclusive lock and atomically flushes to disk.
func (s *SharedState) Update(fn func(data *AgentState)) error {
	s.mu.Lock()
	fn(&s.data)
	s.data.UpdatedAt = time.Now()
	s.mu.Unlock()
	return s.FlushToDisk()
}

// FlushToDisk persists the current state atomically to disk if stateFilePath is configured.
func (s *SharedState) FlushToDisk() error {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()

	s.mu.RLock()
	if s.stateFilePath == "" {
		s.mu.RUnlock()
		return nil
	}
	dataBytes, err := json.MarshalIndent(s.data, "", "  ")
	s.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	dir := filepath.Dir(s.stateFilePath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create state directory %s: %w", dir, err)
	}

	tmpFile := fmt.Sprintf("%s.tmp.%d", s.stateFilePath, time.Now().UnixNano())
	if err := os.WriteFile(tmpFile, dataBytes, 0600); err != nil {
		return fmt.Errorf("failed to write temp state file: %w", err)
	}
	if err := os.Rename(tmpFile, s.stateFilePath); err != nil {
		_ = os.Remove(tmpFile)
		return fmt.Errorf("failed to rename temp state file to %s: %w", s.stateFilePath, err)
	}
	return nil
}

// LoadFromDisk populates the state from stateFilePath if it exists on disk.
func (s *SharedState) LoadFromDisk() error {
	s.fileMu.Lock()
	defer s.fileMu.Unlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stateFilePath == "" {
		return nil
	}

	dataBytes, err := os.ReadFile(s.stateFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read state file: %w", err)
	}

	var loaded AgentState
	if err := json.Unmarshal(dataBytes, &loaded); err != nil {
		return fmt.Errorf("failed to unmarshal state file: %w", err)
	}

	s.data = loaded
	return nil
}

// LockWorkspace acquires exclusive single-writer access to the workspace.
func (s *SharedState) LockWorkspace() {
	s.workspaceMu.Lock()
}

// UnlockWorkspace releases exclusive single-writer access to the workspace.
func (s *SharedState) UnlockWorkspace() {
	s.workspaceMu.Unlock()
}

// RLockWorkspace acquires shared read-only access to the workspace.
func (s *SharedState) RLockWorkspace() {
	s.workspaceMu.RLock()
}

// RUnlockWorkspace releases shared read-only access to the workspace.
func (s *SharedState) RUnlockWorkspace() {
	s.workspaceMu.RUnlock()
}
