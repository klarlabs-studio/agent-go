package planner

import (
	"context"
	"sync"

	"go.klarlabs.de/agent/domain/agent"
)

// HybridPlanner combines a rule-based planner with a fallback planner.
// It evaluates rules first; if no rule matches, it delegates to the fallback.
// Specific states can be configured as rules-only, preventing fallback delegation.
type HybridPlanner struct {
	rules      *RuleBasedPlanner
	fallback   Planner
	rulesOnly  map[agent.State]bool
	mu         sync.RWMutex
	lastSource string // "rules" or "fallback" - for observability
}

// NewHybridPlanner creates a hybrid planner that tries rules first,
// then falls back to the given planner when no rule matches.
func NewHybridPlanner(rules *RuleBasedPlanner, fallback Planner) *HybridPlanner {
	return &HybridPlanner{
		rules:     rules,
		fallback:  fallback,
		rulesOnly: make(map[agent.State]bool),
	}
}

// ForceRulesOnly configures specific states to only use rule evaluation.
// When in these states, if no rule matches, the rule-based planner's fallback
// decision is returned instead of delegating to the fallback planner.
func (p *HybridPlanner) ForceRulesOnly(states ...agent.State) *HybridPlanner {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, s := range states {
		p.rulesOnly[s] = true
	}
	return p
}

// Plan evaluates rules first; if no rule matches and the current state is not
// configured as rules-only, delegates to the fallback planner.
func (p *HybridPlanner) Plan(ctx context.Context, req PlanRequest) (agent.Decision, error) {
	p.mu.RLock()
	isRulesOnly := p.rulesOnly[req.CurrentState]
	p.mu.RUnlock()

	// Check if a rule matches
	matched := p.rules.Matched(req)
	if matched != "" {
		p.setLastSource("rules")
		return p.rules.Plan(ctx, req)
	}

	// No rule matched. If rules-only for this state, use rule planner's fallback.
	if isRulesOnly {
		p.setLastSource("rules")
		return p.rules.Plan(ctx, req)
	}

	// Delegate to fallback planner
	p.setLastSource("fallback")
	return p.fallback.Plan(ctx, req)
}

// LastSource returns which planner produced the last decision: "rules" or "fallback".
// Useful for observability and debugging.
func (p *HybridPlanner) LastSource() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.lastSource
}

func (p *HybridPlanner) setLastSource(source string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.lastSource = source
}
