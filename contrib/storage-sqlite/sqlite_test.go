package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/cache"
	"github.com/felixgeelhaar/agent-go/domain/event"
	"github.com/felixgeelhaar/agent-go/domain/run"
	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("failed to open database: %v", err)
	}
	return db
}

// Cache Tests

func TestCache_EnsureSchema(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	c := NewCache(db)
	ctx := context.Background()

	err := c.EnsureSchema(ctx)
	if err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Verify table exists
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='cache_entries'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query table: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 table, got %d", count)
	}
}

func TestCache_SetGet(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	c := NewCache(db)
	ctx := context.Background()

	if err := c.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Set a value
	key := "test-key"
	value := []byte("test-value")
	err := c.Set(ctx, key, value, cache.SetOptions{})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get the value
	got, found, err := c.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Fatal("expected value to be found")
	}
	if string(got) != string(value) {
		t.Errorf("expected %s, got %s", value, got)
	}
}

func TestCache_TTL(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	c := NewCache(db)
	ctx := context.Background()

	if err := c.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Set a value with short TTL
	key := "ttl-key"
	value := []byte("ttl-value")
	err := c.Set(ctx, key, value, cache.SetOptions{TTL: 50 * time.Millisecond})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get immediately - should exist
	_, found, err := c.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if !found {
		t.Fatal("expected value to be found")
	}

	// Wait for expiry with extra buffer
	time.Sleep(200 * time.Millisecond)

	// Get after expiry - should not exist
	_, found, err = c.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if found {
		t.Fatal("expected value to be expired")
	}

	// Verify it was actually deleted from the database
	exists, err := c.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Fatal("expected expired key to not exist")
	}
}

func TestCache_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	c := NewCache(db)
	ctx := context.Background()

	if err := c.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Set a value
	key := "delete-key"
	value := []byte("delete-value")
	err := c.Set(ctx, key, value, cache.SetOptions{})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Delete the value
	err = c.Delete(ctx, key)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Get should return not found
	_, found, err := c.Get(ctx, key)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if found {
		t.Fatal("expected value to be deleted")
	}
}

func TestCache_Exists(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	c := NewCache(db)
	ctx := context.Background()

	if err := c.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	key := "exists-key"

	// Should not exist initially
	exists, err := c.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Fatal("expected key to not exist")
	}

	// Set a value
	err = c.Set(ctx, key, []byte("value"), cache.SetOptions{})
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Should exist now
	exists, err = c.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Fatal("expected key to exist")
	}
}

func TestCache_Clear(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	c := NewCache(db)
	ctx := context.Background()

	if err := c.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Set multiple values
	for i := 0; i < 5; i++ {
		key := "key-" + string(rune('0'+i))
		value := []byte("value-" + string(rune('0'+i)))
		err := c.Set(ctx, key, value, cache.SetOptions{})
		if err != nil {
			t.Fatalf("Set failed: %v", err)
		}
	}

	// Clear all
	err := c.Clear(ctx)
	if err != nil {
		t.Fatalf("Clear failed: %v", err)
	}

	// Verify all are gone
	for i := 0; i < 5; i++ {
		key := "key-" + string(rune('0'+i))
		exists, err := c.Exists(ctx, key)
		if err != nil {
			t.Fatalf("Exists failed: %v", err)
		}
		if exists {
			t.Fatalf("expected key %s to be cleared", key)
		}
	}
}

// EventStore Tests

func TestEventStore_EnsureSchema(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewEventStore(db)
	ctx := context.Background()

	err := s.EnsureSchema(ctx)
	if err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Verify table exists
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='events'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query table: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 table, got %d", count)
	}
}

