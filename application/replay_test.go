package application_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/application"
	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/event"
)

// mockEventStore implements event.Store for testing.
type mockEventStore struct {
	appendFn         func(ctx context.Context, events ...event.Event) error
	loadEventsFn     func(ctx context.Context, runID string) ([]event.Event, error)
	loadEventsFromFn func(ctx context.Context, runID string, fromSeq uint64) ([]event.Event, error)
	subscribeFn      func(ctx context.Context, runID string) (<-chan event.Event, error)
}

func (m *mockEventStore) Append(ctx context.Context, events ...event.Event) error {
	if m.appendFn != nil {
		return m.appendFn(ctx, events...)
	}
	return nil
}

func (m *mockEventStore) LoadEvents(ctx context.Context, runID string) ([]event.Event, error) {
	if m.loadEventsFn != nil {
		return m.loadEventsFn(ctx, runID)
	}
	return []event.Event{}, nil
}

func (m *mockEventStore) LoadEventsFrom(ctx context.Context, runID string, fromSeq uint64) ([]event.Event, error) {
	if m.loadEventsFromFn != nil {
		return m.loadEventsFromFn(ctx, runID, fromSeq)
	}
	return []event.Event{}, nil
}

func (m *mockEventStore) Subscribe(ctx context.Context, runID string) (<-chan event.Event, error) {
	if m.subscribeFn != nil {
		return m.subscribeFn(ctx, runID)
	}
	return nil, nil
}

func createTestEvent(runID string, eventType event.Type, payload any, seq uint64) event.Event {
	data, _ := json.Marshal(payload)
	return event.Event{
		ID:        "evt-" + runID,
		RunID:     runID,
		Type:      eventType,
		Timestamp: time.Now(),
		Payload:   data,
		Sequence:  seq,
	}
}

func TestNewReplay(t *testing.T) {
	t.Parallel()

	store := &mockEventStore{}
	replay := application.NewReplay(store)

	if replay == nil {
		t.Error("NewReplay should return non-nil replay")
	}
}

func TestReplay_ReconstructRun(t *testing.T) {
	t.Parallel()

	t.Run("load error", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return nil, errors.New("load error")
			},
		}
		replay := application.NewReplay(store)

		_, err := replay.ReconstructRun(context.Background(), "run-1")
		if err == nil {
			t.Error("ReconstructRun() should return error")
		}
	})

	t.Run("no events", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{}, nil
			},
		}
		replay := application.NewReplay(store)

		_, err := replay.ReconstructRun(context.Background(), "run-1")
		if !errors.Is(err, event.ErrRunNotFound) {
			t.Errorf("ReconstructRun() error = %v, want %v", err, event.ErrRunNotFound)
		}
	})

	t.Run("reconstruct from run.started", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test goal",
						Vars: map[string]any{"key": "value"},
					}, 1),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if run.Goal != "Test goal" {
			t.Errorf("Run.Goal = %s, want 'Test goal'", run.Goal)
		}
	})

	t.Run("reconstruct with state transitions", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test goal",
					}, 1),
					createTestEvent(runID, event.TypeStateTransitioned, event.StateTransitionedPayload{
						FromState: agent.StateIntake,
						ToState:   agent.StateExplore,
						Reason:    "begin exploration",
					}, 2),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if run.CurrentState != agent.StateExplore {
			t.Errorf("Run.CurrentState = %s, want %s", run.CurrentState, agent.StateExplore)
		}
	})

	t.Run("reconstruct with completion", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test goal",
					}, 1),
					createTestEvent(runID, event.TypeRunCompleted, event.RunCompletedPayload{
						Result:   json.RawMessage(`{"success": true}`),
						Duration: 5 * time.Second,
					}, 2),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if run.CurrentState != agent.StateDone {
			t.Errorf("Run.CurrentState = %s, want %s", run.CurrentState, agent.StateDone)
		}
	})

	t.Run("reconstruct with failure", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test goal",
					}, 1),
					createTestEvent(runID, event.TypeRunFailed, event.RunFailedPayload{
						Error:    "something went wrong",
						State:    agent.StateAct,
						Duration: 3 * time.Second,
					}, 2),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if run.CurrentState != agent.StateFailed {
			t.Errorf("Run.CurrentState = %s, want %s", run.CurrentState, agent.StateFailed)
		}
	})

	t.Run("reconstruct with pause and resume", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test goal",
					}, 1),
					createTestEvent(runID, event.TypeRunPaused, nil, 2),
					createTestEvent(runID, event.TypeRunResumed, nil, 3),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if run == nil {
			t.Error("Run should not be nil")
		}
	})

	t.Run("reconstruct with evidence", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test goal",
					}, 1),
					createTestEvent(runID, event.TypeEvidenceAdded, event.EvidenceAddedPayload{
						Type:    "observation",
						Source:  "read_file",
						Content: json.RawMessage(`{"data": "test"}`),
					}, 2),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if len(run.Evidence) != 1 {
			t.Errorf("len(Run.Evidence) = %d, want 1", len(run.Evidence))
		}
	})

	t.Run("reconstruct with variable set", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test goal",
					}, 1),
					createTestEvent(runID, event.TypeVariableSet, event.VariableSetPayload{
						Key:   "myVar",
						Value: "myValue",
					}, 2),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if run.Vars["myVar"] != "myValue" {
			t.Errorf("Run.Vars[myVar] = %v, want 'myValue'", run.Vars["myVar"])
		}
	})

	t.Run("handles tool and audit events", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test goal",
					}, 1),
					createTestEvent(runID, event.TypeToolCalled, event.ToolCalledPayload{
						ToolName: "read_file",
					}, 2),
					createTestEvent(runID, event.TypeToolSucceeded, event.ToolSucceededPayload{
						ToolName: "read_file",
					}, 3),
					createTestEvent(runID, event.TypeDecisionMade, event.DecisionMadePayload{
						DecisionType: "call_tool",
					}, 4),
					createTestEvent(runID, event.TypeBudgetConsumed, event.BudgetConsumedPayload{
						BudgetName: "calls",
					}, 5),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if run == nil {
			t.Error("Run should not be nil")
		}
	})
}

