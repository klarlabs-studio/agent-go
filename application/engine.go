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

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/artifact"
	"go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/domain/knowledge"
	"go.klarlabs.de/agent/domain/ledger"
	"go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/domain/run"
	"go.klarlabs.de/agent/domain/task"
	"go.klarlabs.de/agent/domain/telemetry"
	"go.klarlabs.de/agent/domain/tool"
	"go.klarlabs.de/agent/infrastructure/governance"
	"go.klarlabs.de/agent/infrastructure/logging"
	inframw "go.klarlabs.de/agent/infrastructure/middleware"
	"go.klarlabs.de/agent/infrastructure/planner"
	"go.klarlabs.de/agent/infrastructure/resilience"
	"go.klarlabs.de/agent/infrastructure/statemachine"
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
	tracer       telemetry.Tracer
	meter        telemetry.Meter
	runStore     run.Store
	eventStore   event.Store
	taskCtx      *task.Context
	govFactory   governance.Factory
	logger       *logging.Logger
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
	Tracer       telemetry.Tracer
	Meter        telemetry.Meter
	RunStore     run.Store
	EventStore   event.Store
	TaskContext  *task.Context
	// Governance selects the governance backend (budget + approval +
	// evidence). When nil, the full-delegation KernelFactory is used: each run
	// is ONE axi session, so budget, the destructive-tool approval gate, and
	// the evidence chain are all axi-native.
	Governance governance.Factory
	// Logger is the injected structured logger. When nil, a no-op logger is
	// used and the engine emits no logs — the execution path never falls back
	// to the package-level logging singleton.
	Logger *logging.Logger
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
		tracer:       config.Tracer,
		meter:        config.Meter,
		runStore:     config.RunStore,
		eventStore:   config.EventStore,
		taskCtx:      config.TaskContext,
		govFactory:   config.Governance,
		logger:       config.Logger,
	}

	// Set defaults
	if e.logger == nil {
		// No-op by default: the execution path never depends on the
		// package-level logging singleton. Callers opt in via api.WithLogger.
		e.logger = logging.NewNopLogger()
	}
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
	if e.govFactory == nil {
		// Default governance: FULL axi delegation (spec § Changes Required #1,
		// Track F). Each run executes as ONE axi session, so budget, the
		// destructive-tool approval gate, AND the tamper-evident evidence chain
		// are all axi-native — the only tier that satisfies the spec
		// non-negotiable "budget AND approval always through axi". AxiFactory
		// (approval-only) and PassthroughFactory remain selectable via
		// api.WithGovernance. See infrastructure/governance.
		f, err := governance.NewKernelFactory(e.approver)
		if err != nil {
			return nil, fmt.Errorf("init governance: %w", err)
		}
		e.govFactory = f
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

	// Approval check (for destructive/high-risk tools). Skipped when the
	// governance factory owns approval (axi): the kernel enforces the gate
	// during Authorize, so a middleware gate would double-prompt.
	if !e.govFactory.OwnsApproval() {
		registry.Use(inframw.Approval(inframw.ApprovalConfig{
			Approver: e.approver,
		}))
	}

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
	return e.executeRun(ctx, generateRunID(), goal, vars)
}

