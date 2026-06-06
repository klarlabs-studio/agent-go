// Package planner provides planner implementations for the agent runtime.
package planner

import (
	"context"
	"encoding/json"
	"sync"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/policy"
)

// ToolDef describes a tool available for planning decisions.
type ToolDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

// PlanRequest contains all information needed for planning.
type PlanRequest struct {
	RunID        string
	Goal         string
	CurrentState agent.State
	Evidence     []agent.Evidence
	AllowedTools []string
	ToolDefs     []ToolDef
	Budgets      policy.BudgetSnapshot
	Vars         map[string]any
}

// Planner is the interface for decision engines.
type Planner interface {
	Plan(ctx context.Context, req PlanRequest) (agent.Decision, error)
}

// MockPlanner returns a predefined sequence of decisions for testing.
type MockPlanner struct {
	decisions []agent.Decision
	index     int
	mu        sync.Mutex
}

// NewMockPlanner creates a mock planner with the given decisions.
func NewMockPlanner(decisions ...agent.Decision) *MockPlanner {
	return &MockPlanner{
		decisions: decisions,
		index:     0,
	}
}

// Plan returns the next decision in the sequence.
func (p *MockPlanner) Plan(_ context.Context, _ PlanRequest) (agent.Decision, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.index >= len(p.decisions) {
		// Default to finish if no more decisions
		return agent.NewFinishDecision("completed", nil), nil
	}

	decision := p.decisions[p.index]
	p.index++
	return decision, nil
}

// Reset resets the planner to the beginning.
func (p *MockPlanner) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.index = 0
}

// Remaining returns the number of remaining decisions.
func (p *MockPlanner) Remaining() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.decisions) - p.index
}

// AddDecision appends a decision to the sequence.
func (p *MockPlanner) AddDecision(d agent.Decision) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.decisions = append(p.decisions, d)
}
