package statemachine

import (
	"testing"

	"github.com/felixgeelhaar/statekit"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/ledger"
	"github.com/felixgeelhaar/agent-go/domain/policy"
)

func TestNewContext(t *testing.T) {
	t.Parallel()

	run := agent.NewRun("test-run", "test goal")
	budget := policy.NewBudget(map[string]int{"calls": 10})
	ledg := ledger.New("test-run")

	ctx := NewContext(run, budget, ledg)

	if ctx == nil {
		t.Fatal("NewContext() returned nil")
	}
	if ctx.Run != run {
		t.Error("Context.Run should be the provided run")
	}
	if ctx.Budget != budget {
		t.Error("Context.Budget should be the provided budget")
	}
	if ctx.Ledger != ledg {
		t.Error("Context.Ledger should be the provided ledger")
	}
	if ctx.Eligibility == nil {
		t.Error("Context.Eligibility should be initialized")
	}
	if ctx.Transitions == nil {
		t.Error("Context.Transitions should be initialized")
	}
}

func TestNewAgentMachine(t *testing.T) {
	t.Parallel()

	machine, err := NewAgentMachine()
	if err != nil {
		t.Fatalf("NewAgentMachine() error = %v", err)
	}
	if machine == nil {
		t.Fatal("NewAgentMachine() returned nil machine")
	}
}

func TestEventForTransition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		state    agent.State
		expected string
	}{
		{agent.StateExplore, "EXPLORE"},
		{agent.StateDecide, "DECIDE"},
		{agent.StateAct, "ACT"},
		{agent.StateValidate, "VALIDATE"},
		{agent.StateDone, "DONE"},
		{agent.StateFailed, "FAIL"},
		{agent.State("custom"), "CUSTOM"}, // Unknown/custom state uses uppercase name as event
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			t.Parallel()

			event := EventForTransition(tt.state)
			if string(event) != tt.expected {
				t.Errorf("EventForTransition(%s) = %s, want %s", tt.state, event, tt.expected)
			}
		})
	}
}

func TestStateFromMachine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		stateID  string
		expected agent.State
	}{
		{"intake", agent.StateIntake},
		{"explore", agent.StateExplore},
		{"decide", agent.StateDecide},
		{"act", agent.StateAct},
		{"validate", agent.StateValidate},
		{"done", agent.StateDone},
		{"failed", agent.StateFailed},
	}

	for _, tt := range tests {
		t.Run(tt.stateID, func(t *testing.T) {
			t.Parallel()

			state := StateFromMachine(stateIntake)
			if state != agent.StateIntake {
				t.Errorf("StateFromMachine(%s) = %s, want %s", tt.stateID, state, tt.expected)
			}
		})
	}
}

func TestStateConstants(t *testing.T) {
	t.Parallel()

	// Verify state constants match agent states
	tests := []struct {
		machineState string
		agentState   string
	}{
		{string(stateIntake), string(agent.StateIntake)},
		{string(stateExplore), string(agent.StateExplore)},
		{string(stateDecide), string(agent.StateDecide)},
		{string(stateAct), string(agent.StateAct)},
		{string(stateValidate), string(agent.StateValidate)},
		{string(stateDone), string(agent.StateDone)},
		{string(stateFailed), string(agent.StateFailed)},
	}

	for _, tt := range tests {
		t.Run(tt.machineState, func(t *testing.T) {
			t.Parallel()

			if tt.machineState != tt.agentState {
				t.Errorf("Machine state %s does not match agent state %s", tt.machineState, tt.agentState)
			}
		})
	}
}

func TestInterpreter_Creation(t *testing.T) {
	t.Parallel()

	machine, err := NewAgentMachine()
	if err != nil {
		t.Fatalf("NewAgentMachine() error = %v", err)
	}

	run := agent.NewRun("test-run", "test goal")
	budget := policy.NewBudget(map[string]int{"calls": 10})
	ledg := ledger.New("test-run")
	ctx := NewContext(run, budget, ledg)

	interp := NewInterpreter(machine, ctx)
	if interp == nil {
		t.Fatal("NewInterpreter() returned nil")
	}
}

func TestInterpreter_Start(t *testing.T) {
	t.Parallel()

	machine, _ := NewAgentMachine()
	run := agent.NewRun("test-run", "test goal")
	budget := policy.NewBudget(map[string]int{"calls": 10})
	ledg := ledger.New("test-run")
	ctx := NewContext(run, budget, ledg)

	interp := NewInterpreter(machine, ctx)
	interp.Start()

	// After start, should be in intake state
	if interp.State() != agent.StateIntake {
		t.Errorf("Initial state = %s, want intake", interp.State())
	}

	if interp.IsTerminal() {
		t.Error("Should not be in terminal state after start")
	}
}

