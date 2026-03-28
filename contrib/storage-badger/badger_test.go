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

// --- Additional cache tests ---

// TestCache_SetAndOverwrite tests that Set overwrites existing values.
func TestCache_SetAndOverwrite(t *testing.T) {
	db := newTestDB(t)
	c := NewCache(db)
	ctx := context.Background()

	// Set initial value
	if err := c.Set(ctx, "key1", []byte("value1"), cache.SetOptions{}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Overwrite
	if err := c.Set(ctx, "key1", []byte("value2"), cache.SetOptions{}); err != nil {
		t.Fatalf("Set overwrite failed: %v", err)
	}

	val, found, err := c.Get(ctx, "key1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Error("expected key to be found")
	}
	if string(val) != "value2" {
		t.Errorf("expected %q, got %q", "value2", string(val))
	}
}

// TestCache_DeleteNonexistent tests deleting a key that does not exist.
func TestCache_DeleteNonexistent(t *testing.T) {
	db := newTestDB(t)
	c := NewCache(db)
	ctx := context.Background()

	// Deleting a non-existent key should not error in Badger
	err := c.Delete(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Delete of nonexistent key failed: %v", err)
	}
}

// TestCache_EmptyValue tests storing and retrieving empty byte slices.
func TestCache_EmptyValue(t *testing.T) {
	db := newTestDB(t)
	c := NewCache(db)
	ctx := context.Background()

	if err := c.Set(ctx, "empty", []byte{}, cache.SetOptions{}); err != nil {
		t.Fatalf("Set empty value failed: %v", err)
	}

	val, found, err := c.Get(ctx, "empty")
	if err != nil {
		t.Fatalf("Get empty value failed: %v", err)
	}
	if !found {
		t.Error("expected key to be found")
	}
	if len(val) != 0 {
		t.Errorf("expected empty value, got %d bytes", len(val))
	}
}

// TestCache_BinaryValue tests storing binary data.
func TestCache_BinaryValue(t *testing.T) {
	db := newTestDB(t)
	c := NewCache(db)
	ctx := context.Background()

	// Create a small binary value with all byte values
	binValue := make([]byte, 256)
	for i := range binValue {
		binValue[i] = byte(i)
	}

	if err := c.Set(ctx, "binary", binValue, cache.SetOptions{}); err != nil {
		t.Fatalf("Set binary value failed: %v", err)
	}

	val, found, err := c.Get(ctx, "binary")
	if err != nil {
		t.Fatalf("Get binary value failed: %v", err)
	}
	if !found {
		t.Error("expected key to be found")
	}
	if len(val) != len(binValue) {
		t.Errorf("expected %d bytes, got %d bytes", len(binValue), len(val))
	}
	for i := range val {
		if val[i] != binValue[i] {
			t.Errorf("byte mismatch at index %d: expected %d, got %d", i, binValue[i], val[i])
			break
		}
	}
}

// TestCache_PrefixIsolation verifies that cache keys use the "cache:" prefix
// so they are isolated from other key namespaces in BadgerDB.
func TestCache_PrefixIsolation(t *testing.T) {
	db := newTestDB(t)
	c := NewCache(db)
	ctx := context.Background()

	// Set a cache entry
	if err := c.Set(ctx, "key1", []byte("val1"), cache.SetOptions{}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Verify internal key has cache: prefix
	err := db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte("cache:key1"))
		return err
	})
	if err != nil {
		t.Errorf("expected cache:key1 to exist in badger, got %v", err)
	}

	// A non-prefixed key should not exist
	err = db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte("key1"))
		return err
	})
	if !errors.Is(err, badger.ErrKeyNotFound) {
		t.Errorf("expected raw key1 not to exist, got %v", err)
	}
}

