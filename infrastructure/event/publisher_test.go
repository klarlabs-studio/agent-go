package event_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	domainevent "go.klarlabs.de/agent/domain/event"
	infraevent "go.klarlabs.de/agent/infrastructure/event"
)

// mockEventStore implements event.Store for testing.
type mockEventStore struct {
	events []domainevent.Event
	mu     sync.RWMutex
	err    error
}

func newMockEventStore() *mockEventStore {
	return &mockEventStore{events: make([]domainevent.Event, 0)}
}

func (s *mockEventStore) Append(_ context.Context, events ...domainevent.Event) error {
	if s.err != nil {
		return s.err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, events...)
	return nil
}

func (s *mockEventStore) LoadEvents(_ context.Context, _ string) ([]domainevent.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]domainevent.Event, len(s.events))
	copy(result, s.events)
	return result, nil
}

func (s *mockEventStore) LoadEventsFrom(_ context.Context, _ string, fromSeq uint64) ([]domainevent.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []domainevent.Event
	for _, e := range s.events {
		if e.Sequence >= fromSeq {
			result = append(result, e)
		}
	}
	return result, nil
}

func (s *mockEventStore) Subscribe(_ context.Context, _ string) (<-chan domainevent.Event, error) {
	ch := make(chan domainevent.Event)
	close(ch)
	return ch, nil
}

func (s *mockEventStore) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.events)
}

func (s *mockEventStore) SetError(err error) {
	s.err = err
}

func TestNewPublisher(t *testing.T) {
	t.Parallel()

	t.Run("creates publisher without options", func(t *testing.T) {
		t.Parallel()

		store := newMockEventStore()
		pub := infraevent.NewPublisher(store)

		if pub == nil {
			t.Fatal("NewPublisher() returned nil")
		}
	})

	t.Run("creates publisher with buffer size", func(t *testing.T) {
		t.Parallel()

		store := newMockEventStore()
		pub := infraevent.NewPublisher(store, infraevent.WithBufferSize(10))

		if pub == nil {
			t.Fatal("NewPublisher() with buffer size returned nil")
		}
	})
}

func TestWithBufferSize(t *testing.T) {
	t.Parallel()

	t.Run("creates buffer size option", func(t *testing.T) {
		t.Parallel()

		opt := infraevent.WithBufferSize(100)
		if opt == nil {
			t.Fatal("WithBufferSize() returned nil")
		}
	})
}

func TestPublisher_Publish(t *testing.T) {
	t.Parallel()

	t.Run("publishes events immediately without buffering", func(t *testing.T) {
		t.Parallel()

		store := newMockEventStore()
		pub := infraevent.NewPublisher(store)

		event := domainevent.Event{
			RunID:     "run-1",
			Type:      domainevent.TypeToolCalled,
			Timestamp: time.Now(),
		}

		err := pub.Publish(context.Background(), event)
		if err != nil {
			t.Fatalf("Publish() error = %v", err)
		}

		if store.Len() != 1 {
			t.Errorf("store has %d events, want 1", store.Len())
		}
	})

	t.Run("publishes empty events", func(t *testing.T) {
		t.Parallel()

		store := newMockEventStore()
		pub := infraevent.NewPublisher(store)

		err := pub.Publish(context.Background())
		if err != nil {
			t.Fatalf("Publish() error = %v", err)
		}

		if store.Len() != 0 {
			t.Errorf("store has %d events, want 0", store.Len())
		}
	})

	t.Run("buffers events until buffer size reached", func(t *testing.T) {
		t.Parallel()

		store := newMockEventStore()
		pub := infraevent.NewPublisher(store, infraevent.WithBufferSize(3))

		// Publish 2 events (below buffer size)
		for i := 0; i < 2; i++ {
			event := domainevent.Event{
				RunID:     "run-1",
				Type:      domainevent.TypeToolCalled,
				Timestamp: time.Now(),
			}
			err := pub.Publish(context.Background(), event)
			if err != nil {
				t.Fatalf("Publish() error = %v", err)
			}
		}

		// Store should be empty (events buffered)
		if store.Len() != 0 {
			t.Errorf("store has %d events before flush, want 0", store.Len())
		}

		// Publish one more to trigger flush
		event := domainevent.Event{
			RunID:     "run-1",
			Type:      domainevent.TypeToolCalled,
			Timestamp: time.Now(),
		}
		err := pub.Publish(context.Background(), event)
		if err != nil {
			t.Fatalf("Publish() error = %v", err)
		}

		// All events should be flushed
		if store.Len() != 3 {
			t.Errorf("store has %d events after flush, want 3", store.Len())
		}
	})

	t.Run("returns error from store", func(t *testing.T) {
		t.Parallel()

		store := newMockEventStore()
		store.SetError(errors.New("store error"))
		pub := infraevent.NewPublisher(store)

		event := domainevent.Event{
			RunID:     "run-1",
			Type:      domainevent.TypeToolCalled,
			Timestamp: time.Now(),
		}

		err := pub.Publish(context.Background(), event)
		if err == nil {
			t.Error("Publish() should return error")
		}
	})
}

