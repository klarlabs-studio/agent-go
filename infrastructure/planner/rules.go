package planner

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	"go.klarlabs.de/agent/domain/agent"
)

// Rule defines a condition-decision pair for the rule-based planner.
// When the condition matches, the associated decision is returned.
type Rule struct {
	// Name is a human-readable identifier for the rule.
	Name string

	// Priority determines evaluation order. Lower values are evaluated first.
	// Rules with equal priority are evaluated in insertion order.
	Priority int

	// State matches the agent's current state. Empty matches any state.
	State agent.State

	// EvidencePattern optionally matches against the most recent evidence.
	// Supports prefix wildcards ("*suffix"), suffix wildcards ("prefix*"),
	// and contains wildcards ("*middle*"). Empty matches any evidence.
	EvidencePattern string

	// Condition is an optional function for complex matching logic.
	// If set, it must return true for the rule to match.
	Condition func(PlanRequest) bool

	// Decision is the decision to return when this rule matches.
	Decision agent.Decision
}

// RuleBuilder constructs rules using a fluent API.
type RuleBuilder struct {
	rule Rule
}

// NewRule creates a new rule builder with the given name.
func NewRule(name string) *RuleBuilder {
	return &RuleBuilder{
		rule: Rule{Name: name},
	}
}

// WithPriority sets the rule priority. Lower values are evaluated first.
func (b *RuleBuilder) WithPriority(p int) *RuleBuilder {
	b.rule.Priority = p
	return b
}

// InState restricts the rule to the given state.
func (b *RuleBuilder) InState(s agent.State) *RuleBuilder {
	b.rule.State = s
	return b
}

// WithEvidencePattern sets a pattern to match against the most recent evidence content.
// Supports wildcards: "prefix*", "*suffix", "*contains*", or exact match.
func (b *RuleBuilder) WithEvidencePattern(pattern string) *RuleBuilder {
	b.rule.EvidencePattern = pattern
	return b
}

// When adds a custom condition function.
func (b *RuleBuilder) When(fn func(PlanRequest) bool) *RuleBuilder {
	b.rule.Condition = fn
	return b
}

// Then sets the decision to return when this rule matches.
func (b *RuleBuilder) Then(d agent.Decision) *RuleBuilder {
	b.rule.Decision = d
	return b
}

// Build returns the constructed rule.
func (b *RuleBuilder) Build() Rule {
	return b.rule
}

// RuleBasedPlanner evaluates rules in priority order and returns the first matching decision.
type RuleBasedPlanner struct {
	rules    []Rule
	fallback agent.Decision
	mu       sync.RWMutex
}

// NewRuleBasedPlanner creates a rule-based planner with the given rules and fallback decision.
// Rules are sorted by priority on creation. The fallback is returned when no rule matches.
func NewRuleBasedPlanner(fallback agent.Decision, rules ...Rule) *RuleBasedPlanner {
	sorted := make([]Rule, len(rules))
	copy(sorted, rules)
	sortRules(sorted)

	return &RuleBasedPlanner{
		rules:    sorted,
		fallback: fallback,
	}
}

// Plan evaluates rules in priority order and returns the first matching decision.
// If no rule matches, the fallback decision is returned.
func (p *RuleBasedPlanner) Plan(_ context.Context, req PlanRequest) (agent.Decision, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for i := range p.rules {
		if p.matches(&p.rules[i], req) {
			return p.rules[i].Decision, nil
		}
	}

	return p.fallback, nil
}

// AddRule adds a rule and re-sorts by priority.
func (p *RuleBasedPlanner) AddRule(r Rule) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.rules = append(p.rules, r)
	sortRules(p.rules)
}

// RuleCount returns the number of configured rules.
func (p *RuleBasedPlanner) RuleCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.rules)
}

// Matched returns the name of the rule that would match for the given request,
// or empty string if no rule matches (fallback would be used).
func (p *RuleBasedPlanner) Matched(req PlanRequest) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	for i := range p.rules {
		if p.matches(&p.rules[i], req) {
			return p.rules[i].Name
		}
	}
	return ""
}

// matches checks if a rule matches the given request.
func (p *RuleBasedPlanner) matches(r *Rule, req PlanRequest) bool {
	// Check state match
	if r.State != "" && r.State != req.CurrentState {
		return false
	}

	// Check evidence pattern
	if r.EvidencePattern != "" {
		if !matchEvidence(r.EvidencePattern, req.Evidence) {
			return false
		}
	}

	// Check custom condition
	if r.Condition != nil && !r.Condition(req) {
		return false
	}

	return true
}

// matchEvidence checks if the most recent evidence content matches the pattern.
func matchEvidence(pattern string, evidence []agent.Evidence) bool {
	if len(evidence) == 0 {
		return false
	}

	latest := evidence[len(evidence)-1]
	content := string(latest.Content)

	// If content is a JSON string, unwrap the quotes for pattern matching.
	var unquoted string
	if err := json.Unmarshal(latest.Content, &unquoted); err == nil {
		content = unquoted
	}

	return matchPattern(pattern, content)
}

// matchPattern performs wildcard pattern matching.
// Supports: "prefix*", "*suffix", "*contains*", or exact match.
func matchPattern(pattern, value string) bool {
	if pattern == "*" {
		return true
	}

	startsWithWild := strings.HasPrefix(pattern, "*")
	endsWithWild := strings.HasSuffix(pattern, "*")

	switch {
	case startsWithWild && endsWithWild:
		// *contains*
		inner := pattern[1 : len(pattern)-1]
		return strings.Contains(value, inner)
	case endsWithWild:
		// prefix*
		prefix := pattern[:len(pattern)-1]
		return strings.HasPrefix(value, prefix)
	case startsWithWild:
		// *suffix
		suffix := pattern[1:]
		return strings.HasSuffix(value, suffix)
	default:
		// Exact match
		return value == pattern
	}
}

// sortRules performs a stable insertion sort by priority.
// Insertion sort is used because rule sets are typically small (<100 rules)
// and we need stable ordering for rules with equal priority.
func sortRules(rules []Rule) {
	for i := 1; i < len(rules); i++ {
		key := rules[i]
		j := i - 1
		for j >= 0 && rules[j].Priority > key.Priority {
			rules[j+1] = rules[j]
			j--
		}
		rules[j+1] = key
	}
}

// NoMatchError indicates that no rule matched and no fallback was configured.
// This is used internally; the public API always requires a fallback.
type NoMatchError struct {
	State    agent.State
	Evidence json.RawMessage
}

func (e *NoMatchError) Error() string {
	return "no rule matched in state " + string(e.State)
}
