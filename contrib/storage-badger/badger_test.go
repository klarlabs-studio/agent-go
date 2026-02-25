package badger

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/felixgeelhaar/agent-go/domain/cache"
	"github.com/felixgeelhaar/agent-go/domain/event"
)

// newTestDB creates an in-memory BadgerDB for testing.
func newTestDB(t *testing.T) *badger.DB {
	t.Helper()

	opts := badger.DefaultOptions("").WithInMemory(true)
	db, err := badger.Open(opts)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("failed to close test database: %v", err)
		}
	})

	return db
}

// TestCache_Get tests cache retrieval.
func TestCache_Get(t *testing.T) {
	db := newTestDB(t)
	c := NewCache(db)
	ctx := context.Background()

	// Test non-existent key
	val, found, err := c.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if found {
		t.Error("expected key not found")
	}
	if val != nil {
		t.Error("expected nil value")
	}

	// Set a value
	testData := []byte("test value")
	if err := c.Set(ctx, "key1", testData, cache.SetOptions{}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get the value
	val, found, err = c.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Error("expected key to be found")
	}
	if string(val) != string(testData) {
		t.Errorf("expected %q, got %q", testData, val)
	}
}

// TestCache_Set tests cache storage.
func TestCache_Set(t *testing.T) {
	db := newTestDB(t)
	c := NewCache(db)
	ctx := context.Background()

	// Test setting a value
	testData := []byte("test value")
	if err := c.Set(ctx, "key1", testData, cache.SetOptions{}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify it was stored
	val, found, err := c.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Error("expected key to be found")
	}
	if string(val) != string(testData) {
		t.Errorf("expected %q, got %q", testData, val)
	}

	// Test empty key
	err = c.Set(ctx, "", testData, cache.SetOptions{})
	if err != cache.ErrInvalidKey {
		t.Errorf("expected ErrInvalidKey, got %v", err)
	}
}

// TestCache_TTL tests cache TTL expiry.
func TestCache_TTL(t *testing.T) {
	db := newTestDB(t)
	c := NewCache(db)
	ctx := context.Background()

	testData := []byte("test value")
	ttl := 1 * time.Second

	// Set with TTL
	if err := c.Set(ctx, "key1", testData, cache.SetOptions{TTL: ttl}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Should exist immediately
	val, found, err := c.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Error("expected key to exist immediately after set")
	}
	if string(val) != string(testData) {
		t.Errorf("expected %q, got %q", testData, val)
	}

	// Wait for expiry with enough buffer for BadgerDB GC
	time.Sleep(ttl + 500*time.Millisecond)

	// Trigger garbage collection to clean up expired entries
	if err := db.RunValueLogGC(0.5); err != nil && err != badger.ErrNoRewrite {
		t.Logf("GC warning (non-fatal): %v", err)
	}

	// Should not exist after expiry
	val, found, err = c.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if found {
		t.Error("expected key to be expired")
	}
	if val != nil {
		t.Error("expected nil value after expiry")
	}
}