// TestCache_MultipleKeys tests operations across multiple keys.
func TestCache_MultipleKeys(t *testing.T) {
	db := newTestDB(t)
	c := NewCache(db)
	ctx := context.Background()

	keys := []string{"alpha", "beta", "gamma", "delta"}
	for _, key := range keys {
		if err := c.Set(ctx, key, []byte("value-"+key), cache.SetOptions{}); err != nil {
			t.Fatalf("Set %s failed: %v", key, err)
		}
	}

	for _, key := range keys {
		val, found, err := c.Get(ctx, key)
		if err != nil {
			t.Fatalf("Get %s failed: %v", key, err)
		}
		if !found {
			t.Errorf("expected %s to be found", key)
		}
		expected := "value-" + key
		if string(val) != expected {
			t.Errorf("for key %s, expected %q, got %q", key, expected, string(val))
		}
	}
}

// TestCache_TTL_StillValid tests that a TTL entry is accessible before expiry.
func TestCache_TTL_StillValid(t *testing.T) {
	db := newTestDB(t)
	c := NewCache(db)
	ctx := context.Background()

	// Set with long TTL
	if err := c.Set(ctx, "long-ttl", []byte("value"), cache.SetOptions{TTL: 1 * time.Hour}); err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, found, err := c.Get(ctx, "long-ttl")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Error("expected key to be found before TTL expiry")
	}
	if string(val) != "value" {
		t.Errorf("expected %q, got %q", "value", string(val))
	}

	exists, err := c.Exists(ctx, "long-ttl")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("expected key to exist before TTL expiry")
	}
}

// TestCache_Close tests that Close closes the database without error.
func TestCache_Close(t *testing.T) {
	opts := badger.DefaultOptions("").WithInMemory(true)
	db, err := badger.Open(opts)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	c := NewCache(db)

	if err := c.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	// Note: We do not attempt operations after Close as BadgerDB may panic
	// on writes to a closed database. The Close call itself is the contract.
}

// --- Additional EventStore tests ---

// TestEventStore_AppendEmpty tests appending an empty events slice.
func TestEventStore_AppendEmpty(t *testing.T) {
	db := newTestDB(t)
	store := NewEventStore(db)
	ctx := context.Background()

	err := store.Append(ctx)
	if err != nil {
		t.Fatalf("Append empty failed: %v", err)
	}
}

// TestEventStore_LoadEvents_EmptyRun tests loading events for a non-existent run.
func TestEventStore_LoadEvents_EmptyRun(t *testing.T) {
	db := newTestDB(t)
	store := NewEventStore(db)
	ctx := context.Background()

	events, err := store.LoadEvents(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("LoadEvents failed: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events, got %d", len(events))
	}
}

// TestEventStore_MultipleRuns tests events across multiple runs.
func TestEventStore_MultipleRuns(t *testing.T) {
	db := newTestDB(t)
	store := NewEventStore(db)
	ctx := context.Background()

	// Append events for different runs
	for _, runID := range []string{"run-A", "run-B", "run-C"} {
		events := []event.Event{
			{
				RunID:     runID,
				Type:      "test.event",
				Timestamp: time.Now(),
				Payload:   json.RawMessage(`{"run":"` + runID + `"}`),
			},
		}
		if err := store.Append(ctx, events...); err != nil {
			t.Fatalf("Append for %s failed: %v", runID, err)
		}
	}

	// Each run should have exactly 1 event
	for _, runID := range []string{"run-A", "run-B", "run-C"} {
		events, err := store.LoadEvents(ctx, runID)
		if err != nil {
			t.Fatalf("LoadEvents for %s failed: %v", runID, err)
		}
		if len(events) != 1 {
			t.Errorf("expected 1 event for %s, got %d", runID, len(events))
		}
	}
}

