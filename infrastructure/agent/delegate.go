// Package agent provides infrastructure for agent-to-agent communication.
//
// The DelegateTool wraps an Engine as a tool, enabling agent composition.
// A parent agent can delegate sub-goals to child agents by invoking the
// DelegateTool, which runs the child engine and returns its result.
package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/felixgeelhaar/agent-go/application"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// delegateInput is the expected JSON input for DelegateTool.
type delegateInput struct {
	Goal string `json:"goal"`
}

// delegateOutput is the JSON output from DelegateTool.
type delegateOutput struct {
	RunID  string          `json:"run_id"`
	Status string          `json:"status"`
	Result json.RawMessage `json:"result,omitempty"`
	Error  string          `json:"error,omitempty"`
}

// DelegateTool wraps an Engine as a tool, enabling agent composition.
// When executed, it runs the child engine with the goal extracted from input.
type DelegateTool struct {
	name        string
	description string
	engine      *application.Engine
	riskLevel   tool.RiskLevel
}

// DelegateOption configures a DelegateTool.
type DelegateOption func(*DelegateTool)

// WithRiskLevel sets the risk level for the delegate tool.
// This allows the parent agent to reason about the risk of delegating
// to a child agent that may perform destructive operations.
func WithRiskLevel(level tool.RiskLevel) DelegateOption {
	return func(d *DelegateTool) {
		d.riskLevel = level
	}
}

// NewDelegateTool creates a new DelegateTool that wraps the given engine.
//
// The tool accepts JSON input with a "goal" field and runs the child engine
// with that goal. The child run's result or error is returned as tool output.
//
// Example:
//
//	childEngine, _ := application.NewEngine(childConfig)
//	delegate := agent.NewDelegateTool("research_agent", "Delegates research tasks", childEngine)
func NewDelegateTool(name, description string, engine *application.Engine, opts ...DelegateOption) *DelegateTool {
	d := &DelegateTool{
		name:        name,
		description: description,
		engine:      engine,
		riskLevel:   tool.RiskLow,
	}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Name returns the tool name.
func (d *DelegateTool) Name() string {
	return d.name
}

// Description returns the tool description.
func (d *DelegateTool) Description() string {
	return d.description
}

// InputSchema returns the JSON Schema for the delegate tool input.
func (d *DelegateTool) InputSchema() tool.Schema {
	return tool.NewSchema(json.RawMessage(`{"type":"object","properties":{"goal":{"type":"string","description":"The goal to delegate to the child agent"}},"required":["goal"]}`))
}

// OutputSchema returns the JSON Schema for the delegate tool output.
func (d *DelegateTool) OutputSchema() tool.Schema {
	return tool.NewSchema(json.RawMessage(`{"type":"object","properties":{"run_id":{"type":"string"},"status":{"type":"string"},"result":{},"error":{"type":"string"}}}`))
}

// Annotations returns the tool annotations.
// DelegateTool is marked as ReadOnly because it does not directly perform
// side effects. The child engine may perform side effects, but that is
// governed by the child engine's own policies and constraints.
func (d *DelegateTool) Annotations() tool.Annotations {
	return tool.Annotations{
		ReadOnly:  true,
		RiskLevel: d.riskLevel,
		Tags:      []string{"delegate", "agent-composition"},
	}
}

// Execute runs the child engine with the goal extracted from input.
// It propagates the parent context, so context cancellation affects the child.
func (d *DelegateTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	var in delegateInput
	if err := json.Unmarshal(input, &in); err != nil {
		return tool.Result{}, fmt.Errorf("invalid delegate input: %w", err)
	}

	if in.Goal == "" {
		return tool.Result{}, fmt.Errorf("delegate input requires a non-empty goal")
	}

	run, err := d.engine.Run(ctx, in.Goal)

	out := delegateOutput{}
	if run != nil {
		out.RunID = run.ID
		out.Status = string(run.Status)
		out.Result = run.Result
		if run.Error != "" {
			out.Error = run.Error
		}
	}

	if err != nil {
		// If we have a run with results, still return them alongside the error context.
		if run != nil && run.Result != nil {
			outBytes, marshalErr := json.Marshal(out)
			if marshalErr == nil {
				return tool.NewResult(outBytes), nil
			}
		}
		out.Error = err.Error()
		outBytes, marshalErr := json.Marshal(out)
		if marshalErr != nil {
			return tool.Result{}, fmt.Errorf("delegate execution failed: %w", err)
		}
		return tool.Result{
			Output: outBytes,
			Error:  err,
		}, nil
	}

	outBytes, err := json.Marshal(out)
	if err != nil {
		return tool.Result{}, fmt.Errorf("failed to marshal delegate output: %w", err)
	}

	return tool.NewResult(outBytes), nil
}
