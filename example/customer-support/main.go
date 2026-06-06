// Package main demonstrates a customer support agent using agent-go.
//
// This example shows:
// - Support ticket handling workflow
// - Customer lookup and order status checking
// - Knowledge base search for policy lookup
// - Ticket creation and resolution
// - State-driven execution with proper tool eligibility
//
// Run with: go run ./example/customer-support
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/infrastructure/logging"
	api "go.klarlabs.de/agent/interfaces/api"
)

func main() {
	// Initialize logging
	logging.Init(logging.Config{
		Level:   "info",
		Format:  "console",
		NoColor: false,
	})

	fmt.Printf("=== Customer Support Agent Example ===\n\n")

	if err := runExample(); err != nil {
		fmt.Printf("Example failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n=== Example completed successfully! ===\n")
}

func runExample() error {
	// Create mock data store with sample customers and orders
	store := NewMockDataStore()

	// Create tool registry with support tools
	registry := api.NewToolRegistry()
	if err := registry.Register(NewLookupCustomerTool(store)); err != nil {
		return fmt.Errorf("failed to register lookup_customer: %w", err)
	}
	if err := registry.Register(NewGetOrderStatusTool(store)); err != nil {
		return fmt.Errorf("failed to register get_order_status: %w", err)
	}
	if err := registry.Register(NewSearchKBTool(store)); err != nil {
		return fmt.Errorf("failed to register search_kb: %w", err)
	}
	if err := registry.Register(NewCreateTicketTool(store)); err != nil {
		return fmt.Errorf("failed to register create_ticket: %w", err)
	}
	if err := registry.Register(NewEscalateTool(store)); err != nil {
		return fmt.Errorf("failed to register escalate: %w", err)
	}

	// Configure tool eligibility per state using declarative map
	eligibility := api.NewToolEligibilityWith(api.EligibilityRules{
		// Read-only tools allowed in explore state
		agent.StateExplore: {"lookup_customer", "get_order_status", "search_kb"},
		// Action tools (plus read-only) in act state
		agent.StateAct: {"create_ticket", "escalate", "lookup_customer", "get_order_status"},
		// Validation can verify customer and order info
		agent.StateValidate: {"search_kb", "lookup_customer", "get_order_status"},
	})

	// Create a scripted planner that simulates handling a shipping delay complaint:
	// Scenario: "Where is my order #38291? It's been 2 weeks!"
	planner := api.NewScriptedPlanner(
		// Intake: receive the complaint
		api.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    api.NewTransitionDecision(agent.StateExplore, "Ticket received: shipping delay inquiry"),
		},
		// Explore: look up the customer
		api.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision: api.NewCallToolDecision("lookup_customer",
				json.RawMessage(`{"email": "jane@email.com"}`),
				"Find customer record"),
		},
		// Explore: check order status
		api.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision: api.NewCallToolDecision("get_order_status",
				json.RawMessage(`{"order_id": "38291"}`),
				"Check order shipping status"),
		},
		// Explore: transition to decide
		api.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    api.NewTransitionDecision(agent.StateDecide, "Gathered customer and order info"),
		},
		// Decide: analyze and plan response
		api.ScriptStep{
			ExpectState: agent.StateDecide,
			Decision:    api.NewTransitionDecision(agent.StateAct, "Order delayed. Premium customer. Will offer compensation."),
		},
		// Act: create support ticket
		api.ScriptStep{
			ExpectState: agent.StateAct,
			Decision: api.NewCallToolDecision("create_ticket",
				json.RawMessage(`{"type": "shipping_delay", "priority": "high"}`),
				"Create high-priority ticket for premium customer"),
		},
		// Act: transition to validate
		api.ScriptStep{
			ExpectState: agent.StateAct,
			Decision:    api.NewTransitionDecision(agent.StateValidate, "Ticket created, looking up compensation policy"),
		},
		// Validate: search KB for compensation policy
		api.ScriptStep{
			ExpectState: agent.StateValidate,
			Decision: api.NewCallToolDecision("search_kb",
				json.RawMessage(`{"query": "shipping delay compensation"}`),
				"Find compensation policy"),
		},
		// Finish with resolution
		api.ScriptStep{
			ExpectState: agent.StateValidate,
			Decision: api.NewFinishDecision(
				"Ticket created. 10% refund applied. ETA shared with customer.",
				json.RawMessage(`{
					"resolution": "shipping_delay_compensated",
					"ticket_id": "TKT-9921",
					"compensation": "10% refund",
					"eta_communicated": true,
					"customer_tier": "premium"
				}`)),
		},
	)

	// Create the engine
	engine, err := api.New(
		api.WithRegistry(registry),
		api.WithPlanner(planner),
		api.WithToolEligibility(eligibility),
		api.WithTransitions(api.DefaultTransitions()),
		api.WithBudgets(map[string]int{
			"tool_calls": 10,
		}),
		api.WithMaxSteps(20),
	)
	if err != nil {
		return fmt.Errorf("failed to create engine: %w", err)
	}

	// Run the agent
	ctx := context.Background()
	run, err := engine.Run(ctx, "Where is my order #38291? It's been 2 weeks!")
	if err != nil {
		return fmt.Errorf("run failed: %w", err)
	}

	// Print results
	fmt.Printf("--- Run Results ---\n")
	fmt.Printf("Run ID: %s\n", run.ID)
	fmt.Printf("Status: %s\n", run.Status)
	fmt.Printf("Final State: %s\n", run.CurrentState)
	fmt.Printf("Duration: %v\n", run.Duration())

	if run.Result != nil {
		var result map[string]interface{}
		if err := json.Unmarshal(run.Result, &result); err == nil {
			resultJSON, _ := json.MarshalIndent(result, "", "  ")
			fmt.Printf("Result: %s\n", resultJSON)
		}
	}

	fmt.Printf("\n--- Evidence Trail ---\n")
	for i, ev := range run.Evidence {
		fmt.Printf("%d. [%s] %s\n", i+1, ev.Type, ev.Source)
		if ev.Content != nil {
			var output map[string]interface{}
			if err := json.Unmarshal(ev.Content, &output); err == nil {
				outputJSON, _ := json.MarshalIndent(output, "", "   ")
				fmt.Printf("   Output: %s\n", outputJSON)
			}
		}
	}

	return nil
}
