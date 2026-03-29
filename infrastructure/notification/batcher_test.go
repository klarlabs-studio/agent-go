package notification

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/notification"
)

func newTestEvent(runID string) *notification.Event {
	event, _ := notification.NewEvent("evt-1", notification.EventRunStarted, runID, notification.RunStartedPayload{
		Goal: "test goal",
	})
	return event
}

func TestBatcher_Add(t *testing.T) {
	var batches [][]*notification.Event
	var mu sync.Mutex

	config := BatcherConfig{
		MaxBatchSize: 3,
		MaxWait:      1 * time.Hour, // Long wait so only size triggers flush
		OnBatch: func(ctx context.Context, events []*notification.Event) error {
			mu.Lock()
			defer mu.Unlock()
			eventsCopy := make([]*notification.Event, len(events))
			copy(eventsCopy, events)
			batches = append(batches, eventsCopy)
			return nil
		},
	}

	batcher := NewBatcher(config)
	ctx := context.Background()

	// Add events below batch size
	for i := 0; i < 2; i++ {
		event := newTestEvent("run-1")
		if err := batcher.Add(ctx, event); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
	}

	// Should not have flushed yet
	mu.Lock()
	if len(batches) != 0 {
		t.Errorf("should not flush before batch size reached, got %d batches", len(batches))
	}
	mu.Unlock()

	// Add one more to trigger flush
	event := newTestEvent("run-1")
	if err := batcher.Add(ctx, event); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// Should have flushed
	mu.Lock()
	if len(batches) != 1 {
		t.Errorf("should have 1 batch, got %d", len(batches))
	}
	if len(batches[0]) != 3 {
		t.Errorf("batch should have 3 events, got %d", len(batches[0]))
	}
	mu.Unlock()
}

func TestBatcher_MaxWait(t *testing.T) {
	var batches [][]*notification.Event
	var mu sync.Mutex

	config := BatcherConfig{
		MaxBatchSize: 100,
		MaxWait:      50 * time.Millisecond, // Short wait for testing
		OnBatch: func(ctx context.Context, events []*notification.Event) error {
			mu.Lock()
			defer mu.Unlock()
			eventsCopy := make([]*notification.Event, len(events))
			copy(eventsCopy, events)
			batches = append(batches, eventsCopy)
			return nil
		},
	}

	batcher := NewBatcher(config)
	ctx := context.Background()

	// Add single event
	event := newTestEvent("run-1")
	if err := batcher.Add(ctx, event); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// Wait for timer to trigger
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if len(batches) != 1 {
		t.Errorf("should have 1 batch after max wait, got %d", len(batches))
	}
	if len(batches) > 0 && len(batches[0]) != 1 {
		t.Errorf("batch should have 1 event, got %d", len(batches[0]))
	}
	mu.Unlock()
}

func TestBatcher_Flush(t *testing.T) {
	var batches [][]*notification.Event
	var mu sync.Mutex

	config := BatcherConfig{
		MaxBatchSize: 100,
		MaxWait:      1 * time.Hour,
		OnBatch: func(ctx context.Context, events []*notification.Event) error {
			mu.Lock()
			defer mu.Unlock()
			eventsCopy := make([]*notification.Event, len(events))
			copy(eventsCopy, events)
			batches = append(batches, eventsCopy)
			return nil
		},
	}

	batcher := NewBatcher(config)
	ctx := context.Background()

	// Add events
	for i := 0; i < 5; i++ {
		event := newTestEvent("run-1")
		if err := batcher.Add(ctx, event); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
	}

	// Manual flush
	if err := batcher.Flush(ctx); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	mu.Lock()
	if len(batches) != 1 {
		t.Errorf("should have 1 batch, got %d", len(batches))
	}
	if len(batches[0]) != 5 {
		t.Errorf("batch should have 5 events, got %d", len(batches[0]))
	}
	mu.Unlock()
}