func TestReplay_ReconstructRunFrom(t *testing.T) {
	t.Parallel()

	t.Run("load error", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFromFn: func(ctx context.Context, runID string, fromSeq uint64) ([]event.Event, error) {
				return nil, errors.New("load error")
			},
		}
		replay := application.NewReplay(store)

		_, err := replay.ReconstructRunFrom(context.Background(), "run-1", 5)
		if err == nil {
			t.Error("ReconstructRunFrom() should return error")
		}
	})

	t.Run("no events", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFromFn: func(ctx context.Context, runID string, fromSeq uint64) ([]event.Event, error) {
				return []event.Event{}, nil
			},
		}
		replay := application.NewReplay(store)

		_, err := replay.ReconstructRunFrom(context.Background(), "run-1", 5)
		if !errors.Is(err, event.ErrRunNotFound) {
			t.Errorf("ReconstructRunFrom() error = %v, want %v", err, event.ErrRunNotFound)
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFromFn: func(ctx context.Context, runID string, fromSeq uint64) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test goal",
					}, 5),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRunFrom(context.Background(), "run-1", 5)
		if err != nil {
			t.Fatalf("ReconstructRunFrom() error = %v", err)
		}
		if run == nil {
			t.Error("Run should not be nil")
		}
	})
}

func TestReplay_NewEventIterator(t *testing.T) {
	t.Parallel()

	t.Run("load error", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return nil, errors.New("load error")
			},
		}
		replay := application.NewReplay(store)

		_, err := replay.NewEventIterator(context.Background(), "run-1")
		if err == nil {
			t.Error("NewEventIterator() should return error")
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					{ID: "e1", Sequence: 1},
					{ID: "e2", Sequence: 2},
					{ID: "e3", Sequence: 3},
				}, nil
			},
		}
		replay := application.NewReplay(store)

		iter, err := replay.NewEventIterator(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("NewEventIterator() error = %v", err)
		}
		if iter.Len() != 3 {
			t.Errorf("Len() = %d, want 3", iter.Len())
		}
	})
}

func TestEventIterator(t *testing.T) {
	t.Parallel()

	store := &mockEventStore{
		loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
			return []event.Event{
				{ID: "e1", Sequence: 1},
				{ID: "e2", Sequence: 2},
				{ID: "e3", Sequence: 3},
			}, nil
		},
	}
	replay := application.NewReplay(store)

	iter, _ := replay.NewEventIterator(context.Background(), "run-1")

	t.Run("next", func(t *testing.T) {
		e := iter.Next()
		if e == nil {
			t.Fatal("Next() should return event")
		}
		if e.ID != "e1" {
			t.Errorf("Event.ID = %s, want e1", e.ID)
		}
	})

	t.Run("index", func(t *testing.T) {
		if iter.Index() != 1 {
			t.Errorf("Index() = %d, want 1", iter.Index())
		}
	})

	t.Run("peek", func(t *testing.T) {
		e := iter.Peek()
		if e == nil {
			t.Fatal("Peek() should return event")
		}
		if e.ID != "e2" {
			t.Errorf("Event.ID = %s, want e2", e.ID)
		}
		// Peek should not advance
		if iter.Index() != 1 {
			t.Errorf("Index() = %d, want 1", iter.Index())
		}
	})

	t.Run("reset", func(t *testing.T) {
		iter.Reset()
		if iter.Index() != 0 {
			t.Errorf("Index() = %d, want 0", iter.Index())
		}
	})

	t.Run("iterate to end", func(t *testing.T) {
		iter.Reset()
		count := 0
		for iter.Next() != nil {
			count++
		}
		if count != 3 {
			t.Errorf("count = %d, want 3", count)
		}
		// Next on exhausted iterator
		if iter.Next() != nil {
			t.Error("Next() should return nil after exhausted")
		}
		// Peek on exhausted iterator
		if iter.Peek() != nil {
			t.Error("Peek() should return nil after exhausted")
		}
	})
}

