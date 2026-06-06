# Planners

Planners are the "brain" of your agent - they decide what action to take next. agent-go separates planning from execution, making it easy to swap intelligence layers without changing your agent's structure.

## The Planner Interface

All planners implement a simple interface:

```go
type Planner interface {
    Plan(ctx context.Context, req PlanRequest) (Decision, error)
}

type PlanRequest struct {
    RunID        string              // Current run identifier
    CurrentState State               // Where the agent is
    Evidence     []Evidence          // What the agent has learned
    AllowedTools []string            // Tools available in current state
    Budgets      BudgetSnapshot      // Remaining budget limits
    Vars         map[string]any      // User-defined variables
}
```

## Decision Types

Planners return one of five decision types:

### CallTool

Execute a tool with given input:

```go
decision := agent.NewCallToolDecision(
    "read_file",                              // Tool name
    json.RawMessage(`{"path": "/tmp/x"}`),    // Input
    "gathering information",                  // Reason
)
```

### Transition

Move to a different state:

```go
decision := agent.NewTransitionDecision(
    agent.StateAct,          // Target state
    "ready to make changes", // Reason
)
```

### Finish

Complete successfully:

```go
decision := agent.NewFinishDecision(
    "task completed",                        // Reason
    json.RawMessage(`{"result": "success"}`), // Result
)
```

### Fail

Terminate with failure:

```go
decision := agent.NewFailDecision(
    "cannot proceed without API key", // Reason
)
```

### AskHuman (future)

Request human input:

```go
decision := agent.NewAskHumanDecision(
    "Should I delete these 100 files?",  // Question
    []string{"yes", "no", "review"},     // Options
)
```

## Built-in Planners

### ScriptedPlanner

For deterministic testing. Follows a predefined script:

```go
planner := agent.NewScriptedPlanner(
    agent.ScriptStep{
        ExpectState: agent.StateIntake,
        Decision:    agent.NewTransitionDecision(agent.StateExplore, "starting"),
    },
    agent.ScriptStep{
        ExpectState: agent.StateExplore,
        Decision:    agent.NewCallToolDecision("read_file", input, "reading"),
    },
    agent.ScriptStep{
        ExpectState: agent.StateExplore,
        Decision:    agent.NewFinishDecision("done", result),
    },
)
```

**Use for**: Unit tests, integration tests, demos

### MockPlanner

Returns a single fixed decision:

```go
planner := agent.NewMockPlanner(
    agent.NewFinishDecision("immediate finish", nil),
)
```

**Use for**: Simple tests, edge case testing

### FunctionPlanner

Uses a custom function:

```go
planner := agent.NewFunctionPlanner(func(ctx context.Context, req agent.PlanRequest) (agent.Decision, error) {
    // Custom logic
    if req.CurrentState == agent.StateIntake {
        return agent.NewTransitionDecision(agent.StateExplore, "starting"), nil
    }

    if len(req.Evidence) > 5 {
        return agent.NewFinishDecision("enough info", nil), nil
    }

    return agent.NewCallToolDecision("gather_more", nil, "need more"), nil
})
```

**Use for**: Testing specific behaviors, simple heuristics

## LLM Planners

For production agents, use LLM-powered planners:

### Anthropic (Claude)

```go
import "go.klarlabs.de/agent/infrastructure/planner/provider/anthropic"

provider, _ := anthropic.New(
    anthropic.WithAPIKey(os.Getenv("ANTHROPIC_API_KEY")),
    anthropic.WithModel("claude-sonnet-4-20250514"),
)

llmPlanner := planner.NewLLMPlanner(planner.LLMPlannerConfig{
    Provider:     provider,
    SystemPrompt: "You are a helpful file management agent...",
    Temperature:  0.7,
})
```

### OpenAI (GPT-4)

```go
import "go.klarlabs.de/agent/infrastructure/planner/provider/openai"

provider, _ := openai.New(
    openai.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
    openai.WithModel("gpt-4-turbo"),
)
```

### Google (Gemini)

```go
import "go.klarlabs.de/agent/infrastructure/planner/provider/gemini"

provider, _ := gemini.New(
    gemini.WithAPIKey(os.Getenv("GEMINI_API_KEY")),
    gemini.WithModel("gemini-pro"),
)
```

### Ollama (Local)

