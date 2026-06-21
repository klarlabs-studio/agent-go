package api_test

import (
	"context"
	"errors"
	"testing"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/infrastructure/planner"
	api "go.klarlabs.de/agent/interfaces/api"
)

// selfLoopPlanner always transitions to the current state — a non-productive
// step (no state change, no evidence) that should trip loop detection.
type selfLoopPlanner struct{}

func (selfLoopPlanner) Plan(_ context.Context, req planner.PlanRequest) (agent.Decision, error) {
	return api.NewTransitionDecision(req.CurrentState, "stay"), nil
}

func TestLoopDetection_AbortsNonProgressingRun(t *testing.T) {
	// Allow the self-transition so it executes without erroring; the run then
	// makes no progress and loop detection must abort it.
	trans := api.NewStateTransitions()
	trans.Allow(api.StateIntake, api.StateIntake)

	eng, err := api.New(
		api.WithPlanner(selfLoopPlanner{}),
		api.WithTransitions(trans),
		api.WithMaxNoProgress(3),
		api.WithMaxSteps(100),
	)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	run, err := eng.Run(context.Background(), "spin forever")
	if !errors.Is(err, api.ErrNoProgress) {
		t.Fatalf("expected ErrNoProgress, got err=%v", err)
	}
	if run == nil || run.Status != api.StatusFailed {
		t.Fatalf("expected failed run, got %+v", run)
	}
	// Aborted by no-progress (3), well before MaxSteps (100).
}
