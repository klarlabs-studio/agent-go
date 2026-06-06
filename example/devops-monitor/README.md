# DevOps Monitoring Agent Example

This example demonstrates a DevOps monitoring agent that handles infrastructure incidents using the agent-go runtime.

## Overview

The DevOps Monitoring Agent showcases:

- **Metrics Collection**: Fetch service health metrics (CPU, memory, errors)
- **Log Analysis**: Query and analyze application logs for patterns
- **Automated Remediation**: Restart services with graceful drain
- **Alerting**: Send alerts to on-call teams
- **Human-in-the-Loop**: Destructive actions require approval
- **Incident Validation**: Verify recovery after remediation

## Tools

| Tool | Description | Risk Level | Allowed States |
|------|-------------|------------|----------------|
| `get_metrics` | Fetch service health metrics | Low | explore, act, validate |
| `query_logs` | Search application logs | Low | explore, act, validate |
| `restart_service` | Restart a service | High (requires approval) | act |
| `send_alert` | Send alert to on-call team | Medium | act |

## Scenario

The example simulates handling a high error rate incident:

> "High error rate detected on api-gateway service"

### Workflow

```
intake → explore → decide → act → validate → done
```

1. **Intake**: Alert triggered for high error rate
2. **Explore**:
   - Get service metrics (CPU 23%, Memory 67%, 847 errors/min)
   - Query error logs (pattern: "connection pool exhausted")
3. **Decide**: Diagnose root cause - connection pool exhausted, restart recommended
4. **Act**: Restart service with graceful connection drain (approval required)
5. **Validate**: Check metrics after restart (errors normalized to 2/min)
6. **Done**: Incident resolved, logged for post-mortem

## Running

```bash
# From project root
go run ./example/devops-monitor
```

## Expected Output

```
=== DevOps Monitoring Agent Example ===

--- Run Results ---
Run ID: run-xxxxxxxxx
Status: completed
Final State: done
Duration: Xms
Result: {
  "resolution": "service_restarted",
  "service": "api-gateway",
  "root_cause": "connection_pool_exhausted",
  "action_taken": "graceful_restart",
  "downtime": "3.2s",
  "post_restart_status": "healthy"
}

--- Evidence Trail ---
1. [tool] get_metrics
   Output: {"cpu": "23%", "memory": "67%", "errors": 847, "status": "degraded"}
2. [tool] query_logs
   Output: {"pattern": "connection pool exhausted", "count": 312, "level": "error"}
3. [tool] restart_service
   Output: {"status": "restarted", "downtime": "3.2s"}
4. [tool] get_metrics
   Output: {"cpu": "18%", "memory": "45%", "errors": 2, "status": "healthy"}

--- Post-Incident Service State ---
Service: api-gateway
Status: healthy
CPU: 18%
Memory: 45%
Errors/min: 2
Restarts: 1

=== Example completed successfully! ===
```

## Key Concepts Demonstrated

### Destructive Actions Require Approval

```go
// restart_service is marked as Destructive
api.Annotations{
    ReadOnly:    false,
    Destructive: true,  // Triggers approval workflow
    Idempotent:  true,
    RiskLevel:   api.RiskHigh,
}
```

In production, the agent would pause at `restart_service` and wait for human approval before proceeding.

This example uses `api.AutoApprover()` for demonstration:

```go
engine, err := api.New(
    // ... other options
    api.WithApprover(api.AutoApprover()), // Auto-approve for demo
)
```

For production, replace with an interactive or integration-based approver:

```go
// Interactive CLI approval
api.WithApprover(api.NewCallbackApprover(func(ctx context.Context, req api.ApprovalRequest) (bool, error) {
    fmt.Printf("Approve %s? (y/n): ", req.ToolName)
    // ... get user input
}))
```

### State-Based Tool Eligibility

```go
eligibility := api.NewToolEligibility()

// Explore: only observability tools
eligibility.Allow(agent.StateExplore, "get_metrics")
eligibility.Allow(agent.StateExplore, "query_logs")

// Act: remediation tools (with approval for destructive)
eligibility.Allow(agent.StateAct, "restart_service")
eligibility.Allow(agent.StateAct, "send_alert")

// Validate: verify recovery
eligibility.Allow(agent.StateValidate, "get_metrics")
```

### Mock Infrastructure

The example simulates:

- Multiple services with health states
- Log storage with error patterns and counts
- Service restart with state reset
- Alert delivery tracking

## Extending

To extend this example:

1. **Add more tools**: Scale replicas, rollback deployments, create incidents
2. **Connect real backends**: Prometheus, Grafana, PagerDuty, Kubernetes
3. **Add LLM planner**: Dynamic diagnosis based on log patterns
4. **Implement approval flow**: Real human-in-the-loop for production

## Production Considerations

- **Rate limiting**: Prevent restart loops
- **Cooldown periods**: Wait between restart attempts
- **Blast radius**: Limit concurrent restarts
- **Rollback capability**: Auto-rollback if restart doesn't help
- **Audit logging**: Track all actions with evidence

## Related Examples

- [FileOps Agent](../fileops/) - File operations with path security
- [Customer Support Agent](../customer-support/) - Support ticket handling with KB search

## See Also

- [Website Demo](https://klarlabs-studio.github.io/agent-go/#demo) - Interactive visualization of this agent
- [Documentation](https://klarlabs-studio.github.io/agent-go/docs/) - Full API documentation
- [GitHub Repository](https://github.com/klarlabs-studio/agent-go) - Source code and issues