func TestReplay_NewTimeline(t *testing.T) {
	t.Parallel()

	t.Run("load error", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return nil, errors.New("load error")
			},
		}
		replay := application.NewReplay(store)

		_, err := replay.NewTimeline(context.Background(), "run-1")
		if err == nil {
			t.Error("NewTimeline() should return error")
		}
	})

	t.Run("success", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{}, nil
			},
		}
		replay := application.NewReplay(store)

		tl, err := replay.NewTimeline(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("NewTimeline() error = %v", err)
		}
		if tl == nil {
			t.Error("Timeline should not be nil")
		}
	})
}

func TestTimeline_Duration(t *testing.T) {
	t.Parallel()

	t.Run("no events", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{}, nil
			},
		}
		replay := application.NewReplay(store)
		tl, _ := replay.NewTimeline(context.Background(), "run-1")

		if tl.Duration() != 0 {
			t.Errorf("Duration() = %v, want 0", tl.Duration())
		}
	})

	t.Run("one event", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					{Timestamp: time.Now()},
				}, nil
			},
		}
		replay := application.NewReplay(store)
		tl, _ := replay.NewTimeline(context.Background(), "run-1")

		if tl.Duration() != 0 {
			t.Errorf("Duration() = %v, want 0", tl.Duration())
		}
	})

	t.Run("multiple events", func(t *testing.T) {
		t.Parallel()

		start := time.Now()
		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					{Timestamp: start},
					{Timestamp: start.Add(5 * time.Second)},
				}, nil
			},
		}
		replay := application.NewReplay(store)
		tl, _ := replay.NewTimeline(context.Background(), "run-1")

		if tl.Duration() != 5*time.Second {
			t.Errorf("Duration() = %v, want 5s", tl.Duration())
		}
	})
}

func TestTimeline_EventsInRange(t *testing.T) {
	t.Parallel()

	start := time.Date(2024, 1, 1, 10, 0, 0, 0, time.UTC)
	store := &mockEventStore{
		loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
			return []event.Event{
				{ID: "e1", Timestamp: start},
				{ID: "e2", Timestamp: start.Add(1 * time.Minute)},
				{ID: "e3", Timestamp: start.Add(2 * time.Minute)},
				{ID: "e4", Timestamp: start.Add(3 * time.Minute)},
			}, nil
		},
	}
	replay := application.NewReplay(store)
	tl, _ := replay.NewTimeline(context.Background(), "run-1")

	t.Run("all events with zero times", func(t *testing.T) {
		events := tl.EventsInRange(time.Time{}, time.Time{})
		if len(events) != 4 {
			t.Errorf("len(events) = %d, want 4", len(events))
		}
	})

	t.Run("from time only", func(t *testing.T) {
		events := tl.EventsInRange(start.Add(90*time.Second), time.Time{})
		if len(events) != 2 {
			t.Errorf("len(events) = %d, want 2", len(events))
		}
	})

	t.Run("to time only", func(t *testing.T) {
		events := tl.EventsInRange(time.Time{}, start.Add(90*time.Second))
		if len(events) != 2 {
			t.Errorf("len(events) = %d, want 2", len(events))
		}
	})

	t.Run("both times", func(t *testing.T) {
		events := tl.EventsInRange(start.Add(30*time.Second), start.Add(150*time.Second))
		if len(events) != 2 {
			t.Errorf("len(events) = %d, want 2", len(events))
		}
	})
}

func TestTimeline_EventsByType(t *testing.T) {
	t.Parallel()

	store := &mockEventStore{
		loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
			return []event.Event{
				{Type: event.TypeRunStarted},
				{Type: event.TypeStateTransitioned},
				{Type: event.TypeStateTransitioned},
				{Type: event.TypeRunCompleted},
			}, nil
		},
	}
	replay := application.NewReplay(store)
	tl, _ := replay.NewTimeline(context.Background(), "run-1")

	events := tl.EventsByType(event.TypeStateTransitioned)
	if len(events) != 2 {
		t.Errorf("len(events) = %d, want 2", len(events))
	}
}

