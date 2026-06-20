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

func TestEngine_Fork_ViaAPI(t *testing.T) {
	store := memory.NewEventStore()
	p := api.NewScriptedPlanner(
		api.ScriptStep{ExpectState: api.StateIntake, Decision: api.NewTransitionDecision(api.StateExplore, "begin")},
		api.ScriptStep{ExpectState: api.StateExplore, Decision: api.NewTransitionDecision(api.StateDecide, "decide")},
		api.ScriptStep{ExpectState: api.StateDecide, Decision: api.NewFinishDecision("done", json.RawMessage(`{}`))},
	)
	engine, err := api.New(
		api.WithPlanner(p),
		api.WithEventStore(store),
		api.WithClock(api.NewFixedClock(time.Unix(0, 0).UTC())),
	)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	source, err := engine.Run(context.Background(), "api fork source")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	forked, err := engine.Fork(context.Background(), source.ID, 1)
	if err != nil {
		t.Fatalf("fork: %v", err)
	}
	if forked.ParentRunID != source.ID {
		t.Errorf("fork parent = %q, want %q", forked.ParentRunID, source.ID)
	}
	if forked.CurrentState != api.StateExplore {
		t.Errorf("fork state = %s, want explore", forked.CurrentState)
	}
}

// TestEngine_ContinueRun_ViaAPI proves the api re-export drives a forked run
// further to completion.
func TestEngine_ContinueRun_ViaAPI(t *testing.T) {
	store := memory.NewEventStore()
	srcPlanner := api.NewScriptedPlanner(
		api.ScriptStep{ExpectState: api.StateIntake, Decision: api.NewTransitionDecision(api.StateExplore, "begin")},
		api.ScriptStep{ExpectState: api.StateExplore, Decision: api.NewTransitionDecision(api.StateDecide, "decide")},
		api.ScriptStep{ExpectState: api.StateDecide, Decision: api.NewFinishDecision("done", json.RawMessage(`{}`))},
	)
	srcEngine, err := api.New(api.WithPlanner(srcPlanner), api.WithEventStore(store))
	if err != nil {
		t.Fatalf("new source engine: %v", err)
	}
	source, err := srcEngine.Run(context.Background(), "api continue source")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	forked, err := srcEngine.Fork(context.Background(), source.ID, 1)
	if err != nil {
		t.Fatalf("fork: %v", err)
	}

	contPlanner := api.NewScriptedPlanner(
		api.ScriptStep{ExpectState: api.StateExplore, Decision: api.NewTransitionDecision(api.StateDecide, "decide")},
		api.ScriptStep{ExpectState: api.StateDecide, Decision: api.NewFinishDecision("done", json.RawMessage(`{}`))},
	)
	contEngine, err := api.New(api.WithPlanner(contPlanner), api.WithEventStore(store))
	if err != nil {
		t.Fatalf("new continue engine: %v", err)
	}
	final, err := contEngine.ContinueRun(context.Background(), forked)
	if err != nil {
		t.Fatalf("continue: %v", err)
	}
	if final.Status != api.StatusCompleted {
		t.Errorf("continued run status = %s, want completed", final.Status)
	}
}

func TestWithInputValidation_RejectsOversizedInput(t *testing.T) {
	big := make([]byte, 0, 256)
	big = append(big, []byte(`{"k":"`)...)
	for i := 0; i < 200; i++ {
		big = append(big, 'a')
	}
	big = append(big, []byte(`"}`)...)

	tl := api.NewToolBuilder("sink").
		WithDescription("accepts input").
		WithAnnotations(api.Annotations{ReadOnly: true}).
		WithHandler(func(_ context.Context, _ json.RawMessage) (api.ToolResult, error) {
			return api.ToolResult{Output: json.RawMessage(`{}`)}, nil
		}).
		MustBuild()

	p := api.NewScriptedPlanner(
		api.ScriptStep{ExpectState: api.StateIntake, Decision: api.NewTransitionDecision(api.StateExplore, "begin")},
		api.ScriptStep{ExpectState: api.StateExplore, Decision: api.NewCallToolDecision("sink", json.RawMessage(big), "oversized")},
	)
	eligibility := api.NewToolEligibility()
	eligibility.Allow(api.StateExplore, "sink")

	engine, err := api.New(
		api.WithTool(tl),
		api.WithPlanner(p),
		api.WithToolEligibility(eligibility),
		api.WithInputValidation(32),
	)
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	_, runErr := engine.Run(context.Background(), "oversized input")
	if runErr == nil {
		t.Fatal("expected run to fail on oversized tool input")
	}
}
