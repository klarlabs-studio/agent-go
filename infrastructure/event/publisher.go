// Package event provides event publishing and replay infrastructure.
package event

import (
	"context"
	"sync"

	"go.klarlabs.de/agent/domain/event"
)

// Publisher publishes events to an event store.
type Publisher struct {
	store   event.Store
	buffer  []event.Event
	bufSize int
	mu      sync.Mutex
}

// PublisherOption configures the publisher.
type PublisherOption func(*Publisher)

// WithBufferSize sets the event buffer size.
func WithBufferSize(size int) PublisherOption {
	return func(p *Publisher) {
		p.bufSize = size
	}
}

// NewPublisher creates a new event publisher.
func NewPublisher(store event.Store, opts ...PublisherOption) *Publisher {
	p := &Publisher{
		store:   store,
		bufSize: 0, // No buffering by default
	}
	for _, opt := range opts {
		opt(p)
	}
	if p.bufSize > 0 {
		p.buffer = make([]event.Event, 0, p.bufSize)
	}
	return p
}

// Publish sends events to the event store.
func (p *Publisher) Publish(ctx context.Context, events ...event.Event) error {
	if len(events) == 0 {
		return nil
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// If no buffering, publish immediately
	if p.bufSize == 0 {
		return p.store.Append(ctx, events...)
	}

	// Buffer events
	p.buffer = append(p.buffer, events...)

	// Flush if buffer is full
	if len(p.buffer) >= p.bufSize {
		return p.flush(ctx)
	}

	return nil
}

// Flush writes all buffered events to the store.
func (p *Publisher) Flush(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.flush(ctx)
}

// flush writes buffered events to the store (must hold lock).
func (p *Publisher) flush(ctx context.Context) error {
	if len(p.buffer) == 0 {
		return nil
	}

	err := p.store.Append(ctx, p.buffer...)
	if err != nil {
		return err
	}

	p.buffer = p.buffer[:0]
	return nil
}

// Close flushes remaining events and releases resources.
func (p *Publisher) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if len(p.buffer) > 0 {
		// Best effort flush on close
		ctx := context.Background()
		_ = p.flush(ctx)
	}

	return nil
}

// Ensure Publisher implements event.Publisher
var _ event.Publisher = (*Publisher)(nil)
