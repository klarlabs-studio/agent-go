# Customer Support Agent Example

This example demonstrates a customer support agent that handles support tickets using the agent-go runtime.

## Overview

The Customer Support Agent showcases:

- **Customer Lookup**: Find customers by email or ID
- **Order Status Checking**: Track order shipping status
- **Knowledge Base Search**: Find policy articles for resolution
- **Ticket Creation**: Create and track support tickets
- **Human Escalation**: Escalate complex issues to human agents
- **State-Driven Workflow**: Proper separation of read and write operations

## Tools

| Tool | Description | Risk Level | Allowed States |
|------|-------------|------------|----------------|
| `lookup_customer` | Find customer by email or ID | Low | explore, act, validate |
| `get_order_status` | Check order shipping status | Low | explore, act, validate |
| `search_kb` | Search knowledge base articles | Low | explore, validate |
| `create_ticket` | Create a support ticket | Medium | act |
| `escalate` | Escalate to human agent | High (requires approval) | act |

## Scenario

The example simulates handling a shipping delay complaint:

> "Where is my order #38291? It's been 2 weeks!"

### Workflow

```
intake → explore → decide → act → validate → done
```

1. **Intake**: Receive customer complaint
2. **Explore**:
   - Look up customer by email (Jane Smith, premium tier)
   - Check order status (delayed, ETA 2 days)
3. **Decide**: Analyze situation - premium customer with delayed order
4. **Act**: Create high-priority support ticket
5. **Validate**: Search KB for compensation policy (10% refund)
6. **Done**: Resolution with compensation applied

## Running

```bash
# From project root
go run ./example/customer-support
```

## Expected Output

```
=== Customer Support Agent Example ===

--- Run Results ---
Run ID: run-xxxxxxxxx
Status: completed
Final State: done
Duration: Xms
Result: {
  "resolution": "shipping_delay_compensated",
  "ticket_id": "TKT-9921",
  "compensation": "10% refund",
  "eta_communicated": true,
  "customer_tier": "premium"
}

--- Evidence Trail ---
1. [tool] lookup_customer
   Output: {"id": "cust_847", "name": "Jane Smith", "email": "jane@email.com", "tier": "premium"}
2. [tool] get_order_status
   Output: {"order_id": "38291", "status": "delayed", "carrier": "FedEx", "eta": "2 days"}
3. [tool] create_ticket
   Output: {"ticket_id": "TKT-9921", "status": "open"}
4. [tool] search_kb
   Output: {"article": "POL-201", "title": "Shipping Delay Compensation", "action": "10% refund"}

=== Example completed successfully! ===
```

## Key Concepts Demonstrated

### Tool Annotations by Use Case

```go
// Read-only tools for data gathering
api.Annotations{
    ReadOnly:   true,
    Idempotent: true,
    Cacheable:  true,
    RiskLevel:  api.RiskLow,
}

// Action tools that modify state
api.Annotations{
    ReadOnly:    false,
    Destructive: false,
    Idempotent:  true,  // Creating same ticket twice is safe
    RiskLevel:   api.RiskMedium,
}

// High-risk tools requiring approval
api.Annotations{
    ReadOnly:    false,
    Destructive: true,  // Escalation is irreversible
    Idempotent:  false,
    RiskLevel:   api.RiskHigh,
}
```

### State-Based Tool Eligibility

```go
eligibility := api.NewToolEligibility()

// Explore: only read-only tools
eligibility.Allow(agent.StateExplore, "lookup_customer")
eligibility.Allow(agent.StateExplore, "get_order_status")
eligibility.Allow(agent.StateExplore, "search_kb")

// Act: tools that modify state
eligibility.Allow(agent.StateAct, "create_ticket")
eligibility.Allow(agent.StateAct, "escalate")

// Validate: verification tools only
eligibility.Allow(agent.StateValidate, "search_kb")
```

### Mock Data Store

The example uses an in-memory data store simulating:

- Customer database with tiers (standard, premium)
- Order tracking with status and ETA
- Knowledge base with policy articles
- Ticket system

## Extending

To extend this example:

1. **Add more tools**: Refund processing, email sending, chat integration
2. **Connect real backends**: CRM, order management, ticketing systems
3. **Add LLM planner**: Replace scripted planner with actual LLM for dynamic decisions
4. **Implement human-in-the-loop**: Real approval flow for escalations

## Related Examples

- [FileOps Agent](../fileops/) - File operations with path security
- [DevOps Monitor Agent](../devops-monitor/) - Infrastructure monitoring with incident response

## See Also

- [Website Demo](https://klarlabs-studio.github.io/agent-go/#demo) - Interactive visualization of this agent
- [Documentation](https://klarlabs-studio.github.io/agent-go/docs/) - Full API documentation
- [GitHub Repository](https://github.com/klarlabs-studio/agent-go) - Source code and issues
