# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

This is a **state-driven agent runtime** for Go that enables developers to build trustworthy, adaptable AI-powered systems by designing the structure and constraints of agent behavior rather than scripting intelligence with prompts.

Key principle: **Trust is the product.** Intelligence is constrained by design, not hope.

### External Libraries

- **[statekit](https://go.klarlabs.de/statekit)** - Statechart execution engine with hierarchical states, guards, actions, and XState JSON export
- **[fortify](https://go.klarlabs.de/fortify)** - Resilience patterns (circuit breaker, retry, rate limiter, bulkhead, timeout)
- **[bolt](https://go.klarlabs.de/bolt)** - High-performance zero-allocation structured logging with OpenTelemetry support

## Development Commands

```bash
# Build
make build                    # Build all packages
go build ./...

# Test
make test                     # Run tests with race detection
make test-coverage            # Run tests with coverage profile
go test -race -v ./...

# Coverage (coverctl)
make coverage-check           # Check coverage threshold (80%)
make coverage-report          # Generate detailed report
make coverage-debt            # Show coverage debt by domain
coverctl check --fail-under=80

# Security (nox)
make security                 # Run nox security scan (high+ severity)
make security-secrets         # Scan git history for leaked secrets
make security-diff            # Show new findings vs main branch
nox scan . --severity-threshold=high

# Release (relicta)
make release-plan             # Analyze commits, suggest version
make release-bump             # Calculate and set version
make release-notes            # Generate release notes
make release-publish          # Execute release
relicta plan --analyze

# Lint
make lint                     # Run golangci-lint
golangci-lint run ./...

# Run example
make example                  # Run fileops example
go run ./example/fileops

# All CI checks
make check                    # lint + test + coverage + security
```

## Architecture

### Domain-Driven Design Structure

```
agent-go/
├── domain/                         # Core Domain Layer (no external deps)
│   ├── agent/                      # Agent Aggregate Root
│   │   ├── state.go                # Canonical states with semantics
│   │   ├── decision.go             # Planner decision value objects
│   │   ├── run.go                  # Run aggregate root
│   │   ├── evidence.go             # Evidence value object
│   │   └── errors.go               # Domain errors
│   │
│   ├── tool/                       # Tool Aggregate
│   │   ├── tool.go                 # Tool interface and builder
│   │   ├── annotation.go           # Tool annotations (ReadOnly, Destructive)
│   │   ├── schema.go               # Input/Output schema value object
│   │   ├── result.go               # ToolResult value object
│   │   ├── registry.go             # Tool repository interface
│   │   └── errors.go               # Tool domain errors
│   │
│   ├── policy/                     # Policy Subdomain
│   │   ├── budget.go               # Budget tracking with thread safety
│   │   ├── approval.go             # Approval interfaces and implementations
│   │   ├── constraint.go           # Tool eligibility, state transitions
│   │   └── errors.go               # Policy errors
│   │
│   ├── ledger/                     # Audit Subdomain
│   │   ├── ledger.go               # Ledger aggregate
│   │   ├── entry.go                # LedgerEntry value object
│   │   └── events.go               # Domain events
│   │
│   ├── artifact/                   # Artifact Subdomain
│   │   ├── artifact.go             # ArtifactRef value object
│   │   └── store.go                # ArtifactStore repository interface
│   │
│   ├── protocol/                   # Agent Protocol
│   │   ├── message.go              # Message envelope (request/reply/notify/broadcast)
│   │   ├── capability.go           # Agent capability discovery
│   │   ├── trust.go                # Trust boundaries and permissions
│   │   └── router.go               # Router interface
│   │
│   └── task/                       # Multi-Agent Task Context
│       └── context.go              # Shared state across agent hierarchy
│
├── application/                    # Application Layer (orchestration)
│   ├── engine.go                   # Main orchestration service (Run, Stream, RunInTask)
│   ├── replay.go                   # Replay/Fork engine (Replay, Timeline, EventIterator)
│   └── options.go                  # Functional options
│
├── infrastructure/                 # Infrastructure Layer
│   ├── statemachine/               # Statekit integration
│   │   ├── machine.go              # Agent state machine definition
│   │   ├── guards.go               # Transition guards
│   │   ├── actions.go              # State entry/exit actions
│   │   └── interpreter.go          # State machine interpreter
│   │
│   ├── resilience/                 # Fortify integration
│   │   ├── executor.go             # Resilient tool executor
│   │   └── options.go              # Executor configuration
│   │
│   ├── logging/                    # Bolt integration
│   │   ├── logger.go               # Logger factory
│   │   └── fields.go               # Structured field helpers
│   │
│   ├── storage/                    # Storage implementations
│   │   ├── memory/                 # In-memory stores
│   │   │   └── tool_registry.go
│   │   └── filesystem/             # Filesystem stores
│   │       └── artifact_store.go
│   │
│   ├── planner/                    # Planner implementations
│   │   ├── mock.go                 # MockPlanner for testing
│   │   ├── scripted.go             # ScriptedPlanner for deterministic tests
│   │   ├── rules.go                # RuleBasedPlanner (priority-ordered rules)
│   │   └── hybrid.go               # HybridPlanner (rules + fallback)
│   │
│   ├── agent/                      # Agent composition
│   │   └── delegate.go             # DelegateTool (agent-as-tool)
│   │
│   └── protocol/                   # Protocol implementations
│       └── memory_router.go        # In-process message routing
│
├── interfaces/                     # Interface Adapters
│   └── api/                        # Public API
│       ├── agent.go                # Engine constructor, options, re-exports
│       └── builders.go             # Helper constructors
│
├── cmd/
│   └── agentctl/                   # CLI tool (run, validate, visualize, repl)
│
├── test/                           # Test suites
│   ├── invariant_test.go           # 8 design invariant tests
│   └── integration/                # End-to-end integration tests
│
└── example/
    ├── 01-minimal/                 # Minimum working agent
    ├── 02-tools/                   # Custom tool creation
    ├── 03-policies/                # Budgets and approvals
    ├── 04-llm-planner/             # Real LLM integration
    ├── 06-distributed/             # Multi-worker setup
    ├── flagship/                   # Full platform demo (3 agents, streaming, persistence)
    ├── fileops/                    # File operation tools
    ├── webscraper/                 # Web scraping agent
    ├── customer-support/           # Customer support agent
    ├── devops-monitor/             # DevOps monitoring
    └── governed_adaptivity/        # Governed adaptive behavior
```

### State Machine

The agent operates within a canonical state graph:

| State    | Purpose                | Side Effects | Terminal |
|----------|------------------------|--------------|----------|
| intake   | Normalize goal         | No           | No       |
| explore  | Gather evidence        | No           | No       |
| decide   | Choose next step       | No           | No       |
| act      | Perform side-effects   | **Yes**      | No       |
| validate | Confirm outcome        | No           | No       |
| done     | Terminal success       | No           | Yes      |
| failed   | Terminal failure       | No           | Yes      |

States are **structural constraints**, not behavioral definitions. Tools are explicitly allowed or denied per state.

### Planner Contract

**Input**: State, Evidence, Allowed tools, Budgets, Variables

**Output** (one of):
- `CallTool` - Execute a tool with JSON input
- `Transition` - Move to another state with reason
- `AskHuman` - Request human input (resume with `engine.ResumeWithInput()`)
- `Finish` - Complete successfully with result
- `Fail` - Terminate with failure

**Guarantees**: Bounded outputs, no side effects, conservative bias, deterministic mode available.

### Tool System

Tools have:
- Stable string identifier
- Input/Output JSON schemas
- Annotations: `ReadOnly`, `Destructive`, `Idempotent`, `Cacheable`, risk level
- Handler function with context
- Optional artifact emission

Annotations influence:
- Planner scoring
- Policy enforcement (approval requirements)
- Resilience behavior (retry for idempotent)
- Caching eligibility

### Resilient Execution

Tool execution uses a composition of resilience patterns via Fortify:

```
Bulkhead → Timeout → Circuit Breaker → Retry (if idempotent)
```

Configured via `resilience.Executor`:
- Bulkhead limits concurrent tool executions
- Timeout prevents long-running tools
- Circuit breaker prevents cascading failures
- Retry with exponential backoff for idempotent tools

### Storage Capabilities (All Optional)

- **ToolRegistry**: In-memory tool registration
- **ArtifactStore**: Large outputs with stable references
- **Ledger**: Append-only audit log for all operations
- **RunStore**: Persistent run state (memory, PostgreSQL, SQLite, DynamoDB)
- **EventStore**: Event sourcing with Subscribe() channels (memory, PostgreSQL, SQLite, Badger, MongoDB, NATS)
- **KnowledgeStore**: Vector storage with cosine similarity (SQLite, PostgreSQL)
- **Cache**: TTL-based caching (Redis, Badger, etcd, DynamoDB, NATS, SQLite)

### Event Streaming

The engine publishes 16 event types to an optional EventStore:

- **Run lifecycle**: `run.started`, `run.completed`, `run.failed`, `run.paused`, `run.resumed`
- **State machine**: `state.transitioned`
- **Tool execution**: `tool.called`, `tool.succeeded`, `tool.failed`
- **Decisions**: `planner.proposed`, `decision.made`
- **Policy**: `approval.requested`, `approval.granted`, `approval.denied`
- **Budget**: `budget.consumed`, `budget.exhausted`
- **Data**: `evidence.added`, `variable.set`
- **Agent protocol**: `agent.message.sent`, `agent.message.received`, `agent.delegated`

Use `engine.Stream(ctx, goal)` to get a real-time `<-chan event.Event`.

### Multi-Agent Coordination

- **DelegateTool**: Wraps a child engine as a tool for agent composition
- **TaskContext**: Thread-safe shared state (variables, evidence, artifacts) across agent hierarchy
- **ParentRunID**: Run hierarchy tracking for delegation chains
- **Agent Protocol**: Message envelope with correlation IDs, capability discovery, trust boundaries
- **MemoryRouter**: In-process message routing with trust policy enforcement

### Replay and Fork

- **Replay**: Reconstruct historical runs from events, step through with Timeline/EventIterator
- **Fork**: Branch a run at any step with a different planner for simulation/testing
- **ReplayPlanner**: Deterministic planner that replays recorded decisions

### MCP Integration

- **MCP Server**: Full Model Context Protocol support (stdio + HTTP, JSON-RPC 2.0)
- **Policy-aware**: MCP tool calls route through middleware chain (eligibility, approval, budget, audit)
- **Event auditing**: MCP tool calls publish events to EventStore

### CLI (agentctl)

- `agentctl run --config agent.yaml --goal "..."` — Execute agents from YAML config
- `agentctl validate agent.yaml` — Schema validation
- `agentctl visualize [--format dot|mermaid]` — Export state machine diagrams
- `agentctl repl` — Interactive step-through mode

### Dashboard

- Web UI with SSE real-time event stream
- Run list with status filtering
- Event timeline, evidence viewer, variable inspector
- Embedded via `//go:embed static/*`

### WASM Sandbox

- `contrib/sandbox-wasm`: wazero-based tool isolation
- Memory limits, time limits, filesystem restrictions
- Tools implementing `WASMExecutor` run in WASM; others fall back to direct execution

## Design Invariants

These must hold in all code paths and tests:

1. **Tool eligibility** - Tools only run in explicitly allowed states (wildcard `"*"` counts as explicit)
2. **Transition validity** - State changes follow the defined graph
3. **Approval enforcement** - Destructive actions require approval
4. **Budget enforcement** - Limits are never exceeded
5. **State semantics** - Only Act state allows side effects
6. **Tool registration uniqueness** - Tool names are unique per registry
7. **Run lifecycle** - Runs progress through states to terminal state
8. **Evidence accumulation** - Evidence is append-only, order preserved

## Testing Philosophy

- **Deterministic execution mode** required for all core logic
- **Testable without LLMs** - use `MockPlanner` or `ScriptedPlanner`
- **No required external services** - storage is always optional
- **Explicit failure modes** - no silent recovery
- **Invariant-driven test suite** - verify the 8 invariants above

```go
// Example: ScriptedPlanner for deterministic testing
planner := api.NewScriptedPlanner(
    api.ScriptStep{
        ExpectState: agent.StateIntake,
        Decision:    api.NewTransitionDecision(agent.StateExplore, "begin"),
    },
    api.ScriptStep{
        ExpectState: agent.StateExplore,
        Decision:    api.NewCallToolDecision("read_file", input, "gather info"),
    },
    api.ScriptStep{
        ExpectState: agent.StateExplore,
        Decision:    api.NewFinishDecision("done", result),
    },
)
```

## Public API Usage

```go
import api "go.klarlabs.de/agent/interfaces/api"

// Create tool
tool := api.NewToolBuilder("read_file").
    WithDescription("Reads a file").
    WithAnnotations(api.Annotations{ReadOnly: true}).
    WithExecutor(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
        // Implementation
    }).
    Build()

// Create registry
registry := api.NewToolRegistry()
registry.Register(tool)

// Configure eligibility — three patterns available:

// Pattern 1: Default (wildcard) — all tools in explore, decide, act, validate
eligibility := api.NewDefaultToolEligibility()

// Pattern 2: Explicit per-state — most restrictive, production recommended
eligibility = api.NewToolEligibilityWith(api.EligibilityRules{
    api.StateExplore:  {"read_file", "list_dir"},
    api.StateAct:      {"write_file", "delete_file"},
    api.StateValidate: {"read_file"},
})

// Pattern 3: Hybrid — wildcard for some states, explicit for others
eligibility = api.NewToolEligibilityWith(api.EligibilityRules{
    api.StateExplore:  {"*"},           // all tools for exploration
    api.StateDecide:   {"*"},           // all tools for decision making
    api.StateAct:      {"write_file"},  // only specific tools for side effects
    api.StateValidate: {"read_file"},   // only read tools for validation
})

// Create engine
engine, err := api.New(
    api.WithRegistry(registry),
    api.WithPlanner(myPlanner),
    api.WithToolEligibility(eligibility),
    api.WithTransitions(api.DefaultTransitions()),
    api.WithBudgets(map[string]int{"tool_calls": 100}),
    api.WithMaxSteps(50),
)

// Run agent
run, err := engine.Run(ctx, "Process the data files")

// Stream events in real-time
runID, events, err := engine.Stream(ctx, "Process files")
for evt := range events {
    fmt.Printf("[%s] %s\n", evt.Type, evt.Payload)
}

// Multi-agent delegation
childEngine, _ := api.New(api.WithPlanner(childPlanner), api.WithTool(searchTool))
delegate := infraagent.NewDelegateTool("researcher", "Research agent", childEngine,
    infraagent.WithDelegateTaskContext(taskCtx),
)

// Shared task context
tc := api.NewTaskContext("task-1", "root-run")
tc.SetVar("api_key", os.Getenv("API_KEY"))
engine, _ := api.New(api.WithTaskContext(tc), ...)

// Replay historical runs
replay := api.NewReplay(eventStore)
timeline, _ := replay.NewTimeline(ctx, "run-123")
fmt.Println("Duration:", timeline.Duration())
```

## Explicit Non-Goals

- Dynamic state creation by LLMs (states are structural, defined at build time)
- Model-specific abstractions (planner interface is model-agnostic)
- Prompt-only experimentation (behavior is defined by structure, not prompts)
