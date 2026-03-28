// Package api provides the public API for the agent-go runtime.
//
// agent-go is a state-driven agent framework for Go that enables developers to build
// trustworthy, adaptable AI-powered systems by designing the structure and constraints
// of agent behavior rather than scripting intelligence with prompts.
//
// # Quick Start
//
// Create a minimal agent with a tool and scripted planner:
//
//	// 1. Create a tool
//	echoTool := api.NewToolBuilder("echo").
//	    WithDescription("Echoes input").
//	    WithAnnotations(api.Annotations{ReadOnly: true}).
//	    WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
//	        return tool.Result{Output: input}, nil
//	    }).
//	    MustBuild()
//
//	// 2. Create a planner
//	planner := api.NewScriptedPlanner(
//	    api.ScriptStep{ExpectState: api.StateIntake, Decision: api.NewTransitionDecision(api.StateExplore, "start")},
//	    api.ScriptStep{ExpectState: api.StateExplore, Decision: api.NewCallToolDecision("echo", input, "echo")},
//	    api.ScriptStep{ExpectState: api.StateExplore, Decision: api.NewTransitionDecision(api.StateDecide, "done")},
//	    api.ScriptStep{ExpectState: api.StateDecide, Decision: api.NewFinishDecision("completed", result)},
//	)
//
//	// 3. Configure tool eligibility
//	// Option A: Declarative style (recommended for static configuration)
//	eligibility := api.NewToolEligibilityWith(api.EligibilityRules{
//	    api.StateExplore: {"echo", "read_file"},
//	    api.StateAct:     {"write_file"},
//	})
//
//	// Option B: Imperative style (useful for dynamic configuration)
//	eligibility := api.NewToolEligibility()
//	eligibility.Allow(api.StateExplore, "echo")
//	eligibility.Allow(api.StateExplore, "read_file")
//	eligibility.Allow(api.StateAct, "write_file")
//
//	// 4. Create and run the engine
//	engine, _ := api.New(
//	    api.WithTool(echoTool),
//	    api.WithPlanner(planner),
//	    api.WithToolEligibility(eligibility),
//	)
//	run, _ := engine.Run(ctx, "Echo a message")
//
// # States
//
// The agent operates within a canonical state graph:
//
//   - StateIntake: Normalize and understand the goal
//   - StateExplore: Gather information (read-only tools only)
//   - StateDecide: Choose next action
//   - StateAct: Execute side-effects (destructive tools allowed)
//   - StateValidate: Verify outcomes
//   - StateDone: Terminal success state
//   - StateFailed: Terminal failure state
//
// # Tools
//
// Tools are capabilities the agent can invoke. Each tool has annotations that
// describe its behavior:
//
//   - ReadOnly: Tool does not modify external state
//   - Destructive: Tool performs irreversible operations
//   - Idempotent: Repeated calls produce same result
//   - Cacheable: Results can be cached
//   - RiskLevel: Potential impact (None, Low, Medium, High, Critical)
//
// # Planners
//
// Planners make decisions about what the agent should do next:
//
//   - ScriptedPlanner: Predefined sequence for testing (supports looping, conditional steps, error injection)
//   - MockPlanner: Returns specific decisions for testing
//   - RuleBasedPlanner: Evaluates Go-native rules in priority order, first match wins
//   - HybridPlanner: Combines rule-based with any fallback planner
//   - LLMPlanner: Uses an LLM provider for intelligent planning
//
// # Policies
//
// Policies enforce constraints on agent behavior:
//
//   - ToolEligibility: Which tools can run in which states
//   - StateTransitions: Which state transitions are allowed
//   - Approvers: Human approval for destructive operations
//   - Budgets: Limits on tool calls, tokens, etc.
package api

import (
	"context"

	"github.com/felixgeelhaar/agent-go/application"
	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/artifact"
	"github.com/felixgeelhaar/agent-go/domain/event"
	"github.com/felixgeelhaar/agent-go/domain/knowledge"
	"github.com/felixgeelhaar/agent-go/domain/middleware"
	"github.com/felixgeelhaar/agent-go/domain/policy"
	"github.com/felixgeelhaar/agent-go/domain/run"
	"github.com/felixgeelhaar/agent-go/domain/telemetry"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	inframw "github.com/felixgeelhaar/agent-go/infrastructure/middleware"
	"github.com/felixgeelhaar/agent-go/infrastructure/planner"
	"github.com/felixgeelhaar/agent-go/infrastructure/resilience"
	"github.com/felixgeelhaar/agent-go/infrastructure/storage/memory"
)

