// generated from spec: examples/wallet/wallet.candy
// candy runtime: 0.1
// do not edit — regenerate from spec

package runtime

import (
	"context"
	"log/slog"
	"reflect"
	"sync"
)

// EventBus dispatches events eagerly to registered subscribers.
// Delivery is at-least-once (eager per spec). Subscribers must be idempotent.
type EventBus struct {
	mu   sync.RWMutex
	subs map[reflect.Type][]func(ctx context.Context, ev any)
}

func NewEventBus() *EventBus {
	return &EventBus{subs: make(map[reflect.Type][]func(ctx context.Context, ev any))}
}

// Subscribe registers a handler for the given event type.
func (b *EventBus) Subscribe(eventType reflect.Type, h func(ctx context.Context, ev any)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.subs[eventType] = append(b.subs[eventType], h)
}

// Publish dispatches ev to all registered handlers. Errors are logged; delivery continues.
func (b *EventBus) Publish(ctx context.Context, ev any) {
	t := reflect.TypeOf(ev)
	b.mu.RLock()
	hs := b.subs[t]
	b.mu.RUnlock()
	for _, h := range hs {
		func() {
			defer func() {
				if r := recover(); r != nil {
					slog.Error("eventbus: subscriber panic", "event", t.Name(), "panic", r)
				}
			}()
			h(ctx, ev)
		}()
	}
}