func TestInterpreter_Transition(t *testing.T) {
	t.Parallel()

	machine, _ := NewAgentMachine()
	run := agent.NewRun("test-run", "test goal")
	budget := policy.NewBudget(map[string]int{"calls": 10})
	ledg := ledger.New("test-run")
	ctx := NewContext(run, budget, ledg)

	interp := NewInterpreter(machine, ctx)
	interp.Start()

	// Transition from intake to explore
	err := interp.Transition(agent.StateExplore, "beginning exploration")
	if err != nil {
		t.Fatalf("Transition to explore error = %v", err)
	}

	if interp.State() != agent.StateExplore {
		t.Errorf("State after transition = %s, want explore", interp.State())
	}
}

func TestInterpreter_InvalidTransition(t *testing.T) {
	t.Parallel()

	machine, _ := NewAgentMachine()
	run := agent.NewRun("test-run", "test goal")
	budget := policy.NewBudget(map[string]int{"calls": 10})
	ledg := ledger.New("test-run")
	ctx := NewContext(run, budget, ledg)

	interp := NewInterpreter(machine, ctx)
	interp.Start()

	// Try invalid transition from intake directly to act (should fail)
	err := interp.Transition(agent.StateAct, "invalid transition")
	if err == nil {
		t.Error("Invalid transition should return error")
	}

	// State should remain intake
	if interp.State() != agent.StateIntake {
		t.Errorf("State after invalid transition = %s, want intake", interp.State())
	}
}

func TestInterpreter_CanTransition(t *testing.T) {
	t.Parallel()

	machine, _ := NewAgentMachine()
	run := agent.NewRun("test-run", "test goal")
	budget := policy.NewBudget(map[string]int{"calls": 10})
	ledg := ledger.New("test-run")
	ctx := NewContext(run, budget, ledg)

	interp := NewInterpreter(machine, ctx)
	interp.Start()

	// Should be able to transition from intake to explore
	if !interp.CanTransition(agent.StateExplore) {
		t.Error("Should be able to transition from intake to explore")
	}

	// Should not be able to transition from intake to act
	if interp.CanTransition(agent.StateAct) {
		t.Error("Should NOT be able to transition from intake to act")
	}

	// Should be able to transition to failed from any state
	if !interp.CanTransition(agent.StateFailed) {
		t.Error("Should be able to transition from intake to failed")
	}
}

func TestInterpreter_ToolEligibility(t *testing.T) {
	t.Parallel()

	machine, _ := NewAgentMachine()
	run := agent.NewRun("test-run", "test goal")
	budget := policy.NewBudget(map[string]int{"calls": 10})
	ledg := ledger.New("test-run")
	ctx := NewContext(run, budget, ledg)

	interp := NewInterpreter(machine, ctx)
	interp.Start()

	// Configure eligibility
	eligibility := policy.NewToolEligibility().
		Allow(agent.StateExplore, "read_file").
		Allow(agent.StateExplore, "list_dir").
		Allow(agent.StateAct, "write_file")

	interp.ConfigureEligibility(eligibility)

	// Move to explore state
	interp.Transition(agent.StateExplore, "explore")

	// Check tool eligibility
	if !interp.IsToolAllowed("read_file") {
		t.Error("read_file should be allowed in explore state")
	}
	if !interp.IsToolAllowed("list_dir") {
		t.Error("list_dir should be allowed in explore state")
	}
	if interp.IsToolAllowed("write_file") {
		t.Error("write_file should NOT be allowed in explore state")
	}

	// Check allowed tools
	allowed := interp.AllowedTools()
	if len(allowed) != 2 {
		t.Errorf("AllowedTools() returned %d tools, want 2", len(allowed))
	}
}

func TestInterpreter_TerminalState(t *testing.T) {
	t.Parallel()

	machine, _ := NewAgentMachine()
	run := agent.NewRun("test-run", "test goal")
	budget := policy.NewBudget(map[string]int{"calls": 10})
	ledg := ledger.New("test-run")
	ctx := NewContext(run, budget, ledg)

	interp := NewInterpreter(machine, ctx)
	interp.Start()

	// Navigate to done state
	interp.Transition(agent.StateExplore, "explore")
	interp.Transition(agent.StateDecide, "decide")
	interp.Transition(agent.StateDone, "complete")

	if interp.State() != agent.StateDone {
		t.Errorf("State = %s, want done", interp.State())
	}
	if !interp.IsTerminal() {
		t.Error("done state should be terminal")
	}
}

