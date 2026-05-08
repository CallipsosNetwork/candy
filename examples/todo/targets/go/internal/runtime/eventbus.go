// generated from examples/todo/todo.candy
// candy runtime 0.1
// do not edit — regenerate from spec

package runtime

import (
	"context"
	"log/slog"
	"reflect"
	"sync"
)

// EventBus is a simple in-process event bus with delivery: eager semantics.
// Subscribers must be idempotent (at-least-once delivery).
type EventBus struct {
	mu   sync.RWMutex
	subs map[reflect.Type][]func(context.Context, any)
}

func NewEventBus() *EventBus {
	return &EventBus{subs: make(map[reflect.Type][]func(context.Context, any))}
}

// Subscribe registers a handler for events of type T.
func (b *EventBus) Subscribe(eventType reflect.Type, h func(context.Context, any)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[eventType] = append(b.subs[eventType], h)
}

// Publish delivers ev to all subscribers. Delivery: eager — goroutine per subscriber.
func (b *EventBus) Publish(ctx context.Context, ev any) {
	b.mu.RLock()
	handlers := b.subs[reflect.TypeOf(ev)]
	b.mu.RUnlock()
	for _, h := range handlers {
		h := h
		go func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("event subscriber panicked", "event", reflect.TypeOf(ev).Name(), "panic", r)
				}
			}()
			h(ctx, ev)
		}()
	}
}
