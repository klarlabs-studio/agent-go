package statemachine

import (
	"errors"
	"testing"

	"github.com/felixgeelhaar/statekit"

	"github.com/felixgeelhaar/agent-go/domain/agent"
)

// Helper guards for testing
func alwaysTrue(_ *Context, _ statekit.Event) (bool, error)  { return true, nil }
func alwaysFalse(_ *Context, _ statekit.Event) (bool, error) { return false, nil }

var errGuard = errors.New("guard error")

func alwaysError(_ *Context, _ statekit.Event) (bool, error) { return false, errGuard }

func TestAndGuard(t *testing.T) {
	t.Parallel()

	event := statekit.Event{Type: "TEST"}

	tests := []struct {
		name    string
		guards  []Guard
		wantOK  bool
		wantErr bool
	}{
		{
			name:   "empty guards pass",
			guards: nil,
			wantOK: true,
		},
		{
			name:   "single true passes",
			guards: []Guard{alwaysTrue},
			wantOK: true,
		},
		{
			name:   "single false fails",
			guards: []Guard{alwaysFalse},
			wantOK: false,
		},
		{
			name:   "all true passes",
			guards: []Guard{alwaysTrue, alwaysTrue, alwaysTrue},
			wantOK: true,
		},
		{
			name:   "one false fails",
			guards: []Guard{alwaysTrue, alwaysFalse, alwaysTrue},
			wantOK: false,
		},
		{
			name:    "error propagates",
			guards:  []Guard{alwaysTrue, alwaysError},
			wantOK:  false,
			wantErr: true,
		},
		{
			name:    "error short-circuits",
			guards:  []Guard{alwaysError, alwaysTrue},
			wantOK:  false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			guard := AndGuard(tt.guards...)
			ok, err := guard(nil, event)

			if ok != tt.wantOK {
				t.Errorf("AndGuard() ok = %v, want %v", ok, tt.wantOK)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("AndGuard() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOrGuard(t *testing.T) {
	t.Parallel()

	event := statekit.Event{Type: "TEST"}

	tests := []struct {
		name    string
		guards  []Guard
		wantOK  bool
		wantErr bool
	}{
		{
			name:   "empty guards fail",
			guards: nil,
			wantOK: false,
		},
		{
			name:   "single true passes",
			guards: []Guard{alwaysTrue},
			wantOK: true,
		},
		{
			name:   "single false fails",
			guards: []Guard{alwaysFalse},
			wantOK: false,
		},
		{
			name:   "any true passes",
			guards: []Guard{alwaysFalse, alwaysTrue, alwaysFalse},
			wantOK: true,
		},
		{
			name:   "all false fails",
			guards: []Guard{alwaysFalse, alwaysFalse},
			wantOK: false,
		},
		{
			name:   "error skipped when another passes",
			guards: []Guard{alwaysError, alwaysTrue},
			wantOK: true,
		},
		{
			name:    "error returned when all fail",
			guards:  []Guard{alwaysFalse, alwaysError},
			wantOK:  false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			guard := OrGuard(tt.guards...)
			ok, err := guard(nil, event)

			if ok != tt.wantOK {
				t.Errorf("OrGuard() ok = %v, want %v", ok, tt.wantOK)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("OrGuard() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNotGuard(t *testing.T) {
	t.Parallel()

	event := statekit.Event{Type: "TEST"}

	tests := []struct {
		name    string
		guard   Guard
		wantOK  bool
		wantErr bool
	}{
		{
			name:   "inverts true to false",
			guard:  alwaysTrue,
			wantOK: false,
		},
		{
			name:   "inverts false to true",
			guard:  alwaysFalse,
			wantOK: true,
		},
		{
			name:    "passes error through",
			guard:   alwaysError,
			wantOK:  false,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			guard := NotGuard(tt.guard)
			ok, err := guard(nil, event)

			if ok != tt.wantOK {
				t.Errorf("NotGuard() ok = %v, want %v", ok, tt.wantOK)
			}
			if (err != nil) != tt.wantErr {
				t.Errorf("NotGuard() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGuardComposition(t *testing.T) {
	t.Parallel()

	event := statekit.Event{Type: "TEST"}

	t.Run("AND of OR guards", func(t *testing.T) {
		t.Parallel()

		// (true OR false) AND (false OR true) => true AND true => true
		guard := AndGuard(
			OrGuard(alwaysTrue, alwaysFalse),
			OrGuard(alwaysFalse, alwaysTrue),
		)
		ok, err := guard(nil, event)
		if !ok || err != nil {
			t.Errorf("Nested AND(OR, OR) = (%v, %v), want (true, nil)", ok, err)
		}
	})

	t.Run("OR of AND guards", func(t *testing.T) {
		t.Parallel()

		// (true AND false) OR (true AND true) => false OR true => true
		guard := OrGuard(
			AndGuard(alwaysTrue, alwaysFalse),
			AndGuard(alwaysTrue, alwaysTrue),
		)
		ok, err := guard(nil, event)
		if !ok || err != nil {
			t.Errorf("Nested OR(AND, AND) = (%v, %v), want (true, nil)", ok, err)
		}
	})

	t.Run("NOT of AND", func(t *testing.T) {
		t.Parallel()

		// NOT(true AND true) => false
		guard := NotGuard(AndGuard(alwaysTrue, alwaysTrue))
		ok, err := guard(nil, event)
		if ok || err != nil {
			t.Errorf("NOT(AND(true, true)) = (%v, %v), want (false, nil)", ok, err)
		}
	})

	t.Run("double NOT", func(t *testing.T) {
		t.Parallel()

		guard := NotGuard(NotGuard(alwaysTrue))
		ok, err := guard(nil, event)
		if !ok || err != nil {
			t.Errorf("NOT(NOT(true)) = (%v, %v), want (true, nil)", ok, err)
		}
	})
}

func TestGuardBuilder(t *testing.T) {
	t.Parallel()

	event := statekit.Event{Type: "TEST"}

	t.Run("empty builder passes", func(t *testing.T) {
		t.Parallel()

		guard := NewGuardBuilder().Build()
		ok, err := guard(nil, event)
		if !ok || err != nil {
			t.Errorf("empty builder = (%v, %v), want (true, nil)", ok, err)
		}
	})

	t.Run("single And", func(t *testing.T) {
		t.Parallel()

		guard := NewGuardBuilder().And(alwaysTrue).Build()
		ok, err := guard(nil, event)
		if !ok || err != nil {
			t.Errorf("And(true) = (%v, %v), want (true, nil)", ok, err)
		}
	})

	t.Run("And chain all true", func(t *testing.T) {
		t.Parallel()

		guard := NewGuardBuilder().And(alwaysTrue).And(alwaysTrue).Build()
		ok, err := guard(nil, event)
		if !ok || err != nil {
			t.Errorf("And(true).And(true) = (%v, %v), want (true, nil)", ok, err)
		}
	})

	t.Run("And chain with false", func(t *testing.T) {
		t.Parallel()

		guard := NewGuardBuilder().And(alwaysTrue).And(alwaysFalse).Build()
		ok, err := guard(nil, event)
		if ok {
			t.Error("And(true).And(false) should fail")
		}
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Or creates separate branch", func(t *testing.T) {
		t.Parallel()

		// And(false).Or(true) => false OR true => true
		guard := NewGuardBuilder().And(alwaysFalse).Or(alwaysTrue).Build()
		ok, err := guard(nil, event)
		if !ok || err != nil {
			t.Errorf("And(false).Or(true) = (%v, %v), want (true, nil)", ok, err)
		}
	})

	t.Run("Not negates guard", func(t *testing.T) {
		t.Parallel()

		// And(true).Not(true) => true AND (NOT true) => true AND false => false
		guard := NewGuardBuilder().And(alwaysTrue).Not(alwaysTrue).Build()
		ok, err := guard(nil, event)
		if ok {
			t.Error("And(true).Not(true) should fail")
		}
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("complex composition", func(t *testing.T) {
		t.Parallel()

		// And(true).Or(false).Not(false)
		// Group 1 (from And): [true]
		// Group 2 (from Or+Not): [false, NOT(false)] => [false, true] => AND => false
		// Result: true OR false => true
		guard := NewGuardBuilder().And(alwaysTrue).Or(alwaysFalse).Not(alwaysFalse).Build()
		ok, err := guard(nil, event)
		if !ok || err != nil {
			t.Errorf("complex = (%v, %v), want (true, nil)", ok, err)
		}
	})
}

func TestToStatekitGuard(t *testing.T) {
	t.Parallel()

	event := statekit.Event{Type: "TEST"}

	t.Run("converts true guard", func(t *testing.T) {
		t.Parallel()

		sg := ToStatekitGuard(alwaysTrue)
		if !sg(nil, event) {
			t.Error("converted true guard should return true")
		}
	})

	t.Run("converts false guard", func(t *testing.T) {
		t.Parallel()

		sg := ToStatekitGuard(alwaysFalse)
		if sg(nil, event) {
			t.Error("converted false guard should return false")
		}
	})

	t.Run("error guard returns false", func(t *testing.T) {
		t.Parallel()

		sg := ToStatekitGuard(alwaysError)
		if sg(nil, event) {
			t.Error("converted error guard should return false")
		}
	})
}

func TestFromStatekitGuard(t *testing.T) {
	t.Parallel()

	event := statekit.Event{Type: "TEST"}

	statekitGuard := func(_ *Context, _ statekit.Event) bool { return true }
	guard := FromStatekitGuard(statekitGuard)

	ok, err := guard(nil, event)
	if !ok {
		t.Error("converted statekit guard should return true")
	}
	if err != nil {
		t.Errorf("converted statekit guard should not return error: %v", err)
	}
}

func TestGuardWithContext(t *testing.T) {
	t.Parallel()

	event := statekit.Event{Type: "TEST"}

	// Guard that checks the run's current state
	inExploreGuard := func(ctx *Context, _ statekit.Event) (bool, error) {
		if ctx == nil || ctx.Run == nil {
			return false, errors.New("nil context")
		}
		return ctx.Run.CurrentState == agent.StateExplore, nil
	}

	t.Run("passes with correct state", func(t *testing.T) {
		t.Parallel()

		run := agent.NewRun("test", "goal")
		run.CurrentState = agent.StateExplore
		ctx := &Context{Run: run}

		ok, err := inExploreGuard(ctx, event)
		if !ok || err != nil {
			t.Errorf("guard = (%v, %v), want (true, nil)", ok, err)
		}
	})

	t.Run("fails with wrong state", func(t *testing.T) {
		t.Parallel()

		run := agent.NewRun("test", "goal")
		run.CurrentState = agent.StateAct
		ctx := &Context{Run: run}

		ok, err := inExploreGuard(ctx, event)
		if ok {
			t.Error("guard should fail for wrong state")
		}
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("AND with context guard", func(t *testing.T) {
		t.Parallel()

		run := agent.NewRun("test", "goal")
		run.CurrentState = agent.StateExplore
		ctx := &Context{Run: run}

		guard := AndGuard(inExploreGuard, alwaysTrue)
		ok, err := guard(ctx, event)
		if !ok || err != nil {
			t.Errorf("AND with context = (%v, %v), want (true, nil)", ok, err)
		}
	})
}
