package application

import (
	"go.klarlabs.de/agent/domain/artifact"
	"go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/domain/run"
	"go.klarlabs.de/agent/domain/task"
	"go.klarlabs.de/agent/domain/telemetry"
	"go.klarlabs.de/agent/domain/tool"
	"go.klarlabs.de/agent/infrastructure/logging"
	"go.klarlabs.de/agent/infrastructure/planner"
	"go.klarlabs.de/agent/infrastructure/resilience"
)

// Option configures the engine.
type Option func(*EngineConfig)

// WithRegistry sets the tool registry.
func WithRegistry(r tool.Registry) Option {
	return func(c *EngineConfig) {
		c.Registry = r
	}
}

// WithPlanner sets the planner.
func WithPlanner(p planner.Planner) Option {
	return func(c *EngineConfig) {
		c.Planner = p
	}
}

// WithExecutor sets the resilient executor.
func WithExecutor(e *resilience.Executor) Option {
	return func(c *EngineConfig) {
		c.Executor = e
	}
}

// WithArtifactStore sets the artifact store.
func WithArtifactStore(s artifact.Store) Option {
	return func(c *EngineConfig) {
		c.Artifacts = s
	}
}

// WithEligibility sets the tool eligibility configuration.
func WithEligibility(e *policy.ToolEligibility) Option {
	return func(c *EngineConfig) {
		c.Eligibility = e
	}
}

// WithTransitions sets the state transitions configuration.
func WithTransitions(t *policy.StateTransitions) Option {
	return func(c *EngineConfig) {
		c.Transitions = t
	}
}

// WithApprover sets the approval handler.
func WithApprover(a policy.Approver) Option {
	return func(c *EngineConfig) {
		c.Approver = a
	}
}

// WithBudgets sets budget limits.
func WithBudgets(limits map[string]int) Option {
	return func(c *EngineConfig) {
		c.BudgetLimits = limits
	}
}

// WithMaxSteps sets the maximum number of steps.
func WithMaxSteps(n int) Option {
	return func(c *EngineConfig) {
		c.MaxSteps = n
	}
}

// WithMiddleware sets a custom middleware registry.
// If not set, the engine uses a default middleware chain with:
// - Eligibility middleware (tool access control per state)
// - Approval middleware (human approval for destructive tools)
// - Logging middleware (execution timing and results)
func WithMiddleware(m *middleware.Registry) Option {
	return func(c *EngineConfig) {
		c.Middleware = m
	}
}

// WithTracer sets the OpenTelemetry tracer for distributed tracing.
// When configured, the engine creates spans for runs, steps, planner decisions,
// and tool executions. Pass nil to disable tracing.
func WithTracer(t telemetry.Tracer) Option {
	return func(c *EngineConfig) {
		c.Tracer = t
	}
}

// WithMeter sets the OpenTelemetry meter for metrics collection.
// When configured, the engine records run duration, step counts, and tool latency.
func WithMeter(m telemetry.Meter) Option {
	return func(c *EngineConfig) {
		c.Meter = m
	}
}

// WithRunStore sets the run store for persistent run state.
// When configured, runs are automatically saved on creation and updated on
// each step and terminal state. Persistence is best-effort — failures are
// logged but do not abort the run.
func WithRunStore(s run.Store) Option {
	return func(c *EngineConfig) {
		c.RunStore = s
	}
}

// WithEventStore sets the event store for event sourcing and streaming.
// When configured, domain events (run.started, tool.called, state.transitioned, etc.)
// are published throughout the run. Required for Stream() to work.
func WithEventStore(s event.Store) Option {
	return func(c *EngineConfig) {
		c.EventStore = s
	}
}

// WithTaskContext sets a shared task context for multi-agent coordination.
// When configured, the engine merges shared variables, propagates evidence
// to the task context, and sets ParentRunID/TaskID on runs.
func WithTaskContext(tc *task.Context) Option {
	return func(c *EngineConfig) {
		c.TaskContext = tc
	}
}

// WithLogger sets the injected structured logger for the engine.
// When unset, the engine uses a no-op logger and emits nothing — the
// execution path never depends on the package-level logging singleton.
func WithLogger(l *logging.Logger) Option {
	return func(c *EngineConfig) {
		c.Logger = l
	}
}

// NewEngineWithOptions creates an engine with functional options.
func NewEngineWithOptions(opts ...Option) (*Engine, error) {
	config := EngineConfig{}
	for _, opt := range opts {
		opt(&config)
	}
	return NewEngine(config)
}