func TestEventStore_AppendLoad(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewEventStore(db)
	ctx := context.Background()

	if err := s.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	runID := "test-run-1"

	// Create test events
	events := []event.Event{
		{
			RunID:     runID,
			Type:      "test.event.1",
			Timestamp: time.Now(),
			Payload:   json.RawMessage(`{"key":"value1"}`),
		},
		{
			RunID:     runID,
			Type:      "test.event.2",
			Timestamp: time.Now(),
			Payload:   json.RawMessage(`{"key":"value2"}`),
		},
	}

	// Append events
	err := s.Append(ctx, events...)
	if err != nil {
		t.Fatalf("Append failed: %v", err)
	}

	// Load events
	loaded, err := s.LoadEvents(ctx, runID)
	if err != nil {
		t.Fatalf("LoadEvents failed: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("expected 2 events, got %d", len(loaded))
	}

	// Verify sequence numbers
	if loaded[0].Sequence != 1 {
		t.Errorf("expected sequence 1, got %d", loaded[0].Sequence)
	}
	if loaded[1].Sequence != 2 {
		t.Errorf("expected sequence 2, got %d", loaded[1].Sequence)
	}

	// Verify IDs were assigned
	if loaded[0].ID == "" {
		t.Error("expected event ID to be assigned")
	}
	if loaded[1].ID == "" {
		t.Error("expected event ID to be assigned")
	}
}

func TestEventStore_LoadEventsFrom(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewEventStore(db)
	ctx := context.Background()

	if err := s.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	runID := "test-run-2"

	// Create and append 5 events
	for i := 0; i < 5; i++ {
		e := event.Event{
			RunID:     runID,
			Type:      event.Type("test.event"),
			Timestamp: time.Now(),
			Payload:   json.RawMessage(`{}`),
		}
		err := s.Append(ctx, e)
		if err != nil {
			t.Fatalf("Append failed: %v", err)
		}
	}

	// Load from sequence 3
	loaded, err := s.LoadEventsFrom(ctx, runID, 3)
	if err != nil {
		t.Fatalf("LoadEventsFrom failed: %v", err)
	}

	if len(loaded) != 3 {
		t.Fatalf("expected 3 events, got %d", len(loaded))
	}

	if loaded[0].Sequence != 3 {
		t.Errorf("expected first sequence to be 3, got %d", loaded[0].Sequence)
	}
}

func TestEventStore_Subscribe(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewEventStore(db)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := s.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	runID := "test-run-3"

	// Subscribe to events
	ch, err := s.Subscribe(ctx, runID)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	// Append an event in a goroutine
	go func() {
		time.Sleep(50 * time.Millisecond)
		e := event.Event{
			RunID:     runID,
			Type:      "test.event",
			Timestamp: time.Now(),
			Payload:   json.RawMessage(`{"test":"data"}`),
		}
		_ = s.Append(ctx, e)
	}()

	// Wait for event
	select {
	case evt := <-ch:
		if evt.RunID != runID {
			t.Errorf("expected runID %s, got %s", runID, evt.RunID)
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("timeout waiting for event")
	}

	// Cancel context and verify channel closes
	cancel()
	time.Sleep(50 * time.Millisecond)

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel to be closed")
		}
	default:
		t.Fatal("channel should be closed")
	}
}

// RunStore Tests

func TestRunStore_EnsureSchema(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewRunStore(db)
	ctx := context.Background()

	err := s.EnsureSchema(ctx)
	if err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Verify table exists
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='runs'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query table: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 table, got %d", count)
	}
}

func TestRunStore_SaveGet(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewRunStore(db)
	ctx := context.Background()

	if err := s.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Create a run
	r := &agent.Run{
		ID:           "test-run-1",
		Goal:         "Test goal",
		CurrentState: agent.StateIntake,
		Vars:         map[string]any{"key": "value"},
		Evidence:     []agent.Evidence{},
		Status:       agent.RunStatusRunning,
		StartTime:    time.Now(),
	}

	// Save the run
	err := s.Save(ctx, r)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Get the run
	got, err := s.Get(ctx, r.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.ID != r.ID {
		t.Errorf("expected ID %s, got %s", r.ID, got.ID)
	}
	if got.Goal != r.Goal {
		t.Errorf("expected Goal %s, got %s", r.Goal, got.Goal)
	}
	if got.CurrentState != r.CurrentState {
		t.Errorf("expected CurrentState %s, got %s", r.CurrentState, got.CurrentState)
	}
}

func TestRunStore_SaveDuplicate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewRunStore(db)
	ctx := context.Background()

	if err := s.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	r := &agent.Run{
		ID:        "test-run-dup",
		Goal:      "Test",
		Status:    agent.RunStatusRunning,
		StartTime: time.Now(),
		Vars:      map[string]any{},
		Evidence:  []agent.Evidence{},
	}

	// Save first time - should succeed
	err := s.Save(ctx, r)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Save again - should fail with ErrRunExists
	err = s.Save(ctx, r)
	if err != run.ErrRunExists {
		t.Fatalf("expected ErrRunExists, got %v", err)
	}
}