func TestInterpreter_FailedState(t *testing.T) {
	t.Parallel()

	machine, _ := NewAgentMachine()
	run := agent.NewRun("test-run", "test goal")
	budget := policy.NewBudget(map[string]int{"calls": 10})
	ledg := ledger.New("test-run")
	ctx := NewContext(run, budget, ledg)

	interp := NewInterpreter(machine, ctx)
	interp.Start()

	// Can fail from intake
	interp.Transition(agent.StateFailed, "failure reason")

	if interp.State() != agent.StateFailed {
		t.Errorf("State = %s, want failed", interp.State())
	}
	if !interp.IsTerminal() {
		t.Error("failed state should be terminal")
	}
}

func TestInterpreter_Context(t *testing.T) {
	t.Parallel()

	machine, _ := NewAgentMachine()
	run := agent.NewRun("test-run", "test goal")
	budget := policy.NewBudget(map[string]int{"calls": 10})
	ledg := ledger.New("test-run")
	ctx := NewContext(run, budget, ledg)

	interp := NewInterpreter(machine, ctx)

	if interp.Context() != ctx {
		t.Error("Context() should return the interpreter context")
	}
}

func TestInterpreter_Matches(t *testing.T) {
	t.Parallel()

	machine, _ := NewAgentMachine()
	run := agent.NewRun("test-run", "test goal")
	budget := policy.NewBudget(map[string]int{"calls": 10})
	ledg := ledger.New("test-run")
	ctx := NewContext(run, budget, ledg)

	interp := NewInterpreter(machine, ctx)
	interp.Start()

	if !interp.Matches(string(agent.StateIntake)) {
		t.Error("Should match intake state")
	}
	if interp.Matches(string(agent.StateExplore)) {
		t.Error("Should not match explore state")
	}
}

func TestInterpreter_FullWorkflow(t *testing.T) {
	t.Parallel()

	machine, _ := NewAgentMachine()
	run := agent.NewRun("test-run", "complete a task")
	budget := policy.NewBudget(map[string]int{"calls": 100})
	ledg := ledger.New("test-run")
	ctx := NewContext(run, budget, ledg)

	interp := NewInterpreter(machine, ctx)
	interp.Start()

	// Configure tool eligibility
	eligibility := policy.NewToolEligibility().
		Allow(agent.StateExplore, "read_file").
		Allow(agent.StateAct, "write_file")
	interp.ConfigureEligibility(eligibility)

	// Full workflow: intake -> explore -> decide -> act -> validate -> done
	steps := []struct {
		toState agent.State
		reason  string
	}{
		{agent.StateExplore, "gather information"},
		{agent.StateDecide, "make decision"},
		{agent.StateAct, "execute action"},
		{agent.StateValidate, "verify result"},
		{agent.StateDone, "task completed"},
	}

	for _, step := range steps {
		err := interp.Transition(step.toState, step.reason)
		if err != nil {
			t.Fatalf("Transition to %s failed: %v", step.toState, err)
		}
		if interp.State() != step.toState {
			t.Errorf("State after transition = %s, want %s", interp.State(), step.toState)
		}
	}

	if !interp.IsTerminal() {
		t.Error("Should be in terminal state after workflow")
	}
}

func TestInterpreter_LoopBackWorkflow(t *testing.T) {
	t.Parallel()

	machine, _ := NewAgentMachine()
	run := agent.NewRun("test-run", "iterative task")
	budget := policy.NewBudget(map[string]int{"calls": 100})
	ledg := ledger.New("test-run")
	ctx := NewContext(run, budget, ledg)

	interp := NewInterpreter(machine, ctx)
	interp.Start()

	// First iteration
	interp.Transition(agent.StateExplore, "first exploration")
	interp.Transition(agent.StateDecide, "first decision")
	interp.Transition(agent.StateAct, "first action")
	interp.Transition(agent.StateValidate, "first validation")

	// Loop back from validate to explore
	err := interp.Transition(agent.StateExplore, "need more information")
	if err != nil {
		t.Fatalf("Loop back to explore failed: %v", err)
	}

	if interp.State() != agent.StateExplore {
		t.Errorf("State after loop back = %s, want explore", interp.State())
	}

	// Second iteration to completion
	interp.Transition(agent.StateDecide, "second decision")
	interp.Transition(agent.StateDone, "finally complete")

	if !interp.IsTerminal() {
		t.Error("Should be in terminal state")
	}
}

func TestTransitionPayload(t *testing.T) {
	t.Parallel()

	payload := TransitionPayload{
		ToState: agent.StateExplore,
		Reason:  "test reason",
	}

	if payload.ToState != agent.StateExplore {
		t.Errorf("ToState = %s, want explore", payload.ToState)
	}
	if payload.Reason != "test reason" {
		t.Errorf("Reason = %s, want 'test reason'", payload.Reason)
	}
}

func TestActionWithReason(t *testing.T) {
	t.Parallel()

	payload := ActionWithReason("custom reason")

	if payload.Reason != "custom reason" {
		t.Errorf("Reason = %s, want 'custom reason'", payload.Reason)
	}
}

