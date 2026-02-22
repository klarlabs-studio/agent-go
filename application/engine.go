// Package application provides the application layer for the agent runtime.
package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/artifact"
	"github.com/felixgeelhaar/agent-go/domain/knowledge"
	"github.com/felixgeelhaar/agent-go/domain/ledger"
	"github.com/felixgeelhaar/agent-go/domain/middleware"
	"github.com/felixgeelhaar/agent-go/domain/policy"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/felixgeelhaar/agent-go/infrastructure/logging"
	inframw "github.com/felixgeelhaar/agent-go/infrastructure/middleware"
	"github.com/felixgeelhaar/agent-go/infrastructure/planner"
	"github.com/felixgeelhaar/agent-go/infrastructure/resilience"
	"github.com/felixgeelhaar/agent-go/infrastructure/statemachine"
)

// Engine is the main orchestration service for agent execution.
type Engine struct {
	registry     tool.Registry
	planner      planner.Planner
	executor     *resilience.Executor
	artifacts    artifact.Store
	knowledge    knowledge.Store
	eligibility  *policy.ToolEligibility
	transitions  *policy.StateTransitions
	approver     policy.Approver
	budgetLimits map[string]int
	maxSteps     int
	middleware   *middleware.Registry
}

// EngineConfig contains configuration for the engine.
type EngineConfig struct {
	Registry     tool.Registry
	Planner      planner.Planner
	Executor     *resilience.Executor
	Artifacts    artifact.Store
	Knowledge    knowledge.Store
	Eligibility  *policy.ToolEligibility
	Transitions  *policy.StateTransitions
	Approver     policy.Approver
	BudgetLimits map[string]int
	MaxSteps     int
	Middleware   *middleware.Registry
}

// NewEngine creates a new engine with the given configuration.
func NewEngine(config EngineConfig) (*Engine, error) {
	if config.Registry == nil {
		return nil, errors.New("registry is required")
	}
	if config.Planner == nil {
		return nil, errors.New("planner is required")
	}

	e := &Engine{
		registry:     config.Registry,
		planner:      config.Planner,
		executor:     config.Executor,
		artifacts:    config.Artifacts,
		knowledge:    config.Knowledge,
		eligibility:  config.Eligibility,
		transitions:  config.Transitions,
		approver:     config.Approver,
		budgetLimits: config.BudgetLimits,
		maxSteps:     config.MaxSteps,
		middleware:   config.Middleware,
	}

	// Set defaults
	if e.executor == nil {
		e.executor = resilience.NewDefaultExecutor()
	}
	if e.eligibility == nil {
		e.eligibility = policy.NewToolEligibility()
	}
	if e.transitions == nil {
		e.transitions = policy.DefaultTransitions()
	}
	if e.maxSteps == 0 {
		e.maxSteps = 100
	}
	if e.middleware == nil {
		e.middleware = e.defaultMiddlewareChain()
	}

	return e, nil
}

// defaultMiddlewareChain creates the default middleware chain that replicates
// the original inline policy enforcement behavior.
func (e *Engine) defaultMiddlewareChain() *middleware.Registry {
	registry := middleware.NewRegistry()

	// Input validation (security: validate inputs before any processing)
	registry.Use(inframw.Validation(inframw.DefaultValidationConfig()))

	// Eligibility check (tool allowed in current state)
	registry.Use(inframw.Eligibility(inframw.EligibilityConfig{
		Eligibility: e.eligibility,
	}))

	// Approval check (for destructive/high-risk tools)
	registry.Use(inframw.Approval(inframw.ApprovalConfig{
		Approver: e.approver,
	}))

	// Logging (execution timing and results)
	registry.Use(inframw.Logging(inframw.LoggingConfig{
		LogInput:  false,
		LogOutput: false,
	}))

	return registry
}

// Run executes the agent with the given goal.
func (e *Engine) Run(ctx context.Context, goal string) (*agent.Run, error) {
	return e.RunWithVars(ctx, goal, nil)
}

