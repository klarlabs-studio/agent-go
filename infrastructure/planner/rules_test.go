package planner

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/felixgeelhaar/agent-go/domain/agent"
)

func TestRuleBasedPlanner_Plan(t *testing.T) {
	fallback := agent.NewFailDecision("no rule matched", nil)

	tests := []struct {
		name     string
		rules    []Rule
		req      PlanRequest
		wantType agent.DecisionType
		wantName string // for Matched() check
	}{
		{
			name:  "no rules returns fallback",
			rules: nil,
			req: PlanRequest{
				CurrentState: agent.StateIntake,
			},
			wantType: agent.DecisionFail,
			wantName: "",
		},
		{
			name: "exact state match",
			rules: []Rule{
				NewRule("intake-to-explore").
					InState(agent.StateIntake).
					Then(agent.NewTransitionDecision(agent.StateExplore, "begin")).
					Build(),
			},
			req: PlanRequest{
				CurrentState: agent.StateIntake,
			},
			wantType: agent.DecisionTransition,
			wantName: "intake-to-explore",
		},
		{
			name: "state mismatch falls to fallback",
			rules: []Rule{
				NewRule("only-in-explore").
					InState(agent.StateExplore).
					Then(agent.NewTransitionDecision(agent.StateDecide, "next")).
					Build(),
			},
			req: PlanRequest{
				CurrentState: agent.StateIntake,
			},
			wantType: agent.DecisionFail,
			wantName: "",
		},
		{
			name: "wildcard state matches any",
			rules: []Rule{
				NewRule("catch-all").
					Then(agent.NewFinishDecision("done", nil)).
					Build(),
			},
			req: PlanRequest{
				CurrentState: agent.StateAct,
			},
			wantType: agent.DecisionFinish,
			wantName: "catch-all",
		},
		{
			name: "priority ordering lower wins",
			rules: []Rule{
				NewRule("low-priority").
					WithPriority(10).
					InState(agent.StateIntake).
					Then(agent.NewFailDecision("should not win", nil)).
					Build(),
				NewRule("high-priority").
					WithPriority(1).
					InState(agent.StateIntake).
					Then(agent.NewTransitionDecision(agent.StateExplore, "priority")).
					Build(),
			},
			req: PlanRequest{
				CurrentState: agent.StateIntake,
			},
			wantType: agent.DecisionTransition,
			wantName: "high-priority",
		},
		{
			name: "equal priority preserves insertion order",
			rules: []Rule{
				NewRule("first").
					WithPriority(5).
					InState(agent.StateIntake).
					Then(agent.NewTransitionDecision(agent.StateExplore, "first")).
					Build(),
				NewRule("second").
					WithPriority(5).
					InState(agent.StateIntake).
					Then(agent.NewTransitionDecision(agent.StateDecide, "second")).
					Build(),
			},
			req: PlanRequest{
				CurrentState: agent.StateIntake,
			},
			wantType: agent.DecisionTransition,
			wantName: "first",
		},
		{
			name: "custom condition match",
			rules: []Rule{
				NewRule("has-vars").
					InState(agent.StateExplore).
					When(func(req PlanRequest) bool {
						_, ok := req.Vars["key"]
						return ok
					}).
					Then(agent.NewFinishDecision("found key", nil)).
					Build(),
			},
			req: PlanRequest{
				CurrentState: agent.StateExplore,
				Vars:         map[string]any{"key": "value"},
			},
			wantType: agent.DecisionFinish,
			wantName: "has-vars",
		},
		{
			name: "custom condition no match",
			rules: []Rule{
				NewRule("has-vars").
					InState(agent.StateExplore).
					When(func(req PlanRequest) bool {
						_, ok := req.Vars["key"]
						return ok
					}).
					Then(agent.NewFinishDecision("found key", nil)).
					Build(),
			},
			req: PlanRequest{
				CurrentState: agent.StateExplore,
				Vars:         map[string]any{},
			},
			wantType: agent.DecisionFail,
			wantName: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewRuleBasedPlanner(fallback, tt.rules...)

			decision, err := p.Plan(context.Background(), tt.req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if decision.Type != tt.wantType {
				t.Errorf("decision type = %s, want %s", decision.Type, tt.wantType)
			}

			matched := p.Matched(tt.req)
			if matched != tt.wantName {
				t.Errorf("matched rule = %q, want %q", matched, tt.wantName)
			}
		})
	}
}