func TestTimeline_StateTransitions(t *testing.T) {
	t.Parallel()

	transitionPayload1, _ := json.Marshal(event.StateTransitionedPayload{
		FromState: agent.StateIntake,
		ToState:   agent.StateExplore,
		Reason:    "begin",
	})
	transitionPayload2, _ := json.Marshal(event.StateTransitionedPayload{
		FromState: agent.StateExplore,
		ToState:   agent.StateDecide,
		Reason:    "ready",
	})

	store := &mockEventStore{
		loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
			return []event.Event{
				{Type: event.TypeRunStarted},
				{Type: event.TypeStateTransitioned, Payload: transitionPayload1},
				{Type: event.TypeStateTransitioned, Payload: transitionPayload2},
			}, nil
		},
	}
	replay := application.NewReplay(store)
	tl, _ := replay.NewTimeline(context.Background(), "run-1")

	transitions := tl.StateTransitions()
	if len(transitions) != 2 {
		t.Errorf("len(transitions) = %d, want 2", len(transitions))
	}
	if transitions[0].From != agent.StateIntake {
		t.Errorf("transitions[0].From = %s, want %s", transitions[0].From, agent.StateIntake)
	}
	if transitions[0].To != agent.StateExplore {
		t.Errorf("transitions[0].To = %s, want %s", transitions[0].To, agent.StateExplore)
	}
}

func TestTimeline_ToolCalls(t *testing.T) {
	t.Parallel()

	calledPayload, _ := json.Marshal(event.ToolCalledPayload{
		ToolName: "read_file",
		Input:    json.RawMessage(`{"path": "/test"}`),
		State:    agent.StateExplore,
	})
	succeededPayload, _ := json.Marshal(event.ToolSucceededPayload{
		ToolName: "read_file",
		Output:   json.RawMessage(`{"content": "data"}`),
		Duration: 100 * time.Millisecond,
		Cached:   false,
	})
	failedPayload, _ := json.Marshal(event.ToolFailedPayload{
		ToolName: "write_file",
		Error:    "permission denied",
		Duration: 50 * time.Millisecond,
	})

	store := &mockEventStore{
		loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
			return []event.Event{
				{Type: event.TypeToolCalled, Payload: calledPayload, Sequence: 1},
				{Type: event.TypeToolSucceeded, Payload: succeededPayload, Sequence: 2},
				{Type: event.TypeToolCalled, Payload: json.RawMessage(`{"tool_name": "write_file"}`), Sequence: 3},
				{Type: event.TypeToolFailed, Payload: failedPayload, Sequence: 4},
			}, nil
		},
	}
	replay := application.NewReplay(store)
	tl, _ := replay.NewTimeline(context.Background(), "run-1")

	calls := tl.ToolCalls()
	if len(calls) != 2 {
		t.Errorf("len(calls) = %d, want 2", len(calls))
	}

	// Find the successful call
	var successCall *application.ToolCall
	for i := range calls {
		if calls[i].ToolName == "read_file" {
			successCall = &calls[i]
			break
		}
	}
	if successCall == nil {
		t.Fatal("read_file call not found")
	}
	if !successCall.Success {
		t.Error("read_file call should be successful")
	}
}

