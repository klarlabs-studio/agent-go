# Architecture Guide

This document describes the architecture of agent-go, a state-driven agent runtime built with Domain-Driven Design (DDD) principles.

## Design Philosophy

### Core Principles

1. **Explicit State**: Agent behavior is governed by a well-defined state machine, making the system predictable and debuggable.

2. **Domain-Driven Design**: The codebase is organized around business domains, not technical concerns. The domain layer has zero external dependencies.

3. **Policy as First-Class Citizen**: Budgets, approvals, and tool eligibility are enforced at the runtime level, not left to individual tools.

4. **Resilience by Default**: All tool executions are wrapped with circuit breakers, retries, and timeouts.

5. **Complete Auditability**: Every decision, tool call, and state transition is recorded in an append-only ledger.

## Layer Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      interfaces/api                         │
│              (Public API, Builders, Options)                │
├─────────────────────────────────────────────────────────────┤
│                      application/                           │
│                (Engine, Orchestration)                      │
├─────────────────────────────────────────────────────────────┤
│                     infrastructure/                         │
│    (Statemachine, Resilience, Logging, Storage, Planner)   │
├─────────────────────────────────────────────────────────────┤
│                        domain/                              │
│         (Agent, Tool, Policy, Ledger, Artifact)            │
└─────────────────────────────────────────────────────────────┘
```

### Domain Layer (`domain/`)

The heart of the system. Contains all business logic with **zero external dependencies**.

#### Agent Aggregate (`domain/agent/`)

The central aggregate representing an agent run.

```go
// Run is the aggregate root
type Run struct {
    ID           string
    Goal         string
    CurrentState State
    Vars         map[string]any      // Runtime variables
    Evidence     []Evidence          // Accumulated observations
    Status       RunStatus
    Result       any                 // Final result on success
    Error        string              // Error message on failure
}

// State represents the agent's current phase
type State string // intake, explore, decide, act, validate, done, failed

// Decision represents what the planner wants to do
type Decision struct {
    Type       DecisionType
    CallTool   *CallToolDecision
    Transition *TransitionDecision
    AskHuman   *AskHumanDecision
    Finish     *FinishDecision
    Fail       *FailDecision
}

// Evidence captures observations from tool executions
type Evidence struct {
    Type      EvidenceType
    Source    string          // Tool name or source
    Content   json.RawMessage // The actual data
    Timestamp time.Time
}
```

#### Tool Aggregate (`domain/tool/`)

Represents agent capabilities.

```go
// Tool is the interface all tools must implement
type Tool interface {
    Name() string
    Description() string
    InputSchema() Schema
    OutputSchema() Schema
    Annotations() Annotations
    Execute(ctx context.Context, input json.RawMessage) (Result, error)
}

// Annotations describe tool behavior
type Annotations struct {
    ReadOnly    bool      // Doesn't modify external state
    Destructive bool      // May cause irreversible changes
    Idempotent  bool      // Safe to retry
    Cacheable   bool      // Results can be cached
    RiskLevel   RiskLevel // None, Low, Medium, High, Critical
}

// Registry is the tool repository interface
type Registry interface {
    Register(tool Tool) error
    Get(name string) (Tool, bool)
    List() []Tool
    Names() []string
}
```

#### Policy Subdomain (`domain/policy/`)

Enforces constraints on agent behavior.

```go
// Budget tracks resource consumption
type Budget struct {
    limits   map[string]int
    consumed map[string]int
}

func (b *Budget) CanConsume(name string, amount int) bool
func (b *Budget) Consume(name string, amount int) error

// Approver handles approval requests for high-risk operations
type Approver interface {
    Approve(ctx context.Context, req ApprovalRequest) (ApprovalResponse, error)
}

// ToolEligibility controls which tools are available in each state
type ToolEligibility struct {
    allowed map[State][]string
}
```

#### Ledger Subdomain (`domain/ledger/`)

Provides complete audit trail.

```go
// Ledger is an append-only audit log
type Ledger struct {
    runID   string
    entries []Entry
}

// Entry types include:
// - RunStarted, RunCompleted, RunFailed
// - Transition
// - Decision
// - ToolCall, ToolResult, ToolError
// - ApprovalRequest, ApprovalResult
// - BudgetConsumed, BudgetExhausted
```

#### Artifact Subdomain (`domain/artifact/`)

Handles large binary data outside the main flow.

```go
// Ref is a stable reference to stored content
type Ref struct {
    ID          string
    Name        string
    Size        int64
    ContentType string
    Checksum    string
    Metadata    map[string]string
}