// RunWithVars executes the agent with the given goal and initial variables.
func (e *Engine) RunWithVars(ctx context.Context, goal string, vars map[string]any) (*agent.Run, error) {
	// Generate run ID
	runID := generateRunID()

	// Create run
	run := agent.NewRun(runID, goal)
	for k, v := range vars {
		run.SetVar(k, v)
	}

	// Create supporting components
	budget := policy.NewBudget(e.budgetLimits)
	runLedger := ledger.New(runID)

	// Create state machine context
	machineCtx := statemachine.NewContext(run, budget, runLedger)
	machineCtx.Eligibility = e.eligibility
	machineCtx.Transitions = e.transitions

	// Create state machine
	machine, err := statemachine.NewAgentMachine()
	if err != nil {
		return nil, fmt.Errorf("failed to create state machine: %w", err)
	}

	// Create interpreter
	interp := statemachine.NewInterpreter(machine, machineCtx)

	// Log run start
	logging.Info().
		Add(logging.RunID(runID)).
		Add(logging.Goal(goal)).
		Msg("run started")

	// Start state machine
	interp.Start()
	runLedger.RecordRunStarted(goal)

	// Execute until terminal state or max steps
	steps := 0
	for !interp.IsTerminal() && steps < e.maxSteps {
		select {
		case <-ctx.Done():
			run.Fail("context cancelled")
			runLedger.RecordRunFailed(run.CurrentState, "context cancelled")
			return run, ctx.Err()
		default:
		}

		if err := e.step(ctx, interp, machineCtx); err != nil {
			// Handle human input request specially - not a failure
			if errors.Is(err, agent.ErrAwaitingHumanInput) {
				logging.Info().
					Add(logging.RunID(runID)).
					Add(logging.State(run.CurrentState)).
					Msg("run paused for human input")
				return run, err
			}

			run.Fail(err.Error())
			runLedger.RecordRunFailed(run.CurrentState, err.Error())

			logging.Error().
				Add(logging.RunID(runID)).
				Add(logging.State(run.CurrentState)).
				Add(logging.ErrorField(err)).
				Msg("run failed")

			return run, err
		}
		steps++
	}

	if steps >= e.maxSteps && !interp.IsTerminal() {
		run.Fail("max steps exceeded")
		runLedger.RecordRunFailed(run.CurrentState, "max steps exceeded")
		return run, errors.New("max steps exceeded")
	}

	// Log completion
	logging.Info().
		Add(logging.RunID(runID)).
		Add(logging.State(run.CurrentState)).
		Add(logging.Duration(run.Duration())).
		Msg("run completed")

	if run.Status == agent.RunStatusCompleted {
		runLedger.RecordRunCompleted(run.Result)
	}

	return run, nil
}

// ResumeWithInput continues a paused run with human-provided input.
func (e *Engine) ResumeWithInput(ctx context.Context, run *agent.Run, input string) (*agent.Run, error) {
	// Validate run exists
	if run == nil {
		return nil, errors.New("run is nil")
	}

	// Validate run has pending question
	if !run.HasPendingQuestion() {
		return nil, agent.ErrNoPendingQuestion
	}

	// Validate input against options if constrained
	if len(run.PendingQuestion.Options) > 0 {
		valid := false
		for _, opt := range run.PendingQuestion.Options {
			if opt == input {
				valid = true
				break
			}
		}
		if !valid {
			return nil, fmt.Errorf("%w: must be one of %v", agent.ErrInvalidHumanInput, run.PendingQuestion.Options)
		}
	}

	// Save question before clearing
	question := run.PendingQuestion.Question

	// Add human input as evidence
	evidenceContent, _ := json.Marshal(map[string]string{
		"question": question,
		"response": input,
	})
	run.AddEvidence(agent.NewHumanEvidence(evidenceContent))

	// Clear pending question and resume
	run.ClearPendingQuestion()
	run.Resume()

	// Create supporting components (fresh for this segment)
	budget := policy.NewBudget(e.budgetLimits)
	runLedger := ledger.New(run.ID)

	// Record human input response in ledger
	runLedger.RecordHumanInputResponse(run.CurrentState, question, input)

	// Create state machine context
	machineCtx := statemachine.NewContext(run, budget, runLedger)
	machineCtx.Eligibility = e.eligibility
	machineCtx.Transitions = e.transitions

	// Create state machine
	machine, err := statemachine.NewAgentMachine()
	if err != nil {
		return nil, fmt.Errorf("failed to create state machine: %w", err)
	}

	// Create interpreter and resume from current state
	interp := statemachine.NewInterpreter(machine, machineCtx)

	// Resume state machine from current state
	if err := interp.ResumeFrom(run.CurrentState); err != nil {
		return nil, fmt.Errorf("failed to resume state machine: %w", err)
	}

	// Log resumption
	logging.Info().
		Add(logging.RunID(run.ID)).
		Add(logging.State(run.CurrentState)).
		Add(logging.Str("human_input", input)).
		Msg("run resumed with human input")

	// Execute until terminal state or max steps
	steps := 0
	for !interp.IsTerminal() && steps < e.maxSteps {
		select {
		case <-ctx.Done():
			run.Fail("context cancelled")
			runLedger.RecordRunFailed(run.CurrentState, "context cancelled")
			return run, ctx.Err()
		default:
		}

		if err := e.step(ctx, interp, machineCtx); err != nil {
			// Handle human input request specially - not a failure
			if errors.Is(err, agent.ErrAwaitingHumanInput) {
				logging.Info().
					Add(logging.RunID(run.ID)).
					Add(logging.State(run.CurrentState)).
					Msg("run paused for human input")
				return run, err
			}

			run.Fail(err.Error())
			runLedger.RecordRunFailed(run.CurrentState, err.Error())

			logging.Error().
				Add(logging.RunID(run.ID)).
				Add(logging.State(run.CurrentState)).
				Add(logging.ErrorField(err)).
				Msg("run failed")

			return run, err
		}
		steps++
	}

	if steps >= e.maxSteps && !interp.IsTerminal() {
		run.Fail("max steps exceeded")
		runLedger.RecordRunFailed(run.CurrentState, "max steps exceeded")
		return run, errors.New("max steps exceeded")
	}

	// Log completion
	logging.Info().
		Add(logging.RunID(run.ID)).
		Add(logging.State(run.CurrentState)).
		Add(logging.Duration(run.Duration())).
		Msg("run completed after resume")

	if run.Status == agent.RunStatusCompleted {
		runLedger.RecordRunCompleted(run.Result)
	}

	return run, nil
}