func TestRuleBasedPlanner_EvidencePatterns(t *testing.T) {
	fallback := agent.NewFailDecision("no match", nil)

	makeEvidence := func(content string) []agent.Evidence {
		raw, _ := json.Marshal(content)
		return []agent.Evidence{
			{Content: raw},
		}
	}

	tests := []struct {
		name     string
		pattern  string
		evidence []agent.Evidence
		want     bool
	}{
		{
			name:     "prefix wildcard match",
			pattern:  "error*",
			evidence: makeEvidence("error: file not found"),
			want:     true,
		},
		{
			name:     "prefix wildcard no match",
			pattern:  "error*",
			evidence: makeEvidence("success: file created"),
			want:     false,
		},
		{
			name:     "suffix wildcard match",
			pattern:  "*found",
			evidence: makeEvidence("file not found"),
			want:     true,
		},
		{
			name:     "contains wildcard match",
			pattern:  "*not*",
			evidence: makeEvidence("file not found"),
			want:     true,
		},
		{
			name:     "exact match",
			pattern:  "exact",
			evidence: makeEvidence("exact"),
			want:     true,
		},
		{
			name:     "exact no match",
			pattern:  "exact",
			evidence: makeEvidence("not exact"),
			want:     false,
		},
		{
			name:     "star matches anything",
			pattern:  "*",
			evidence: makeEvidence("anything"),
			want:     true,
		},
		{
			name:     "no evidence fails pattern",
			pattern:  "anything",
			evidence: nil,
			want:     false,
		},
		{
			name:    "matches most recent evidence",
			pattern: "*recent*",
			evidence: []agent.Evidence{
				{Content: json.RawMessage(`"old evidence"`)},
				{Content: json.RawMessage(`"most recent data"`)},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rule := NewRule("test").
				WithEvidencePattern(tt.pattern).
				Then(agent.NewFinishDecision("matched", nil)).
				Build()

			p := NewRuleBasedPlanner(fallback, rule)
			req := PlanRequest{Evidence: tt.evidence}

			decision, err := p.Plan(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			got := decision.Type == agent.DecisionFinish
			if got != tt.want {
				t.Errorf("matched = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRuleBasedPlanner_AddRule(t *testing.T) {
	fallback := agent.NewFailDecision("no match", nil)
	p := NewRuleBasedPlanner(fallback)

	if p.RuleCount() != 0 {
		t.Fatalf("initial rule count = %d, want 0", p.RuleCount())
	}

	p.AddRule(NewRule("dynamic").
		InState(agent.StateIntake).
		Then(agent.NewTransitionDecision(agent.StateExplore, "added")).
		Build(),
	)

	if p.RuleCount() != 1 {
		t.Fatalf("rule count after add = %d, want 1", p.RuleCount())
	}

	decision, err := p.Plan(context.Background(), PlanRequest{
		CurrentState: agent.StateIntake,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Type != agent.DecisionTransition {
		t.Errorf("decision type = %s, want %s", decision.Type, agent.DecisionTransition)
	}
}

func TestRuleBasedPlanner_PriorityOrdering(t *testing.T) {
	fallback := agent.NewFailDecision("no match", nil)

	// Add rules out of priority order and verify correct sorting.
	rules := []Rule{
		NewRule("priority-50").WithPriority(50).
			Then(agent.NewFailDecision("50", nil)).Build(),
		NewRule("priority-10").WithPriority(10).
			Then(agent.NewTransitionDecision(agent.StateExplore, "10")).Build(),
		NewRule("priority-30").WithPriority(30).
			Then(agent.NewFailDecision("30", nil)).Build(),
		NewRule("priority-1").WithPriority(1).
			Then(agent.NewFinishDecision("1", nil)).Build(),
	}

	p := NewRuleBasedPlanner(fallback, rules...)

	decision, err := p.Plan(context.Background(), PlanRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if decision.Type != agent.DecisionFinish {
		t.Errorf("expected priority-1 to win, got type %s", decision.Type)
	}

	matched := p.Matched(PlanRequest{})
	if matched != "priority-1" {
		t.Errorf("matched = %q, want %q", matched, "priority-1")
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern string
		value   string
		want    bool
	}{
		{"*", "", true},
		{"*", "anything", true},
		{"hello*", "hello world", true},
		{"hello*", "world hello", false},
		{"*world", "hello world", true},
		{"*world", "world hello", false},
		{"*llo wo*", "hello world", true},
		{"*missing*", "hello world", false},
		{"exact", "exact", true},
		{"exact", "not exact", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.value, func(t *testing.T) {
			got := matchPattern(tt.pattern, tt.value)
			if got != tt.want {
				t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.value, got, tt.want)
			}
		})
	}
}
