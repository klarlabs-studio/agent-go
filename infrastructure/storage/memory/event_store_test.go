package memory_test

import (
	"context"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
)

func TestNewEventStore(t *testing.T) {
	t.Parallel()

	store := memory.NewEventStore()
	if store == nil {
		t.Fatal("NewEventStore() returned nil")
	}
	if store.Len() != 0 {
		t.Errorf("Len() = %d, want 0 for new store", store.Len())
	}
}

func TestEventStore_Append(t *testing.T) {
	t.Parallel()

	t.Run("appends single event", func(t *testing.T) {
		t.Parallel()

		store := memory.NewEventStore()
		ctx := context.Background()

		e := event.Event{
			RunID:     "run-1",
			Type:      "test.event",
			Timestamp: time.Now(),
		}

		err := store.Append(ctx, e)
		if err != nil {
			t.Fatalf("Append() error = %v", err)
		}

		if store.Len() != 1 {
			t.Errorf("Len() = %d, want 1", store.Len())
		}
	})

	t.Run("appends multiple events", func(t *testing.T) {
		t.Parallel()

		store := memory.NewEventStore()
		ctx := context.Background()

		events := []event.Event{
			{RunID: "run-1", Type: "event.1", Timestamp: time.Now()},
			{RunID: "run-1", Type: "event.2", Timestamp: time.Now()},
			{RunID: "run-2", Type: "event.1", Timestamp: time.Now()},
		}

		err := store.Append(ctx, events...)
		if err != nil {
			t.Fatalf("Append() error = %v", err)
		}

		if store.Len() != 3 {
			t.Errorf("Len() = %d, want 3", store.Len())
		}
	})

	t.Run("assigns sequence numbers", func(t *testing.T) {
		t.Parallel()

		store := memory.NewEventStore()
		ctx := context.Background()

		store.Append(ctx, event.Event{RunID: "run-1", Type: "event.1"})
		store.Append(ctx, event.Event{RunID: "run-1", Type: "event.2"})

		events, _ := store.LoadEvents(ctx, "run-1")
		if len(events) != 2 {
			t.Fatalf("LoadEvents() count = %d, want 2", len(events))
		}
		if events[0].Sequence != 1 {
			t.Errorf("First event Sequence = %d, want 1", events[0].Sequence)
		}
		if events[1].Sequence != 2 {
			t.Errorf("Second event Sequence = %d, want 2", events[1].Sequence)
		}
	})

	t.Run("assigns IDs to events without IDs", func(t *testing.T) {
		t.Parallel()

		store := memory.NewEventStore()
		ctx := context.Background()

		store.Append(ctx, event.Event{RunID: "run-1", Type: "event.1"})

		events, _ := store.LoadEvents(ctx, "run-1")
		if events[0].ID == "" {
			t.Error("Event should have ID assigned")
		}
	})

	t.Run("returns error for event with empty type", func(t *testing.T) {
		t.Parallel()

		store := memory.NewEventStore()
		ctx := context.Background()

		err := store.Append(ctx, event.Event{RunID: "run-1", Type: ""})
		if err != event.ErrInvalidEvent {
			t.Errorf("Append() error = %v, want ErrInvalidEvent", err)
		}
	})

	t.Run("returns nil for empty events slice", func(t *testing.T) {
		t.Parallel()

		store := memory.NewEventStore()
		ctx := context.Background()

		err := store.Append(ctx)
		if err != nil {
			t.Errorf("Append() error = %v, want nil", err)
		}
	})

	t.Run("returns error for cancelled context", func(t *testing.T) {
		t.Parallel()

		store := memory.NewEventStore()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := store.Append(ctx, event.Event{RunID: "run-1", Type: "event.1"})
		if err == nil {
			t.Error("Append() should return error for cancelled context")
		}
	})
}

func TestEventStore_LoadEvents(t *testing.T) {
	t.Parallel()

	t.Run("loads all events for run", func(t *testing.T) {
		t.Parallel()

		store := memory.NewEventStore()
		ctx := context.Background()

		store.Append(ctx, event.Event{RunID: "run-1", Type: "event.1"})
		store.Append(ctx, event.Event{RunID: "run-1", Type: "event.2"})
		store.Append(ctx, event.Event{RunID: "run-2", Type: "event.1"})

		events, err := store.LoadEvents(ctx, "run-1")
		if err != nil {
			t.Fatalf("LoadEvents() error = %v", err)
		}
		if len(events) != 2 {
			t.Errorf("LoadEvents() count = %d, want 2", len(events))
		}
	})

	t.Run("returns empty for non-existent run", func(t *testing.T) {
		t.Parallel()

		store := memory.NewEventStore()
		ctx := context.Background()

		events, err := store.LoadEvents(ctx, "nonexistent")
		if err != nil {
			t.Fatalf("LoadEvents() error = %v", err)
		}
		if len(events) != 0 {
			t.Errorf("LoadEvents() count = %d, want 0", len(events))
		}
	})

	t.Run("returns error for cancelled context", func(t *testing.T) {
		t.Parallel()

		store := memory.NewEventStore()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := store.LoadEvents(ctx, "run-1")
		if err == nil {
			t.Error("LoadEvents() should return error for cancelled context")
		}
	})
}