// TestEventStore_SequenceIsMonotonic tests that sequences are monotonically increasing
// across multiple appends.
func TestEventStore_SequenceIsMonotonic(t *testing.T) {
	db := newTestDB(t)
	store := NewEventStore(db)
	ctx := context.Background()
	runID := "mono-run"

	// Append events in multiple batches
	for batch := 0; batch < 3; batch++ {
		events := []event.Event{
			{
				RunID:     runID,
				Type:      event.Type("batch.event"),
				Timestamp: time.Now(),
				Payload:   json.RawMessage(`{}`),
			},
			{
				RunID:     runID,
				Type:      event.Type("batch.event"),
				Timestamp: time.Now(),
				Payload:   json.RawMessage(`{}`),
			},
		}
		if err := store.Append(ctx, events...); err != nil {
			t.Fatalf("Append batch %d failed: %v", batch, err)
		}
	}

	loaded, err := store.LoadEvents(ctx, runID)
	if err != nil {
		t.Fatalf("LoadEvents failed: %v", err)
	}
	if len(loaded) != 6 {
		t.Fatalf("expected 6 events, got %d", len(loaded))
	}

	for i, e := range loaded {
		expected := uint64(i + 1)
		if e.Sequence != expected {
			t.Errorf("expected sequence %d, got %d", expected, e.Sequence)
		}
	}
}