func TestPublisher_Flush(t *testing.T) {
	t.Parallel()

	t.Run("flushes buffered events", func(t *testing.T) {
		t.Parallel()

		store := newMockEventStore()
		pub := infraevent.NewPublisher(store, infraevent.WithBufferSize(10))

		// Publish some events
		for i := 0; i < 3; i++ {
			event := domainevent.Event{
				RunID:     "run-1",
				Type:      domainevent.TypeToolCalled,
				Timestamp: time.Now(),
			}
			_ = pub.Publish(context.Background(), event)
		}

		// Store should be empty
		if store.Len() != 0 {
			t.Errorf("store has %d events before flush, want 0", store.Len())
		}

		// Flush
		err := pub.Flush(context.Background())
		if err != nil {
			t.Fatalf("Flush() error = %v", err)
		}

		// Events should be in store
		if store.Len() != 3 {
			t.Errorf("store has %d events after flush, want 3", store.Len())
		}
	})

	t.Run("flush with empty buffer", func(t *testing.T) {
		t.Parallel()

		store := newMockEventStore()
		pub := infraevent.NewPublisher(store, infraevent.WithBufferSize(10))

		err := pub.Flush(context.Background())
		if err != nil {
			t.Fatalf("Flush() error = %v", err)
		}

		if store.Len() != 0 {
			t.Errorf("store has %d events, want 0", store.Len())
		}
	})

	t.Run("returns error from store", func(t *testing.T) {
		t.Parallel()

		store := newMockEventStore()
		pub := infraevent.NewPublisher(store, infraevent.WithBufferSize(10))

		// Add an event to buffer
		event := domainevent.Event{
			RunID:     "run-1",
			Type:      domainevent.TypeToolCalled,
			Timestamp: time.Now(),
		}
		_ = pub.Publish(context.Background(), event)

		// Set error and flush
		store.SetError(errors.New("flush error"))

		err := pub.Flush(context.Background())
		if err == nil {
			t.Error("Flush() should return error")
		}
	})
}

func TestPublisher_Close(t *testing.T) {
	t.Parallel()

	t.Run("closes publisher with empty buffer", func(t *testing.T) {
		t.Parallel()

		store := newMockEventStore()
		pub := infraevent.NewPublisher(store)

		err := pub.Close()
		if err != nil {
			t.Fatalf("Close() error = %v", err)
		}
	})

	t.Run("flushes buffer on close", func(t *testing.T) {
		t.Parallel()

		store := newMockEventStore()
		pub := infraevent.NewPublisher(store, infraevent.WithBufferSize(10))

		// Add events to buffer
		for i := 0; i < 3; i++ {
			event := domainevent.Event{
				RunID:     "run-1",
				Type:      domainevent.TypeToolCalled,
				Timestamp: time.Now(),
			}
			_ = pub.Publish(context.Background(), event)
		}

		// Close should flush
		err := pub.Close()
		if err != nil {
			t.Fatalf("Close() error = %v", err)
		}

		// Events should be in store
		if store.Len() != 3 {
			t.Errorf("store has %d events after close, want 3", store.Len())
		}
	})
}