// step executes a single step of the agent.
func (e *Engine) step(ctx context.Context, interp *statemachine.Interpreter, machineCtx *statemachine.Context) error {
	run := machineCtx.Run
	runLedger := machineCtx.Ledger

	// Get allowed tools for current state
	allowedTools := interp.AllowedTools()

	// Request decision from planner
	req := planner.PlanRequest{
		RunID:        run.ID,
		Goal:         run.Goal,
		CurrentState: run.CurrentState,
		Evidence:     run.Evidence,
		AllowedTools: allowedTools,
		ToolDefs:     e.buildToolDefs(allowedTools),
		Budgets:      machineCtx.Budget.Snapshot(),
		Vars:         run.Vars,
	}

	decision, err := e.planner.Plan(ctx, req)
	if err != nil {
		return fmt.Errorf("planner error: %w", err)
	}

	// Record decision
	runLedger.RecordDecision(run.CurrentState, decision)

	logging.Debug().
		Add(logging.RunID(run.ID)).
		Add(logging.State(run.CurrentState)).
		Add(logging.Decision(decision.Type)).
		Msg("planner decision")

	// Execute decision
	switch decision.Type {
	case agent.DecisionCallTool:
		return e.executeToolDecision(ctx, interp, machineCtx, decision.CallTool)
	case agent.DecisionTransition:
		return e.executeTransition(ctx, interp, decision.Transition)
	case agent.DecisionAskHuman:
		return e.executeAskHuman(ctx, interp, machineCtx, decision.AskHuman)
	case agent.DecisionFinish:
		return e.executeFinish(ctx, interp, machineCtx, decision.Finish)
	case agent.DecisionFail:
		return e.executeFail(ctx, interp, machineCtx, decision.Fail)
	default:
		return fmt.Errorf("unknown decision type: %s", decision.Type)
	}
}