// TestEventStore_VersionDefault tests that version defaults to 1 when not set.
func TestEventStore_VersionDefault(t *testing.T) {
	db := newTestDB(t)
	store := NewEventStore(db)
	ctx := context.Background()

	events := []event.Event{
		{
			RunID:     "test-run-ver",
			Type:      "test.event",
			Timestamp: time.Now(),
			Payload:   json.RawMessage(`{}`),
			// Version is 0 (zero value)
		},
	}

	if err := store.Append(ctx, events...); err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	loaded, err := store.LoadEvents(ctx, "test-run-ver")
	if err != nil {
		t.Fatalf("LoadEvents failed: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("expected 1 event, got %d", len(loaded))
	}
	if loaded[0].Version != 1 {
		t.Errorf("expected default version 1, got %d", loaded[0].Version)
	}
}

// TestEventStore_Subscribe_MultipleSubscribers tests multiple subscribers for the same run.
func TestEventStore_Subscribe_MultipleSubscribers(t *testing.T) {
	db := newTestDB(t)
	store := NewEventStore(db)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	runID := "multi-sub-run"

	ch1, err := store.Subscribe(ctx, runID)
	if err != nil {
		t.Fatalf("Subscribe 1 failed: %v", err)
	}

	ch2, err := store.Subscribe(ctx, runID)
	if err != nil {
		t.Fatalf("Subscribe 2 failed: %v", err)
	}

	// Append an event
	go func() {
		time.Sleep(50 * time.Millisecond)
		events := []event.Event{
			{
				RunID:     runID,
				Type:      "test.event",
				Timestamp: time.Now(),
				Payload:   json.RawMessage(`{"multi":true}`),
			},
		}
		store.Append(context.Background(), events...)
	}()

	// Both subscribers should receive the event
	select {
	case e := <-ch1:
		if e.Type != "test.event" {
			t.Errorf("ch1: expected test.event, got %s", e.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("ch1: timeout waiting for event")
	}

	select {
	case e := <-ch2:
		if e.Type != "test.event" {
			t.Errorf("ch2: expected test.event, got %s", e.Type)
		}
	case <-time.After(time.Second):
		t.Fatal("ch2: timeout waiting for event")
	}
}

// TestEventStore_Subscribe_Cancellation tests that cancelling context cleans up subscriber.
func TestEventStore_Subscribe_Cancellation(t *testing.T) {
	db := newTestDB(t)
	store := NewEventStore(db)

	ctx, cancel := context.WithCancel(context.Background())

	ch, err := store.Subscribe(ctx, "cancel-run")
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	cancel()
	time.Sleep(50 * time.Millisecond)

	// Channel should be closed
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed after context cancellation")
	}

	// Subscriber should be removed
	store.mu.RLock()
	subs := store.subscribers["cancel-run"]
	store.mu.RUnlock()

	if len(subs) != 0 {
		t.Errorf("expected 0 subscribers after cancel, got %d", len(subs))
	}
}

// TestEventStore_WrapError_Nil tests that nil errors pass through.
func TestEventStore_WrapError_Nil(t *testing.T) {
	store := &EventStore{}
	if err := store.wrapError(nil); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// TestEventStore_WrapError_DeadlineExceeded tests deadline wrapping.
func TestEventStore_WrapError_DeadlineExceeded(t *testing.T) {
	store := &EventStore{}
	err := store.wrapError(context.DeadlineExceeded)

	if !errors.Is(err, event.ErrOperationTimeout) {
		t.Errorf("expected ErrOperationTimeout, got %v", err)
	}
}

// TestEventStore_WrapError_GenericError tests generic error wrapping.
func TestEventStore_WrapError_GenericError(t *testing.T) {
	store := &EventStore{}
	original := errors.New("badger error")
	err := store.wrapError(original)

	if !errors.Is(err, event.ErrConnectionFailed) {
		t.Errorf("expected ErrConnectionFailed, got %v", err)
	}
	if !errors.Is(err, original) {
		t.Errorf("expected original error in chain")
	}
}

// TestCache_WrapError_Nil tests that nil errors pass through for cache.
func TestCache_WrapError_Nil(t *testing.T) {
	c := &Cache{}
	if err := c.wrapError(nil); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

// TestCache_WrapError_DeadlineExceeded tests deadline wrapping for cache.
func TestCache_WrapError_DeadlineExceeded(t *testing.T) {
	c := &Cache{}
	err := c.wrapError(context.DeadlineExceeded)

	if !errors.Is(err, cache.ErrOperationTimeout) {
		t.Errorf("expected ErrOperationTimeout, got %v", err)
	}
}

// TestCache_WrapError_GenericError tests generic error wrapping for cache.
func TestCache_WrapError_GenericError(t *testing.T) {
	c := &Cache{}
	original := errors.New("badger error")
	err := c.wrapError(original)

	if !errors.Is(err, cache.ErrConnectionFailed) {
		t.Errorf("expected ErrConnectionFailed, got %v", err)
	}
	if !errors.Is(err, original) {
		t.Errorf("expected original error in chain")
	}
}

// --- Interface compliance tests ---

func TestCache_ImplementsCacheInterface(t *testing.T) {
	var _ cache.Cache = (*Cache)(nil)
}

func TestEventStore_ImplementsEventStoreInterface(t *testing.T) {
	var _ event.Store = (*EventStore)(nil)
}

// TestEventStore_Close tests that Close is callable and returns nil on a healthy DB.
// We use a separate DB instance not managed by newTestDB to avoid double-close.
func TestEventStore_Close(t *testing.T) {
	opts := badger.DefaultOptions("").WithInMemory(true)
	db, err := badger.Open(opts)
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	store := NewEventStore(db)

	if err := store.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}
	// Do not use the store after Close to avoid BadgerDB panics.
}

// TestCache_PrefixKey tests the key prefixing logic.
func TestCache_PrefixKey(t *testing.T) {
	c := &Cache{}
	result := c.prefixKey("mykey")
	if result != "cache:mykey" {
		t.Errorf("expected %q, got %q", "cache:mykey", result)
	}
}

// TestEventStore_EventKey tests the event key generation.
func TestEventStore_EventKey(t *testing.T) {
	store := &EventStore{}

	key := store.eventKey("run-1", 42)
	expected := "events:run-1:0000000000000042"
	if key != expected {
		t.Errorf("expected %q, got %q", expected, key)
	}
}

// TestEventStore_EventPrefix tests the event prefix generation.
func TestEventStore_EventPrefix(t *testing.T) {
	store := &EventStore{}

	prefix := store.eventPrefix("run-1")
	expected := "events:run-1:"
	if prefix != expected {
		t.Errorf("expected %q, got %q", expected, prefix)
	}
}
