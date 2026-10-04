package messaging

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"malus-be/internal/kernel"
)

type Handler func(ctx context.Context, env Envelope) error

type MemoryBus struct {
	source   string
	mu       sync.RWMutex
	handlers []Handler
}

func NewMemoryBus(source string) *MemoryBus {
	return &MemoryBus{source: source}
}

func (b *MemoryBus) Subscribe(h Handler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers = append(b.handlers, h)
}

func (b *MemoryBus) Publish(ctx context.Context, events ...kernel.Event) error {
	b.mu.RLock()
	handlers := append([]Handler(nil), b.handlers...)
	b.mu.RUnlock()

	var errs []error
	for _, e := range events {
		env, err := NewEnvelope(b.source, e)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, h := range handlers {
			if err := h(ctx, env); err != nil {
				errs = append(errs, err)
			}
		}
	}
	return errors.Join(errs...)
}

func LogHandler(log *slog.Logger) Handler {
	return func(ctx context.Context, env Envelope) error {
		log.InfoContext(ctx, "event published", "type", env.Type, "subject", env.Subject, "event_id", env.ID)
		return nil
	}
}
