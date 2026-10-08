// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package runtime

import (
	"fmt"
	"time"
)

// PriorityLevel defines the urgency level for runtime events and messages.
type PriorityLevel int

const (
	// P0_Control represents emergency directives (halt, cancel, kill).
	P0_Control PriorityLevel = iota
	// P1_HighPriority represents direct user directives or task pivots.
	P1_HighPriority
	// P2_StandardAsync represents peer updates, chatter, or background notices.
	P2_StandardAsync
	// P3_TelemetryQuery represents fast telemetry, ping, or status queries.
	P3_TelemetryQuery
)

// String returns human-readable name of PriorityLevel.
func (p PriorityLevel) String() string {
	switch p {
	case P0_Control:
		return "P0_Control"
	case P1_HighPriority:
		return "P1_HighPriority"
	case P2_StandardAsync:
		return "P2_StandardAsync"
	case P3_TelemetryQuery:
		return "P3_TelemetryQuery"
	default:
		return fmt.Sprintf("PriorityLevel(%d)", p)
	}
}

// Event represents a discrete unit of work or notification across subsystems.
type Event struct {
	ID        string        `json:"id"`
	Priority  PriorityLevel `json:"priority"`
	Source    string        `json:"source"` // e.g., "valkey:inbox", "watcher:sonar"
	Target    string        `json:"target"` // e.g., "repl-main", "state-store", "*"
	Payload   any           `json:"payload"`
	Timestamp time.Time     `json:"timestamp"`
}

// BackplaneEnvelope represents an authenticated cross-agent message over the backplane.
type BackplaneEnvelope struct {
	ID        string    `json:"id"`
	Sender    string    `json:"sender"`
	Recipient string    `json:"recipient"`
	Payload   string    `json:"payload"`
	Signature string    `json:"signature"`
	Timestamp time.Time `json:"timestamp"`
}
