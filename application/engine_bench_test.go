package application

import (
	"context"
	"encoding/json"
	"testing"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/domain/tool"
	"go.klarlabs.de/agent/infrastructure/planner"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
)

// BenchmarkEngineRun_SimpleFlow benchmarks a minimal flow: intake -> explore -> decide -> done.
func BenchmarkEngineRun_SimpleFlow(b *testing.B) {
	registry := memory.NewToolRegistry()

	p := &benchSimplePlanner{}
	engine, err := NewEngine(EngineConfig{
		Registry: registry,
		Planner:  p,
	})
	if err != nil {
		b.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := engine.Run(ctx, "benchmark simple flow")
		if err != nil {
			b.Fatalf("run failed: %v", err)
		}
	}
}

// benchSimplePlanner follows: intake -> explore -> decide -> done
type benchSimplePlanner struct{}

func (p *benchSimplePlanner) Plan(_ context.Context, req planner.PlanRequest) (agent.Decision, error) {
	switch req.CurrentState {
	case agent.StateIntake:
		return agent.NewTransitionDecision(agent.StateExplore, "explore"), nil
	case agent.StateExplore:
		return agent.NewTransitionDecision(agent.StateDecide, "decide"), nil
	case agent.StateDecide:
		return agent.NewFinishDecision("done", json.RawMessage(`{}`)), nil
	default:
		return agent.NewFinishDecision("done", json.RawMessage(`{}`)), nil
	}
}

// BenchmarkEngineRun_ToolExecution benchmarks a flow with tool calls.
func BenchmarkEngineRun_ToolExecution(b *testing.B) {
	readTool, err := tool.NewBuilder("read_file").
		WithDescription("Read a file").
		WithAnnotations(tool.Annotations{ReadOnly: true}).
		WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: json.RawMessage(`{"content":"data"}`)}, nil
		}).
		Build()
	if err != nil {
		b.Fatalf("failed to build tool: %v", err)
	}

	registry := memory.NewToolRegistry()
	registry.Register(readTool)

	eligibility := policy.NewToolEligibility()
	eligibility.Allow(agent.StateExplore, "read_file")

	p := &benchToolPlanner{}
	engine, err := NewEngine(EngineConfig{
		Registry:    registry,
		Planner:     p,
		Eligibility: eligibility,
	})
	if err != nil {
		b.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, err := engine.Run(ctx, "benchmark tool execution")
		if err != nil {
			b.Fatalf("run failed: %v", err)
		}
	}
}

// benchToolPlanner: intake -> explore (3 tool calls) -> decide -> done
type benchToolPlanner struct{}

func (p *benchToolPlanner) Plan(_ context.Context, req planner.PlanRequest) (agent.Decision, error) {
	switch req.CurrentState {
	case agent.StateIntake:
		return agent.NewTransitionDecision(agent.StateExplore, "explore"), nil
	case agent.StateExplore:
		if len(req.Evidence) < 3 {
			return agent.NewCallToolDecision("read_file", json.RawMessage(`{"path":"test"}`), "read"), nil
		}
		return agent.NewTransitionDecision(agent.StateDecide, "decide"), nil
	case agent.StateDecide:
		return agent.NewFinishDecision("done", json.RawMessage(`{"result":"ok"}`)), nil
	default:
		return agent.NewFinishDecision("done", json.RawMessage(`{}`)), nil
	}
}

// BenchmarkEngineRun_ManySteps benchmarks a flow with 20+ state transitions.
func BenchmarkEngineRun_ManySteps(b *testing.B) {
	readTool, err := tool.NewBuilder("read_file").
		WithDescription("Read a file").
		WithAnnotations(tool.Annotations{ReadOnly: true}).
		WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: json.RawMessage(`{"ok":true}`)}, nil
		}).
		Build()
	if err != nil {
		b.Fatalf("failed to build tool: %v", err)
	}

	writeTool, err := tool.NewBuilder("write_file").
		WithDescription("Write a file").
		WithHandler(func(_ context.Context, _ json.RawMessage) (tool.Result, error) {
			return tool.Result{Output: json.RawMessage(`{"ok":true}`)}, nil
		}).
		Build()
	if err != nil {
		b.Fatalf("failed to build tool: %v", err)
	}

	registry := memory.NewToolRegistry()
	registry.Register(readTool)
	registry.Register(writeTool)

	eligibility := policy.NewToolEligibility()
	eligibility.Allow(agent.StateExplore, "read_file")
	eligibility.Allow(agent.StateAct, "write_file")
	eligibility.Allow(agent.StateValidate, "read_file")

	p := &benchManyStepsPlanner{targetLoops: 5}
	engine, err := NewEngine(EngineConfig{
		Registry:    registry,
		Planner:     p,
		Eligibility: eligibility,
		MaxSteps:    100,
	})
	if err != nil {
		b.Fatalf("failed to create engine: %v", err)
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		p.reset()
		_, err := engine.Run(ctx, "benchmark many steps")
		if err != nil {
			b.Fatalf("run failed: %v", err)
		}
	}
}

// benchManyStepsPlanner loops through explore -> decide -> act -> validate N times
// before finishing. This generates 4*N + 2 transitions (intake->explore + N loops + decide->done).
type benchManyStepsPlanner struct {
	targetLoops int
	loopCount   int
}

func (p *benchManyStepsPlanner) reset() {
	p.loopCount = 0
}

func (p *benchManyStepsPlanner) Plan(_ context.Context, req planner.PlanRequest) (agent.Decision, error) {
	switch req.CurrentState {
	case agent.StateIntake:
		return agent.NewTransitionDecision(agent.StateExplore, "explore"), nil
	case agent.StateExplore:
		return agent.NewTransitionDecision(agent.StateDecide, "decide"), nil
	case agent.StateDecide:
		if p.loopCount >= p.targetLoops {
			return agent.NewFinishDecision("done after loops", json.RawMessage(`{}`)), nil
		}
		return agent.NewTransitionDecision(agent.StateAct, "act"), nil
	case agent.StateAct:
		return agent.NewTransitionDecision(agent.StateValidate, "validate"), nil
	case agent.StateValidate:
		p.loopCount++
		return agent.NewTransitionDecision(agent.StateExplore, "loop back"), nil
	default:
		return agent.NewFinishDecision("done", json.RawMessage(`{}`)), nil
	}
}
