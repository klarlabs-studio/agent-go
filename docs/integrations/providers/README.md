# LLM Providers

agent-go supports multiple LLM providers for intelligent planning. All providers implement the same `Provider` interface, making them interchangeable.

## Quick Start

```go
import (
    "go.klarlabs.de/agent/infrastructure/planner"
)

// Create provider
provider := planner.NewAnthropicProvider(planner.AnthropicConfig{
    APIKey: os.Getenv("ANTHROPIC_API_KEY"),
    Model:  "claude-sonnet-4-20250514",
})

// Create LLM planner
llmPlanner := planner.NewLLMPlanner(planner.LLMPlannerConfig{
    Provider:    provider,
    Temperature: 0.3,
    SystemPrompt: `You are a helpful assistant.`,
})

// Use with engine
engine, _ := api.New(
    api.WithPlanner(llmPlanner),
    // ...
)
```

## Available Providers

| Provider | Import | Env Variable | Models |
|----------|--------|--------------|--------|
| Anthropic | `planner.NewAnthropicProvider` | `ANTHROPIC_API_KEY` | claude-sonnet-4-20250514, claude-3-haiku-20240307 |
| OpenAI | `planner.NewOpenAIProvider` | `OPENAI_API_KEY` | gpt-4-turbo, gpt-4o, gpt-3.5-turbo |
| Gemini | `planner.NewGeminiProvider` | `GEMINI_API_KEY` | gemini-pro, gemini-1.5-pro |
| Ollama | `planner.NewOllamaProvider` | `OLLAMA_URL` | llama3, mistral, codellama |

## Provider Configuration

### Anthropic (Claude)

```go
provider := planner.NewAnthropicProvider(planner.AnthropicConfig{
    APIKey:  os.Getenv("ANTHROPIC_API_KEY"), // Required
    BaseURL: "https://api.anthropic.com",    // Optional, for proxies
    Model:   "claude-sonnet-4-20250514",              // Recommended
    Timeout: 120,                            // Seconds
})
```

**Models:**
- `claude-sonnet-4-20250514` - Best balance of speed and capability (recommended)
- `claude-3-haiku-20240307` - Fastest, good for simple tasks

### OpenAI (GPT)

```go
provider := planner.NewOpenAIProvider(planner.OpenAIConfig{
    APIKey:  os.Getenv("OPENAI_API_KEY"), // Required
    BaseURL: "",                          // Optional, for Azure or proxies
    Model:   "gpt-4-turbo",               // Recommended
    Timeout: 120,
})
```

**Models:**
- `gpt-4-turbo` - Most capable
- `gpt-4o` - Optimized for chat
- `gpt-3.5-turbo` - Faster, more economical

### Gemini (Google)

```go
provider := planner.NewGeminiProvider(planner.GeminiConfig{
    APIKey:  os.Getenv("GEMINI_API_KEY"),
    Model:   "gemini-pro",
    Timeout: 120,
})
```

### Ollama (Local)

Run models locally without API keys:

```go
provider := planner.NewOllamaProvider(planner.OllamaConfig{
    BaseURL: "http://localhost:11434", // Ollama server URL
    Model:   "llama3",                 // Any installed model
    Timeout: 120,
})
```

**Setup:**
```bash
# Install Ollama
brew install ollama

# Pull a model
ollama pull llama3

# Start the server
ollama serve
```

## LLM Planner Configuration

The `LLMPlanner` wraps any provider and handles decision parsing:

```go
llmPlanner := planner.NewLLMPlanner(planner.LLMPlannerConfig{
    Provider:     provider,         // Required
    Model:        "",               // Override provider's default
    Temperature:  0.7,              // 0.0-1.0, lower = more deterministic
    MaxTokens:    1024,             // Response length limit
    SystemPrompt: "",               // Custom system prompt
})
```

### Custom System Prompts

Override the default system prompt for specialized behavior:

```go
llmPlanner := planner.NewLLMPlanner(planner.LLMPlannerConfig{
    Provider: provider,
    SystemPrompt: `You are a database assistant. You help users query and analyze data.

When asked to find information:
1. First use db_tables to see available tables
2. Use db_schema to understand table structure
3. Use db_query to retrieve data
4. Summarize findings clearly

Always explain your reasoning.`,
})
```

## Decision Format

The LLM responds in JSON format. The planner parses these into agent decisions:

```json
// Call a tool
{"decision": "call_tool", "tool_name": "read_file", "input": {"path": "/tmp/data.txt"}, "reason": "Reading config"}

// Transition state
{"decision": "transition", "to_state": "decide", "reason": "Gathered enough info"}

// Finish successfully
{"decision": "finish", "result": {"answer": 42}, "summary": "Found the answer"}

// Fail
{"decision": "fail", "reason": "Cannot access required file"}
```

## Testing Without LLMs

Use `ScriptedPlanner` or `MockPlanner` for deterministic testing:

```go
// ScriptedPlanner - predefined steps
planner := api.NewScriptedPlanner(
    api.ScriptStep{
        ExpectState: agent.StateIntake,
        Decision:    api.NewTransitionDecision(agent.StateExplore, "begin"),
    },
    api.ScriptStep{
        ExpectState: agent.StateExplore,
        Decision:    api.NewCallToolDecision("read_file", input, "gathering info"),
    },
    api.ScriptStep{
        ExpectState: agent.StateExplore,
        Decision:    api.NewTransitionDecision(agent.StateDecide, "ready"),
    },
    api.ScriptStep{
        ExpectState: agent.StateDecide,
        Decision:    api.NewFinishDecision("done", result),
    },
)
```

## Error Handling

Providers return structured errors:

```go
resp, err := provider.Complete(ctx, req)
if err != nil {
    // Network/timeout errors
    log.Printf("Provider error: %v", err)
    return
}

if resp.Error != nil {
    // API errors (rate limit, invalid key, etc.)
    log.Printf("API error: %s - %s", resp.Error.Type, resp.Error.Message)
    return
}
```

## Best Practices

1. **Use environment variables** for API keys, never hardcode
2. **Set appropriate timeouts** - LLM calls can take 10-30 seconds
3. **Configure temperature** - lower (0.1-0.3) for consistent behavior
4. **Test with scripted planners** - don't require LLM for unit tests
5. **Handle rate limits** - implement retry with backoff
6. **Monitor token usage** - track costs with `resp.Usage`

## Example: Dynamic Provider Selection

```go
func selectProvider() planner.Provider {
    if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
        return planner.NewAnthropicProvider(planner.AnthropicConfig{
            APIKey: key,
            Model:  "claude-sonnet-4-20250514",
        })
    }
    if key := os.Getenv("OPENAI_API_KEY"); key != "" {
        return planner.NewOpenAIProvider(planner.OpenAIConfig{
            APIKey: key,
            Model:  "gpt-4-turbo",
        })
    }
    if url := os.Getenv("OLLAMA_URL"); url != "" {
        return planner.NewOllamaProvider(planner.OllamaConfig{
            BaseURL: url,
            Model:   "llama3",
        })
    }
    return nil // Use scripted planner
}
```

## See Also

- [Example: LLM Planner](../../example/04-llm-planner/) - Complete working example
- [Planners Concept](../concepts/planners.md) - Understanding planners
- [Testing Guide](../testing.md) - Testing with mock planners