// executeToolDecision executes a tool call decision using the middleware chain.
func (e *Engine) executeToolDecision(ctx context.Context, _ *statemachine.Interpreter, machineCtx *statemachine.Context, decision *agent.CallToolDecision) error {
	run := machineCtx.Run
	runLedger := machineCtx.Ledger
	budget := machineCtx.Budget

	// Get tool from registry
	t, ok := e.registry.Get(decision.ToolName)
	if !ok {
		return fmt.Errorf("%w: %s", tool.ErrToolNotFound, decision.ToolName)
	}

	// Check budget before execution
	if !budget.CanConsume("tool_calls", 1) {
		runLedger.RecordBudgetExhausted(run.CurrentState, "tool_calls")
		return policy.ErrBudgetExceeded
	}

	// Build execution context for middleware
	execCtx := &middleware.ExecutionContext{
		RunID:        run.ID,
		CurrentState: run.CurrentState,
		Tool:         t,
		Input:        decision.Input,
		Reason:       decision.Reason,
		Budget:       budget,
		Vars:         run.Vars,
	}

	// Record tool call in ledger
	runLedger.RecordToolCall(run.CurrentState, decision.ToolName, decision.Input)

	// Core handler wraps the resilient executor
	coreHandler := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
		return e.executor.Execute(ctx, ec.Tool, ec.Input)
	}

	// Execute through middleware chain
	handler := e.middleware.Chain()(coreHandler)
	result, err := handler(ctx, execCtx)

	// Handle errors
	if err != nil {
		runLedger.RecordToolError(run.CurrentState, decision.ToolName, err)
		return fmt.Errorf("tool execution failed: %w", err)
	}

	// Consume budget on success
	_ = budget.Consume("tool_calls", 1) // validated by CanConsume check above
	runLedger.RecordBudgetConsumed(run.CurrentState, "tool_calls", 1, budget.Remaining("tool_calls"))
	runLedger.RecordToolResult(run.CurrentState, decision.ToolName, result.Output, result.Duration, result.Cached)

	// Add evidence
	run.AddEvidence(agent.NewToolEvidence(decision.ToolName, result.Output))

	return nil
}

// executeTransition executes a state transition decision.
func (e *Engine) executeTransition(_ context.Context, interp *statemachine.Interpreter, decision *agent.TransitionDecision) error {
	return interp.Transition(decision.ToState, decision.Reason)
}

// executeAskHuman handles human input requests by pausing the run.
func (e *Engine) executeAskHuman(_ context.Context, _ *statemachine.Interpreter, machineCtx *statemachine.Context, decision *agent.AskHumanDecision) error {
	run := machineCtx.Run
	runLedger := machineCtx.Ledger

	// Set the pending question on the run
	run.AskHuman(decision.Question, decision.Options)

	// Record in ledger
	runLedger.RecordHumanInputRequest(run.CurrentState, decision.Question, decision.Options)

	logging.Info().
		Add(logging.RunID(run.ID)).
		Add(logging.State(run.CurrentState)).
		Add(logging.Str("question", decision.Question)).
		Msg("awaiting human input")

	// Return special error to signal the run should pause
	return agent.ErrAwaitingHumanInput
}

// executeFinish completes the run successfully.
func (e *Engine) executeFinish(_ context.Context, interp *statemachine.Interpreter, machineCtx *statemachine.Context, decision *agent.FinishDecision) error {
	run := machineCtx.Run
	// Transition first, then mark complete (order matters - transition checks current state)
	if err := interp.Transition(agent.StateDone, decision.Summary); err != nil {
		return err
	}
	run.Result = decision.Result
	run.Status = agent.RunStatusCompleted
	return nil
}

// executeFail terminates the run with failure.
func (e *Engine) executeFail(_ context.Context, interp *statemachine.Interpreter, machineCtx *statemachine.Context, decision *agent.FailDecision) error {
	run := machineCtx.Run
	// Transition first, then mark failed (order matters - transition checks current state)
	if err := interp.Transition(agent.StateFailed, decision.Reason); err != nil {
		return err
	}
	run.Error = decision.Reason
	run.Status = agent.RunStatusFailed
	return nil
}

// generateRunID creates a unique run ID using timestamp and random bytes.
func generateRunID() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("run-%d-%s", time.Now().UnixNano(), hex.EncodeToString(b))
}

// buildToolDefs converts allowed tool names into ToolDef structs for the planner.
func (e *Engine) buildToolDefs(allowedTools []string) []planner.ToolDef {
	defs := make([]planner.ToolDef, 0, len(allowedTools))
	for _, name := range allowedTools {
		if t, ok := e.registry.Get(name); ok {
			defs = append(defs, planner.ToolDef{
				Name:        t.Name(),
				Description: t.Description(),
				InputSchema: t.InputSchema().Raw(),
			})
		}
	}
	return defs
}

// Knowledge returns the knowledge store, if configured.
func (e *Engine) Knowledge() knowledge.Store {
	return e.knowledge
}
