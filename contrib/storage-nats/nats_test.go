package nats

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	natsserver "github.com/nats-io/nats-server/v2/server"
	natstest "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.klarlabs.de/agent/domain/event"
)

// startTestServer starts an embedded NATS server with JetStream enabled for testing.
func startTestServer(t *testing.T) (*natsserver.Server, nats.JetStreamContext) {
	t.Helper()

	opts := natstest.DefaultTestOptions
	opts.JetStream = true
	opts.Port = -1 // Use random port
	opts.StoreDir = t.TempDir()

	srv := natstest.RunServer(&opts)
	t.Cleanup(srv.Shutdown)

	nc, err := nats.Connect(srv.ClientURL())
	require.NoError(t, err)
	t.Cleanup(func() { nc.Close() })

	js, err := nc.JetStream()
	require.NoError(t, err)

	// Create stream
	_, err = js.AddStream(&nats.StreamConfig{
		Name:     "AGENT_EVENTS",
		Subjects: []string{"agent.events.>"},
		Storage:  nats.MemoryStorage,
	})
	require.NoError(t, err)

	return srv, js
}

func TestEventStore_Append(t *testing.T) {
	_, js := startTestServer(t)
	store := NewEventStore(js, "AGENT_EVENTS")

	ctx := context.Background()
	runID := "test-run-1"

	// Create test events
	payload1 := map[string]string{"action": "start"}
	data1, _ := json.Marshal(payload1)

	payload2 := map[string]string{"action": "complete"}
	data2, _ := json.Marshal(payload2)

	events := []event.Event{
		{
			RunID:   runID,
			Type:    event.Type("run.started"),
			Payload: data1,
		},
		{
			RunID:   runID,
			Type:    event.Type("run.completed"),
			Payload: data2,
		},
	}

	// Append events
	err := store.Append(ctx, events...)
	require.NoError(t, err)

	// Verify sequence numbers were assigned
	assert.Equal(t, uint64(1), events[0].Sequence)
	assert.Equal(t, uint64(2), events[1].Sequence)

	// Verify IDs were assigned
	assert.NotEmpty(t, events[0].ID)
	assert.NotEmpty(t, events[1].ID)
}

func TestEventStore_AppendInvalidEvent(t *testing.T) {
	_, js := startTestServer(t)
	store := NewEventStore(js, "AGENT_EVENTS")

	ctx := context.Background()

	// Event without type
	invalidEvent := event.Event{
		RunID:   "test-run",
		Payload: json.RawMessage(`{}`),
	}

	err := store.Append(ctx, invalidEvent)
	assert.ErrorIs(t, err, event.ErrInvalidEvent)
}

func TestEventStore_LoadEvents(t *testing.T) {
	_, js := startTestServer(t)
	store := NewEventStore(js, "AGENT_EVENTS")

	ctx := context.Background()
	runID := "test-run-2"

	// Append test events
	payload1 := map[string]string{"action": "start"}
	data1, _ := json.Marshal(payload1)

	payload2 := map[string]string{"action": "process"}
	data2, _ := json.Marshal(payload2)

	payload3 := map[string]string{"action": "complete"}
	data3, _ := json.Marshal(payload3)

	events := []event.Event{
		{
			RunID:   runID,
			Type:    event.Type("run.started"),
			Payload: data1,
		},
		{
			RunID:   runID,
			Type:    event.Type("tool.executed"),
			Payload: data2,
		},
		{
			RunID:   runID,
			Type:    event.Type("run.completed"),
			Payload: data3,
		},
	}

	err := store.Append(ctx, events...)
	require.NoError(t, err)

	// Wait for JetStream to persist
	time.Sleep(100 * time.Millisecond)

	// Load events
	loaded, err := store.LoadEvents(ctx, runID)
	require.NoError(t, err)
	require.Len(t, loaded, 3)

	// Verify order
	assert.Equal(t, uint64(1), loaded[0].Sequence)
	assert.Equal(t, uint64(2), loaded[1].Sequence)
	assert.Equal(t, uint64(3), loaded[2].Sequence)

	// Verify types
	assert.Equal(t, event.Type("run.started"), loaded[0].Type)
	assert.Equal(t, event.Type("tool.executed"), loaded[1].Type)
	assert.Equal(t, event.Type("run.completed"), loaded[2].Type)
}