func TestRunStore_Update(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewRunStore(db)
	ctx := context.Background()

	if err := s.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Create and save a run
	r := &agent.Run{
		ID:           "test-run-update",
		Goal:         "Initial goal",
		CurrentState: agent.StateIntake,
		Status:       agent.RunStatusRunning,
		StartTime:    time.Now(),
		Vars:         map[string]any{},
		Evidence:     []agent.Evidence{},
	}

	err := s.Save(ctx, r)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Update the run
	r.Goal = "Updated goal"
	r.CurrentState = agent.StateExplore
	err = s.Update(ctx, r)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// Get and verify
	got, err := s.Get(ctx, r.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if got.Goal != "Updated goal" {
		t.Errorf("expected updated goal, got %s", got.Goal)
	}
	if got.CurrentState != agent.StateExplore {
		t.Errorf("expected explore state, got %s", got.CurrentState)
	}
}

func TestRunStore_Delete(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewRunStore(db)
	ctx := context.Background()

	if err := s.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Create and save a run
	r := &agent.Run{
		ID:        "test-run-delete",
		Goal:      "Test",
		Status:    agent.RunStatusRunning,
		StartTime: time.Now(),
		Vars:      map[string]any{},
		Evidence:  []agent.Evidence{},
	}

	err := s.Save(ctx, r)
	if err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Delete the run
	err = s.Delete(ctx, r.ID)
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	// Get should return ErrRunNotFound
	_, err = s.Get(ctx, r.ID)
	if err != run.ErrRunNotFound {
		t.Fatalf("expected ErrRunNotFound, got %v", err)
	}
}

func TestRunStore_List(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewRunStore(db)
	ctx := context.Background()

	if err := s.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Create multiple runs
	for i := 0; i < 5; i++ {
		r := &agent.Run{
			ID:        "test-run-" + string(rune('0'+i)),
			Goal:      "Goal " + string(rune('0'+i)),
			Status:    agent.RunStatusRunning,
			StartTime: time.Now().Add(time.Duration(i) * time.Second),
			Vars:      map[string]any{},
			Evidence:  []agent.Evidence{},
		}
		err := s.Save(ctx, r)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	// List all runs
	runs, err := s.List(ctx, run.ListFilter{})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(runs) != 5 {
		t.Fatalf("expected 5 runs, got %d", len(runs))
	}

	// List with limit
	runs, err = s.List(ctx, run.ListFilter{Limit: 2})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(runs) != 2 {
		t.Fatalf("expected 2 runs, got %d", len(runs))
	}

	// List with filter
	runs, err = s.List(ctx, run.ListFilter{
		Status: []agent.RunStatus{agent.RunStatusRunning},
	})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(runs) != 5 {
		t.Fatalf("expected 5 running runs, got %d", len(runs))
	}
}

func TestRunStore_ListWithGoalPattern(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewRunStore(db)
	ctx := context.Background()

	if err := s.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Create runs with different goals
	runs := []*agent.Run{
		{
			ID:        "run-1",
			Goal:      "Process data files",
			Status:    agent.RunStatusRunning,
			StartTime: time.Now(),
			Vars:      map[string]any{},
			Evidence:  []agent.Evidence{},
		},
		{
			ID:        "run-2",
			Goal:      "Generate report",
			Status:    agent.RunStatusRunning,
			StartTime: time.Now(),
			Vars:      map[string]any{},
			Evidence:  []agent.Evidence{},
		},
		{
			ID:        "run-3",
			Goal:      "Process images",
			Status:    agent.RunStatusRunning,
			StartTime: time.Now(),
			Vars:      map[string]any{},
			Evidence:  []agent.Evidence{},
		},
	}

	for _, r := range runs {
		err := s.Save(ctx, r)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	// Filter by goal pattern
	filtered, err := s.List(ctx, run.ListFilter{GoalPattern: "Process"})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	if len(filtered) != 2 {
		t.Fatalf("expected 2 runs matching 'Process', got %d", len(filtered))
	}
}

func TestRunStore_Count(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	s := NewRunStore(db)
	ctx := context.Background()

	if err := s.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Create runs
	for i := 0; i < 10; i++ {
		r := &agent.Run{
			ID:        "test-run-count-" + string(rune('0'+i)),
			Goal:      "Goal",
			Status:    agent.RunStatusRunning,
			StartTime: time.Now(),
			Vars:      map[string]any{},
			Evidence:  []agent.Evidence{},
		}
		err := s.Save(ctx, r)
		if err != nil {
			t.Fatalf("Save failed: %v", err)
		}
	}

	// Count all
	count, err := s.Count(ctx, run.ListFilter{})
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}

	if count != 10 {
		t.Fatalf("expected 10 runs, got %d", count)
	}

	// Count with filter
	count, err = s.Count(ctx, run.ListFilter{
		Status: []agent.RunStatus{agent.RunStatusRunning},
	})
	if err != nil {
		t.Fatalf("Count failed: %v", err)
	}

	if count != 10 {
		t.Fatalf("expected 10 running runs, got %d", count)
	}
}
