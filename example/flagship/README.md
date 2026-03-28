# Flagship Example — Multi-Agent Coordination

This example demonstrates the full agent-go platform in a single program:

- **3 agents**: coordinator delegates to researcher + executor
- **Shared task context**: variables and evidence visible across all agents
- **Event streaming**: 21 real-time events consumed via `Stream()` API
- **Run persistence**: all runs saved with parent-child hierarchy
- **Policy enforcement**: budgets, tool eligibility, approval for destructive tools
- **State machine**: full traversal (intake → explore → decide → act → validate → done)

## Running

```bash
go run ./example/flagship
```

## Architecture

```
Coordinator (orchestrates)
  ├── research_agent (DelegateTool)
  │   └── analyze_data (read-only, cacheable)
  └── executor_agent (DelegateTool)
      └── write_report (destructive, auto-approved)

Shared: TaskContext, EventStore, RunStore
```

## What It Shows

| Feature | How It's Used |
|---------|--------------|
| Multi-agent | DelegateTool wraps child engines as tools |
| Shared state | TaskContext propagates vars + evidence |
| Event stream | `engine.Stream()` returns real-time event channel |
| Persistence | RunStore saves all 3 runs with ParentRunID links |
| Budgets | Coordinator has 50 tool call budget |
| Approval | Executor uses AutoApprover for destructive write_report |
| Tracing | ParentRunID tracks delegation hierarchy |