// TestReplay_ApplyEvents_MalformedPayloads tests handling of invalid JSON payloads.
func TestReplay_ApplyEvents_MalformedPayloads(t *testing.T) {
	t.Parallel()

	t.Run("malformed run.started payload", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					{
						ID:        "evt-1",
						RunID:     runID,
						Type:      event.TypeRunStarted,
						Payload:   json.RawMessage(`{invalid json`),
						Timestamp: time.Now(),
						Sequence:  1,
					},
				}, nil
			},
		}
		replay := application.NewReplay(store)

		_, err := replay.ReconstructRun(context.Background(), "run-1")
		if err == nil {
			t.Error("ReconstructRun() should return error for malformed payload")
		}
	})

	t.Run("malformed run.completed payload", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test",
					}, 1),
					{
						ID:        "evt-2",
						RunID:     runID,
						Type:      event.TypeRunCompleted,
						Payload:   json.RawMessage(`{broken`),
						Timestamp: time.Now(),
						Sequence:  2,
					},
				}, nil
			},
		}
		replay := application.NewReplay(store)

		_, err := replay.ReconstructRun(context.Background(), "run-1")
		if err == nil {
			t.Error("ReconstructRun() should return error for malformed run.completed payload")
		}
	})

	t.Run("malformed run.failed payload", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test",
					}, 1),
					{
						ID:        "evt-2",
						RunID:     runID,
						Type:      event.TypeRunFailed,
						Payload:   json.RawMessage(`not valid json`),
						Timestamp: time.Now(),
						Sequence:  2,
					},
				}, nil
			},
		}
		replay := application.NewReplay(store)

		_, err := replay.ReconstructRun(context.Background(), "run-1")
		if err == nil {
			t.Error("ReconstructRun() should return error for malformed run.failed payload")
		}
	})

	t.Run("malformed state.transitioned payload", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test",
					}, 1),
					{
						ID:        "evt-2",
						RunID:     runID,
						Type:      event.TypeStateTransitioned,
						Payload:   json.RawMessage(`{{{`),
						Timestamp: time.Now(),
						Sequence:  2,
					},
				}, nil
			},
		}
		replay := application.NewReplay(store)

		_, err := replay.ReconstructRun(context.Background(), "run-1")
		if err == nil {
			t.Error("ReconstructRun() should return error for malformed state.transitioned payload")
		}
	})

	t.Run("malformed evidence.added payload", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test",
					}, 1),
					{
						ID:        "evt-2",
						RunID:     runID,
						Type:      event.TypeEvidenceAdded,
						Payload:   json.RawMessage(`[[[`),
						Timestamp: time.Now(),
						Sequence:  2,
					},
				}, nil
			},
		}
		replay := application.NewReplay(store)

		_, err := replay.ReconstructRun(context.Background(), "run-1")
		if err == nil {
			t.Error("ReconstructRun() should return error for malformed evidence.added payload")
		}
	})

	t.Run("malformed variable.set payload", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test",
					}, 1),
					{
						ID:        "evt-2",
						RunID:     runID,
						Type:      event.TypeVariableSet,
						Payload:   json.RawMessage(`}`),
						Timestamp: time.Now(),
						Sequence:  2,
					},
				}, nil
			},
		}
		replay := application.NewReplay(store)

		_, err := replay.ReconstructRun(context.Background(), "run-1")
		if err == nil {
			t.Error("ReconstructRun() should return error for malformed variable.set payload")
		}
	})
}

// TestReplay_ApplyEvents_EventsWithoutRun tests events that occur when run is nil.
func TestReplay_ApplyEvents_EventsWithoutRun(t *testing.T) {
	t.Parallel()

	t.Run("events without run.started are skipped", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunCompleted, event.RunCompletedPayload{
						Result:   json.RawMessage(`{"done": true}`),
						Duration: 1 * time.Second,
					}, 1),
					createTestEvent(runID, event.TypeStateTransitioned, event.StateTransitionedPayload{
						FromState: agent.StateIntake,
						ToState:   agent.StateExplore,
					}, 2),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		_, err := replay.ReconstructRun(context.Background(), "run-1")
		if !errors.Is(err, event.ErrRunNotFound) {
			t.Errorf("ReconstructRun() error = %v, want %v", err, event.ErrRunNotFound)
		}
	})
}

// TestReplay_ApplyEvents_RunStartedVariants tests different run.started scenarios.
func TestReplay_ApplyEvents_RunStartedVariants(t *testing.T) {
	t.Parallel()

	t.Run("run.started with nil vars", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test",
						Vars: nil,
					}, 1),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if run.Vars == nil {
			t.Error("Run.Vars should be initialized to empty map")
		}
		if len(run.Vars) != 0 {
			t.Errorf("len(Run.Vars) = %d, want 0", len(run.Vars))
		}
	})

	t.Run("run.started with empty vars", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test",
						Vars: map[string]any{},
					}, 1),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if run.Vars == nil {
			t.Error("Run.Vars should not be nil")
		}
	})

	t.Run("run.started with multiple vars", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test",
						Vars: map[string]any{
							"key1": "value1",
							"key2": 42,
							"key3": true,
						},
					}, 1),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if len(run.Vars) != 3 {
			t.Errorf("len(Run.Vars) = %d, want 3", len(run.Vars))
		}
		if run.Vars["key1"] != "value1" {
			t.Errorf("Run.Vars[key1] = %v, want 'value1'", run.Vars["key1"])
		}
	})
}

// TestReplay_ApplyEvents_ApprovalEvents tests approval event handling.
func TestReplay_ApplyEvents_ApprovalEvents(t *testing.T) {
	t.Parallel()

	t.Run("approval events are processed without error", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test",
					}, 1),
					createTestEvent(runID, event.TypeApprovalRequested, event.ApprovalRequestedPayload{
						ToolName:  "delete_file",
						Input:     json.RawMessage(`{"path": "/test"}`),
						RiskLevel: "high",
					}, 2),
					createTestEvent(runID, event.TypeApprovalGranted, event.ApprovalResultPayload{
						ToolName: "delete_file",
						Approver: "admin",
						Reason:   "safe operation",
					}, 3),
					createTestEvent(runID, event.TypeApprovalDenied, event.ApprovalResultPayload{
						ToolName: "write_file",
						Approver: "admin",
						Reason:   "unsafe",
					}, 4),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if run == nil {
			t.Error("Run should not be nil")
		}
	})
}