// Re-export core types for convenience.
type (
	// Run represents a single execution of the agent.
	Run = agent.Run

	// State represents a structural constraint in the agent's execution.
	State = agent.State

	// Decision represents the planner's output.
	Decision = agent.Decision

	// Evidence represents an observation during a run.
	Evidence = agent.Evidence

	// Tool represents a registered capability the agent can invoke.
	Tool = tool.Tool

	// Annotations describe tool behavior.
	Annotations = tool.Annotations

	// RiskLevel indicates the potential impact of a tool execution.
	RiskLevel = tool.RiskLevel
)

// Re-export state constants.
const (
	StateIntake   = agent.StateIntake
	StateExplore  = agent.StateExplore
	StateDecide   = agent.StateDecide
	StateAct      = agent.StateAct
	StateValidate = agent.StateValidate
	StateDone     = agent.StateDone
	StateFailed   = agent.StateFailed
)

// Re-export risk levels.
const (
	RiskNone     = tool.RiskNone
	RiskLow      = tool.RiskLow
	RiskMedium   = tool.RiskMedium
	RiskHigh     = tool.RiskHigh
	RiskCritical = tool.RiskCritical
)

// Re-export run status.
type RunStatus = agent.RunStatus

const (
	StatusPending   = agent.RunStatusPending
	StatusRunning   = agent.RunStatusRunning
	StatusPaused    = agent.RunStatusPaused
	StatusCompleted = agent.RunStatusCompleted
	StatusFailed    = agent.RunStatusFailed
)

// Re-export human input related errors.
var (
	// ErrAwaitingHumanInput is returned when the agent pauses for human input.
	// Check run.PendingQuestion for the question and options.
	ErrAwaitingHumanInput = agent.ErrAwaitingHumanInput

	// ErrNoPendingQuestion is returned when attempting to resume a run
	// that has no pending question.
	ErrNoPendingQuestion = agent.ErrNoPendingQuestion

	// ErrInvalidHumanInput is returned when the provided input doesn't
	// match the allowed options for the pending question.
	ErrInvalidHumanInput = agent.ErrInvalidHumanInput
)

// Re-export knowledge types for RAG capabilities.
type (
	// Vector represents an embedding with associated text and metadata.
	Vector = knowledge.Vector

	// SearchResult represents a similarity search result.
	SearchResult = knowledge.SearchResult

	// ListFilter provides filtering options for knowledge list operations.
	ListFilter = knowledge.ListFilter

	// KnowledgeStore is the interface for vector knowledge storage.
	KnowledgeStore = knowledge.Store

	// KnowledgeStats provides statistics about the knowledge store.
	KnowledgeStats = knowledge.Stats
)

// Re-export knowledge errors.
var (
	// ErrKnowledgeNotFound indicates the requested vector was not found.
	ErrKnowledgeNotFound = knowledge.ErrNotFound

	// ErrKnowledgeInvalidID indicates the vector ID is empty or invalid.
	ErrKnowledgeInvalidID = knowledge.ErrInvalidID

	// ErrKnowledgeInvalidEmbedding indicates the embedding is empty or invalid.
	ErrKnowledgeInvalidEmbedding = knowledge.ErrInvalidEmbedding

	// ErrKnowledgeDimensionMismatch indicates the embedding dimension doesn't match.
	ErrKnowledgeDimensionMismatch = knowledge.ErrDimensionMismatch
)

// Engine is the main runtime for agent execution.
type Engine struct {
	engine *application.Engine
}

// New creates a new Engine with the provided options.
func New(opts ...Option) (*Engine, error) {
	config := &engineConfig{
		registry: memory.NewToolRegistry(),
	}

	for _, opt := range opts {
		opt(config)
	}

	appConfig := application.EngineConfig{
		Registry:     config.registry,
		Planner:      config.planner,
		Executor:     config.executor,
		Artifacts:    config.artifacts,
		Knowledge:    config.knowledge,
		Eligibility:  config.eligibility,
		Transitions:  config.transitions,
		Approver:     config.approver,
		BudgetLimits: config.budgets,
		MaxSteps:     config.maxSteps,
		Middleware:   config.middleware,
		Tracer:       config.tracer,
		Meter:        config.meter,
		RunStore:     config.runStore,
		EventStore:   config.eventStore,
	}

	engine, err := application.NewEngine(appConfig)
	if err != nil {
		return nil, err
	}

	return &Engine{engine: engine}, nil
}