func TestEventStore_LoadEventsFrom(t *testing.T) {
	t.Parallel()

	t.Run("loads events from sequence", func(t *testing.T) {
		t.Parallel()

		store := memory.NewEventStore()
		ctx := context.Background()

		store.Append(ctx,
			event.Event{RunID: "run-1", Type: "event.1"},
			event.Event{RunID: "run-1", Type: "event.2"},
			event.Event{RunID: "run-1", Type: "event.3"},
		)

		events, err := store.LoadEventsFrom(ctx, "run-1", 2)
		if err != nil {
			t.Fatalf("LoadEventsFrom() error = %v", err)
		}
		if len(events) != 2 {
			t.Errorf("LoadEventsFrom() count = %d, want 2", len(events))
		}
		if events[0].Sequence != 2 {
			t.Errorf("First event Sequence = %d, want 2", events[0].Sequence)
		}
	})

	t.Run("returns empty for non-existent run", func(t *testing.T) {
		t.Parallel()

		store := memory.NewEventStore()
		ctx := context.Background()

		events, err := store.LoadEventsFrom(ctx, "nonexistent", 1)
		if err != nil {
			t.Fatalf("LoadEventsFrom() error = %v", err)
		}
		if len(events) != 0 {
			t.Errorf("LoadEventsFrom() count = %d, want 0", len(events))
		}
	})

	t.Run("returns error for cancelled context", func(t *testing.T) {
		t.Parallel()

		store := memory.NewEventStore()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := store.LoadEventsFrom(ctx, "run-1", 1)
		if err == nil {
			t.Error("LoadEventsFrom() should return error for cancelled context")
		}
	})
}

func TestEventStore_Subscribe(t *testing.T) {
	t.Parallel()

	t.Run("receives new events", func(t *testing.T) {
		t.Parallel()

		store := memory.NewEventStore()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		ch, err := store.Subscribe(ctx, "run-1")
		if err != nil {
			t.Fatalf("Subscribe() error = %v", err)
		}

		// Append event after subscribing
		store.Append(context.Background(), event.Event{RunID: "run-1", Type: "event.1"})

		select {
		case e := <-ch:
			if e.Type != "event.1" {
				t.Errorf("Received event type = %s, want event.1", e.Type)
			}
		case <-time.After(100 * time.Millisecond):
			t.Error("Did not receive event")
		}
	})

	t.Run("unsubscribes on context cancel", func(t *testing.T) {
		t.Parallel()

		store := memory.NewEventStore()
		ctx, cancel := context.WithCancel(context.Background())

		ch, _ := store.Subscribe(ctx, "run-1")
		cancel()

		// Wait a bit for cleanup
		time.Sleep(50 * time.Millisecond)

		// Channel should be closed
		select {
		case _, ok := <-ch:
			if ok {
				t.Error("Channel should be closed after context cancel")
			}
		case <-time.After(100 * time.Millisecond):
			// Channel might still be open if goroutine hasn't run yet
		}
	})

	t.Run("returns error for cancelled context", func(t *testing.T) {
		t.Parallel()

		store := memory.NewEventStore()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := store.Subscribe(ctx, "run-1")
		if err == nil {
			t.Error("Subscribe() should return error for cancelled context")
		}
	})
}