```go
import "go.klarlabs.de/agent/infrastructure/planner/provider/ollama"

provider, _ := ollama.New(
    ollama.WithBaseURL("http://localhost:11434"),
    ollama.WithModel("llama3"),
)
```

## Creating Custom Planners

### Basic Custom Planner

```go
type MyPlanner struct {
    rules []Rule
}

func (p *MyPlanner) Plan(ctx context.Context, req agent.PlanRequest) (agent.Decision, error) {
    // Apply rules in order
    for _, rule := range p.rules {
        if decision, ok := rule.Apply(req); ok {
            return decision, nil
        }
    }

    // Default behavior
    return agent.NewFailDecision("no applicable rule"), nil
}
```

### Composite Planner

Combine multiple planners:

```go
type CompositeP struct {
    primary   agent.Planner
    fallback  agent.Planner
}

func (p *CompositeP) Plan(ctx context.Context, req agent.PlanRequest) (agent.Decision, error) {
    decision, err := p.primary.Plan(ctx, req)
    if err != nil {
        // Fall back to secondary planner
        return p.fallback.Plan(ctx, req)
    }
    return decision, nil
}
```

### Caching Planner

Add caching to any planner:

```go
type CachingPlanner struct {
    inner agent.Planner
    cache map[string]agent.Decision
}

func (p *CachingPlanner) Plan(ctx context.Context, req agent.PlanRequest) (agent.Decision, error) {
    key := computeCacheKey(req)
    if cached, ok := p.cache[key]; ok {
        return cached, nil
    }

    decision, err := p.inner.Plan(ctx, req)
    if err == nil {
        p.cache[key] = decision
    }
    return decision, err
}
```

## Planner Guarantees

Regardless of implementation, planners must satisfy these guarantees:

### 1. Bounded Output

Decisions are finite and well-defined. A planner cannot return arbitrary actions.

### 2. No Side Effects

Planners only analyze and decide. They never execute. The engine handles execution.

### 3. Conservative Bias

When uncertain, planners should prefer safe options (read over write, wait over act).

### 4. Deterministic Mode

For testing, planners must support deterministic behavior (e.g., ScriptedPlanner).

## Best Practices

### 1. Test with ScriptedPlanner

Always test your agent with deterministic planners first:

```go
func TestAgentBehavior(t *testing.T) {
    planner := agent.NewScriptedPlanner(expectedSteps...)

    engine, _ := agent.New(
        agent.WithPlanner(planner),
        agent.WithTools(tools...),
    )

    run, err := engine.Run(ctx, "test goal")
    assert.NoError(t, err)
    assert.Equal(t, agent.StatusDone, run.Status)
}
```

### 2. Separate Planning from Execution

Don't let planners access tools directly:

```go
// Good - planner only decides
func (p *MyPlanner) Plan(ctx context.Context, req PlanRequest) (Decision, error) {
    return NewCallToolDecision("read_file", input, "need info"), nil
}

// Bad - planner executes (violates separation)
func (p *BadPlanner) Plan(ctx context.Context, req PlanRequest) (Decision, error) {
    content, _ := os.ReadFile(path)  // Don't do this!
    return NewFinishDecision("done", content), nil
}
```

### 3. Include Reasoning

Always provide a reason for decisions:

```go
// Good - explains why
NewCallToolDecision("delete_file", input, "file is temporary and task complete")

// Bad - no context
NewCallToolDecision("delete_file", input, "")
```

### 4. Respect Allowed Tools

Only suggest tools from `AllowedTools`:

```go
func (p *MyPlanner) Plan(ctx context.Context, req PlanRequest) (Decision, error) {
    // Check if desired tool is allowed
    for _, allowed := range req.AllowedTools {
        if allowed == "write_file" {
            return NewCallToolDecision("write_file", input, "updating"), nil
        }
    }
    // Tool not available in current state
    return NewTransitionDecision(StateAct, "need write access"), nil
}
```

### 5. Handle Budget Exhaustion

Check budgets before making decisions:

```go
func (p *MyPlanner) Plan(ctx context.Context, req PlanRequest) (Decision, error) {
    if remaining, ok := req.Budgets["tool_calls"]; ok && remaining <= 0 {
        return NewFailDecision("budget exhausted"), nil
    }
    // Continue normal planning...
}
```

## Next Steps

- [Policies](policies.md) - Enforcing limits on planner decisions
- [Integration Guides](../integrations/) - Setting up LLM providers