// Run executes the agent with the given goal.
func (e *Engine) Run(ctx context.Context, goal string) (*Run, error) {
	return e.engine.Run(ctx, goal)
}

// RunWithVars executes the agent with the given goal and initial variables.
func (e *Engine) RunWithVars(ctx context.Context, goal string, vars map[string]any) (*Run, error) {
	return e.engine.RunWithVars(ctx, goal, vars)
}

// ResumeWithInput continues a paused run with human-provided input.
// This is used after a run returns ErrAwaitingHumanInput to provide the
// response to the pending question.
//
// Example:
//
//	run, err := engine.Run(ctx, "Process data and ask for confirmation")
//	if errors.Is(err, api.ErrAwaitingHumanInput) {
//	    fmt.Printf("Agent asks: %s\n", run.PendingQuestion.Question)
//	    input := getUserInput()
//	    run, err = engine.ResumeWithInput(ctx, run, input)
//	}
func (e *Engine) ResumeWithInput(ctx context.Context, run *Run, input string) (*Run, error) {
	return e.engine.ResumeWithInput(ctx, run, input)
}

// Stream executes the agent in the background and returns a channel of events.
// Requires an EventStore to be configured via WithEventStore.
//
// Example:
//
//	runID, events, _ := engine.Stream(ctx, "Process files")
//	for evt := range events {
//	    fmt.Printf("[%s] %s\n", evt.Type, evt.Payload)
//	}
func (e *Engine) Stream(ctx context.Context, goal string) (string, <-chan event.Event, error) {
	return e.engine.Stream(ctx, goal)
}

// Knowledge returns the knowledge store, if configured.
// Returns nil if no knowledge store was provided via WithKnowledgeStore.
func (e *Engine) Knowledge() knowledge.Store {
	return e.engine.Knowledge()
}

// engineConfig holds configuration for engine creation.
type engineConfig struct {
	registry    tool.Registry
	planner     planner.Planner
	executor    *resilience.Executor
	artifacts   artifact.Store
	knowledge   knowledge.Store
	eligibility *policy.ToolEligibility
	transitions *policy.StateTransitions
	approver    policy.Approver
	budgets     map[string]int
	maxSteps    int
	middleware  *middleware.Registry
	tracer      telemetry.Tracer
	meter       telemetry.Meter
	runStore    run.Store
	eventStore  event.Store
}

// Option configures the Engine.
type Option func(*engineConfig)

// WithRegistry sets the tool registry.
func WithRegistry(r tool.Registry) Option {
	return func(c *engineConfig) {
		c.registry = r
	}
}

// WithTool registers a tool with the engine's registry.
// Can be called multiple times to register multiple tools.
// If a tool with the same name already exists, it will be silently ignored.
// Use WithRegistry to get full control over tool registration errors.
func WithTool(t tool.Tool) Option {
	return func(c *engineConfig) {
		_ = c.registry.Register(t) // Ignore duplicate registration errors
	}
}

// WithPlanner sets the planner.
func WithPlanner(p planner.Planner) Option {
	return func(c *engineConfig) {
		c.planner = p
	}
}

// WithExecutor sets the resilient executor.
func WithExecutor(e *resilience.Executor) Option {
	return func(c *engineConfig) {
		c.executor = e
	}
}

// WithArtifactStore sets the artifact store.
func WithArtifactStore(s artifact.Store) Option {
	return func(c *engineConfig) {
		c.artifacts = s
	}
}

// WithKnowledgeStore sets the knowledge store for RAG (Retrieval-Augmented Generation).
// The knowledge store enables agents to store and retrieve knowledge based on semantic
// similarity using vector embeddings.
func WithKnowledgeStore(s knowledge.Store) Option {
	return func(c *engineConfig) {
		c.knowledge = s
	}
}

// WithToolEligibility sets tool eligibility per state.
func WithToolEligibility(e *policy.ToolEligibility) Option {
	return func(c *engineConfig) {
		c.eligibility = e
	}
}

// WithTransitions sets allowed state transitions.
func WithTransitions(t *policy.StateTransitions) Option {
	return func(c *engineConfig) {
		c.transitions = t
	}
}