// executeRun is the internal run method that accepts a pre-generated run ID.
func (e *Engine) executeRun(ctx context.Context, runID, goal string, vars map[string]any) (*agent.Run, error) {

	// Start trace span for the entire run
	if e.tracer != nil {
		var span telemetry.Span
		ctx, span = e.tracer.StartSpan(ctx, "agent.run",
			telemetry.WithAttributes(
				telemetry.String("agent.run_id", runID),
				telemetry.String("agent.goal", goal),
			),
		)
		defer span.End()
	}

	// Create run
	r := agent.NewRun(runID, goal)

	// Wire task context for multi-agent coordination
	if e.taskCtx != nil {
		r.TaskID = e.taskCtx.ID
		// Merge shared vars as defaults (run vars override)
		for k, v := range e.taskCtx.Vars() {
			if _, exists := vars[k]; !exists {
				r.SetVar(k, v)
			}
		}
	}

	// Set parent run ID from Go context (set by parent engine)
	if parentID := task.RunIDFromContext(ctx); parentID != "" {
		r.ParentRunID = parentID
	}

	// Propagate run ID into context for child agents
	ctx = task.WithRunID(ctx, runID)

	for k, v := range vars {
		r.SetVar(k, v)
		e.publishEvent(ctx, runID, event.TypeVariableSet, event.VariableSetPayload{Key: k, Value: v})
	}

	// Persist run creation
	e.saveRun(ctx, r)

	// Create supporting components
	budget := policy.NewBudget(e.budgetLimits)
	runLedger := ledger.New(runID)

	// Create state machine context
	machineCtx := statemachine.NewContext(r, budget, runLedger)
	machineCtx.Eligibility = e.eligibility
	machineCtx.Transitions = e.transitions
	machineCtx.Governor = e.govFactory.Governor(ctx, budget)
	// Some Governors (full axi delegation) hold a per-run axi session that
	// must be released when the run ends. Close it on every exit path.
	defer e.closeGovernor(machineCtx.Governor)

	// Create state machine
	machine, err := statemachine.NewAgentMachine()
	if err != nil {
		return nil, fmt.Errorf("failed to create state machine: %w", err)
	}

	// Create interpreter
	interp := statemachine.NewInterpreter(machine, machineCtx)

	// Log run start
	e.logger.Info().
		Add(logging.RunID(runID)).
		Add(logging.Goal(goal)).
		Msg("run started")

	// Start state machine
	interp.Start()
	runLedger.RecordRunStarted(goal)

	// Publish run.started event
	e.publishEvent(ctx, runID, event.TypeRunStarted, event.RunStartedPayload{
		Goal: goal,
		Vars: vars,
	})

	// Execute until terminal state or max steps
	steps := 0
	for !interp.IsTerminal() && steps < e.maxSteps {
		select {
		case <-ctx.Done():
			r.Fail("context cancelled")
			runLedger.RecordRunFailed(r.CurrentState, "context cancelled")
			e.publishEvent(ctx, runID, event.TypeRunFailed, event.RunFailedPayload{
				Error: "context cancelled", State: r.CurrentState, Duration: r.Duration(),
			})
			e.updateRun(ctx, r)
			return r, ctx.Err()
		default:
		}

		if err := e.step(ctx, interp, machineCtx); err != nil {
			// Handle human input request specially - not a failure
			if errors.Is(err, agent.ErrAwaitingHumanInput) {
				e.logger.Info().
					Add(logging.RunID(runID)).
					Add(logging.State(r.CurrentState)).
					Msg("run paused for human input")
				// Capture the governor's evidence-chain snapshot onto the run
				// so the resumed segment continues ONE physical chain across
				// the pause (the deferred closeGovernor tears the session down).
				e.captureGovernorEvidence(r, machineCtx.Governor)
				e.publishEvent(ctx, runID, event.TypeRunPaused, nil)
				e.updateRun(ctx, r)
				return r, err
			}

			r.Fail(err.Error())
			runLedger.RecordRunFailed(r.CurrentState, err.Error())

			e.logger.Error().
				Add(logging.RunID(runID)).
				Add(logging.State(r.CurrentState)).
				Add(logging.ErrorField(err)).
				Msg("run failed")

			e.publishEvent(ctx, runID, event.TypeRunFailed, event.RunFailedPayload{
				Error: err.Error(), State: r.CurrentState, Duration: r.Duration(),
			})
			e.updateRun(ctx, r)

			// Record error on trace span
			if e.tracer != nil {
				if _, span := e.tracer.StartSpan(ctx, ""); span != nil {
					span.RecordError(err)
					span.End()
				}
			}

			return r, err
		}
		steps++

		// Persist run state after each step
		e.updateRun(ctx, r)
	}

	if steps >= e.maxSteps && !interp.IsTerminal() {
		r.Fail("max steps exceeded")
		runLedger.RecordRunFailed(r.CurrentState, "max steps exceeded")
		e.publishEvent(ctx, runID, event.TypeRunFailed, event.RunFailedPayload{
			Error: "max steps exceeded", State: r.CurrentState, Duration: r.Duration(),
		})
		e.updateRun(ctx, r)
		return r, errors.New("max steps exceeded")
	}

	// Verify the run's evidence chain before declaring success. Under full
	// axi delegation the Governor accumulates one tamper-evident chain per
	// run; a broken chain means the audit trail was tampered with, which is
	// a run failure, not a silent success.
	if r.Status == agent.RunStatusCompleted {
		if err := verifyRunEvidence(machineCtx.Governor); err != nil {
			r.Fail(err.Error())
			runLedger.RecordRunFailed(r.CurrentState, err.Error())
			e.publishEvent(ctx, runID, event.TypeRunFailed, event.RunFailedPayload{
				Error: err.Error(), State: r.CurrentState, Duration: r.Duration(),
			})
			e.updateRun(ctx, r)
			return r, err
		}
	}

	// Log completion
	e.logger.Info().
		Add(logging.RunID(runID)).
		Add(logging.State(r.CurrentState)).
		Add(logging.Duration(r.Duration())).
		Msg("run completed")

	if r.Status == agent.RunStatusCompleted {
		runLedger.RecordRunCompleted(r.Result)
		e.publishEvent(ctx, runID, event.TypeRunCompleted, event.RunCompletedPayload{
			Result: r.Result, Duration: r.Duration(),
		})
	}

	e.updateRun(ctx, r)
	return r, nil
}

