package notification

import (
	"context"
	"sync"
	"time"

	"go.klarlabs.de/agent/domain/notification"
)

// BatcherConfig configures the event batcher.
type BatcherConfig struct {
	// MaxBatchSize is the maximum number of events per batch.
	MaxBatchSize int
	// MaxWait is the maximum time to wait before flushing a batch.
	MaxWait time.Duration
	// OnBatch is called when a batch is ready to send.
	OnBatch func(ctx context.Context, events []*notification.Event) error
}

// DefaultBatcherConfig returns a sensible default configuration.
func DefaultBatcherConfig() BatcherConfig {
	return BatcherConfig{
		MaxBatchSize: 100,
		MaxWait:      5 * time.Second,
	}
}

// Batcher accumulates events and flushes them in batches.
type Batcher struct {
	config   BatcherConfig
	events   []*notification.Event
	mu       sync.Mutex
	timer    *time.Timer
	closed   bool
	closedMu sync.RWMutex
}

// NewBatcher creates a new event batcher.
func NewBatcher(config BatcherConfig) *Batcher {
	if config.MaxBatchSize <= 0 {
		config.MaxBatchSize = 100
	}
	if config.MaxWait <= 0 {
		config.MaxWait = 5 * time.Second
	}

	return &Batcher{
		config: config,
		events: make([]*notification.Event, 0, config.MaxBatchSize),
	}
}

// Add adds an event to the batch.
// If the batch is full, it will be flushed immediately.
func (b *Batcher) Add(ctx context.Context, event *notification.Event) error {
	b.closedMu.RLock()
	if b.closed {
		b.closedMu.RUnlock()
		return notification.ErrNotifierClosed
	}
	b.closedMu.RUnlock()

	b.mu.Lock()
	defer b.mu.Unlock()

	b.events = append(b.events, event)

	// Start timer on first event
	if len(b.events) == 1 && b.timer == nil {
		b.timer = time.AfterFunc(b.config.MaxWait, func() {
			_ = b.Flush(ctx)
		})
	}

	// Flush if batch is full
	if len(b.events) >= b.config.MaxBatchSize {
		return b.flushLocked(ctx)
	}

	return nil
}

// Flush flushes any pending events immediately.
func (b *Batcher) Flush(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.flushLocked(ctx)
}

// flushLocked flushes events while holding the lock.
func (b *Batcher) flushLocked(ctx context.Context) error {
	if len(b.events) == 0 {
		return nil
	}

	// Stop timer
	if b.timer != nil {
		b.timer.Stop()
		b.timer = nil
	}

	// Copy events to send
	events := make([]*notification.Event, len(b.events))
	copy(events, b.events)
	b.events = b.events[:0]

	// Send batch
	if b.config.OnBatch != nil {
		return b.config.OnBatch(ctx, events)
	}

	return nil
}

// Close stops the batcher and flushes any remaining events.
func (b *Batcher) Close(ctx context.Context) error {
	b.closedMu.Lock()
	b.closed = true
	b.closedMu.Unlock()

	return b.Flush(ctx)
}

// PendingCount returns the number of events waiting to be flushed.
func (b *Batcher) PendingCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.events)
}