// WithApprover sets the approval handler.
func WithApprover(a policy.Approver) Option {
	return func(c *engineConfig) {
		c.approver = a
	}
}

// WithBudgets sets budget limits.
func WithBudgets(budgets map[string]int) Option {
	return func(c *engineConfig) {
		c.budgets = budgets
	}
}

// WithBudget sets a single budget limit.
// This is a convenience function that can be called multiple times.
func WithBudget(name string, limit int) Option {
	return func(c *engineConfig) {
		if c.budgets == nil {
			c.budgets = make(map[string]int)
		}
		c.budgets[name] = limit
	}
}

// WithMaxSteps sets the maximum number of execution steps.
func WithMaxSteps(n int) Option {
	return func(c *engineConfig) {
		c.maxSteps = n
	}
}

// WithMiddleware sets a custom middleware registry for tool execution.
// If not set, the engine uses a default middleware chain with:
// - Eligibility middleware (tool access control per state)
// - Approval middleware (human approval for destructive tools)
// - Logging middleware (execution timing and results)
func WithMiddleware(middlewares ...middleware.Middleware) Option {
	return func(c *engineConfig) {
		if c.middleware == nil {
			c.middleware = middleware.NewRegistry()
		}
		c.middleware.UseMany(middlewares...)
	}
}

// WithRateLimit enables rate limiting for tool executions.
// This uses fortify's token bucket rate limiter to control request rates.
//
// Parameters:
//   - rate: Number of tokens added per second
//   - burst: Maximum tokens (bucket capacity) for handling bursts
//
// Example:
//
//	engine, _ := api.New(
//	    api.WithPlanner(planner),
//	    api.WithRateLimit(100, 100), // 100 requests/sec, burst of 100
//	)
func WithRateLimit(rate, burst int) Option {
	return func(c *engineConfig) {
		if c.middleware == nil {
			c.middleware = middleware.NewRegistry()
		}
		c.middleware.Use(inframw.RateLimit(inframw.RateLimitConfig{
			Rate:  rate,
			Burst: burst,
		}))
	}
}

// WithPerToolRateLimit enables per-tool rate limiting.
// Each tool can have its own rate limit, falling back to defaults.
//
// Example:
//
//	engine, _ := api.New(
//	    api.WithPlanner(planner),
//	    api.WithPerToolRateLimit(10, 10, map[string]api.ToolRateConfig{
//	        "fast_tool": {Rate: 100, Burst: 100},
//	        "slow_tool": {Rate: 5, Burst: 5},
//	    }),
//	)
func WithPerToolRateLimit(defaultRate, defaultBurst int, toolRates map[string]ToolRateConfig) Option {
	return func(c *engineConfig) {
		if c.middleware == nil {
			c.middleware = middleware.NewRegistry()
		}
		rates := make(map[string]inframw.RateLimitConfig)
		for name, cfg := range toolRates {
			rates[name] = inframw.RateLimitConfig{
				Rate:  cfg.Rate,
				Burst: cfg.Burst,
			}
		}
		c.middleware.Use(inframw.PerToolRateLimit(inframw.PerToolRateLimitConfig{
			DefaultRate:  defaultRate,
			DefaultBurst: defaultBurst,
			ToolRates:    rates,
		}))
	}
}

// ToolRateConfig configures rate limits for a specific tool.
type ToolRateConfig struct {
	// Rate is the number of tokens added per second.
	Rate int
	// Burst is the maximum tokens (bucket capacity).
	Burst int
}

// WithTracer sets the OpenTelemetry tracer for distributed tracing.
// When configured, the engine creates spans for runs, steps, planner decisions,
// and tool executions.
func WithTracer(t telemetry.Tracer) Option {
	return func(c *engineConfig) {
		c.tracer = t
	}
}

// WithMeter sets the OpenTelemetry meter for metrics collection.
func WithMeter(m telemetry.Meter) Option {
	return func(c *engineConfig) {
		c.meter = m
	}
}

// WithRunStore sets the run store for persistent run state.
// Runs are automatically saved on creation and updated on each step.
func WithRunStore(s run.Store) Option {
	return func(c *engineConfig) {
		c.runStore = s
	}
}

// WithEventStore sets the event store for event sourcing and streaming.
// Required for the Stream() method to work.
func WithEventStore(s event.Store) Option {
	return func(c *engineConfig) {
		c.eventStore = s
	}
}
