// generated from examples/auth/auth.candy
// candy runtime 0.1
// DO NOT EDIT — regenerate from spec

package runtime

import (
	"context"
	"log/slog"
	"reflect"
	"sync"
)

// EventBus is a minimal in-process event bus supporting delivery:eager semantics.
// At-least-once delivery within a single process; subscribers must be idempotent.
type EventBus struct {
	mu          sync.RWMutex
	subscribers map[reflect.Type][]func(ctx context.Context, ev any)
}

// NewEventBus constructs an EventBus.
func NewEventBus() *EventBus {
	return &EventBus{
		subscribers: make(map[reflect.Type][]func(ctx context.Context, ev any)),
	}
}

// Subscribe registers a handler for events of the given type.
func (b *EventBus) Subscribe(eventType reflect.Type, h func(ctx context.Context, ev any)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subscribers[eventType] = append(b.subscribers[eventType], h)
}

// Publish dispatches the event to all registered subscribers.
// delivery: eager — fires handlers in background goroutines (at-least-once).
func (b *EventBus) Publish(ctx context.Context, ev any) {
	b.mu.RLock()
	handlers := b.subscribers[reflect.TypeOf(ev)]
	b.mu.RUnlock()

	for _, h := range handlers {
		h := h
		go func() {
			if err := safeCall(ctx, h, ev); err != nil {
				slog.ErrorContext(ctx, "event handler error", "event", reflect.TypeOf(ev).Name(), "err", err)
			}
		}()
	}
}

func safeCall(ctx context.Context, h func(ctx context.Context, ev any), ev any) (err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "event handler panic", "panic", r)
		}
	}()
	h(ctx, ev)
	return nil
}
