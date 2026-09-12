package libbp

import (
	"errors"
	"testing"
)

func TestGuardAllowedCommands(t *testing.T) {
	// Standard allowed commands
	allowed := []struct {
		cmd  string
		args []string
	}{
		{"XADD", []string{"agent-1:out", "*", "content", "hello"}},
		{"XADD", []string{"agent-2:inbox", "*", "content", "hello"}},
		{"XREAD", []string{"STREAMS", "agent-1:inbox", "0-0"}},
		{"INCR", []string{"agent-1:seq"}},
		{"SET", []string{"agent-1:status", "busy", "EX", "300"}},
		{"GET", []string{"agent-2:status"}},
		{"HSET", []string{"agent-1:finger", "role", "coder"}},
		{"HGETALL", []string{"agent-2:finger"}},
		{"SCAN", []string{"0", "MATCH", "*:out"}},
	}

	for _, tc := range allowed {
		if err := AssertAllowed("agent-1", tc.cmd, tc.args...); err != nil {
			t.Errorf("expected command %s %v to be allowed, got error: %v", tc.cmd, tc.args, err)
		}
	}
}

func TestGuardDangerousCommands(t *testing.T) {
	dangerous := []string{"FLUSHALL", "FLUSHDB", "SHUTDOWN", "DEBUG", "CONFIG", "KEYS"}

	for _, cmd := range dangerous {
		err := AssertAllowed("agent-1", cmd)
		if !errors.Is(err, ErrForbiddenCommand) {
			t.Errorf("expected ErrForbiddenCommand for %s, got: %v", cmd, err)
		}
	}
}

func TestGuardDestructivePeerInboxTrimming(t *testing.T) {
	// Refuse MAXLEN / MINID on peer inbox
	err := AssertAllowed("agent-1", "XADD", "agent-2:inbox", "MAXLEN", "10", "*", "field", "val")
	if !errors.Is(err, ErrDestructiveAction) {
		t.Errorf("expected ErrDestructiveAction for XADD MAXLEN on peer inbox, got: %v", err)
	}

	errMinID := AssertAllowed("agent-1", "XADD", "agent-2:inbox", "MINID", "1000", "*", "field", "val")
	if !errors.Is(errMinID, ErrDestructiveAction) {
		t.Errorf("expected ErrDestructiveAction for XADD MINID on peer inbox, got: %v", errMinID)
	}

	// Owner IS allowed to manage their own inbox or outbox stream
	if err := AssertAllowed("agent-1", "XADD", "agent-1:inbox", "MAXLEN", "1000", "*", "field", "val"); err != nil {
		t.Errorf("owner should be allowed to manage own inbox, got: %v", err)
	}

	// Empty args check for XADD
	if err := AssertAllowed("agent-1", "XADD"); err == nil {
		t.Errorf("expected error for XADD with no args, got nil")
	}
}