// Store is the artifact repository interface
type Store interface {
    Store(ctx context.Context, content io.Reader, opts StoreOptions) (Ref, error)
    Retrieve(ctx context.Context, ref Ref) (io.ReadCloser, error)
    Delete(ctx context.Context, ref Ref) error
    Exists(ctx context.Context, ref Ref) (bool, error)
    Metadata(ctx context.Context, ref Ref) (Ref, error)
}
```

### Infrastructure Layer (`infrastructure/`)

Implements interfaces defined in the domain layer and integrates external libraries.

#### State Machine (`infrastructure/statemachine/`)

Integrates with [statekit](https://go.klarlabs.de/statekit) for state management.

```go
// Context carries run state through the machine
type Context struct {
    Run         *agent.Run
    Budget      *policy.Budget
    Ledger      *ledger.Ledger
    Eligibility *policy.ToolEligibility
    Transitions *policy.StateTransitions
}

// Interpreter wraps statekit for agent-specific operations
type Interpreter struct {
    machine *statekit.Interpreter[*Context]
    context *Context
}

func (i *Interpreter) Transition(to agent.State, reason string) error
func (i *Interpreter) AllowedTools() []string
func (i *Interpreter) IsToolAllowed(name string) bool
func (i *Interpreter) IsTerminal() bool
```

**State Transition Rules:**

| From | To | Condition |
|------|-----|-----------|
| intake | explore | Always allowed |
| explore | decide | Always allowed |
| decide | act | Tool call decision |
| decide | done | Finish decision |
| decide | failed | Fail decision |
| act | validate | After tool execution |
| validate | explore | Need more info |
| validate | decide | Ready to decide |
| validate | done | Goal achieved |
| validate | failed | Unrecoverable error |

#### Resilience (`infrastructure/resilience/`)

Integrates with [fortify](https://go.klarlabs.de/fortify) for fault tolerance.

```go
type Executor struct {
    bulkhead bulkhead.Bulkhead[tool.Result]
    breaker  circuitbreaker.CircuitBreaker[tool.Result]
    retry    retry.Retry[tool.Result]
    timeout  time.Duration
}

// Execute applies resilience patterns in order:
// Bulkhead → Timeout → Circuit Breaker → Retry (if idempotent)
func (e *Executor) Execute(ctx context.Context, t tool.Tool, input json.RawMessage) (tool.Result, error)
```

**Resilience Patterns:**

| Pattern | Purpose | Configuration |
|---------|---------|---------------|
| Bulkhead | Limit concurrent executions | `MaxConcurrent: 10` |
| Timeout | Prevent hanging | `DefaultTimeout: 30s` |
| Circuit Breaker | Prevent cascading failures | `Threshold: 5, Timeout: 30s` |
| Retry | Handle transient failures | `MaxAttempts: 3, Backoff: exponential` |

#### Logging (`infrastructure/logging/`)

Integrates with [bolt](https://go.klarlabs.de/bolt) for structured logging.

```go
// Field helpers for consistent log structure
func RunID(id string) bolt.Field
func State(s string) bolt.Field
func ToolName(name string) bolt.Field
func Decision(d string) bolt.Field
func Duration(d time.Duration) bolt.Field
func ErrorField(err error) bolt.Field
```

#### Storage (`infrastructure/storage/`)

Implements repository interfaces.

- **memory/**: In-memory implementations for testing
  - `ToolRegistry`: Thread-safe tool storage

- **filesystem/**: Persistent implementations
  - `ArtifactStore`: File-based artifact storage with checksums

#### Planner (`infrastructure/planner/`)

Planner implementations.

- **MockPlanner**: Returns pre-configured decisions
- **ScriptedPlanner**: Follows a script of decisions
- **LLMPlanner**: (Future) LLM-based planning

### Application Layer (`application/`)

Orchestrates domain objects and infrastructure.

```go
type Engine struct {
    registry    tool.Registry
    planner     planner.Planner
    executor    *resilience.Executor
    artifacts   artifact.Store
    eligibility *policy.ToolEligibility
    transitions *policy.StateTransitions
    approver    policy.Approver
    budgetLimits map[string]int
    maxSteps    int
}

// Run executes the agent with the given goal
func (e *Engine) Run(ctx context.Context, goal string) (*agent.Run, error)

// RunWithVars executes with initial variables
func (e *Engine) RunWithVars(ctx context.Context, goal string, vars map[string]any) (*agent.Run, error)
```

**Execution Loop:**

```
1. Create Run with goal
2. Initialize Budget, Ledger, State Machine
3. Start state machine (enters "intake")
4. Loop until terminal state:
   a. Get allowed tools for current state
   b. Request decision from Planner
   c. Record decision in Ledger
   d. Execute decision:
      - CallTool: Check eligibility, budget, approval; execute with resilience
      - Transition: Validate and perform state transition
      - Finish: Transition to "done", set result
      - Fail: Transition to "failed", set error
   e. Increment step counter
5. Return Run with final status
```

### Interface Layer (`interfaces/api/`)

Clean public API hiding internal complexity.

```go
// NewEngine creates an engine with functional options
func NewEngine(opts ...Option) (*Engine, error)

