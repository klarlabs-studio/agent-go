package policy_test

import (
	"testing"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/policy"
)

func TestEligibilitySnapshot(t *testing.T) {
	t.Parallel()

	t.Run("new snapshot", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.NewEligibilitySnapshot()
		if snapshot.StateTools == nil {
			t.Error("StateTools should be initialized")
		}
	})

	t.Run("add tool", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.NewEligibilitySnapshot()
		snapshot.AddTool(agent.StateExplore, "read_file")

		if !snapshot.IsAllowed(agent.StateExplore, "read_file") {
			t.Error("Tool should be allowed after adding")
		}
	})

	t.Run("add tool to nil map", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.EligibilitySnapshot{}
		snapshot.AddTool(agent.StateExplore, "read_file")

		if !snapshot.IsAllowed(agent.StateExplore, "read_file") {
			t.Error("Tool should be allowed after adding")
		}
	})

	t.Run("add duplicate tool", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.NewEligibilitySnapshot()
		snapshot.AddTool(agent.StateExplore, "read_file")
		snapshot.AddTool(agent.StateExplore, "read_file")

		if len(snapshot.StateTools[agent.StateExplore]) != 1 {
			t.Errorf("Tool count = %d, want 1", len(snapshot.StateTools[agent.StateExplore]))
		}
	})

	t.Run("remove tool", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.NewEligibilitySnapshot()
		snapshot.AddTool(agent.StateExplore, "read_file")
		snapshot.AddTool(agent.StateExplore, "write_file")
		snapshot.RemoveTool(agent.StateExplore, "read_file")

		if snapshot.IsAllowed(agent.StateExplore, "read_file") {
			t.Error("Tool should not be allowed after removing")
		}
		if !snapshot.IsAllowed(agent.StateExplore, "write_file") {
			t.Error("Other tools should still be allowed")
		}
	})

	t.Run("remove tool from nil map", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.EligibilitySnapshot{}
		snapshot.RemoveTool(agent.StateExplore, "read_file") // Should not panic
	})

	t.Run("remove non-existent tool", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.NewEligibilitySnapshot()
		snapshot.AddTool(agent.StateExplore, "read_file")
		snapshot.RemoveTool(agent.StateExplore, "write_file") // Should not panic

		if !snapshot.IsAllowed(agent.StateExplore, "read_file") {
			t.Error("Existing tool should still be allowed")
		}
	})

	t.Run("is allowed - not found state", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.NewEligibilitySnapshot()
		snapshot.AddTool(agent.StateExplore, "read_file")

		if snapshot.IsAllowed(agent.StateAct, "read_file") {
			t.Error("Tool should not be allowed in different state")
		}
	})

	t.Run("is allowed - not found tool", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.NewEligibilitySnapshot()
		snapshot.AddTool(agent.StateExplore, "read_file")

		if snapshot.IsAllowed(agent.StateExplore, "write_file") {
			t.Error("Non-existent tool should not be allowed")
		}
	})
}

func TestTransitionSnapshot(t *testing.T) {
	t.Parallel()

	t.Run("new snapshot", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.NewTransitionSnapshot()
		if snapshot.Transitions == nil {
			t.Error("Transitions should be initialized")
		}
	})

	t.Run("add transition", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.NewTransitionSnapshot()
		snapshot.AddTransition(agent.StateIntake, agent.StateExplore)

		if !snapshot.IsAllowed(agent.StateIntake, agent.StateExplore) {
			t.Error("Transition should be allowed after adding")
		}
	})

	t.Run("add transition to nil map", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.TransitionSnapshot{}
		snapshot.AddTransition(agent.StateIntake, agent.StateExplore)

		if !snapshot.IsAllowed(agent.StateIntake, agent.StateExplore) {
			t.Error("Transition should be allowed after adding")
		}
	})

	t.Run("add duplicate transition", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.NewTransitionSnapshot()
		snapshot.AddTransition(agent.StateIntake, agent.StateExplore)
		snapshot.AddTransition(agent.StateIntake, agent.StateExplore)

		if len(snapshot.Transitions[agent.StateIntake]) != 1 {
			t.Errorf("Transition count = %d, want 1", len(snapshot.Transitions[agent.StateIntake]))
		}
	})

	t.Run("remove transition", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.NewTransitionSnapshot()
		snapshot.AddTransition(agent.StateIntake, agent.StateExplore)
		snapshot.AddTransition(agent.StateIntake, agent.StateDecide)
		snapshot.RemoveTransition(agent.StateIntake, agent.StateExplore)

		if snapshot.IsAllowed(agent.StateIntake, agent.StateExplore) {
			t.Error("Transition should not be allowed after removing")
		}
		if !snapshot.IsAllowed(agent.StateIntake, agent.StateDecide) {
			t.Error("Other transitions should still be allowed")
		}
	})

	t.Run("remove transition from nil map", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.TransitionSnapshot{}
		snapshot.RemoveTransition(agent.StateIntake, agent.StateExplore) // Should not panic
	})

	t.Run("remove non-existent transition", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.NewTransitionSnapshot()
		snapshot.AddTransition(agent.StateIntake, agent.StateExplore)
		snapshot.RemoveTransition(agent.StateIntake, agent.StateDecide) // Should not panic

		if !snapshot.IsAllowed(agent.StateIntake, agent.StateExplore) {
			t.Error("Existing transition should still be allowed")
		}
	})

	t.Run("is allowed - not found from state", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.NewTransitionSnapshot()
		snapshot.AddTransition(agent.StateIntake, agent.StateExplore)

		if snapshot.IsAllowed(agent.StateExplore, agent.StateDecide) {
			t.Error("Transition from different state should not be allowed")
		}
	})

	t.Run("is allowed - not found to state", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.NewTransitionSnapshot()
		snapshot.AddTransition(agent.StateIntake, agent.StateExplore)

		if snapshot.IsAllowed(agent.StateIntake, agent.StateDecide) {
			t.Error("Transition to non-allowed state should not be allowed")
		}
	})
}