func TestBatcher_FlushEmpty(t *testing.T) {
	var callCount int32

	config := BatcherConfig{
		MaxBatchSize: 100,
		MaxWait:      1 * time.Hour,
		OnBatch: func(ctx context.Context, events []*notification.Event) error {
			atomic.AddInt32(&callCount, 1)
			return nil
		},
	}

	batcher := NewBatcher(config)
	ctx := context.Background()

	// Flush without events
	if err := batcher.Flush(ctx); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	if atomic.LoadInt32(&callCount) != 0 {
		t.Error("OnBatch should not be called for empty flush")
	}
}

func TestBatcher_Close(t *testing.T) {
	var batches [][]*notification.Event
	var mu sync.Mutex

	config := BatcherConfig{
		MaxBatchSize: 100,
		MaxWait:      1 * time.Hour,
		OnBatch: func(ctx context.Context, events []*notification.Event) error {
			mu.Lock()
			defer mu.Unlock()
			eventsCopy := make([]*notification.Event, len(events))
			copy(eventsCopy, events)
			batches = append(batches, eventsCopy)
			return nil
		},
	}

	batcher := NewBatcher(config)
	ctx := context.Background()

	// Add events
	for i := 0; i < 5; i++ {
		event := newTestEvent("run-1")
		if err := batcher.Add(ctx, event); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
	}

	// Close should flush
	if err := batcher.Close(ctx); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	mu.Lock()
	if len(batches) != 1 {
		t.Errorf("should have 1 batch after close, got %d", len(batches))
	}
	mu.Unlock()

	// Add after close should fail
	event := newTestEvent("run-1")
	err := batcher.Add(ctx, event)
	if err != notification.ErrNotifierClosed {
		t.Errorf("Add() after close should return ErrNotifierClosed, got %v", err)
	}
}

func TestBatcher_PendingCount(t *testing.T) {
	config := BatcherConfig{
		MaxBatchSize: 100,
		MaxWait:      1 * time.Hour,
		OnBatch: func(ctx context.Context, events []*notification.Event) error {
			return nil
		},
	}

	batcher := NewBatcher(config)
	ctx := context.Background()

	if batcher.PendingCount() != 0 {
		t.Errorf("PendingCount() should be 0, got %d", batcher.PendingCount())
	}

	// Add events
	for i := 0; i < 5; i++ {
		event := newTestEvent("run-1")
		if err := batcher.Add(ctx, event); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
	}

	if batcher.PendingCount() != 5 {
		t.Errorf("PendingCount() should be 5, got %d", batcher.PendingCount())
	}

	// Flush
	if err := batcher.Flush(ctx); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}

	if batcher.PendingCount() != 0 {
		t.Errorf("PendingCount() after flush should be 0, got %d", batcher.PendingCount())
	}
}

func TestBatcher_DefaultConfig(t *testing.T) {
	config := DefaultBatcherConfig()

	if config.MaxBatchSize != 100 {
		t.Errorf("MaxBatchSize = %d, want 100", config.MaxBatchSize)
	}
	if config.MaxWait != 5*time.Second {
		t.Errorf("MaxWait = %v, want 5s", config.MaxWait)
	}
}

func TestBatcher_ConfigDefaults(t *testing.T) {
	// Test that invalid config values get corrected
	config := BatcherConfig{
		MaxBatchSize: 0, // Invalid
		MaxWait:      0, // Invalid
		OnBatch:      nil,
	}

	batcher := NewBatcher(config)

	// Verify defaults were applied by checking internal state
	// Add 100 events - should trigger flush at default MaxBatchSize
	ctx := context.Background()
	for i := 0; i < 99; i++ {
		event := newTestEvent("run-1")
		if err := batcher.Add(ctx, event); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
	}

	// Should have 99 pending
	if batcher.PendingCount() != 99 {
		t.Errorf("PendingCount() should be 99, got %d", batcher.PendingCount())
	}

	// Add one more to trigger flush at default 100
	event := newTestEvent("run-1")
	if err := batcher.Add(ctx, event); err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	// Should have 0 pending after flush (no OnBatch, so events are just cleared)
	if batcher.PendingCount() != 0 {
		t.Errorf("PendingCount() after flush should be 0, got %d", batcher.PendingCount())
	}
}
