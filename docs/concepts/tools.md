# Tools

Tools are the agent's capabilities - the actions it can take to accomplish its goals. Each tool in agent-go is a well-defined unit with explicit behavior declarations.

## Anatomy of a Tool

Every tool has four components:

```go
tool := agent.NewToolBuilder("tool_name").      // 1. Identity
    WithDescription("What this tool does").     // 2. Description
    WithAnnotations(agent.Annotations{...}).    // 3. Behavior metadata
    WithHandler(executionFunc).                 // 4. Implementation
    MustBuild()
```

## Creating Tools

### Basic Tool

```go
import (
    "context"
    "encoding/json"

    agent "go.klarlabs.de/agent/interfaces/api"
    "go.klarlabs.de/agent/domain/tool"
)

readFile := agent.NewToolBuilder("read_file").
    WithDescription("Reads the contents of a file at the given path").
    WithAnnotations(agent.Annotations{
        ReadOnly:   true,
        Idempotent: true,
        Cacheable:  true,
        RiskLevel:  agent.RiskLow,
    }).
    WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
        // Parse input
        var in struct {
            Path string `json:"path"`
        }
        if err := json.Unmarshal(input, &in); err != nil {
            return tool.Result{}, fmt.Errorf("invalid input: %w", err)
        }

        // Execute
        content, err := os.ReadFile(in.Path)
        if err != nil {
            return tool.Result{}, fmt.Errorf("failed to read file: %w", err)
        }

        // Return result
        output, _ := json.Marshal(map[string]string{
            "content": string(content),
        })
        return tool.Result{Output: output}, nil
    }).
    MustBuild()
```

### Tool with Input Schema

For better validation and documentation:

```go
readFile := agent.NewToolBuilder("read_file").
    WithDescription("Reads file contents").
    WithInputSchema(tool.NewSchema(json.RawMessage(`{
        "type": "object",
        "properties": {
            "path": {
                "type": "string",
                "description": "Absolute path to the file"
            },
            "encoding": {
                "type": "string",
                "enum": ["utf-8", "ascii", "base64"],
                "default": "utf-8"
            }
        },
        "required": ["path"]
    }`))).
    WithOutputSchema(tool.NewSchema(json.RawMessage(`{
        "type": "object",
        "properties": {
            "content": {"type": "string"},
            "size": {"type": "integer"}
        }
    }`))).
    WithAnnotations(agent.Annotations{ReadOnly: true}).
    WithHandler(readHandler).
    MustBuild()
```

## Annotations

Annotations declare a tool's behavioral characteristics. The runtime uses these to make decisions:

### ReadOnly

```go
WithAnnotations(agent.Annotations{
    ReadOnly: true,  // Tool does not modify external state
})
```

- **Effect**: Can run in `explore` and `validate` states
- **Use for**: Reading files, querying databases, API GET requests

### Destructive

```go
WithAnnotations(agent.Annotations{
    Destructive: true,  // May cause irreversible changes
})
```

- **Effect**: Requires approval (if approver configured), only in `act` state
- **Use for**: Deleting files, dropping tables, sending emails

### Idempotent

```go
WithAnnotations(agent.Annotations{
    Idempotent: true,  // Same input always produces same result, safe to retry
})
```

- **Effect**: Automatic retry with exponential backoff on transient failures
- **Use for**: Reading, PUT operations, idempotent writes

### Cacheable

```go
WithAnnotations(agent.Annotations{
    Cacheable: true,  // Results can be cached based on input
})
```

- **Effect**: Results are memoized; same input returns cached result
- **Use for**: Expensive computations, static data retrieval

### RiskLevel

```go
WithAnnotations(agent.Annotations{
    RiskLevel: agent.RiskHigh,  // None, Low, Medium, High, Critical
})
```

- **Effect**: Influences approval requirements and audit logging
- **Use for**: Risk classification for governance

### Combined Example

```go
// A high-risk, destructive, non-idempotent tool
deleteDatabase := agent.NewToolBuilder("delete_database").
    WithAnnotations(agent.Annotations{
        ReadOnly:    false,
        Destructive: true,
        Idempotent:  false,  // Can't delete twice
        Cacheable:   false,
        RiskLevel:   agent.RiskCritical,
    }).
    WithHandler(deleteHandler).
    MustBuild()
```