// TestCache_Delete tests cache deletion.
func TestCache_Delete(t *testing.T) {
	db := newTestDB(t)
	c := NewCache(db)
	ctx := context.Background()

	testData := []byte("test value")

	// Set a value
	if err := c.Set(ctx, "key1", testData, cache.SetOptions{}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify it exists
	found, err := c.Exists(ctx, "key1")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !found {
		t.Error("expected key to exist")
	}

	// Delete it
	if err := c.Delete(ctx, "key1"); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Verify it's gone
	found, err = c.Exists(ctx, "key1")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if found {
		t.Error("expected key to be deleted")
	}
}

// TestCache_Exists tests cache existence check.
func TestCache_Exists(t *testing.T) {
	db := newTestDB(t)
	c := NewCache(db)
	ctx := context.Background()

	// Test non-existent key
	found, err := c.Exists(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if found {
		t.Error("expected key not to exist")
	}

	// Set a value
	testData := []byte("test value")
	if err := c.Set(ctx, "key1", testData, cache.SetOptions{}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Check existence
	found, err = c.Exists(ctx, "key1")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !found {
		t.Error("expected key to exist")
	}
}

// TestCache_Clear tests clearing all cache entries.
func TestCache_Clear(t *testing.T) {
	db := newTestDB(t)
	c := NewCache(db)
	ctx := context.Background()

	// Set multiple values
	for i := 0; i < 10; i++ {
		key := "key" + string(rune('0'+i))
		if err := c.Set(ctx, key, []byte("value"), cache.SetOptions{}); err != nil {
			t.Fatalf("Set failed: %v", err)
		}
	}

	// Clear all
	if err := c.Clear(ctx); err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// Verify all keys are gone
	for i := 0; i < 10; i++ {
		key := "key" + string(rune('0'+i))
		found, err := c.Exists(ctx, key)
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if found {
			t.Errorf("expected key %s to be cleared", key)
		}
	}
}

// TestEventStore_Append tests event appending.
func TestEventStore_Append(t *testing.T) {
	db := newTestDB(t)
	store := NewEventStore(db)
	ctx := context.Background()

	runID := "test-run"

	// Create test events
	events := []event.Event{
		{
			RunID:     runID,
			Type:      "test.event1",
			Timestamp: time.Now(),
			Payload:   json.RawMessage(`{"message":"first"}`),
		},
		{
			RunID:     runID,
			Type:      "test.event2",
			Timestamp: time.Now(),
			Payload:   json.RawMessage(`{"message":"second"}`),
		},
	}

	// Append events
	if err := store.Append(ctx, events...); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Verify events were stored
	loaded, err := store.LoadEvents(ctx, runID)
	if err != nil {
		t.Fatalf("LoadEvents failed: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("expected 2 events, got %d", len(loaded))
	}

	// Check sequence numbers
	if loaded[0].Sequence != 1 {
		t.Errorf("expected sequence 1, got %d", loaded[0].Sequence)
	}
	if loaded[1].Sequence != 2 {
		t.Errorf("expected sequence 2, got %d", loaded[1].Sequence)
	}

	// Check IDs were assigned
	if loaded[0].ID == "" {
		t.Error("expected ID to be assigned")
	}
	if loaded[1].ID == "" {
		t.Error("expected ID to be assigned")
	}
}

// TestEventStore_LoadEvents tests event loading.
func TestEventStore_LoadEvents(t *testing.T) {
	db := newTestDB(t)
	store := NewEventStore(db)
	ctx := context.Background()

	runID := "test-run"

	// Create test events with different types
	events := []event.Event{
		{
			RunID:     runID,
			Type:      "test.event1",
			Timestamp: time.Now(),
			Payload:   json.RawMessage(`{"message":"first"}`),
		},
		{
			RunID:     runID,
			Type:      "test.event2",
			Timestamp: time.Now(),
			Payload:   json.RawMessage(`{"message":"second"}`),
		},
		{
			RunID:     runID,
			Type:      "test.event3",
			Timestamp: time.Now(),
			Payload:   json.RawMessage(`{"message":"third"}`),
		},
	}

	// Append events
	if err := store.Append(ctx, events...); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Load all events
	loaded, err := store.LoadEvents(ctx, runID)
	if err != nil {
		t.Fatalf("LoadEvents failed: %v", err)
	}

	if len(loaded) != 3 {
		t.Fatalf("expected 3 events, got %d", len(loaded))
	}

	// Verify order
	if loaded[0].Type != "test.event1" {
		t.Errorf("expected first event type test.event1, got %s", loaded[0].Type)
	}
	if loaded[2].Type != "test.event3" {
		t.Errorf("expected third event type test.event3, got %s", loaded[2].Type)
	}
}

// TestEventStore_LoadEventsFrom tests loading events from a sequence number.
func TestEventStore_LoadEventsFrom(t *testing.T) {
	db := newTestDB(t)
	store := NewEventStore(db)
	ctx := context.Background()

	runID := "test-run"

	// Create test events
	events := []event.Event{
		{
			RunID:     runID,
			Type:      "test.event1",
			Timestamp: time.Now(),
			Payload:   json.RawMessage(`{"message":"first"}`),
		},
		{
			RunID:     runID,
			Type:      "test.event2",
			Timestamp: time.Now(),
			Payload:   json.RawMessage(`{"message":"second"}`),
		},
		{
			RunID:     runID,
			Type:      "test.event3",
			Timestamp: time.Now(),
			Payload:   json.RawMessage(`{"message":"third"}`),
		},
	}

	// Append events
	if err := store.Append(ctx, events...); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Load from sequence 2
	loaded, err := store.LoadEventsFrom(ctx, runID, 2)
	if err != nil {
		t.Fatalf("LoadEventsFrom failed: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("expected 2 events, got %d", len(loaded))
	}

	// Verify correct events
	if loaded[0].Sequence != 2 {
		t.Errorf("expected sequence 2, got %d", loaded[0].Sequence)
	}
	if loaded[1].Sequence != 3 {
		t.Errorf("expected sequence 3, got %d", loaded[1].Sequence)
	}
}

// TestEventStore_Subscribe tests event subscription.
func TestEventStore_Subscribe(t *testing.T) {
	db := newTestDB(t)
	store := NewEventStore(db)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	runID := "test-run"

	// Subscribe to events
	ch, err := store.Subscribe(ctx, runID)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Create and append events in a goroutine
	go func() {
		time.Sleep(50 * time.Millisecond)
		events := []event.Event{
			{
				RunID:     runID,
				Type:      "test.event1",
				Timestamp: time.Now(),
				Payload:   json.RawMessage(`{"message":"first"}`),
			},
			{
				RunID:     runID,
				Type:      "test.event2",
				Timestamp: time.Now(),
				Payload:   json.RawMessage(`{"message":"second"}`),
			},
		}
		if err := store.Append(context.Background(), events...); err != nil {
			t.Errorf("Append failed: %v", err)
		}
	}()

	// Receive events from subscription
	var received []event.Event
	timeout := time.After(1 * time.Second)

receiveLoop:
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				break receiveLoop
			}
			received = append(received, e)
			if len(received) == 2 {
				cancel() // Cancel context to close subscription
				break receiveLoop
			}
		case <-timeout:
			t.Fatal("timeout waiting for events")
		}
	}

	// Verify received events
	if len(received) != 2 {
		t.Fatalf("expected 2 events, got %d", len(received))
	}

	if received[0].Type != "test.event1" {
		t.Errorf("expected first event type test.event1, got %s", received[0].Type)
	}
	if received[1].Type != "test.event2" {
		t.Errorf("expected second event type test.event2, got %s", received[1].Type)
	}
}

// TestEventStore_InvalidEvent tests validation of invalid events.
func TestEventStore_InvalidEvent(t *testing.T) {
	db := newTestDB(t)
	store := NewEventStore(db)
	ctx := context.Background()

	// Test event with no type
	events := []event.Event{
		{
			RunID:     "test-run",
			Type:      "", // Invalid: empty type
			Timestamp: time.Now(),
			Payload:   json.RawMessage(`{}`),
		},
	}

	err := store.Append(ctx, events...)
	if !errors.Is(err, event.ErrInvalidEvent) {
		t.Errorf("expected error chain to contain ErrInvalidEvent, got %v", err)
	}
}

// TestCache_ContextCancellation tests context cancellation handling.
func TestCache_ContextCancellation(t *testing.T) {
	db := newTestDB(t)
	c := NewCache(db)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// All operations should respect context cancellation
	_, _, err := c.Get(ctx, "key")
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	err = c.Set(ctx, "key", []byte("value"), cache.SetOptions{})
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	err = c.Delete(ctx, "key")
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	_, err = c.Exists(ctx, "key")
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	err = c.Clear(ctx)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

// TestEventStore_ContextCancellation tests context cancellation for event store.
func TestEventStore_ContextCancellation(t *testing.T) {
	db := newTestDB(t)
	store := NewEventStore(db)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	// All operations should respect context cancellation
	err := store.Append(ctx, event.Event{
		RunID: "test",
		Type:  "test.event",
	})
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	_, err = store.LoadEvents(ctx, "test")
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	_, err = store.LoadEventsFrom(ctx, "test", 1)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}

	_, err = store.Subscribe(ctx, "test")
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
