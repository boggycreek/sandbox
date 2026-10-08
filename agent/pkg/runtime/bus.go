// Copyright (c) 2026 Boggy Creek Software LLC
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package runtime

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

var (
	// ErrBusClosed is returned when attempting operations on a closed EventBus.
	ErrBusClosed = errors.New("event bus is closed")
)

// EventBus is an in-memory priority-aware event dispatcher.
type EventBus struct {
	mu          sync.RWMutex
	p0          []Event
	p1          []Event
	p2          []Event
	p3          []Event
	subscribers map[string][]chan Event
	notify      chan struct{}
	closedChan  chan struct{}
	isClosed    bool
	seqCounter  atomic.Uint64
}

// NewEventBus instantiates a new thread-safe EventBus.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[string][]chan Event),
		notify:      make(chan struct{}, 1),
		closedChan:  make(chan struct{}),
	}
}

// Publish enqueues an event according to its priority and notifies subscribers.
func (b *EventBus) Publish(e Event) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.isClosed {
		return ErrBusClosed
	}

	if e.ID == "" {
		seq := b.seqCounter.Add(1)
		e.ID = fmt.Sprintf("evt-%d-%d", time.Now().UnixNano(), seq)
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}

	switch e.Priority {
	case P0_Control:
		b.p0 = append(b.p0, e)
	case P1_HighPriority:
		b.p1 = append(b.p1, e)
	case P2_StandardAsync:
		b.p2 = append(b.p2, e)
	case P3_TelemetryQuery:
		b.p3 = append(b.p3, e)
	default:
		b.p2 = append(b.p2, e)
	}

	// Dispatch to direct and wildcard subscribers
	b.dispatchToSubscribersLocked(e)

	// Notify waiting Next() callers
	select {
	case b.notify <- struct{}{}:
	default:
	}

	return nil
}

func (b *EventBus) dispatchToSubscribersLocked(e Event) {
	targets := []string{e.Target, "*"}
	for _, target := range targets {
		if chans, ok := b.subscribers[target]; ok {
			for _, ch := range chans {
				select {
				case ch <- e:
				default:
					// Non-blocking send: subscriber channel buffer full
				}
			}
		}
	}
}

// Poll checks for the highest-priority pending event non-blockingly.
func (b *EventBus) Poll() (Event, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.p0) > 0 {
		e := b.p0[0]
		b.p0 = b.p0[1:]
		return e, true
	}
	if len(b.p1) > 0 {
		e := b.p1[0]
		b.p1 = b.p1[1:]
		return e, true
	}
	if len(b.p2) > 0 {
		e := b.p2[0]
		b.p2 = b.p2[1:]
		return e, true
	}
	if len(b.p3) > 0 {
		e := b.p3[0]
		b.p3 = b.p3[1:]
		return e, true
	}

	return Event{}, false
}

// Next blocks until the highest-priority event is available or ctx is canceled.
func (b *EventBus) Next(ctx context.Context) (Event, error) {
	for {
		if e, ok := b.Poll(); ok {
			return e, nil
		}

		select {
		case <-ctx.Done():
			return Event{}, ctx.Err()
		case <-b.closedChan:
			// Drain remaining events if any
			if e, ok := b.Poll(); ok {
				return e, nil
			}
			return Event{}, ErrBusClosed
		case <-b.notify:
			// Loop back to poll with strict priority ordering
		}
	}
}

// Subscribe returns a channel receiving events matching the target (or "*") and an unsubscribe function.
func (b *EventBus) Subscribe(target string, bufferSize int) (<-chan Event, func()) {
	if bufferSize <= 0 {
		bufferSize = 64
	}
	ch := make(chan Event, bufferSize)

	b.mu.Lock()
	b.subscribers[target] = append(b.subscribers[target], ch)
	b.mu.Unlock()

	unsubscribe := func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		subs := b.subscribers[target]
		for i, subCh := range subs {
			if subCh == ch {
				b.subscribers[target] = append(subs[:i], subs[i+1:]...)
				close(ch)
				break
			}
		}
		if len(b.subscribers[target]) == 0 {
			delete(b.subscribers, target)
		}
	}

	return ch, unsubscribe
}

// QueueDepth returns the count of queued events by priority level.
func (b *EventBus) QueueDepth() map[PriorityLevel]int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return map[PriorityLevel]int{
		P0_Control:        len(b.p0),
		P1_HighPriority:   len(b.p1),
		P2_StandardAsync:  len(b.p2),
		P3_TelemetryQuery: len(b.p3),
	}
}

// TotalQueueDepth returns the total number of all pending events across all priorities.
func (b *EventBus) TotalQueueDepth() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.p0) + len(b.p1) + len(b.p2) + len(b.p3)
}

// Close closes the event bus, waking up all blocking consumers.
func (b *EventBus) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.isClosed {
		return
	}
	b.isClosed = true
	close(b.closedChan)

	// Close all subscriber channels
	for target, chans := range b.subscribers {
		for _, ch := range chans {
			close(ch)
		}
		delete(b.subscribers, target)
	}
}
