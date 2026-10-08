// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package libbp

import (
	"context"
	"testing"
	"time"
)

func TestPollIntervalProtocolConstant(t *testing.T) {
	if KeyPollInterval != "poll-interval" {
		t.Fatalf("expected KeyPollInterval to be 'poll-interval', got %q", KeyPollInterval)
	}
}

func TestGuardAllowsPollInterval(t *testing.T) {
	callers := []string{"agent-1", "agent-2", "operator", "arbitrary-caller", "root"}

	for _, caller := range callers {
		// GET poll-interval
		if err := AssertAllowed(caller, "GET", KeyPollInterval); err != nil {
			t.Errorf("AssertAllowed failed for %s GET %s: %v", caller, KeyPollInterval, err)
		}
		if err := AssertAllowed(caller, "get", "poll-interval"); err != nil {
			t.Errorf("AssertAllowed case-insensitive failed for %s get poll-interval: %v", caller, err)
		}

		// SET poll-interval
		if err := AssertAllowed(caller, "SET", KeyPollInterval, "120"); err != nil {
			t.Errorf("AssertAllowed failed for %s SET %s: %v", caller, KeyPollInterval, err)
		}
		if err := AssertAllowed(caller, "set", "poll-interval", "300"); err != nil {
			t.Errorf("AssertAllowed case-insensitive failed for %s set poll-interval: %v", caller, err)
		}
	}
}

func TestClientPollInterval(t *testing.T) {
	srv, port := startMockValkeyServer(t)
	defer srv.close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg := ClientConfig{
		Host:    "127.0.0.1",
		Port:    port,
		AgentID: "agent-1",
	}

	client, err := Dial(ctx, cfg)
	if err != nil {
		t.Fatalf("Dial failed: %v", err)
	}
	defer client.Close()

	// 1. Unset: returns default 60
	interval, err := client.GetPollInterval(ctx)
	if err != nil {
		t.Fatalf("GetPollInterval on unset key returned error: %v", err)
	}
	if interval != 60 {
		t.Errorf("expected default 60 when unset, got %d", interval)
	}

	// 2. Set valid values: 5, 120, 3600
	validValues := []int{5, 30, 60, 120, 3600}
	for _, v := range validValues {
		if err := client.SetPollInterval(ctx, v); err != nil {
			t.Errorf("SetPollInterval(%d) failed: %v", v, err)
		}
		got, err := client.GetPollInterval(ctx)
		if err != nil {
			t.Errorf("GetPollInterval() after setting %d failed: %v", v, err)
		}
		if got != v {
			t.Errorf("expected %d, got %d", v, got)
		}
	}

	// 3. Set invalid values: < 5 or > 3600
	invalidValues := []int{-10, 0, 1, 4, 3601, 7200}
	for _, iv := range invalidValues {
		if err := client.SetPollInterval(ctx, iv); err == nil {
			t.Errorf("expected error when setting invalid poll interval %d, got nil", iv)
		}
	}

	// 4. Stored invalid value in Valkey: non-numeric string -> fallback to 60
	if _, err := client.Exec(ctx, "SET", KeyPollInterval, "invalid-not-a-number"); err != nil {
		t.Fatalf("exec SET failed: %v", err)
	}
	got, err := client.GetPollInterval(ctx)
	if err != nil {
		t.Fatalf("GetPollInterval on non-numeric key returned error: %v", err)
	}
	if got != 60 {
		t.Errorf("expected fallback 60 for non-numeric value, got %d", got)
	}

	// 5. Stored invalid value in Valkey: out of range (e.g. 2, 5000) -> fallback to 60
	if _, err := client.Exec(ctx, "SET", KeyPollInterval, "2"); err != nil {
		t.Fatalf("exec SET failed: %v", err)
	}
	got, err = client.GetPollInterval(ctx)
	if err != nil {
		t.Fatalf("GetPollInterval on out of range key returned error: %v", err)
	}
	if got != 60 {
		t.Errorf("expected fallback 60 for value 2, got %d", got)
	}

	if _, err := client.Exec(ctx, "SET", KeyPollInterval, "5000"); err != nil {
		t.Fatalf("exec SET failed: %v", err)
	}
	got, err = client.GetPollInterval(ctx)
	if err != nil {
		t.Fatalf("GetPollInterval on out of range key returned error: %v", err)
	}
	if got != 60 {
		t.Errorf("expected fallback 60 for value 5000, got %d", got)
	}

	// 6. Closed client behavior
	cClosed := &Client{closed: true, cfg: cfg}
	gotClosed, err := cClosed.GetPollInterval(ctx)
	if err == nil {
		t.Errorf("expected error on GetPollInterval with closed client")
	}
	if gotClosed != 60 {
		t.Errorf("expected default 60 on error with closed client, got %d", gotClosed)
	}

	if err := cClosed.SetPollInterval(ctx, 30); err == nil {
		t.Errorf("expected error on SetPollInterval with closed client")
	}
}
