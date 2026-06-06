package statemachine

import (
	"errors"
	"testing"

	"go.klarlabs.de/statekit"
)

func TestActionRegistry_Register(t *testing.T) {
	t.Parallel()

	t.Run("registers valid action", func(t *testing.T) {
		t.Parallel()

		reg := NewActionRegistry()
		err := reg.Register(RegisteredAction{
			Name:     "test_action",
			Priority: 1,
			Execute: func(_ *Context, _ statekit.Event) error {
				return nil
			},
		})
		if err != nil {
			t.Fatalf("Register() error = %v", err)
		}
	})

	t.Run("rejects empty name", func(t *testing.T) {
		t.Parallel()

		reg := NewActionRegistry()
		err := reg.Register(RegisteredAction{
			Name: "",
			Execute: func(_ *Context, _ statekit.Event) error {
				return nil
			},
		})
		if err == nil {
			t.Error("Register() should reject empty name")
		}
	})

	t.Run("rejects duplicate name", func(t *testing.T) {
		t.Parallel()

		reg := NewActionRegistry()
		action := RegisteredAction{
			Name: "dup",
			Execute: func(_ *Context, _ statekit.Event) error {
				return nil
			},
		}
		_ = reg.Register(action)

		err := reg.Register(action)
		if err == nil {
			t.Error("Register() should reject duplicate name")
		}
	})
}

func TestActionRegistry_Get(t *testing.T) {
	t.Parallel()

	reg := NewActionRegistry()
	_ = reg.Register(RegisteredAction{
		Name: "my_action",
		Execute: func(_ *Context, _ statekit.Event) error {
			return nil
		},
	})

	t.Run("finds registered action", func(t *testing.T) {
		t.Parallel()

		action := reg.Get("my_action")
		if action == nil {
			t.Fatal("Get() should find registered action")
		}
		if action.Name != "my_action" {
			t.Errorf("Get() name = %s, want my_action", action.Name)
		}
	})

	t.Run("returns nil for unknown action", func(t *testing.T) {
		t.Parallel()

		action := reg.Get("unknown")
		if action != nil {
			t.Error("Get() should return nil for unknown action")
		}
	})
}

func TestActionRegistry_Names(t *testing.T) {
	t.Parallel()

	reg := NewActionRegistry()
	_ = reg.Register(RegisteredAction{
		Name:    "alpha",
		Execute: func(_ *Context, _ statekit.Event) error { return nil },
	})
	_ = reg.Register(RegisteredAction{
		Name:    "beta",
		Execute: func(_ *Context, _ statekit.Event) error { return nil },
	})

	names := reg.Names()
	if len(names) != 2 {
		t.Fatalf("Names() returned %d names, want 2", len(names))
	}

	nameSet := make(map[string]bool)
	for _, n := range names {
		nameSet[n] = true
	}
	if !nameSet["alpha"] || !nameSet["beta"] {
		t.Errorf("Names() = %v, want [alpha, beta]", names)
	}
}

func TestActionRegistry_Execute_Priority(t *testing.T) {
	t.Parallel()

	var order []string
	event := statekit.Event{Type: "TEST"}

	reg := NewActionRegistry()
	_ = reg.Register(RegisteredAction{
		Name:     "third",
		Priority: 30,
		Execute: func(_ *Context, _ statekit.Event) error {
			order = append(order, "third")
			return nil
		},
	})
	_ = reg.Register(RegisteredAction{
		Name:     "first",
		Priority: 10,
		Execute: func(_ *Context, _ statekit.Event) error {
			order = append(order, "first")
			return nil
		},
	})
	_ = reg.Register(RegisteredAction{
		Name:     "second",
		Priority: 20,
		Execute: func(_ *Context, _ statekit.Event) error {
			order = append(order, "second")
			return nil
		},
	})

	err := reg.Execute(nil, event)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(order) != 3 {
		t.Fatalf("expected 3 actions executed, got %d", len(order))
	}
	if order[0] != "first" || order[1] != "second" || order[2] != "third" {
		t.Errorf("execution order = %v, want [first, second, third]", order)
	}
}

func TestActionRegistry_Execute_EqualPriority_PreservesOrder(t *testing.T) {
	t.Parallel()

	var order []string
	event := statekit.Event{Type: "TEST"}

	reg := NewActionRegistry()
	_ = reg.Register(RegisteredAction{
		Name:     "alpha",
		Priority: 10,
		Execute: func(_ *Context, _ statekit.Event) error {
			order = append(order, "alpha")
			return nil
		},
	})
	_ = reg.Register(RegisteredAction{
		Name:     "beta",
		Priority: 10,
		Execute: func(_ *Context, _ statekit.Event) error {
			order = append(order, "beta")
			return nil
		},
	})

	err := reg.Execute(nil, event)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	if len(order) != 2 || order[0] != "alpha" || order[1] != "beta" {
		t.Errorf("execution order = %v, want [alpha, beta]", order)
	}
}