// Stream executes the agent in the background and returns a channel of events.
// The returned channel receives events as the agent executes. It is closed when
// the run completes, fails, or the context is cancelled.
// Requires an EventStore to be configured via WithEventStore.
func (e *Engine) Stream(ctx context.Context, goal string) (string, <-chan event.Event, error) {
	if e.eventStore == nil {
		return "", nil, errors.New("streaming requires an event store (use WithEventStore)")
	}

	runID := generateRunID()

	// Subscribe before starting the run to avoid missing early events
	ch, err := e.eventStore.Subscribe(ctx, runID)
	if err != nil {
		return "", nil, fmt.Errorf("failed to subscribe to events: %w", err)
	}

	// Run in background — use the pre-generated runID.
	// Errors are captured via run.failed events in the event stream.
	go func() {
		if _, err := e.runWithID(ctx, runID, goal, nil); err != nil {
			e.logger.Error().Add(logging.RunID(runID)).Add(logging.ErrorField(err)).Msg("streamed run failed")
		}
	}()

	return runID, ch, nil
}

// runWithID is an internal method that executes with a specified run ID.
func (e *Engine) runWithID(ctx context.Context, runID, goal string, vars map[string]any) (*agent.Run, error) {
	return e.executeRun(ctx, runID, goal, vars)
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

	// Create supporting components for this segment. The budget is SEEDED with
	// the tool calls already consumed before the pause (the persisted count of
	// tool-result evidence), so the run-spanning tool_calls limit survives the
	// pause instead of silently resetting to full.
	budget := policy.NewBudget(e.budgetLimits)
	if consumed := run.ConsumedToolCalls(); consumed > 0 {
		_ = budget.Consume("tool_calls", consumed)
	}
	runLedger := ledger.New(run.ID)

	// Record human input response in ledger
	runLedger.RecordHumanInputResponse(run.CurrentState, question, input)

	// Create state machine context
	machineCtx := statemachine.NewContext(run, budget, runLedger)
	machineCtx.Eligibility = e.eligibility
	machineCtx.Transitions = e.transitions
	machineCtx.Governor = e.govFactory.Governor(ctx, budget)
	defer e.closeGovernor(machineCtx.Governor)

	// Rehydrate the evidence chain from the snapshot captured at pause time so
	// the resumed run continues ONE continuous, tamper-evident chain.
	if err := rehydrateGovernorEvidence(machineCtx.Governor, run.GovernanceEvidence); err != nil {
		return nil, fmt.Errorf("failed to rehydrate evidence chain: %w", err)
	}

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
	e.logger.Info().
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
				e.logger.Info().
					Add(logging.RunID(run.ID)).
					Add(logging.State(run.CurrentState)).
					Msg("run paused for human input")
				// Re-capture the (now-extended) evidence chain so a further
				// resume continues the same continuous chain.
				e.captureGovernorEvidence(run, machineCtx.Governor)
				e.updateRun(ctx, run)
				return run, err
			}

			run.Fail(err.Error())
			runLedger.RecordRunFailed(run.CurrentState, err.Error())

			e.logger.Error().
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

	// Verify the run's evidence chain before declaring success — the resumed
	// run carries ONE continuous chain across the pause, so a broken chain is
	// a run failure, exactly as on the initial path (parity with executeRun).
	if run.Status == agent.RunStatusCompleted {
		if err := verifyRunEvidence(machineCtx.Governor); err != nil {
			run.Fail(err.Error())
			runLedger.RecordRunFailed(run.CurrentState, err.Error())
			return run, err
		}
	}

	// Log completion
	e.logger.Info().
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

	// Trace planner decision
	var planSpan telemetry.Span
	if e.tracer != nil {
		ctx, planSpan = e.tracer.StartSpan(ctx, "agent.planner.decide",
			telemetry.WithAttributes(telemetry.String("agent.state", string(run.CurrentState))),
		)
	}

	decision, err := e.planner.Plan(ctx, req)

	if planSpan != nil {
		if err != nil {
			planSpan.RecordError(err)
		} else {
			planSpan.SetAttributes(telemetry.String("agent.decision", string(decision.Type)))
		}
		planSpan.End()
	}

	if err != nil {
		return fmt.Errorf("planner error: %w", err)
	}

	// Publish planner.proposed (what the planner intended, before execution)
	pp := event.PlannerProposedPayload{DecisionType: string(decision.Type)}
	if decision.CallTool != nil {
		pp.ToolName = decision.CallTool.ToolName
		pp.Reason = decision.CallTool.Reason
		pp.Input = decision.CallTool.Input
	} else if decision.Transition != nil {
		pp.ToState = decision.Transition.ToState
		pp.Reason = decision.Transition.Reason
	}
	e.publishEvent(ctx, run.ID, event.TypePlannerProposed, pp)

	// Record decision
	runLedger.RecordDecision(run.CurrentState, decision)

	// Publish decision.made (after recording, confirms execution intent)
	dp := event.DecisionMadePayload{DecisionType: string(decision.Type)}
	if decision.CallTool != nil {
		dp.ToolName = decision.CallTool.ToolName
		dp.Reason = decision.CallTool.Reason
		dp.Input = decision.CallTool.Input
	} else if decision.Transition != nil {
		dp.ToState = decision.Transition.ToState
		dp.Reason = decision.Transition.Reason
	}
	e.publishEvent(ctx, run.ID, event.TypeDecisionMade, dp)

	e.logger.Debug().
		Add(logging.RunID(run.ID)).
		Add(logging.State(run.CurrentState)).
		Add(logging.Decision(decision.Type)).
		Msg("planner decision")

	// Execute decision
	switch decision.Type {
	case agent.DecisionCallTool:
		return e.executeToolDecision(ctx, interp, machineCtx, decision.CallTool)
	case agent.DecisionTransition:
		return e.executeTransition(ctx, interp, machineCtx, decision.Transition)
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
	gov := machineCtx.Governor

	// Get tool from registry
	t, ok := e.registry.Get(decision.ToolName)
	if !ok {
		return fmt.Errorf("%w: %s", tool.ErrToolNotFound, decision.ToolName)
	}

	// STRUCTURAL ACT-GATE (non-negotiable: "side effects ONLY in act").
	//
	// This gate is enforced here, in the execution path, independent of and
	// BEFORE tool-eligibility, governance, and approval. A side-effecting tool
	// is rejected with a hard error in any state that does not permit side
	// effects. It is driven purely by the tool's annotations and the state's
	// side-effect semantics — there is no configuration that can relax or
	// bypass it. Eligibility name maps (including wildcards) cannot widen it.
	if t.Annotations().HasSideEffects() && !e.stateAllowsSideEffects(machineCtx, run.CurrentState) {
		return fmt.Errorf("%w: %s in state %s",
			tool.ErrSideEffectInNonActState, decision.ToolName, run.CurrentState)
	}

	// Authorize the tool call through the governance seam: budget always,
	// and the approval gate when the Governor owns approval (axi). When it
	// does not (passthrough), approval stays with the middleware below.
	govReq := governance.ToolRequest{
		RunID:           run.ID,
		State:           string(run.CurrentState),
		ToolName:        decision.ToolName,
		Input:           decision.Input,
		Reason:          decision.Reason,
		RiskLevel:       t.Annotations().RiskLevel.String(),
		RequireApproval: gov.OwnsApproval() && t.Annotations().ShouldRequireApproval(),
	}
	auth, authErr := gov.Authorize(ctx, govReq)
	if authErr != nil {
		// A tool that needs approval with no approver configured is the
		// engine-level "approval required" invariant.
		if errors.Is(authErr, governance.ErrNoApprover) {
			return fmt.Errorf("%w: %s", tool.ErrApprovalRequired, decision.ToolName)
		}
		return fmt.Errorf("governance authorization failed: %w", authErr)
	}
	switch auth.Decision {
	case governance.DecisionBudgetExhausted:
		runLedger.RecordBudgetExhausted(run.CurrentState, "tool_calls")
		e.publishEvent(ctx, run.ID, event.TypeBudgetExhausted, event.BudgetExhaustedPayload{
			BudgetName: "tool_calls",
		})
		return policy.ErrBudgetExceeded
	case governance.DecisionDenied:
		e.publishEvent(ctx, run.ID, event.TypeApprovalDenied, event.ApprovalResultPayload{
			ToolName: decision.ToolName, Approver: auth.Approver, Reason: auth.Reason,
		})
		return fmt.Errorf("%w: %s", tool.ErrApprovalDenied, auth.Reason)
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
		EventPublisher: func(eventType string, payload any) {
			e.publishEvent(ctx, run.ID, event.Type(eventType), payload)
		},
	}

	// Record tool call in ledger
	runLedger.RecordToolCall(run.CurrentState, decision.ToolName, decision.Input)

	// Publish tool.called event
	e.publishEvent(ctx, run.ID, event.TypeToolCalled, event.ToolCalledPayload{
		ToolName: decision.ToolName, Input: decision.Input,
		State: run.CurrentState, Reason: decision.Reason,
	})

	// Trace tool execution
	var toolSpan telemetry.Span
	if e.tracer != nil {
		ctx, toolSpan = e.tracer.StartSpan(ctx, "agent.tool.execute",
			telemetry.WithAttributes(
				telemetry.String("agent.tool", decision.ToolName),
				telemetry.String("agent.state", string(run.CurrentState)),
			),
		)
	}

	// Core handler wraps the resilient executor
	coreHandler := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
		return e.executor.Execute(ctx, ec.Tool, ec.Input)
	}

	// Execute through middleware chain
	handler := e.middleware.Chain()(coreHandler)
	result, err := handler(ctx, execCtx)

	// End tool span
	if toolSpan != nil {
		if err != nil {
			toolSpan.RecordError(err)
		}
		toolSpan.End()
	}

	// Handle errors
	if err != nil {
		runLedger.RecordToolError(run.CurrentState, decision.ToolName, err)
		e.publishEvent(ctx, run.ID, event.TypeToolFailed, event.ToolFailedPayload{
			ToolName: decision.ToolName, Error: err.Error(), Duration: result.Duration,
		})
		return fmt.Errorf("tool execution failed: %w", err)
	}

	// Account the successful tool call against the budget via the Governor.
	commit, commitErr := gov.Commit(ctx, govReq, governance.Outcome{
		Success:  true,
		Output:   result.Output,
		Duration: result.Duration,
	})
	if commitErr != nil {
		return fmt.Errorf("governance commit failed: %w", commitErr)
	}
	runLedger.RecordBudgetConsumed(run.CurrentState, "tool_calls", 1, commit.Remaining)
	runLedger.RecordToolResult(run.CurrentState, decision.ToolName, result.Output, result.Duration, result.Cached)

	// Publish budget consumed event
	e.publishEvent(ctx, run.ID, event.TypeBudgetConsumed, event.BudgetConsumedPayload{
		BudgetName: "tool_calls", Amount: 1, Remaining: commit.Remaining,
	})

	// Publish tool.succeeded event
	e.publishEvent(ctx, run.ID, event.TypeToolSucceeded, event.ToolSucceededPayload{
		ToolName: decision.ToolName, Output: result.Output,
		Duration: result.Duration, Cached: result.Cached,
	})

	// Add evidence to run, task context, and event stream
	evidence := agent.NewToolEvidence(decision.ToolName, result.Output)
	run.AddEvidence(evidence)
	if e.taskCtx != nil {
		e.taskCtx.AddEvidence(evidence)
	}
	e.publishEvent(ctx, run.ID, event.TypeEvidenceAdded, event.EvidenceAddedPayload{
		Type: string(agent.EvidenceToolResult), Source: decision.ToolName, Content: result.Output,
	})

	return nil
}