// TestReplay_ApplyEvents_DecisionEvents tests decision event handling.
func TestReplay_ApplyEvents_DecisionEvents(t *testing.T) {
	t.Parallel()

	t.Run("decision events are processed without error", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test",
					}, 1),
					createTestEvent(runID, event.TypeDecisionMade, event.DecisionMadePayload{
						DecisionType: "call_tool",
						ToolName:     "read_file",
						Reason:       "need data",
						Input:        json.RawMessage(`{"path": "/test"}`),
					}, 2),
					createTestEvent(runID, event.TypeDecisionMade, event.DecisionMadePayload{
						DecisionType: "transition",
						ToState:      agent.StateExplore,
						Reason:       "ready to explore",
					}, 3),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if run == nil {
			t.Error("Run should not be nil")
		}
	})
}

// TestReplay_ApplyEvents_BudgetEvents tests budget event handling.
func TestReplay_ApplyEvents_BudgetEvents(t *testing.T) {
	t.Parallel()

	t.Run("budget events are processed without error", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test",
					}, 1),
					createTestEvent(runID, event.TypeBudgetConsumed, event.BudgetConsumedPayload{
						BudgetName: "tool_calls",
						Amount:     1,
						Remaining:  99,
					}, 2),
					createTestEvent(runID, event.TypeBudgetExhausted, event.BudgetExhaustedPayload{
						BudgetName: "tool_calls",
					}, 3),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if run == nil {
			t.Error("Run should not be nil")
		}
	})
}

// TestReplay_ApplyEvents_ComplexSequence tests a complex event sequence.
func TestReplay_ApplyEvents_ComplexSequence(t *testing.T) {
	t.Parallel()

	t.Run("complex sequence with all event types", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Complex test",
						Vars: map[string]any{"initial": true},
					}, 1),
					createTestEvent(runID, event.TypeStateTransitioned, event.StateTransitionedPayload{
						FromState: agent.StateIntake,
						ToState:   agent.StateExplore,
						Reason:    "begin",
					}, 2),
					createTestEvent(runID, event.TypeEvidenceAdded, event.EvidenceAddedPayload{
						Type:    "observation",
						Source:  "sensor",
						Content: json.RawMessage(`{"temp": 20}`),
					}, 3),
					createTestEvent(runID, event.TypeVariableSet, event.VariableSetPayload{
						Key:   "temp",
						Value: 20,
					}, 4),
					createTestEvent(runID, event.TypeToolCalled, event.ToolCalledPayload{
						ToolName: "read_sensor",
						Input:    json.RawMessage(`{}`),
						State:    agent.StateExplore,
					}, 5),
					createTestEvent(runID, event.TypeToolSucceeded, event.ToolSucceededPayload{
						ToolName: "read_sensor",
						Output:   json.RawMessage(`{"value": 42}`),
						Duration: 100 * time.Millisecond,
					}, 6),
					createTestEvent(runID, event.TypeDecisionMade, event.DecisionMadePayload{
						DecisionType: "finish",
						Reason:       "goal achieved",
					}, 7),
					createTestEvent(runID, event.TypeRunCompleted, event.RunCompletedPayload{
						Result:   json.RawMessage(`{"success": true}`),
						Duration: 5 * time.Second,
					}, 8),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if run.CurrentState != agent.StateDone {
			t.Errorf("Run.CurrentState = %s, want %s", run.CurrentState, agent.StateDone)
		}
		if len(run.Evidence) != 1 {
			t.Errorf("len(Run.Evidence) = %d, want 1", len(run.Evidence))
		}
		if run.Vars["temp"] != float64(20) {
			t.Errorf("Run.Vars[temp] = %v, want 20", run.Vars["temp"])
		}
	})
}

// TestReplay_ApplyEvents_OutOfOrderEvents tests handling of out-of-order events.
func TestReplay_ApplyEvents_OutOfOrderEvents(t *testing.T) {
	t.Parallel()

	t.Run("state transition before run.started is skipped", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeStateTransitioned, event.StateTransitionedPayload{
						FromState: agent.StateIntake,
						ToState:   agent.StateExplore,
					}, 1),
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test",
					}, 2),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		// Run should be in intake state, not explore
		if run.CurrentState != agent.StateIntake {
			t.Errorf("Run.CurrentState = %s, want %s", run.CurrentState, agent.StateIntake)
		}
	})

	t.Run("evidence added before run.started is skipped", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeEvidenceAdded, event.EvidenceAddedPayload{
						Type:    "observation",
						Source:  "test",
						Content: json.RawMessage(`{}`),
					}, 1),
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test",
					}, 2),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if len(run.Evidence) != 0 {
			t.Errorf("len(Run.Evidence) = %d, want 0", len(run.Evidence))
		}
	})
}