// Options
func WithTool(t tool.Tool) Option
func WithPlanner(p Planner) Option
func WithBudget(name string, limit int) Option
func WithApprover(a Approver) Option
func WithMaxSteps(n int) Option

// ToolBuilder provides fluent tool construction
type ToolBuilder struct { ... }

func NewToolBuilder(name string) *ToolBuilder
func (b *ToolBuilder) WithDescription(desc string) *ToolBuilder
func (b *ToolBuilder) WithAnnotations(a Annotations) *ToolBuilder
func (b *ToolBuilder) WithInputSchema(s Schema) *ToolBuilder
func (b *ToolBuilder) WithHandler(h Handler) *ToolBuilder
func (b *ToolBuilder) MustBuild() Tool
```

## Extension Points

### Custom Planners

Implement the `Planner` interface to control agent behavior:

```go
type Planner interface {
    Plan(ctx context.Context, req PlanRequest) (Decision, error)
}
```

The planner receives:
- Current state and run ID
- All accumulated evidence
- List of allowed tools (filtered by eligibility)
- Budget snapshot
- Runtime variables

### Custom Tools

Use `ToolBuilder` or implement `Tool` interface directly:

```go
type Tool interface {
    Name() string
    Description() string
    InputSchema() Schema
    OutputSchema() Schema
    Annotations() Annotations
    Execute(ctx context.Context, input json.RawMessage) (Result, error)
}
```

### Custom Approvers

Implement approval workflow:

```go
type Approver interface {
    Approve(ctx context.Context, req ApprovalRequest) (ApprovalResponse, error)
}
```

### Custom Storage

Implement `artifact.Store` for custom artifact backends (S3, GCS, etc.).

## Data Flow

```
┌──────────┐     ┌─────────┐     ┌──────────┐
│  Planner │────►│ Engine  │────►│  Tools   │
└──────────┘     └────┬────┘     └──────────┘
                      │
         ┌────────────┼────────────┐
         │            │            │
         ▼            ▼            ▼
    ┌────────┐   ┌────────┐   ┌────────┐
    │ Budget │   │ Ledger │   │  Run   │
    └────────┘   └────────┘   └────────┘
```

1. **Planner** receives state and evidence, returns decision
2. **Engine** validates decision against policies
3. **Tool** executes (if applicable) with resilience
4. **Budget** tracks consumption
5. **Ledger** records everything
6. **Run** accumulates evidence and state

## Thread Safety

- `Run`: Not thread-safe, owned by single engine goroutine
- `Budget`: Thread-safe with mutex
- `Ledger`: Thread-safe with mutex
- `Registry`: Thread-safe with RWMutex
- `Executor`: Thread-safe, designed for concurrent use

## Error Handling

### Domain Errors

```go
// tool package
var (
    ErrToolNotFound    = errors.New("tool not found")
    ErrToolExists      = errors.New("tool already exists")
    ErrToolNotAllowed  = errors.New("tool not allowed in current state")
    ErrApprovalRequired = errors.New("approval required")
    ErrApprovalDenied  = errors.New("approval denied")
)

// policy package
var (
    ErrBudgetExceeded = errors.New("budget exceeded")
)

// artifact package
var (
    ErrArtifactNotFound = errors.New("artifact not found")
    ErrInvalidRef       = errors.New("invalid artifact reference")
)
```

### Error Propagation

1. Tool errors are wrapped and recorded in ledger
2. Policy violations prevent execution and return specific errors
3. State machine errors indicate invalid transitions
4. Context cancellation is respected throughout

## Testing Strategy

### Unit Tests

Each domain package has focused unit tests:
- `domain/agent/`: State transitions, decision validation
- `domain/tool/`: Schema validation, annotation behavior
- `domain/policy/`: Budget arithmetic, eligibility rules
- `domain/ledger/`: Entry recording, immutability

### Invariant Tests

`test/invariant_test.go` verifies system-wide invariants:
1. Tool eligibility enforcement
2. Transition validity
3. Approval enforcement
4. Budget enforcement
5. Tool registration uniqueness
6. Run lifecycle correctness
7. Evidence accumulation
8. Ledger immutability

### Integration Tests

Full engine tests with scripted planners verify end-to-end behavior.

## Performance Considerations

1. **Bulkhead**: Limits concurrent tool executions to prevent resource exhaustion
2. **Evidence Growth**: Evidence is append-only; consider pruning for long runs
3. **Ledger Size**: Ledger grows with each action; consider external storage for production
4. **Tool Caching**: Cacheable tools can have results cached (not implemented yet)

## Future Considerations

1. **Distributed Execution**: Run tools on remote workers
2. **Persistent Runs**: Resume runs after restart
3. **Streaming Evidence**: Handle large evidence incrementally
4. **Tool Versioning**: Support multiple tool versions
5. **Hierarchical Agents**: Parent-child agent relationships