// stateAllowsSideEffects reports whether the given state permits side-effecting
// tools. It consults the run's StateRegistry so custom states that declare
// AllowsSideEffects are honored, and otherwise falls back to the canonical
// rule (only the act state permits side effects). This backs the structural
// act-gate and is never configurable away.
func (e *Engine) stateAllowsSideEffects(machineCtx *statemachine.Context, state agent.State) bool {
	if machineCtx != nil && machineCtx.StateRegistry != nil {
		return machineCtx.StateRegistry.AllowsSideEffects(state)
	}
	return state.AllowsSideEffects()
}

// executeTransition executes a state transition decision.
func (e *Engine) executeTransition(ctx context.Context, interp *statemachine.Interpreter, machineCtx *statemachine.Context, decision *agent.TransitionDecision) error {
	fromState := machineCtx.Run.CurrentState
	if err := interp.Transition(decision.ToState, decision.Reason); err != nil {
		return err
	}
	e.publishEvent(ctx, machineCtx.Run.ID, event.TypeStateTransitioned, event.StateTransitionedPayload{
		FromState: fromState, ToState: decision.ToState, Reason: decision.Reason,
	})
	return nil
}

// executeAskHuman handles human input requests by pausing the run.
func (e *Engine) executeAskHuman(_ context.Context, _ *statemachine.Interpreter, machineCtx *statemachine.Context, decision *agent.AskHumanDecision) error {
	run := machineCtx.Run
	runLedger := machineCtx.Ledger

	// Set the pending question on the run
	run.AskHuman(decision.Question, decision.Options)

	// Record in ledger
	runLedger.RecordHumanInputRequest(run.CurrentState, decision.Question, decision.Options)

	e.logger.Info().
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

// saveRun persists a new run. Best-effort — logs errors but doesn't fail the run.
func (e *Engine) saveRun(ctx context.Context, r *agent.Run) {
	if e.runStore == nil {
		return
	}
	if err := e.runStore.Save(ctx, r); err != nil {
		e.logger.Error().Add(logging.RunID(r.ID)).Add(logging.ErrorField(err)).Msg("failed to save run")
	}
}

// updateRun persists run state changes. Best-effort — logs errors but doesn't fail the run.
func (e *Engine) updateRun(ctx context.Context, r *agent.Run) {
	if e.runStore == nil {
		return
	}
	if err := e.runStore.Update(ctx, r); err != nil {
		e.logger.Error().Add(logging.RunID(r.ID)).Add(logging.ErrorField(err)).Msg("failed to update run")
	}
}

// publishEvent publishes a domain event to the event store. Best-effort.
func (e *Engine) publishEvent(ctx context.Context, runID string, eventType event.Type, payload any) {
	if e.eventStore == nil {
		return
	}
	evt, err := event.NewEvent(runID, eventType, payload)
	if err != nil {
		e.logger.Error().Add(logging.RunID(runID)).Add(logging.ErrorField(err)).Msg("failed to create event")
		return
	}
	if err := e.eventStore.Append(ctx, evt); err != nil {
		e.logger.Error().Add(logging.RunID(runID)).Add(logging.ErrorField(err)).Msg("failed to publish event")
	}
}

// verifyRunEvidence verifies the run's axi-native evidence chain when the
// Governor exposes one (full axi delegation). Governors without an evidence
// chain (passthrough, approval-only axi) verify trivially. A broken chain is
// returned as an error so the engine fails the run.
func verifyRunEvidence(gov governance.Governor) error {
	verifier, ok := gov.(governance.EvidenceVerifier)
	if !ok {
		return nil
	}
	if err := verifier.VerifyEvidenceChain(); err != nil {
		return fmt.Errorf("run evidence chain verification failed: %w", err)
	}
	return nil
}

// captureGovernorEvidence persists the Governor's evidence-chain snapshot onto
// the run so a human-input pause does not discard it: the resumed segment
// rehydrates from it to continue ONE continuous chain. Governors without an
// evidence chain (passthrough, approval-only axi) leave the run untouched.
// Best-effort — a snapshot error is logged, not fatal to the pause.
func (e *Engine) captureGovernorEvidence(r *agent.Run, gov governance.Governor) {
	snapshotter, ok := gov.(governance.EvidenceSnapshotter)
	if !ok {
		return
	}
	data, err := snapshotter.EvidenceSnapshot()
	if err != nil {
		e.logger.Error().Add(logging.RunID(r.ID)).Add(logging.ErrorField(err)).
			Msg("failed to snapshot governance evidence at pause")
		return
	}
	r.GovernanceEvidence = data
}

// rehydrateGovernorEvidence restores a previously captured evidence-chain
// snapshot into the resumed run's Governor so the chain is continuous across
// the pause. Governors without rehydration support (or an empty snapshot) are
// a no-op.
func rehydrateGovernorEvidence(gov governance.Governor, data json.RawMessage) error {
	if len(data) == 0 {
		return nil
	}
	rehydrator, ok := gov.(governance.EvidenceRehydrator)
	if !ok {
		return nil
	}
	return rehydrator.RehydrateEvidence(data)
}

// closeGovernor releases a per-run Governor that holds resources (e.g. the
// full-delegation kernel governor's in-flight axi session). Governors without
// a Close method are left untouched. Errors are logged, not propagated — a
// run's success does not hinge on session teardown.
func (e *Engine) closeGovernor(gov governance.Governor) {
	closer, ok := gov.(interface{ Close() error })
	if !ok {
		return
	}
	if err := closer.Close(); err != nil {
		e.logger.Error().Add(logging.ErrorField(err)).Msg("failed to close run governor")
	}
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

// RunInTask executes the agent within a shared task context.
// The task context enables sharing variables, evidence, and artifact references
// between parent and child agents in a delegation hierarchy.
func (e *Engine) RunInTask(ctx context.Context, goal string, tc *task.Context) (*agent.Run, error) {
	origTC := e.taskCtx
	e.taskCtx = tc
	defer func() { e.taskCtx = origTC }()
	return e.RunWithVars(ctx, goal, nil)
}

// Knowledge returns the knowledge store, if configured.
func (e *Engine) Knowledge() knowledge.Store {
	return e.knowledge
}