// TestReplay_ReconstructRunFrom_PartialSequence tests reconstructing from partial event sequences.
func TestReplay_ReconstructRunFrom_PartialSequence(t *testing.T) {
	t.Parallel()

	t.Run("reconstruct from middle of sequence", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFromFn: func(ctx context.Context, runID string, fromSeq uint64) ([]event.Event, error) {
				// Simulate loading from sequence 5 onward
				if fromSeq == 5 {
					return []event.Event{
						createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
							Goal: "Test",
						}, 5),
						createTestEvent(runID, event.TypeStateTransitioned, event.StateTransitionedPayload{
							FromState: agent.StateIntake,
							ToState:   agent.StateExplore,
						}, 6),
					}, nil
				}
				return []event.Event{}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRunFrom(context.Background(), "run-1", 5)
		if err != nil {
			t.Fatalf("ReconstructRunFrom() error = %v", err)
		}
		if run.CurrentState != agent.StateExplore {
			t.Errorf("Run.CurrentState = %s, want %s", run.CurrentState, agent.StateExplore)
		}
	})
}

// TestEventIterator_EdgeCases tests iterator edge cases.
func TestEventIterator_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("empty iterator", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{}, nil
			},
		}
		replay := application.NewReplay(store)

		iter, err := replay.NewEventIterator(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("NewEventIterator() error = %v", err)
		}

		if iter.Len() != 0 {
			t.Errorf("Len() = %d, want 0", iter.Len())
		}
		if iter.Next() != nil {
			t.Error("Next() should return nil for empty iterator")
		}
		if iter.Peek() != nil {
			t.Error("Peek() should return nil for empty iterator")
		}
		if iter.Index() != 0 {
			t.Errorf("Index() = %d, want 0", iter.Index())
		}
	})

	t.Run("single event iterator", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					{ID: "e1", Sequence: 1},
				}, nil
			},
		}
		replay := application.NewReplay(store)

		iter, _ := replay.NewEventIterator(context.Background(), "run-1")

		if iter.Len() != 1 {
			t.Errorf("Len() = %d, want 1", iter.Len())
		}

		e := iter.Next()
		if e == nil || e.ID != "e1" {
			t.Error("Next() should return first event")
		}

		if iter.Next() != nil {
			t.Error("Next() should return nil after single event")
		}
	})
}

// TestTimeline_EdgeCases tests timeline edge cases.
func TestTimeline_EdgeCases(t *testing.T) {
	t.Parallel()

	t.Run("empty timeline events in range", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{}, nil
			},
		}
		replay := application.NewReplay(store)
		tl, _ := replay.NewTimeline(context.Background(), "run-1")

		events := tl.EventsInRange(time.Now(), time.Now().Add(1*time.Hour))
		if len(events) != 0 {
			t.Errorf("len(events) = %d, want 0", len(events))
		}
	})

	t.Run("events by type with no matches", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					{Type: event.TypeRunStarted},
					{Type: event.TypeRunCompleted},
				}, nil
			},
		}
		replay := application.NewReplay(store)
		tl, _ := replay.NewTimeline(context.Background(), "run-1")

		events := tl.EventsByType(event.TypeToolCalled)
		if len(events) != 0 {
			t.Errorf("len(events) = %d, want 0", len(events))
		}
	})

	t.Run("state transitions with malformed payload", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					{Type: event.TypeStateTransitioned, Payload: json.RawMessage(`invalid`)},
				}, nil
			},
		}
		replay := application.NewReplay(store)
		tl, _ := replay.NewTimeline(context.Background(), "run-1")

		transitions := tl.StateTransitions()
		if len(transitions) != 0 {
			t.Errorf("len(transitions) = %d, want 0", len(transitions))
		}
	})

	t.Run("tool calls with no matches", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					{Type: event.TypeRunStarted},
				}, nil
			},
		}
		replay := application.NewReplay(store)
		tl, _ := replay.NewTimeline(context.Background(), "run-1")

		calls := tl.ToolCalls()
		if len(calls) != 0 {
			t.Errorf("len(calls) = %d, want 0", len(calls))
		}
	})

	t.Run("tool calls with malformed payloads", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					{Type: event.TypeToolCalled, Payload: json.RawMessage(`invalid`), Sequence: 1},
					{Type: event.TypeToolSucceeded, Payload: json.RawMessage(`bad`), Sequence: 2},
					{Type: event.TypeToolFailed, Payload: json.RawMessage(`broken`), Sequence: 3},
				}, nil
			},
		}
		replay := application.NewReplay(store)
		tl, _ := replay.NewTimeline(context.Background(), "run-1")

		calls := tl.ToolCalls()
		// Should return empty slice since all payloads are malformed
		if len(calls) != 0 {
			t.Errorf("len(calls) = %d, want 0", len(calls))
		}
	})

	t.Run("tool calls with cached result", func(t *testing.T) {
		t.Parallel()

		calledPayload, _ := json.Marshal(event.ToolCalledPayload{
			ToolName: "cached_tool",
			Input:    json.RawMessage(`{}`),
			State:    agent.StateExplore,
		})
		succeededPayload, _ := json.Marshal(event.ToolSucceededPayload{
			ToolName: "cached_tool",
			Output:   json.RawMessage(`{"cached": true}`),
			Duration: 1 * time.Millisecond,
			Cached:   true,
		})

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					{Type: event.TypeToolCalled, Payload: calledPayload, Sequence: 1},
					{Type: event.TypeToolSucceeded, Payload: succeededPayload, Sequence: 2},
				}, nil
			},
		}
		replay := application.NewReplay(store)
		tl, _ := replay.NewTimeline(context.Background(), "run-1")

		calls := tl.ToolCalls()
		if len(calls) != 1 {
			t.Fatalf("len(calls) = %d, want 1", len(calls))
		}
		if !calls[0].Cached {
			t.Error("Tool call should be marked as cached")
		}
	})
}

