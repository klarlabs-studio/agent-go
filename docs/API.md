# API Reference

Complete API documentation for agent-go.

## Table of Contents

- [Engine](#engine)
- [Tools](#tools)
- [Planners](#planners)
- [Policies](#policies)
- [Types](#types)

---

## Engine

### Creating an Engine

```go
import agent "go.klarlabs.de/agent/interfaces/api"

engine, err := agent.NewEngine(
    agent.WithTool(readFileTool),
    agent.WithTool(writeFileTool),
    agent.WithPlanner(myPlanner),
)
```

### Engine Options

#### `WithTool(tool Tool) Option`

Registers a tool with the engine.

```go
agent.WithTool(myTool)
```

#### `WithPlanner(planner Planner) Option`

Sets the decision-making planner.

```go
agent.WithPlanner(myPlanner)
```

#### `WithBudget(name string, limit int) Option`

Sets a budget limit.

```go
agent.WithBudget("tool_calls", 100)
agent.WithBudget("tokens", 50000)
```

#### `WithApprover(approver Approver) Option`

Sets the approval handler for high-risk operations.

```go
agent.WithApprover(myApprover)
```

#### `WithMaxSteps(n int) Option`

Limits the number of execution steps (default: 100).

```go
agent.WithMaxSteps(50)
```

#### `WithToolEligibility(eligibility map[State][]string) Option`

Controls which tools are available in each state.

```go
agent.WithToolEligibility(map[agent.State][]string{
    agent.StateExplore: {"read_file", "list_dir"},
    agent.StateAct:     {"read_file", "write_file"},
})
```

#### `WithExecutorConfig(config ExecutorConfig) Option`

Configures resilience settings.

```go
agent.WithExecutorConfig(resilience.ExecutorConfig{
    MaxConcurrent:           10,
    CircuitBreakerThreshold: 5,
    CircuitBreakerTimeout:   30 * time.Second,
    RetryMaxAttempts:        3,
    RetryInitialDelay:       100 * time.Millisecond,
    RetryBackoffMultiplier:  2.0,
    DefaultTimeout:          30 * time.Second,
})
```

### Running the Engine

#### `Run(ctx context.Context, goal string) (*Run, error)`

Executes the agent with a goal.

```go
run, err := engine.Run(ctx, "Process the input files")
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Status: %s\n", run.Status)
fmt.Printf("Result: %v\n", run.Result)
```

#### `RunWithVars(ctx context.Context, goal string, vars map[string]any) (*Run, error)`

Executes with initial variables.

```go
run, err := engine.RunWithVars(ctx, "Process files in directory", map[string]any{
    "directory": "/tmp/input",
    "pattern":   "*.txt",
})
```

---

## Tools

### ToolBuilder

Fluent builder for creating tools.

```go
import (
    "go.klarlabs.de/agent/interfaces/api"
    "go.klarlabs.de/agent/domain/tool"
)

myTool := api.NewToolBuilder("my_tool").
    WithDescription("Does something useful").
    WithAnnotations(api.Annotations{
        ReadOnly:   true,
        Idempotent: true,
        RiskLevel:  api.RiskLow,
    }).
    WithInputSchema(tool.NewSchema(json.RawMessage(`{
        "type": "object",
        "properties": {
            "input": {"type": "string"}
        },
        "required": ["input"]
    }`))).
    WithOutputSchema(tool.NewSchema(json.RawMessage(`{
        "type": "object",
        "properties": {
            "output": {"type": "string"}
        }
    }`))).
    WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
        var in struct{ Input string `json:"input"` }
        if err := json.Unmarshal(input, &in); err != nil {
            return tool.Result{}, err
        }

        output, _ := json.Marshal(map[string]string{
            "output": "processed: " + in.Input,
        })
        return tool.Result{Output: output}, nil
    }).
    MustBuild()
```

### Builder Methods

#### `NewToolBuilder(name string) *ToolBuilder`

Creates a new builder with the given tool name.

#### `WithDescription(desc string) *ToolBuilder`

Sets the tool description.

#### `WithAnnotations(a Annotations) *ToolBuilder`

Sets behavioral annotations.

#### `WithInputSchema(s Schema) *ToolBuilder`

Sets the JSON Schema for input validation.

#### `WithOutputSchema(s Schema) *ToolBuilder`

Sets the JSON Schema for output.

#### `WithHandler(h Handler) *ToolBuilder`

Sets the execution handler.

```go
type Handler func(ctx context.Context, input json.RawMessage) (tool.Result, error)
```

#### `Build() (Tool, error)`

Builds the tool, returning an error if invalid.

#### `MustBuild() Tool`

Builds the tool, panicking on error.

### Annotations

```go
type Annotations struct {
    ReadOnly    bool      // Tool doesn't modify external state
    Destructive bool      // Tool may cause irreversible changes
    Idempotent  bool      // Safe to retry on failure
    Cacheable   bool      // Results can be cached
    RiskLevel   RiskLevel // Risk classification
}
```

### Risk Levels

```go
const (
    RiskNone     RiskLevel = iota // No risk
    RiskLow                       // Low risk, no approval needed
    RiskMedium                    // Medium risk, logging recommended
    RiskHigh                      // High risk, approval recommended
    RiskCritical                  // Critical risk, approval required
)
```

### Tool Result

```go
type Result struct {
    Output   json.RawMessage // Tool output data
    Duration time.Duration   // Execution time
    Cached   bool            // Whether result was cached
}
```

---

## Planners

### Planner Interface

```go
type Planner interface {
    Plan(ctx context.Context, req PlanRequest) (Decision, error)
}
```

### PlanRequest

```go
type PlanRequest struct {
    RunID        string            // Unique run identifier
    CurrentState State             // Current agent state
    Evidence     []Evidence        // All accumulated evidence
    AllowedTools []string          // Tools available in current state
    Budgets      BudgetSnapshot    // Current budget status
    Vars         map[string]any    // Runtime variables
}
```

### Decision Types

#### CallTool Decision

```go
decision := agent.Decision{
    Type: agent.DecisionCallTool,
    CallTool: &agent.CallToolDecision{
        ToolName: "read_file",
        Input:    json.RawMessage(`{"path": "/tmp/file.txt"}`),
        Reason:   "Need to read file contents",
    },
}
```

#### Transition Decision

```go
decision := agent.Decision{
    Type: agent.DecisionTransition,
    Transition: &agent.TransitionDecision{
        ToState: agent.StateExplore,
        Reason:  "Need more information",
    },
}
```

#### Finish Decision

```go
decision := agent.Decision{
    Type: agent.DecisionFinish,
    Finish: &agent.FinishDecision{
        Result:  map[string]any{"processed": 10},
        Summary: "Successfully processed 10 files",
    },
}
```

#### Fail Decision

```go
decision := agent.Decision{
    Type: agent.DecisionFail,
    Fail: &agent.FailDecision{
        Reason: "Unable to access required files",
    },
}
```

### ScriptedPlanner

For testing with predetermined decisions:

```go
import "go.klarlabs.de/agent/infrastructure/planner"

scripted := planner.NewScriptedPlanner([]agent.Decision{
    {
        Type: agent.DecisionTransition,
        Transition: &agent.TransitionDecision{
            ToState: agent.StateExplore,
            Reason:  "Start exploring",
        },
    },
    {
        Type: agent.DecisionCallTool,
        CallTool: &agent.CallToolDecision{
            ToolName: "read_file",
            Input:    json.RawMessage(`{"path": "input.txt"}`),
        },
    },
    {
        Type: agent.DecisionFinish,
        Finish: &agent.FinishDecision{
            Result: "done",
        },
    },
})
```

### MockPlanner

Returns a single decision repeatedly:

```go
mock := planner.NewMockPlanner(agent.Decision{
    Type: agent.DecisionFinish,
    Finish: &agent.FinishDecision{
        Result: "mock result",
    },
})
```

---

## Policies

### Budget

Track and limit resource consumption:

```go
import "go.klarlabs.de/agent/domain/policy"

budget := policy.NewBudget(map[string]int{
    "tool_calls": 100,
    "tokens":     50000,
})

if budget.CanConsume("tool_calls", 1) {
    budget.Consume("tool_calls", 1)
}

remaining := budget.Remaining("tool_calls")
snapshot := budget.Snapshot()
```

### Approver

Handle approval requests for high-risk operations:

```go
type Approver interface {
    Approve(ctx context.Context, req ApprovalRequest) (ApprovalResponse, error)
}

type ApprovalRequest struct {
    RunID     string
    ToolName  string
    Input     json.RawMessage
    Reason    string
    RiskLevel string
    Timestamp time.Time
}

type ApprovalResponse struct {
    Approved  bool
    Approver  string
    Reason    string
    Timestamp time.Time
}
```

Example implementation:

```go
type ConsoleApprover struct{}

func (a *ConsoleApprover) Approve(ctx context.Context, req ApprovalRequest) (ApprovalResponse, error) {
    fmt.Printf("Approval requested for %s: %s\n", req.ToolName, req.Reason)
    fmt.Print("Approve? (y/n): ")

    var response string
    fmt.Scanln(&response)

    return ApprovalResponse{
        Approved:  response == "y",
        Approver:  "console",
        Timestamp: time.Now(),
    }, nil
}
```

### Tool Eligibility

Control which tools are available in each state:

```go
eligibility := policy.NewToolEligibility()
eligibility.Allow(agent.StateExplore, "read_file", "list_dir")
eligibility.Allow(agent.StateAct, "read_file", "write_file", "delete_file")

// Check eligibility
tools := eligibility.AllowedTools(agent.StateExplore) // ["read_file", "list_dir"]
allowed := eligibility.IsAllowed(agent.StateAct, "write_file") // true
```

---

## Types

### State

```go
type State string

const (
    StateIntake   State = "intake"   // Initial state
    StateExplore  State = "explore"  // Information gathering
    StateDecide   State = "decide"   // Decision making
    StateAct      State = "act"      // Tool execution
    StateValidate State = "validate" // Result verification
    StateDone     State = "done"     // Successful completion
    StateFailed   State = "failed"   // Terminal failure
)
```

### Run

```go
type Run struct {
    ID           string
    Goal         string
    CurrentState State
    Vars         map[string]any
    Evidence     []Evidence
    Status       RunStatus
    Result       any
    Error        string
    StartTime    time.Time
    EndTime      time.Time
}

func (r *Run) Duration() time.Duration
func (r *Run) SetVar(key string, value any)
func (r *Run) GetVar(key string) (any, bool)
```

### RunStatus

```go
type RunStatus string

const (
    RunStatusPending   RunStatus = "pending"
    RunStatusRunning   RunStatus = "running"
    RunStatusCompleted RunStatus = "completed"
    RunStatusFailed    RunStatus = "failed"
)
```

### Evidence

```go
type Evidence struct {
    Type      EvidenceType
    Source    string          // Tool name or source
    Content   json.RawMessage // The actual data
    Timestamp time.Time
}

type EvidenceType string

const (
    EvidenceTypeTool   EvidenceType = "tool"
    EvidenceTypeHuman  EvidenceType = "human"
    EvidenceTypeSystem EvidenceType = "system"
)
```

### Schema

```go
// Create from JSON Schema
schema := tool.NewSchema(json.RawMessage(`{
    "type": "object",
    "properties": {
        "name": {"type": "string"}
    },
    "required": ["name"]
}`))

// Get raw schema
raw := schema.Raw()
```

---

## Errors

### Tool Errors

```go
import "go.klarlabs.de/agent/domain/tool"

tool.ErrToolNotFound    // Tool not in registry
tool.ErrToolExists      // Tool already registered
tool.ErrToolNotAllowed  // Tool not allowed in current state
tool.ErrApprovalRequired // High-risk tool needs approval
tool.ErrApprovalDenied  // Approval was denied
```

### Policy Errors

```go
import "go.klarlabs.de/agent/domain/policy"

policy.ErrBudgetExceeded // Budget limit reached
```

### Artifact Errors

```go
import "go.klarlabs.de/agent/domain/artifact"

artifact.ErrArtifactNotFound // Artifact doesn't exist
artifact.ErrInvalidRef       // Invalid artifact reference
```

---

## Complete Example

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"

    agent "go.klarlabs.de/agent/interfaces/api"
    "go.klarlabs.de/agent/domain/tool"
    "go.klarlabs.de/agent/infrastructure/planner"
)

func main() {
    // Create tools
    greetTool := api.NewToolBuilder("greet").
        WithDescription("Generates a greeting").
        WithAnnotations(api.Annotations{
            ReadOnly:   true,
            Idempotent: true,
            RiskLevel:  api.RiskNone,
        }).
        WithInputSchema(tool.NewSchema(json.RawMessage(`{
            "type": "object",
            "properties": {
                "name": {"type": "string"}
            },
            "required": ["name"]
        }`))).
        WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
            var in struct{ Name string `json:"name"` }
            json.Unmarshal(input, &in)

            output, _ := json.Marshal(map[string]string{
                "greeting": fmt.Sprintf("Hello, %s!", in.Name),
            })
            return tool.Result{Output: output}, nil
        }).
        MustBuild()

    // Create planner with script
    p := planner.NewScriptedPlanner([]agent.Decision{
        {
            Type: agent.DecisionTransition,
            Transition: &agent.TransitionDecision{
                ToState: agent.StateExplore,
            },
        },
        {
            Type: agent.DecisionTransition,
            Transition: &agent.TransitionDecision{
                ToState: agent.StateDecide,
            },
        },
        {
            Type: agent.DecisionTransition,
            Transition: &agent.TransitionDecision{
                ToState: agent.StateAct,
            },
        },
        {
            Type: agent.DecisionCallTool,
            CallTool: &agent.CallToolDecision{
                ToolName: "greet",
                Input:    json.RawMessage(`{"name": "World"}`),
            },
        },
        {
            Type: agent.DecisionTransition,
            Transition: &agent.TransitionDecision{
                ToState: agent.StateValidate,
            },
        },
        {
            Type: agent.DecisionFinish,
            Finish: &agent.FinishDecision{
                Result:  "Greeting complete",
                Summary: "Successfully greeted the world",
            },
        },
    })

    // Create engine
    engine, err := agent.NewEngine(
        agent.WithTool(greetTool),
        agent.WithPlanner(p),
        agent.WithBudget("tool_calls", 10),
    )
    if err != nil {
        log.Fatal(err)
    }

    // Run
    run, err := engine.Run(context.Background(), "Greet someone")
    if err != nil {
        log.Fatal(err)
    }

    fmt.Printf("Status: %s\n", run.Status)
    fmt.Printf("Result: %v\n", run.Result)

    // Print evidence
    for _, ev := range run.Evidence {
        fmt.Printf("Evidence from %s: %s\n", ev.Source, ev.Content)
    }
}
```