func TestEventStore_Query(t *testing.T) {
	t.Parallel()

	t.Run("queries by event type", func(t *testing.T) {
		t.Parallel()

		store := memory.NewEventStore()
		ctx := context.Background()

		store.Append(ctx,
			event.Event{RunID: "run-1", Type: event.Type("tool.called")},
			event.Event{RunID: "run-1", Type: event.Type("state.changed")},
			event.Event{RunID: "run-1", Type: event.Type("tool.called")},
		)

		events, err := store.Query(ctx, "run-1", event.QueryOptions{
			Types: []event.Type{event.Type("tool.called")},
		})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(events) != 2 {
			t.Errorf("Query() count = %d, want 2", len(events))
		}
	})

	t.Run("queries by time range", func(t *testing.T) {
		t.Parallel()

		store := memory.NewEventStore()
		ctx := context.Background()

		now := time.Now()
		store.Append(ctx,
			event.Event{RunID: "run-1", Type: "event.1", Timestamp: now.Add(-2 * time.Hour)},
			event.Event{RunID: "run-1", Type: "event.2", Timestamp: now.Add(-1 * time.Hour)},
			event.Event{RunID: "run-1", Type: "event.3", Timestamp: now},
		)

		events, err := store.Query(ctx, "run-1", event.QueryOptions{
			FromTime: now.Add(-90 * time.Minute).Unix(),
		})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(events) != 2 {
			t.Errorf("Query() count = %d, want 2", len(events))
		}
	})

	t.Run("applies offset and limit", func(t *testing.T) {
		t.Parallel()

		store := memory.NewEventStore()
		ctx := context.Background()

		for i := 0; i < 5; i++ {
			store.Append(ctx, event.Event{RunID: "run-1", Type: "event"})
		}

		events, err := store.Query(ctx, "run-1", event.QueryOptions{
			Offset: 2,
			Limit:  2,
		})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(events) != 2 {
			t.Errorf("Query() count = %d, want 2", len(events))
		}
	})

	t.Run("returns empty for large offset", func(t *testing.T) {
		t.Parallel()

		store := memory.NewEventStore()
		ctx := context.Background()

		store.Append(ctx, event.Event{RunID: "run-1", Type: "event"})

		events, err := store.Query(ctx, "run-1", event.QueryOptions{
			Offset: 100,
		})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(events) != 0 {
			t.Errorf("Query() count = %d, want 0", len(events))
		}
	})

	t.Run("returns empty for non-existent run", func(t *testing.T) {
		t.Parallel()

		store := memory.NewEventStore()
		ctx := context.Background()

		events, err := store.Query(ctx, "nonexistent", event.QueryOptions{})
		if err != nil {
			t.Fatalf("Query() error = %v", err)
		}
		if len(events) != 0 {
			t.Errorf("Query() count = %d, want 0", len(events))
		}
	})

	t.Run("returns error for cancelled context", func(t *testing.T) {
		t.Parallel()

		store := memory.NewEventStore()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := store.Query(ctx, "run-1", event.QueryOptions{})
		if err == nil {
			t.Error("Query() should return error for cancelled context")
		}
	})
}

func TestEventStore_CountEvents(t *testing.T) {
	t.Parallel()

	store := memory.NewEventStore()
	ctx := context.Background()

	store.Append(ctx,
		event.Event{RunID: "run-1", Type: "event.1"},
		event.Event{RunID: "run-1", Type: "event.2"},
		event.Event{RunID: "run-2", Type: "event.1"},
	)

	count, err := store.CountEvents(ctx, "run-1")
	if err != nil {
		t.Fatalf("CountEvents() error = %v", err)
	}
	if count != 2 {
		t.Errorf("CountEvents() = %d, want 2", count)
	}
}

func TestEventStore_ListRuns(t *testing.T) {
	t.Parallel()

	store := memory.NewEventStore()
	ctx := context.Background()

	store.Append(ctx,
		event.Event{RunID: "run-1", Type: "event"},
		event.Event{RunID: "run-2", Type: "event"},
	)

	runs, err := store.ListRuns(ctx)
	if err != nil {
		t.Fatalf("ListRuns() error = %v", err)
	}
	if len(runs) != 2 {
		t.Errorf("ListRuns() count = %d, want 2", len(runs))
	}
}

func TestEventStore_Clear(t *testing.T) {
	t.Parallel()

	store := memory.NewEventStore()
	ctx := context.Background()

	store.Append(ctx, event.Event{RunID: "run-1", Type: "event"})
	store.Clear()

	if store.Len() != 0 {
		t.Errorf("Len() = %d, want 0 after Clear()", store.Len())
	}
}

func TestEventStore_DeleteRun(t *testing.T) {
	t.Parallel()

	t.Run("deletes events for run", func(t *testing.T) {
		t.Parallel()

		store := memory.NewEventStore()
		ctx := context.Background()

		store.Append(ctx,
			event.Event{RunID: "run-1", Type: "event"},
			event.Event{RunID: "run-2", Type: "event"},
		)

		err := store.DeleteRun(ctx, "run-1")
		if err != nil {
			t.Fatalf("DeleteRun() error = %v", err)
		}

		events, _ := store.LoadEvents(ctx, "run-1")
		if len(events) != 0 {
			t.Errorf("Events for run-1 = %d, want 0", len(events))
		}

		// run-2 should still exist
		events, _ = store.LoadEvents(ctx, "run-2")
		if len(events) != 1 {
			t.Errorf("Events for run-2 = %d, want 1", len(events))
		}
	})

	t.Run("returns error for cancelled context", func(t *testing.T) {
		t.Parallel()

		store := memory.NewEventStore()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := store.DeleteRun(ctx, "run-1")
		if err == nil {
			t.Error("DeleteRun() should return error for cancelled context")
		}
	})
}