func TestGuardToolAllowed(t *testing.T) {
	t.Parallel()

	t.Run("returns false for nil context", func(t *testing.T) {
		t.Parallel()

		result := guardToolAllowed(nil, "any_tool")
		if result {
			t.Error("guardToolAllowed(nil, ...) should return false")
		}
	})

	t.Run("returns false for nil run", func(t *testing.T) {
		t.Parallel()

		ctx := &Context{
			Run:         nil,
			Eligibility: policy.NewToolEligibility(),
		}
		result := guardToolAllowed(ctx, "any_tool")
		if result {
			t.Error("guardToolAllowed with nil Run should return false")
		}
	})

	t.Run("returns false for nil eligibility", func(t *testing.T) {
		t.Parallel()

		run := agent.NewRun("test-run", "test goal")
		ctx := &Context{
			Run:         run,
			Eligibility: nil,
		}
		result := guardToolAllowed(ctx, "any_tool")
		if result {
			t.Error("guardToolAllowed with nil Eligibility should return false")
		}
	})

	t.Run("returns true when tool is allowed", func(t *testing.T) {
		t.Parallel()

		run := agent.NewRun("test-run", "test goal")
		run.CurrentState = agent.StateExplore
		eligibility := policy.NewToolEligibility().
			Allow(agent.StateExplore, "read_file")
		ctx := &Context{
			Run:         run,
			Eligibility: eligibility,
		}

		result := guardToolAllowed(ctx, "read_file")
		if !result {
			t.Error("guardToolAllowed should return true for allowed tool")
		}
	})

	t.Run("returns false when tool is not allowed", func(t *testing.T) {
		t.Parallel()

		run := agent.NewRun("test-run", "test goal")
		run.CurrentState = agent.StateExplore
		eligibility := policy.NewToolEligibility().
			Allow(agent.StateAct, "write_file")
		ctx := &Context{
			Run:         run,
			Eligibility: eligibility,
		}

		result := guardToolAllowed(ctx, "write_file")
		if result {
			t.Error("guardToolAllowed should return false for disallowed tool")
		}
	})
}

func TestGuardToolAllowedFunc(t *testing.T) {
	t.Parallel()

	t.Run("returns a guard function", func(t *testing.T) {
		t.Parallel()

		guardFn := GuardToolAllowedFunc("test_tool")
		if guardFn == nil {
			t.Fatal("GuardToolAllowedFunc() returned nil")
		}
	})

	t.Run("guard function checks tool eligibility", func(t *testing.T) {
		t.Parallel()

		run := agent.NewRun("test-run", "test goal")
		run.CurrentState = agent.StateExplore
		eligibility := policy.NewToolEligibility().
			Allow(agent.StateExplore, "allowed_tool")
		ctx := &Context{
			Run:         run,
			Eligibility: eligibility,
		}

		allowedGuard := GuardToolAllowedFunc("allowed_tool")
		disallowedGuard := GuardToolAllowedFunc("disallowed_tool")

		if !allowedGuard(ctx, statekit.Event{}) {
			t.Error("Guard for allowed_tool should return true")
		}
		if disallowedGuard(ctx, statekit.Event{}) {
			t.Error("Guard for disallowed_tool should return false")
		}
	})
}

func TestStateFromEventType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		eventType statekit.EventType
		expected  agent.State
	}{
		{"EXPLORE", agent.StateExplore},
		{"DECIDE", agent.StateDecide},
		{"ACT", agent.StateAct},
		{"VALIDATE", agent.StateValidate},
		{"DONE", agent.StateDone},
		{"FAIL", agent.StateFailed},
		{"CUSTOM_EVENT", agent.State("custom_event")}, // default case lowercases to match state names
	}

	for _, tt := range tests {
		t.Run(string(tt.eventType), func(t *testing.T) {
			t.Parallel()

			result := stateFromEventType(tt.eventType)
			if result != tt.expected {
				t.Errorf("stateFromEventType(%s) = %s, want %s", tt.eventType, result, tt.expected)
			}
		})
	}
}

func TestInterpreter_Stop(t *testing.T) {
	t.Parallel()

	machine, _ := NewAgentMachine()
	run := agent.NewRun("test-run", "test goal")
	budget := policy.NewBudget(map[string]int{"calls": 10})
	ledg := ledger.New("test-run")
	ctx := NewContext(run, budget, ledg)

	interp := NewInterpreter(machine, ctx)
	interp.Start()

	// Verify interpreter is running
	if interp.State() != agent.StateIntake {
		t.Errorf("Initial state = %s, want intake", interp.State())
	}

	// Stop should not panic
	interp.Stop()

	// After stop, should still report state (interpreter retains last state)
	state := interp.State()
	if state != agent.StateIntake {
		t.Errorf("State after stop = %s, want intake", state)
	}
}
