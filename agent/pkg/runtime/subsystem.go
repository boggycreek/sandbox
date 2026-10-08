// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package runtime

import "context"

// Subsystem defines a concurrent worker component managed by the Supervisor.
type Subsystem interface {
	// ID returns the unique name of the subsystem (e.g., "gateway-valkey", "repl-main").
	ID() string

	// Start executes the worker loop. It must run until ctx is canceled or an unrecoverable error occurs.
	Start(ctx context.Context, bus *EventBus, state *SharedState) error
}

// ResilientSubsystem is an optional interface for subsystems that specify custom restart policies.
type ResilientSubsystem interface {
	Subsystem
	// IsResilient returns true if the subsystem should be auto-restarted on panic or transient error.
	IsResilient() bool
}