func TestEventStore_LoadEventsFrom(t *testing.T) {
	_, js := startTestServer(t)
	store := NewEventStore(js, "AGENT_EVENTS")

	ctx := context.Background()
	runID := "test-run-3"

	// Append test events
	var events []event.Event
	for i := 0; i < 5; i++ {
		payload := map[string]int{"step": i}
		data, _ := json.Marshal(payload)

		events = append(events, event.Event{
			RunID:   runID,
			Type:    event.Type("step.executed"),
			Payload: data,
		})
	}

	err := store.Append(ctx, events...)
	require.NoError(t, err)

	// Wait for JetStream to persist
	time.Sleep(100 * time.Millisecond)

	// Load from sequence 3
	loaded, err := store.LoadEventsFrom(ctx, runID, 3)
	require.NoError(t, err)
	require.Len(t, loaded, 3)

	// Verify sequences
	assert.Equal(t, uint64(3), loaded[0].Sequence)
	assert.Equal(t, uint64(4), loaded[1].Sequence)
	assert.Equal(t, uint64(5), loaded[2].Sequence)
}

func TestEventStore_Subscribe(t *testing.T) {
	_, js := startTestServer(t)
	store := NewEventStore(js, "AGENT_EVENTS")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runID := "test-run-4"

	// Subscribe before appending
	ch, err := store.Subscribe(ctx, runID)
	require.NoError(t, err)

	// Append events in a goroutine
	go func() {
		time.Sleep(50 * time.Millisecond)

		payload := map[string]string{"action": "start"}
		data, _ := json.Marshal(payload)

		ev := event.Event{
			RunID:   runID,
			Type:    event.Type("run.started"),
			Payload: data,
		}

		store.Append(context.Background(), ev)
	}()

	// Receive event
	select {
	case received := <-ch:
		assert.Equal(t, runID, received.RunID)
		assert.Equal(t, event.Type("run.started"), received.Type)
	case <-time.After(2 * time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestEventStore_SubscribeMultipleEvents(t *testing.T) {
	_, js := startTestServer(t)
	store := NewEventStore(js, "AGENT_EVENTS")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runID := "test-run-5"

	// Subscribe
	ch, err := store.Subscribe(ctx, runID)
	require.NoError(t, err)

	// Append multiple events
	go func() {
		time.Sleep(50 * time.Millisecond)

		for i := 0; i < 3; i++ {
			payload := map[string]int{"step": i}
			data, _ := json.Marshal(payload)

			ev := event.Event{
				RunID:   runID,
				Type:    event.Type("step.executed"),
				Payload: data,
			}

			store.Append(context.Background(), ev)
			time.Sleep(20 * time.Millisecond)
		}
	}()

	// Receive events
	var received []event.Event
	for i := 0; i < 3; i++ {
		select {
		case ev := <-ch:
			received = append(received, ev)
		case <-time.After(2 * time.Second):
			t.Fatalf("timeout waiting for event %d", i+1)
		}
	}

	assert.Len(t, received, 3)
	for i, ev := range received {
		assert.Equal(t, runID, ev.RunID)
		assert.Equal(t, event.Type("step.executed"), ev.Type)
		assert.Equal(t, uint64(i+1), ev.Sequence)
	}
}

func TestEventStore_SubscribeCancellation(t *testing.T) {
	_, js := startTestServer(t)
	store := NewEventStore(js, "AGENT_EVENTS")

	ctx, cancel := context.WithCancel(context.Background())

	runID := "test-run-6"

	// Subscribe
	ch, err := store.Subscribe(ctx, runID)
	require.NoError(t, err)

	// Cancel context
	cancel()

	// Wait for cleanup
	time.Sleep(100 * time.Millisecond)

	// Channel should be closed
	_, ok := <-ch
	assert.False(t, ok, "channel should be closed")
}

func TestEventStore_MultipleRuns(t *testing.T) {
	_, js := startTestServer(t)
	store := NewEventStore(js, "AGENT_EVENTS")

	ctx := context.Background()

	// Append events for different runs
	for i := 1; i <= 3; i++ {
		runID := string(rune('A' + i - 1))
		payload := map[string]string{"run": runID}
		data, _ := json.Marshal(payload)

		ev := event.Event{
			RunID:   runID,
			Type:    event.Type("run.started"),
			Payload: data,
		}

		err := store.Append(ctx, ev)
		require.NoError(t, err)
	}

	// Wait for JetStream to persist
	time.Sleep(100 * time.Millisecond)

	// Load events for each run
	for i := 1; i <= 3; i++ {
		runID := string(rune('A' + i - 1))
		events, err := store.LoadEvents(ctx, runID)
		require.NoError(t, err)
		require.Len(t, events, 1)
		assert.Equal(t, runID, events[0].RunID)
	}
}

func TestEventStore_EventSubject(t *testing.T) {
	_, js := startTestServer(t)

	tests := []struct {
		name        string
		config      EventStoreConfig
		runID       string
		wantSubject string
	}{
		{
			name: "default prefix",
			config: EventStoreConfig{
				StreamName: "AGENT_EVENTS",
			},
			runID:       "run-1",
			wantSubject: "agent.events.run-1",
		},
		{
			name: "custom prefix",
			config: EventStoreConfig{
				StreamName:    "AGENT_EVENTS",
				SubjectPrefix: "custom.prefix",
			},
			runID:       "run-2",
			wantSubject: "custom.prefix.run-2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := NewEventStoreWithConfig(js, tt.config)
			subject := store.eventSubject(tt.runID)
			assert.Equal(t, tt.wantSubject, subject)
		})
	}
}

func TestEventStore_SequenceIncrement(t *testing.T) {
	_, js := startTestServer(t)
	store := NewEventStore(js, "AGENT_EVENTS")

	ctx := context.Background()
	runID := "test-run-sequence"

	// Append events in batches
	for batch := 0; batch < 3; batch++ {
		var events []event.Event
		for i := 0; i < 2; i++ {
			payload := map[string]int{"batch": batch, "item": i}
			data, _ := json.Marshal(payload)

			events = append(events, event.Event{
				RunID:   runID,
				Type:    event.Type("batch.item"),
				Payload: data,
			})
		}

		err := store.Append(ctx, events...)
		require.NoError(t, err)
	}

	// Wait for JetStream to persist
	time.Sleep(100 * time.Millisecond)

	// Load all events
	loaded, err := store.LoadEvents(ctx, runID)
	require.NoError(t, err)
	require.Len(t, loaded, 6)

	// Verify sequences are sequential
	for i, ev := range loaded {
		assert.Equal(t, uint64(i+1), ev.Sequence)
	}
}

func BenchmarkEventStore_Append(b *testing.B) {
	opts := natstest.DefaultTestOptions
	opts.JetStream = true
	opts.Port = -1
	opts.StoreDir = b.TempDir()

	srv := natstest.RunServer(&opts)
	defer srv.Shutdown()

	nc, _ := nats.Connect(srv.ClientURL())
	defer nc.Close()

	js, _ := nc.JetStream()
	js.AddStream(&nats.StreamConfig{
		Name:     "AGENT_EVENTS",
		Subjects: []string{"agent.events.>"},
		Storage:  nats.MemoryStorage,
	})

	store := NewEventStore(js, "AGENT_EVENTS")
	ctx := context.Background()

	payload := map[string]string{"test": "data"}
	data, _ := json.Marshal(payload)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ev := event.Event{
			RunID:   "bench-run",
			Type:    event.Type("test.event"),
			Payload: data,
		}
		store.Append(ctx, ev)
	}
}

func BenchmarkEventStore_LoadEvents(b *testing.B) {
	opts := natstest.DefaultTestOptions
	opts.JetStream = true
	opts.Port = -1
	opts.StoreDir = b.TempDir()

	srv := natstest.RunServer(&opts)
	defer srv.Shutdown()

	nc, _ := nats.Connect(srv.ClientURL())
	defer nc.Close()

	js, _ := nc.JetStream()
	js.AddStream(&nats.StreamConfig{
		Name:     "AGENT_EVENTS",
		Subjects: []string{"agent.events.>"},
		Storage:  nats.MemoryStorage,
	})

	store := NewEventStore(js, "AGENT_EVENTS")
	ctx := context.Background()

	// Prepopulate with events
	runID := "bench-run"
	for i := 0; i < 100; i++ {
		payload := map[string]int{"step": i}
		data, _ := json.Marshal(payload)

		ev := event.Event{
			RunID:   runID,
			Type:    event.Type("step.executed"),
			Payload: data,
		}
		store.Append(ctx, ev)
	}

	time.Sleep(100 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		store.LoadEvents(ctx, runID)
	}
}

// TestEventStore_ServerRestart simulates server restart scenario.
func TestEventStore_ServerRestart(t *testing.T) {
	// This test demonstrates that events are durable in JetStream
	// For actual persistence across restarts, use FileStorage instead of MemoryStorage

	storeDir := t.TempDir()

	// Start first server
	opts := natstest.DefaultTestOptions
	opts.JetStream = true
	opts.Port = -1
	opts.StoreDir = storeDir

	srv1 := natstest.RunServer(&opts)

	nc1, err := nats.Connect(srv1.ClientURL())
	require.NoError(t, err)

	js1, err := nc1.JetStream()
	require.NoError(t, err)

	_, err = js1.AddStream(&nats.StreamConfig{
		Name:     "AGENT_EVENTS",
		Subjects: []string{"agent.events.>"},
		Storage:  nats.FileStorage, // Use file storage for persistence
	})
	require.NoError(t, err)

	store1 := NewEventStore(js1, "AGENT_EVENTS")

	// Append events
	ctx := context.Background()
	runID := "persistent-run"

	payload := map[string]string{"action": "start"}
	data, _ := json.Marshal(payload)

	ev := event.Event{
		RunID:   runID,
		Type:    event.Type("run.started"),
		Payload: data,
	}

	err = store1.Append(ctx, ev)
	require.NoError(t, err)

	// Close connection and shutdown server
	nc1.Close()
	srv1.Shutdown()

	// Start second server with same store directory
	srv2 := natstest.RunServer(&opts)
	defer srv2.Shutdown()

	nc2, err := nats.Connect(srv2.ClientURL())
	require.NoError(t, err)
	defer nc2.Close()

	js2, err := nc2.JetStream()
	require.NoError(t, err)

	store2 := NewEventStore(js2, "AGENT_EVENTS")

	// Load events from new connection
	loaded, err := store2.LoadEvents(ctx, runID)
	require.NoError(t, err)
	require.Len(t, loaded, 1)

	assert.Equal(t, runID, loaded[0].RunID)
	assert.Equal(t, event.Type("run.started"), loaded[0].Type)
}

// --- Additional tests ---

func TestEventStore_AppendEmptySlice(t *testing.T) {
	_, js := startTestServer(t)
	store := NewEventStore(js, "AGENT_EVENTS")

	ctx := context.Background()

	// Append with no events should be a no-op
	err := store.Append(ctx)
	require.NoError(t, err)
}

func TestEventStore_LoadEvents_EmptyRun(t *testing.T) {
	_, js := startTestServer(t)
	store := NewEventStore(js, "AGENT_EVENTS")

	ctx := context.Background()

	// Load events for a run that has no events
	events, err := store.LoadEvents(ctx, "nonexistent-run")
	require.NoError(t, err)
	assert.Empty(t, events)
}

func TestEventStore_LoadEventsFrom_NoMatch(t *testing.T) {
	_, js := startTestServer(t)
	store := NewEventStore(js, "AGENT_EVENTS")

	ctx := context.Background()
	runID := "test-run-from-nomatch"

	// Append 3 events
	for i := 0; i < 3; i++ {
		payload := map[string]int{"step": i}
		data, _ := json.Marshal(payload)
		ev := event.Event{
			RunID:   runID,
			Type:    event.Type("step.executed"),
			Payload: data,
		}
		err := store.Append(ctx, ev)
		require.NoError(t, err)
	}

	time.Sleep(100 * time.Millisecond)

	// Load from sequence beyond what exists
	loaded, err := store.LoadEventsFrom(ctx, runID, 100)
	require.NoError(t, err)
	assert.Empty(t, loaded)
}

func TestEventStore_AppendMultipleRunsSameCall(t *testing.T) {
	_, js := startTestServer(t)
	store := NewEventStore(js, "AGENT_EVENTS")

	ctx := context.Background()

	// Append events for two different runs in a single call
	events := []event.Event{
		{
			RunID:   "run-A",
			Type:    event.Type("run.started"),
			Payload: json.RawMessage(`{"run":"A"}`),
		},
		{
			RunID:   "run-B",
			Type:    event.Type("run.started"),
			Payload: json.RawMessage(`{"run":"B"}`),
		},
	}

	err := store.Append(ctx, events...)
	require.NoError(t, err)

	// Each run should have sequence 1
	assert.Equal(t, uint64(1), events[0].Sequence)
	assert.Equal(t, uint64(1), events[1].Sequence)
}

func TestEventStore_VersionDefaults(t *testing.T) {
	_, js := startTestServer(t)
	store := NewEventStore(js, "AGENT_EVENTS")

	ctx := context.Background()

	ev := event.Event{
		RunID:   "test-run-version",
		Type:    event.Type("test.event"),
		Payload: json.RawMessage(`{}`),
	}

	// Version should be 0 before append
	assert.Equal(t, 0, ev.Version)

	err := store.Append(ctx, ev)
	require.NoError(t, err)

	// After append, version should default to 1
	// But since ev is a copy, check via LoadEvents
	time.Sleep(100 * time.Millisecond)

	loaded, err := store.LoadEvents(ctx, "test-run-version")
	require.NoError(t, err)
	require.Len(t, loaded, 1)
	assert.Equal(t, 1, loaded[0].Version)
}

func TestEventStore_IDAssignment(t *testing.T) {
	_, js := startTestServer(t)
	store := NewEventStore(js, "AGENT_EVENTS")

	ctx := context.Background()

	// Event with pre-set ID
	events := []event.Event{
		{
			ID:      "custom-id-1",
			RunID:   "test-run-id",
			Type:    event.Type("test.event"),
			Payload: json.RawMessage(`{}`),
		},
		{
			RunID:   "test-run-id",
			Type:    event.Type("test.event"),
			Payload: json.RawMessage(`{}`),
		},
	}

	err := store.Append(ctx, events...)
	require.NoError(t, err)

	// Custom ID should be preserved
	assert.Equal(t, "custom-id-1", events[0].ID)
	// Auto-generated ID should be non-empty
	assert.NotEmpty(t, events[1].ID)
}

func TestEventStore_WrapError_Nil(t *testing.T) {
	store := &EventStore{}
	err := store.wrapError(nil)
	assert.Nil(t, err)
}

func TestEventStore_WrapError_Timeout(t *testing.T) {
	store := &EventStore{}
	err := store.wrapError(context.DeadlineExceeded)
	assert.ErrorIs(t, err, event.ErrOperationTimeout)
	assert.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestEventStore_WrapError_NATSTimeout(t *testing.T) {
	store := &EventStore{}
	err := store.wrapError(nats.ErrTimeout)
	assert.ErrorIs(t, err, event.ErrOperationTimeout)
}

func TestEventStore_WrapError_GenericError(t *testing.T) {
	store := &EventStore{}
	original := assert.AnError
	err := store.wrapError(original)
	assert.ErrorIs(t, err, event.ErrConnectionFailed)
}

func TestEventStore_InterfaceCompliance(t *testing.T) {
	var _ event.Store = (*EventStore)(nil)
}

func TestNewEventStoreWithConfig_DefaultPrefix(t *testing.T) {
	_, js := startTestServer(t)
	store := NewEventStoreWithConfig(js, EventStoreConfig{
		StreamName: "AGENT_EVENTS",
	})

	subject := store.eventSubject("run-1")
	assert.Equal(t, "agent.events.run-1", subject)
}

func TestNewEventStoreWithConfig_EmptyPrefix(t *testing.T) {
	_, js := startTestServer(t)
	store := NewEventStoreWithConfig(js, EventStoreConfig{
		StreamName:    "AGENT_EVENTS",
		SubjectPrefix: "",
	})

	// Empty prefix should default to "agent.events"
	subject := store.eventSubject("run-1")
	assert.Equal(t, "agent.events.run-1", subject)
}

// Example demonstrates typical usage of the NATS event store.
func ExampleNewEventStore() {
	// Connect to NATS
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		panic(err)
	}
	defer nc.Close()

	// Create JetStream context
	js, err := nc.JetStream()
	if err != nil {
		panic(err)
	}

	// Create or update stream
	js.AddStream(&nats.StreamConfig{
		Name:     "AGENT_EVENTS",
		Subjects: []string{"agent.events.>"},
		Storage:  nats.FileStorage,
	})

	// Create event store
	store := NewEventStore(js, "AGENT_EVENTS")

	// Append events
	ctx := context.Background()
	payload := map[string]string{"goal": "process data"}
	data, _ := json.Marshal(payload)

	ev := event.Event{
		RunID:   "example-run",
		Type:    event.Type("run.started"),
		Payload: data,
	}

	err = store.Append(ctx, ev)
	if err != nil {
		panic(err)
	}

	// Load events
	events, err := store.LoadEvents(ctx, "example-run")
	if err != nil {
		panic(err)
	}

	for _, e := range events {
		_ = e // Process event
	}
}
