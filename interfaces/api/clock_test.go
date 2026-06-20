package api_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go.klarlabs.de/agent/infrastructure/storage/memory"
	api "go.klarlabs.de/agent/interfaces/api"
)

// TestWithClock_DeterministicEventTimestamps proves the injected clock drives
// event timestamps, so a replayed/forked run reproduces identical times.
func TestWithClock_DeterministicEventTimestamps(t *testing.T) {
	anchor := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	clk := api.NewFixedClock(anchor)
	store := memory.NewEventStore()

	p := api.NewScriptedPlanner(
		api.ScriptStep{ExpectState: api.StateIntake, Decision: api.NewTransitionDecision(api.StateExplore, "begin")},
		api.ScriptStep{ExpectState: api.StateExplore, Decision: api.NewTransitionDecision(api.StateDecide, "decide")},
		api.ScriptStep{ExpectState: api.StateDecide, Decision: api.NewFinishDecision("done", json.RawMessage(`{}`))},
	)

	engine, err := api.New(
		api.WithPlanner(p),
		api.WithEventStore(store),
		api.WithClock(clk),
	)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	run, err := engine.Run(context.Background(), "deterministic clock")
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	events, err := store.LoadEvents(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("load events: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected events to be recorded")
	}
	for _, e := range events {
		if !e.Timestamp.Equal(anchor) {
			t.Errorf("event %s timestamp = %v, want fixed anchor %v", e.Type, e.Timestamp, anchor)
		}
	}

	// Run start time must also come from the injected clock.
	if !run.StartTime.Equal(anchor) {
		t.Errorf("run start time = %v, want fixed anchor %v", run.StartTime, anchor)
	}
}
