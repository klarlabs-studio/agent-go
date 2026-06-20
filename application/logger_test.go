package application

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"go.klarlabs.de/bolt"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/infrastructure/logging"
	"go.klarlabs.de/agent/infrastructure/planner"
)

func simpleFinishPlanner() *planner.ScriptedPlanner {
	return planner.NewScriptedPlanner(
		planner.ScriptStep{ExpectState: agent.StateIntake, Decision: agent.NewTransitionDecision(agent.StateExplore, "begin")},
		planner.ScriptStep{ExpectState: agent.StateExplore, Decision: agent.NewTransitionDecision(agent.StateDecide, "decide")},
		planner.ScriptStep{ExpectState: agent.StateDecide, Decision: agent.NewFinishDecision("done", json.RawMessage(`{}`))},
	)
}

// TestEngine_InjectedLogger_ReceivesOutput proves the engine logs through the
// injected logger, not the package-level singleton.
func TestEngine_InjectedLogger_ReceivesOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	boltLogger := bolt.New(bolt.NewJSONHandler(buf)).SetLevel(bolt.TRACE)

	engine, err := NewEngine(EngineConfig{
		Registry: newTestRegistry(),
		Planner:  simpleFinishPlanner(),
		Logger:   logging.NewLogger(boltLogger),
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}

	run, err := engine.Run(context.Background(), "injected logger run")
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "run started") {
		t.Errorf("expected injected logger to capture 'run started', got %q", out)
	}
	if !strings.Contains(out, run.ID) {
		t.Errorf("expected injected logger output to contain run id %q, got %q", run.ID, out)
	}
}

// TestEngine_DefaultLogger_EmitsNothing proves the default engine uses a no-op
// logger (no dependency on the global singleton) and still runs.
func TestEngine_DefaultLogger_EmitsNothing(t *testing.T) {
	engine, err := NewEngine(EngineConfig{
		Registry: newTestRegistry(),
		Planner:  simpleFinishPlanner(),
		// No Logger provided -> no-op default.
	})
	if err != nil {
		t.Fatalf("new engine: %v", err)
	}
	if engine.logger == nil {
		t.Fatal("expected a non-nil no-op logger default")
	}

	run, err := engine.Run(context.Background(), "default logger run")
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if run.Status != agent.RunStatusCompleted {
		t.Errorf("expected completed run, got %s", run.Status)
	}
}