func TestActionRegistry_Execute_Rollback(t *testing.T) {
	t.Parallel()

	var order []string
	actionErr := errors.New("action failed")
	event := statekit.Event{Type: "TEST"}

	reg := NewActionRegistry()
	_ = reg.Register(RegisteredAction{
		Name:     "first",
		Priority: 10,
		Execute: func(_ *Context, _ statekit.Event) error {
			order = append(order, "exec_first")
			return nil
		},
		Rollback: func(_ *Context, _ statekit.Event) error {
			order = append(order, "rollback_first")
			return nil
		},
	})
	_ = reg.Register(RegisteredAction{
		Name:     "second",
		Priority: 20,
		Execute: func(_ *Context, _ statekit.Event) error {
			order = append(order, "exec_second")
			return nil
		},
		Rollback: func(_ *Context, _ statekit.Event) error {
			order = append(order, "rollback_second")
			return nil
		},
	})
	_ = reg.Register(RegisteredAction{
		Name:     "third_fails",
		Priority: 30,
		Execute: func(_ *Context, _ statekit.Event) error {
			order = append(order, "exec_third")
			return actionErr
		},
		Rollback: func(_ *Context, _ statekit.Event) error {
			order = append(order, "rollback_third")
			return nil
		},
	})

	err := reg.Execute(nil, event)
	if err == nil {
		t.Fatal("Execute() should return error")
	}
	if !errors.Is(err, actionErr) {
		t.Errorf("Execute() error = %v, want wrapped %v", err, actionErr)
	}

	// Execution: first, second, third (fails)
	// Rollback: second (reverse order), first (reverse order)
	// third's rollback should NOT be called because it's the one that failed
	expected := []string{"exec_first", "exec_second", "exec_third", "rollback_second", "rollback_first"}
	if len(order) != len(expected) {
		t.Fatalf("operation order = %v, want %v", order, expected)
	}
	for i, op := range expected {
		if order[i] != op {
			t.Errorf("order[%d] = %s, want %s", i, order[i], op)
		}
	}
}

func TestActionRegistry_Execute_RollbackContinuesOnError(t *testing.T) {
	t.Parallel()

	var rollbackOrder []string
	event := statekit.Event{Type: "TEST"}

	reg := NewActionRegistry()
	_ = reg.Register(RegisteredAction{
		Name:     "first",
		Priority: 10,
		Execute:  func(_ *Context, _ statekit.Event) error { return nil },
		Rollback: func(_ *Context, _ statekit.Event) error {
			rollbackOrder = append(rollbackOrder, "first")
			return nil
		},
	})
	_ = reg.Register(RegisteredAction{
		Name:     "second",
		Priority: 20,
		Execute:  func(_ *Context, _ statekit.Event) error { return nil },
		Rollback: func(_ *Context, _ statekit.Event) error {
			rollbackOrder = append(rollbackOrder, "second")
			return errors.New("rollback failed")
		},
	})
	_ = reg.Register(RegisteredAction{
		Name:     "third_fails",
		Priority: 30,
		Execute:  func(_ *Context, _ statekit.Event) error { return errors.New("fail") },
	})

	_ = reg.Execute(nil, event)

	// Even though second's rollback fails, first's rollback should still run
	if len(rollbackOrder) != 2 {
		t.Fatalf("rollback count = %d, want 2", len(rollbackOrder))
	}
	if rollbackOrder[0] != "second" || rollbackOrder[1] != "first" {
		t.Errorf("rollback order = %v, want [second, first]", rollbackOrder)
	}
}

func TestActionRegistry_Execute_NoRollbackOnSuccess(t *testing.T) {
	t.Parallel()

	rollbackCalled := false
	event := statekit.Event{Type: "TEST"}

	reg := NewActionRegistry()
	_ = reg.Register(RegisteredAction{
		Name:    "action",
		Execute: func(_ *Context, _ statekit.Event) error { return nil },
		Rollback: func(_ *Context, _ statekit.Event) error {
			rollbackCalled = true
			return nil
		},
	})

	err := reg.Execute(nil, event)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if rollbackCalled {
		t.Error("rollback should not be called on success")
	}
}

func TestActionRegistry_Execute_NilRollback(t *testing.T) {
	t.Parallel()

	event := statekit.Event{Type: "TEST"}

	reg := NewActionRegistry()
	_ = reg.Register(RegisteredAction{
		Name:     "first",
		Priority: 10,
		Execute:  func(_ *Context, _ statekit.Event) error { return nil },
		Rollback: nil, // No rollback
	})
	_ = reg.Register(RegisteredAction{
		Name:     "fails",
		Priority: 20,
		Execute:  func(_ *Context, _ statekit.Event) error { return errors.New("fail") },
	})

	// Should not panic even though first has nil rollback
	err := reg.Execute(nil, event)
	if err == nil {
		t.Error("Execute() should return error")
	}
}

func TestActionRegistry_All(t *testing.T) {
	t.Parallel()

	reg := NewActionRegistry()
	_ = reg.Register(RegisteredAction{
		Name:    "a",
		Execute: func(_ *Context, _ statekit.Event) error { return nil },
	})
	_ = reg.Register(RegisteredAction{
		Name:    "b",
		Execute: func(_ *Context, _ statekit.Event) error { return nil },
	})

	all := reg.All()
	if len(all) != 2 {
		t.Fatalf("All() returned %d actions, want 2", len(all))
	}
}

func TestActionRegistry_Execute_Empty(t *testing.T) {
	t.Parallel()

	reg := NewActionRegistry()
	event := statekit.Event{Type: "TEST"}

	err := reg.Execute(nil, event)
	if err != nil {
		t.Fatalf("Execute() on empty registry error = %v", err)
	}
}
