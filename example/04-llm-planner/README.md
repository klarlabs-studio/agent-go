# 04 - LLM Planner

Demonstrates using real LLM providers (Anthropic, OpenAI, Ollama) for intelligent planning instead of scripted steps.

## What This Example Shows

- Configuring different LLM providers
- Creating an LLM-powered planner
- How the same engine works with different intelligence backends
- Graceful fallback when no API key is available

## Prerequisites

Set one of these environment variables:

```bash
# For Anthropic Claude
export ANTHROPIC_API_KEY="your-api-key"

# For OpenAI GPT-4
export OPENAI_API_KEY="your-api-key"

# For local Ollama
export OLLAMA_URL="http://localhost:11434"
```

## Run It

```bash
# With API key set
go run main.go

# Without API key (uses mock planner)
go run main.go
```

## Expected Output (with LLM)

```
=== LLM Planner Example ===
Provider: Anthropic (Claude)

Goal: What is 15 multiplied by 7?

  [calculate] multiply(15, 7) = 105

=== Result ===
Status: done
Steps: 3
Result: {"answer": 105, "explanation": "15 × 7 = 105"}
```

## Provider Configuration

### Anthropic (Claude)

```go
import "go.klarlabs.de/agent/contrib/planner-llm/providers/anthropic"

provider, _ := anthropic.New(
    anthropic.WithAPIKey(os.Getenv("ANTHROPIC_API_KEY")),
    anthropic.WithModel("claude-sonnet-4-20250514"),
)
```

### OpenAI (GPT-4)

```go
import "go.klarlabs.de/agent/contrib/planner-llm/providers/openai"

provider, _ := openai.New(
    openai.WithAPIKey(os.Getenv("OPENAI_API_KEY")),
    openai.WithModel("gpt-4-turbo"),
)
```

### Google Gemini

```go
import "go.klarlabs.de/agent/contrib/planner-llm/providers/gemini"

provider, _ := gemini.New(
    gemini.WithAPIKey(os.Getenv("GEMINI_API_KEY")),
    gemini.WithModel("gemini-pro"),
)
```

### Ollama (Local)

```go
import "go.klarlabs.de/agent/contrib/planner-llm/providers/ollama"

provider, _ := ollama.New(
    ollama.WithBaseURL("http://localhost:11434"),
    ollama.WithModel("llama3"),
)
```

## Creating an LLM Planner

```go
import llmplanner "go.klarlabs.de/agent/contrib/planner-llm"

llmPlanner := llmplanner.NewLLMPlanner(llmplanner.LLMPlannerConfig{
    Provider:     provider,
    Temperature:  0.3,              // Lower = more deterministic
    SystemPrompt: "You are a...",   // Instructions for the LLM
})
```

## Key Concept: Planner Swappability

The engine doesn't care which planner you use. All planners implement the same interface:

```go
type Planner interface {
    Plan(ctx context.Context, req PlanRequest) (Decision, error)
}
```

This means you can:
1. **Develop** with `ScriptedPlanner` (fast, no API costs)
2. **Test** with `MockPlanner` (controlled responses)
3. **Deploy** with `LLMPlanner` (real intelligence)

```go
// Same engine configuration
engine, _ := agent.New(
    agent.WithTool(myTool),
    agent.WithPlanner(planner),  // Swap this!
)
```

## System Prompts

Guide the LLM's behavior with system prompts:

```go
SystemPrompt: `You are a helpful assistant that processes files.

Available tools:
- read_file: Read file contents
- write_file: Write to a file

Always read before writing. Verify your work.`
```

## Next Steps

- **[05-observability](../05-observability/)** - Add tracing and metrics
- **[06-distributed](../06-distributed/)** - Scale with multiple workers
