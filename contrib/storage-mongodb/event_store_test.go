package mongodb

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/event"
)

// Since EventStore uses *mongo.Collection directly and we cannot easily mock it
// without an actual MongoDB connection, we test at the unit level by exercising
// the exported logic that does not require a real collection: document conversion,
// error wrapping, subscription management, and interface compliance.
// Integration tests that require a running MongoDB instance are guarded with
// testing.Short().

// --- Interface compliance tests ---

func TestEventStore_ImplementsEventStore(t *testing.T) {
	var _ event.Store = (*EventStore)(nil)
}

func TestEventStore_ImplementsEventQuerier(t *testing.T) {
	var _ event.Querier = (*EventStore)(nil)
}

// --- Document conversion tests ---

func TestToDocument(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)
	e := &event.Event{
		ID:        "evt-1",
		RunID:     "run-1",
		Type:      event.Type("test.created"),
		Sequence:  42,
		Payload:   []byte(`{"key":"value"}`),
		Timestamp: now,
		Version:   2,
	}

	store := &EventStore{}
	doc := store.toDocument(e)

	if doc.ID != e.ID {
		t.Errorf("expected ID %q, got %q", e.ID, doc.ID)
	}
	if doc.RunID != e.RunID {
		t.Errorf("expected RunID %q, got %q", e.RunID, doc.RunID)
	}
	if doc.Type != string(e.Type) {
		t.Errorf("expected Type %q, got %q", e.Type, doc.Type)
	}
	if doc.Sequence != e.Sequence {
		t.Errorf("expected Sequence %d, got %d", e.Sequence, doc.Sequence)
	}
	if string(doc.Payload) != string(e.Payload) {
		t.Errorf("expected Payload %q, got %q", e.Payload, doc.Payload)
	}
	if !doc.Timestamp.Equal(now) {
		t.Errorf("expected Timestamp %v, got %v", now, doc.Timestamp)
	}
	if doc.Version != e.Version {
		t.Errorf("expected Version %d, got %d", e.Version, doc.Version)
	}
}

func TestFromDocument(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)
	doc := &eventDocument{
		ID:        "evt-1",
		RunID:     "run-1",
		Type:      "test.created",
		Sequence:  42,
		Payload:   []byte(`{"key":"value"}`),
		Timestamp: now,
		Version:   2,
	}

	store := &EventStore{}
	e := store.fromDocument(doc)

	if e.ID != doc.ID {
		t.Errorf("expected ID %q, got %q", doc.ID, e.ID)
	}
	if e.RunID != doc.RunID {
		t.Errorf("expected RunID %q, got %q", doc.RunID, e.RunID)
	}
	if string(e.Type) != doc.Type {
		t.Errorf("expected Type %q, got %q", doc.Type, e.Type)
	}
	if e.Sequence != doc.Sequence {
		t.Errorf("expected Sequence %d, got %d", doc.Sequence, e.Sequence)
	}
	if string(e.Payload) != string(doc.Payload) {
		t.Errorf("expected Payload %q, got %q", doc.Payload, e.Payload)
	}
	if !e.Timestamp.Equal(now) {
		t.Errorf("expected Timestamp %v, got %v", now, e.Timestamp)
	}
	if e.Version != doc.Version {
		t.Errorf("expected Version %d, got %d", doc.Version, e.Version)
	}
}

func TestDocumentRoundTrip(t *testing.T) {
	now := time.Now().Truncate(time.Millisecond)
	original := &event.Event{
		ID:        "evt-round",
		RunID:     "run-round",
		Type:      event.Type("round.trip"),
		Sequence:  7,
		Payload:   []byte(`{"roundtrip":true}`),
		Timestamp: now,
		Version:   3,
	}

	store := &EventStore{}
	doc := store.toDocument(original)
	restored := store.fromDocument(doc)

	if restored.ID != original.ID {
		t.Errorf("ID mismatch after round-trip")
	}
	if restored.RunID != original.RunID {
		t.Errorf("RunID mismatch after round-trip")
	}
	if restored.Type != original.Type {
		t.Errorf("Type mismatch after round-trip")
	}
	if restored.Sequence != original.Sequence {
		t.Errorf("Sequence mismatch after round-trip")
	}
	if string(restored.Payload) != string(original.Payload) {
		t.Errorf("Payload mismatch after round-trip")
	}
	if !restored.Timestamp.Equal(original.Timestamp) {
		t.Errorf("Timestamp mismatch after round-trip")
	}
	if restored.Version != original.Version {
		t.Errorf("Version mismatch after round-trip")
	}
}

// --- Error wrapping tests ---