func TestBudgetLimitsSnapshot(t *testing.T) {
	t.Parallel()

	t.Run("new snapshot", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.NewBudgetLimitsSnapshot()
		if snapshot.Limits == nil {
			t.Error("Limits should be initialized")
		}
	})

	t.Run("set and get limit", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.NewBudgetLimitsSnapshot()
		snapshot.SetLimit("calls", 100)

		limit, ok := snapshot.GetLimit("calls")
		if !ok {
			t.Error("Limit should exist")
		}
		if limit != 100 {
			t.Errorf("Limit = %d, want 100", limit)
		}
	})

	t.Run("set limit on nil map", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.BudgetLimitsSnapshot{}
		snapshot.SetLimit("calls", 50)

		limit, ok := snapshot.GetLimit("calls")
		if !ok {
			t.Error("Limit should exist")
		}
		if limit != 50 {
			t.Errorf("Limit = %d, want 50", limit)
		}
	})

	t.Run("get non-existent limit", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.NewBudgetLimitsSnapshot()

		_, ok := snapshot.GetLimit("nonexistent")
		if ok {
			t.Error("Non-existent limit should return false")
		}
	})

	t.Run("override limit", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.NewBudgetLimitsSnapshot()
		snapshot.SetLimit("calls", 100)
		snapshot.SetLimit("calls", 200)

		limit, _ := snapshot.GetLimit("calls")
		if limit != 200 {
			t.Errorf("Limit = %d, want 200", limit)
		}
	})
}

func TestApprovalSnapshot(t *testing.T) {
	t.Parallel()

	t.Run("new snapshot", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.NewApprovalSnapshot()
		if snapshot.RequiredTools == nil {
			t.Error("RequiredTools should be initialized")
		}
	})

	t.Run("require approval", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.NewApprovalSnapshot()
		snapshot.RequireApproval("delete_tool")

		if !snapshot.IsRequired("delete_tool") {
			t.Error("Tool should require approval")
		}
	})

	t.Run("require approval duplicate", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.NewApprovalSnapshot()
		snapshot.RequireApproval("delete_tool")
		snapshot.RequireApproval("delete_tool")

		if len(snapshot.RequiredTools) != 1 {
			t.Errorf("RequiredTools count = %d, want 1", len(snapshot.RequiredTools))
		}
	})

	t.Run("remove approval", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.NewApprovalSnapshot()
		snapshot.RequireApproval("delete_tool")
		snapshot.RequireApproval("write_tool")
		snapshot.RemoveApproval("delete_tool")

		if snapshot.IsRequired("delete_tool") {
			t.Error("Tool should not require approval after removal")
		}
		if !snapshot.IsRequired("write_tool") {
			t.Error("Other tools should still require approval")
		}
	})

	t.Run("remove non-existent approval", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.NewApprovalSnapshot()
		snapshot.RequireApproval("delete_tool")
		snapshot.RemoveApproval("write_tool") // Should not panic

		if !snapshot.IsRequired("delete_tool") {
			t.Error("Existing tool should still require approval")
		}
	})

	t.Run("is required - not found", func(t *testing.T) {
		t.Parallel()

		snapshot := policy.NewApprovalSnapshot()
		snapshot.RequireApproval("delete_tool")

		if snapshot.IsRequired("read_tool") {
			t.Error("Non-required tool should return false")
		}
	})
}