## Tool Registry

Tools are registered in a registry for the engine to access:

```go
// Create registry
registry := agent.NewToolRegistry()

// Register tools
registry.Register(readFile)
registry.Register(writeFile)
registry.Register(listDir)

// Or register multiple at once
registry.RegisterAll(readFile, writeFile, listDir)

// Build engine with registry
engine, _ := agent.New(
    agent.WithRegistry(registry),
)
```

### Tool Uniqueness

Tool names must be unique within a registry:

```go
registry.Register(readFile)
err := registry.Register(anotherReadFile)  // Error: duplicate name
```

## Tool Results

Tools return structured results:

```go
type Result struct {
    Output   json.RawMessage  // The tool's output
    Metadata map[string]any   // Optional metadata
}
```

### Returning Output

```go
func handler(ctx context.Context, input json.RawMessage) (tool.Result, error) {
    // Do work...

    output, _ := json.Marshal(map[string]any{
        "files": []string{"a.txt", "b.txt"},
        "count": 2,
    })

    return tool.Result{Output: output}, nil
}
```

### Returning Errors

```go
func handler(ctx context.Context, input json.RawMessage) (tool.Result, error) {
    // Errors are recorded in the ledger and may trigger retry (if idempotent)
    return tool.Result{}, fmt.Errorf("file not found: %s", path)
}
```

### Adding Metadata

```go
func handler(ctx context.Context, input json.RawMessage) (tool.Result, error) {
    start := time.Now()
    // Do work...

    return tool.Result{
        Output: output,
        Metadata: map[string]any{
            "duration_ms": time.Since(start).Milliseconds(),
            "cache_hit":   false,
        },
    }, nil
}
```

## Tool Context

The context passed to handlers carries useful information:

```go
func handler(ctx context.Context, input json.RawMessage) (tool.Result, error) {
    // Access run ID from context
    runID := agent.RunIDFromContext(ctx)

    // Check for cancellation
    select {
    case <-ctx.Done():
        return tool.Result{}, ctx.Err()
    default:
    }

    // Proceed with work...
}
```

## Domain Packs

For common use cases, use pre-built tool packs:

```go
import "go.klarlabs.de/agent/pack/database"
import "go.klarlabs.de/agent/pack/git"

// Database pack provides: query, execute, schema, tables
dbPack := database.New(dbConnection, database.WithMaxRows(1000))

// Git pack provides: status, log, diff, commit, branch
gitPack := git.New("/path/to/repo")

// Register pack tools
for _, t := range dbPack.Tools() {
    registry.Register(t)
}
```

## Best Practices

### 1. Be Specific About Side Effects

```go
// Good - clearly indicates behavior
WithAnnotations(agent.Annotations{
    ReadOnly:    false,
    Destructive: true,
})

// Bad - unclear behavior
WithAnnotations(agent.Annotations{})  // Defaults may not match reality
```

### 2. Validate Input Early

```go
func handler(ctx context.Context, input json.RawMessage) (tool.Result, error) {
    var in InputStruct
    if err := json.Unmarshal(input, &in); err != nil {
        return tool.Result{}, fmt.Errorf("invalid input: %w", err)
    }

    // Validate business rules
    if in.Path == "" {
        return tool.Result{}, errors.New("path is required")
    }

    // Then execute...
}
```

### 3. Use Meaningful Error Messages

```go
// Good - actionable error
return tool.Result{}, fmt.Errorf("failed to read %s: file not found", path)

// Bad - unhelpful
return tool.Result{}, errors.New("error")
```

### 4. Consider Idempotency

If a tool is safe to retry, mark it:

```go
// This tool can be called multiple times with same input
WithAnnotations(agent.Annotations{
    Idempotent: true,  // Enables automatic retry
})
```

### 5. Use Schemas for Complex Tools

```go
// For tools with complex input, add a schema
WithInputSchema(tool.NewSchema(schema))  // Enables validation
```

## Next Steps

- [Planners](planners.md) - How planners decide which tools to call
- [Policies](policies.md) - Controlling tool access with eligibility rules