// TestReplay_ApplyEvents_MultipleVariableSets tests multiple variable updates.
func TestReplay_ApplyEvents_MultipleVariableSets(t *testing.T) {
	t.Parallel()

	t.Run("multiple variable sets", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test",
					}, 1),
					createTestEvent(runID, event.TypeVariableSet, event.VariableSetPayload{
						Key:   "counter",
						Value: 1,
					}, 2),
					createTestEvent(runID, event.TypeVariableSet, event.VariableSetPayload{
						Key:   "counter",
						Value: 2,
					}, 3),
					createTestEvent(runID, event.TypeVariableSet, event.VariableSetPayload{
						Key:   "name",
						Value: "test",
					}, 4),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if run.Vars["counter"] != float64(2) {
			t.Errorf("Run.Vars[counter] = %v, want 2", run.Vars["counter"])
		}
		if run.Vars["name"] != "test" {
			t.Errorf("Run.Vars[name] = %v, want 'test'", run.Vars["name"])
		}
	})
}

// TestReplay_ApplyEvents_MultipleEvidence tests multiple evidence additions.
func TestReplay_ApplyEvents_MultipleEvidence(t *testing.T) {
	t.Parallel()

	t.Run("multiple evidence additions", func(t *testing.T) {
		t.Parallel()

		store := &mockEventStore{
			loadEventsFn: func(ctx context.Context, runID string) ([]event.Event, error) {
				return []event.Event{
					createTestEvent(runID, event.TypeRunStarted, event.RunStartedPayload{
						Goal: "Test",
					}, 1),
					createTestEvent(runID, event.TypeEvidenceAdded, event.EvidenceAddedPayload{
						Type:    "observation",
						Source:  "sensor1",
						Content: json.RawMessage(`{"value": 1}`),
					}, 2),
					createTestEvent(runID, event.TypeEvidenceAdded, event.EvidenceAddedPayload{
						Type:    "measurement",
						Source:  "sensor2",
						Content: json.RawMessage(`{"value": 2}`),
					}, 3),
					createTestEvent(runID, event.TypeEvidenceAdded, event.EvidenceAddedPayload{
						Type:    "observation",
						Source:  "sensor3",
						Content: json.RawMessage(`{"value": 3}`),
					}, 4),
				}, nil
			},
		}
		replay := application.NewReplay(store)

		run, err := replay.ReconstructRun(context.Background(), "run-1")
		if err != nil {
			t.Fatalf("ReconstructRun() error = %v", err)
		}
		if len(run.Evidence) != 3 {
			t.Errorf("len(Run.Evidence) = %d, want 3", len(run.Evidence))
		}
		if run.Evidence[0].Source != "sensor1" {
			t.Errorf("Evidence[0].Source = %s, want 'sensor1'", run.Evidence[0].Source)
		}
		if run.Evidence[1].Type != agent.EvidenceType("measurement") {
			t.Errorf("Evidence[1].Type = %s, want 'measurement'", run.Evidence[1].Type)
		}
	})
}
