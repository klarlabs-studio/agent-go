# FileOps Example

This example demonstrates the agent-go runtime with file operation tools.

## Overview

The FileOps example showcases:

- **Tool Registration**: Four file operation tools with proper annotations
- **State-Driven Execution**: Tools are only available in appropriate states
- **Policy Enforcement**: Tool eligibility and state transitions
- **Scripted Planner**: Deterministic execution for testing/demo
- **Budget Management**: Limiting tool calls

## Tools

| Tool | Description | Risk Level | Allowed States |
|------|-------------|------------|----------------|
| `read_file` | Reads file content | Low | explore, act, validate |
| `write_file` | Writes content to a file | Medium | act |
| `delete_file` | Deletes a file | High (requires approval) | act |
| `list_dir` | Lists directory contents | Low | explore, act, validate |

## State Machine Flow

```
intake → explore → decide → act → validate → done
```

The example follows this workflow:
1. **Intake**: Start state
2. **Explore**: List directory contents (read-only)
3. **Decide**: Determine action to take
4. **Act**: Create a file (side effects allowed)
5. **Validate**: Read back and verify file
6. **Done**: Complete with result

## Running

```bash
# From project root
go run ./example/fileops

# Or using make
make example
```

## Expected Output

```
=== FileOps Agent Example ===
Workspace: /tmp/fileops-example-xxxxx

--- Run Results ---
Run ID: run-xxxxxxxxx
Status: completed
Final State: done
Duration: Xms
Result: {
  "file": "hello.txt",
  "status": "created"
}

--- Evidence Trail ---
1. [tool] list_dir
   Output: {"files": [], "count": 0}
2. [tool] write_file
   Output: {"bytes_written": 19, "created": true}
3. [tool] read_file
   Output: {"content": "Hello, Agent World!", "size": 19}
4. [tool] list_dir
   Output: {"files": [{"name": "hello.txt", ...}], "count": 1}

--- File Verification ---
File path: /tmp/fileops-example-xxxxx/hello.txt
Content: Hello, Agent World!

=== Example completed successfully! ===
```

## Key Concepts Demonstrated

### Tool Annotations

```go
api.Annotations{
    ReadOnly:    true,   // No side effects
    Destructive: false,  // Doesn't destroy data
    Idempotent:  true,   // Same result if called multiple times
    Cacheable:   true,   // Results can be cached
    RiskLevel:   api.RiskLow,
}
```

### Tool Eligibility

```go
eligibility := api.NewToolEligibility()
// Read-only tools in explore
eligibility.Allow(agent.StateExplore, "read_file")
eligibility.Allow(agent.StateExplore, "list_dir")
// All tools in act state
eligibility.Allow(agent.StateAct, "write_file")
eligibility.Allow(agent.StateAct, "delete_file")
```

### Scripted Planner

```go
planner := api.NewScriptedPlanner(
    api.ScriptStep{
        ExpectState: agent.StateIntake,
        Decision:    api.NewTransitionDecision(agent.StateExplore, "Begin"),
    },
    api.ScriptStep{
        ExpectState: agent.StateExplore,
        Decision:    api.NewCallToolDecision("list_dir", input, "reason"),
    },
    // ... more steps
)
```

### Budget Enforcement

```go
api.WithBudgets(map[string]int{
    "tool_calls": 10,  // Max 10 tool calls per run
})
```

## Security

The tools implement path traversal protection:

```go
func isSubPath(base, path string) bool {
    // Ensures paths stay within the workspace directory
}
```

## Extending

To add more tools:

1. Define input/output types
2. Create the tool with `api.NewToolBuilder()`
3. Set appropriate annotations
4. Register with the registry
5. Configure eligibility for each state

## Related Examples

- [Customer Support Agent](../customer-support/) - Support ticket handling with KB search
- [DevOps Monitor Agent](../devops-monitor/) - Infrastructure monitoring with incident response

## See Also

- [Website Demo](https://klarlabs-studio.github.io/agent-go/#demo) - Interactive visualization of this agent
- [Documentation](https://klarlabs-studio.github.io/agent-go/docs/) - Full API documentation
- [GitHub Repository](https://github.com/klarlabs-studio/agent-go) - Source code and issues