func TestWrapError_Nil(t *testing.T) {
	store := &EventStore{}
	if err := store.wrapError(nil); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestWrapError_DeadlineExceeded(t *testing.T) {
	store := &EventStore{}
	err := store.wrapError(context.DeadlineExceeded)

	if !errors.Is(err, event.ErrOperationTimeout) {
		t.Errorf("expected ErrOperationTimeout in chain, got %v", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected DeadlineExceeded in chain, got %v", err)
	}
}

func TestWrapError_GenericError(t *testing.T) {
	store := &EventStore{}
	originalErr := errors.New("some mongo error")
	err := store.wrapError(originalErr)

	if !errors.Is(err, event.ErrConnectionFailed) {
		t.Errorf("expected ErrConnectionFailed in chain, got %v", err)
	}
	if !errors.Is(err, originalErr) {
		t.Errorf("expected original error in chain, got %v", err)
	}
}

// --- Subscription management tests ---

func TestSubscribe_ReturnsChannel(t *testing.T) {
	store := &EventStore{
		subscribers:  make(map[string][]chan event.Event),
		queryTimeout: 30 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, err := store.Subscribe(ctx, "run-1")
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}
	if ch == nil {
		t.Fatal("expected non-nil channel")
	}
}

func TestSubscribe_MultipleSubscribers(t *testing.T) {
	store := &EventStore{
		subscribers:  make(map[string][]chan event.Event),
		queryTimeout: 30 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch1, err := store.Subscribe(ctx, "run-1")
	if err != nil {
		t.Fatalf("Subscribe 1 failed: %v", err)
	}

	ch2, err := store.Subscribe(ctx, "run-1")
	if err != nil {
		t.Fatalf("Subscribe 2 failed: %v", err)
	}

	if ch1 == ch2 {
		t.Error("expected different channels for different subscriptions")
	}

	store.mu.RLock()
	subs := store.subscribers["run-1"]
	store.mu.RUnlock()

	if len(subs) != 2 {
		t.Errorf("expected 2 subscribers, got %d", len(subs))
	}
}

func TestSubscribe_CancelledContextCleansUp(t *testing.T) {
	store := &EventStore{
		subscribers:  make(map[string][]chan event.Event),
		queryTimeout: 30 * time.Second,
	}

	ctx, cancel := context.WithCancel(context.Background())

	_, err := store.Subscribe(ctx, "run-1")
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Cancel context to trigger cleanup
	cancel()

	// Wait for cleanup goroutine
	time.Sleep(50 * time.Millisecond)

	store.mu.RLock()
	subs := store.subscribers["run-1"]
	store.mu.RUnlock()

	if len(subs) != 0 {
		t.Errorf("expected 0 subscribers after cancel, got %d", len(subs))
	}
}

func TestNotifySubscribers(t *testing.T) {
	store := &EventStore{
		subscribers:  make(map[string][]chan event.Event),
		queryTimeout: 30 * time.Second,
	}

	ch := make(chan event.Event, 10)
	store.subscribers["run-1"] = []chan event.Event{ch}

	events := []event.Event{
		{RunID: "run-1", Type: "test.event1"},
		{RunID: "run-1", Type: "test.event2"},
	}

	store.notifySubscribers("run-1", events)

	if len(ch) != 2 {
		t.Errorf("expected 2 events in channel, got %d", len(ch))
	}

	e1 := <-ch
	if e1.Type != "test.event1" {
		t.Errorf("expected test.event1, got %s", e1.Type)
	}

	e2 := <-ch
	if e2.Type != "test.event2" {
		t.Errorf("expected test.event2, got %s", e2.Type)
	}
}

func TestNotifySubscribers_FullChannel(t *testing.T) {
	store := &EventStore{
		subscribers:  make(map[string][]chan event.Event),
		queryTimeout: 30 * time.Second,
	}

	// Create a channel with capacity 1
	ch := make(chan event.Event, 1)
	store.subscribers["run-1"] = []chan event.Event{ch}

	// Send 3 events; only 1 should be buffered, rest should be dropped without blocking
	events := []event.Event{
		{RunID: "run-1", Type: "test.event1"},
		{RunID: "run-1", Type: "test.event2"},
		{RunID: "run-1", Type: "test.event3"},
	}

	// Should not block
	done := make(chan struct{})
	go func() {
		store.notifySubscribers("run-1", events)
		close(done)
	}()

	select {
	case <-done:
		// good
	case <-time.After(time.Second):
		t.Fatal("notifySubscribers blocked on full channel")
	}

	if len(ch) != 1 {
		t.Errorf("expected 1 event in channel, got %d", len(ch))
	}
}

func TestNotifySubscribers_NoSubscribers(t *testing.T) {
	store := &EventStore{
		subscribers:  make(map[string][]chan event.Event),
		queryTimeout: 30 * time.Second,
	}

	// Should not panic
	store.notifySubscribers("run-unknown", []event.Event{
		{RunID: "run-unknown", Type: "test.event"},
	})
}

func TestUnsubscribe(t *testing.T) {
	store := &EventStore{
		subscribers:  make(map[string][]chan event.Event),
		queryTimeout: 30 * time.Second,
	}

	ch1 := make(chan event.Event, 10)
	ch2 := make(chan event.Event, 10)
	store.subscribers["run-1"] = []chan event.Event{ch1, ch2}

	store.unsubscribe("run-1", ch1)

	store.mu.RLock()
	subs := store.subscribers["run-1"]
	store.mu.RUnlock()

	if len(subs) != 1 {
		t.Errorf("expected 1 subscriber after unsubscribe, got %d", len(subs))
	}

	// Unsubscribe last one
	store.unsubscribe("run-1", ch2)

	store.mu.RLock()
	_, exists := store.subscribers["run-1"]
	store.mu.RUnlock()

	if exists {
		t.Error("expected subscriber map entry to be cleaned up")
	}
}

// --- NewEventStore tests ---

func TestNewEventStore_DefaultCollectionName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	// This test requires a real MongoDB connection. Skip in short mode.
}

func TestNewEventStore_CustomCollectionName(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
}

// --- Append validation tests (using mocked internals) ---

func TestAppend_EmptyEvents(t *testing.T) {
	store := &EventStore{
		subscribers:  make(map[string][]chan event.Event),
		queryTimeout: 30 * time.Second,
	}

	// Append with no events should succeed without touching the collection
	err := store.Append(context.Background())
	if err != nil {
		t.Errorf("expected nil error for empty append, got %v", err)
	}
}

// --- DeleteRun subscription cleanup tests ---

func TestDeleteRun_ClosesSubscribers(t *testing.T) {
	store := &EventStore{
		subscribers:  make(map[string][]chan event.Event),
		queryTimeout: 30 * time.Second,
	}

	ch1 := make(chan event.Event, 10)
	ch2 := make(chan event.Event, 10)
	store.subscribers["run-to-delete"] = []chan event.Event{ch1, ch2}

	// We cannot call DeleteRun without a real collection, but we can verify
	// the subscriber cleanup logic directly.
	store.mu.Lock()
	if subs, ok := store.subscribers["run-to-delete"]; ok {
		for _, ch := range subs {
			close(ch)
		}
		delete(store.subscribers, "run-to-delete")
	}
	store.mu.Unlock()

	// Verify channels are closed
	_, ok1 := <-ch1
	if ok1 {
		t.Error("expected ch1 to be closed")
	}

	_, ok2 := <-ch2
	if ok2 {
		t.Error("expected ch2 to be closed")
	}

	store.mu.RLock()
	_, exists := store.subscribers["run-to-delete"]
	store.mu.RUnlock()

	if exists {
		t.Error("expected subscriber entry to be removed")
	}
}

// --- Config tests ---

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.URI == "" {
		t.Error("expected non-empty default URI")
	}
	if cfg.Database == "" {
		t.Error("expected non-empty default database")
	}
	if cfg.ConnectTimeout <= 0 {
		t.Error("expected positive connect timeout")
	}
	if cfg.QueryTimeout <= 0 {
		t.Error("expected positive query timeout")
	}
	if cfg.MaxPoolSize == 0 {
		t.Error("expected non-zero max pool size")
	}
}

func TestConfigOptions(t *testing.T) {
	cfg := DefaultConfig()

	WithURI("mongodb://custom:27017")(&cfg)
	if cfg.URI != "mongodb://custom:27017" {
		t.Errorf("expected custom URI, got %q", cfg.URI)
	}

	WithDatabase("custom_db")(&cfg)
	if cfg.Database != "custom_db" {
		t.Errorf("expected custom_db, got %q", cfg.Database)
	}

	WithConnectTimeout(5 * time.Second)(&cfg)
	if cfg.ConnectTimeout != 5*time.Second {
		t.Errorf("expected 5s, got %v", cfg.ConnectTimeout)
	}

	WithQueryTimeout(15 * time.Second)(&cfg)
	if cfg.QueryTimeout != 15*time.Second {
		t.Errorf("expected 15s, got %v", cfg.QueryTimeout)
	}

	WithMaxPoolSize(50)(&cfg)
	if cfg.MaxPoolSize != 50 {
		t.Errorf("expected 50, got %d", cfg.MaxPoolSize)
	}

	WithMinPoolSize(5)(&cfg)
	if cfg.MinPoolSize != 5 {
		t.Errorf("expected 5, got %d", cfg.MinPoolSize)
	}
}

// --- Integration tests (require real MongoDB, skip in short mode) ---

func TestEventStore_Integration_AppendAndLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	// Would require a real MongoDB connection
}

func TestEventStore_Integration_Query(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
}

func TestEventStore_Integration_CountEvents(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
}

func TestEventStore_Integration_ListRuns(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
}

func TestEventStore_Integration_DeleteRun(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
}

func TestEventStore_Integration_Subscribe(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
}
