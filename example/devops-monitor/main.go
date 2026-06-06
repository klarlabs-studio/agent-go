// Package main demonstrates a DevOps monitoring agent using agent-go.
//
// This example shows:
// - Infrastructure monitoring workflow
// - Metrics collection and log analysis
// - Automated incident response with human approval
// - Service restart with validation
// - State-driven execution with approval for destructive actions
//
// Run with: go run ./example/devops-monitor
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

	fmt.Printf("=== DevOps Monitoring Agent Example ===\n\n")

	if err := runExample(); err != nil {
		fmt.Printf("Example failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\n=== Example completed successfully! ===\n")
}

func runExample() error {
	// Create mock infrastructure with sample services
	infra := NewMockInfrastructure()

	// Create tool registry with monitoring tools
	registry := api.NewToolRegistry()
	if err := registry.Register(NewGetMetricsTool(infra)); err != nil {
		return fmt.Errorf("failed to register get_metrics: %w", err)
	}
	if err := registry.Register(NewQueryLogsTool(infra)); err != nil {
		return fmt.Errorf("failed to register query_logs: %w", err)
	}
	if err := registry.Register(NewRestartServiceTool(infra)); err != nil {
		return fmt.Errorf("failed to register restart_service: %w", err)
	}
	if err := registry.Register(NewSendAlertTool(infra)); err != nil {
		return fmt.Errorf("failed to register send_alert: %w", err)
	}

	// Configure tool eligibility per state using declarative map
	eligibility := api.NewToolEligibilityWith(api.EligibilityRules{
		// Read-only tools allowed in explore state
		agent.StateExplore: {"get_metrics", "query_logs"},
		// Action tools in act state (restart requires approval due to Destructive annotation)
		agent.StateAct: {"restart_service", "send_alert", "get_metrics", "query_logs"},
		// Validation can check metrics
		agent.StateValidate: {"get_metrics", "query_logs"},
	})

	// Create a scripted planner that simulates incident response:
	// Scenario: High error rate detected on api-gateway service
	planner := api.NewScriptedPlanner(
		// Intake: alert triggered
		api.ScriptStep{
			ExpectState: agent.StateIntake,
			Decision:    api.NewTransitionDecision(agent.StateExplore, "Alert triggered: high error rate on api-gateway"),
		},
		// Explore: get service metrics
		api.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision: api.NewCallToolDecision("get_metrics",
				json.RawMessage(`{"service": "api-gateway"}`),
				"Check service health metrics"),
		},
		// Explore: query error logs
		api.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision: api.NewCallToolDecision("query_logs",
				json.RawMessage(`{"service": "api-gateway", "level": "error", "limit": 10}`),
				"Analyze error patterns"),
		},
		// Explore: transition to decide
		api.ScriptStep{
			ExpectState: agent.StateExplore,
			Decision:    api.NewTransitionDecision(agent.StateDecide, "Gathered metrics and logs"),
		},
		// Decide: diagnose and plan
		api.ScriptStep{
			ExpectState: agent.StateDecide,
			Decision:    api.NewTransitionDecision(agent.StateAct, "Connection pool exhausted. Service restart recommended."),
		},
		// Note: In a real scenario, the Destructive annotation on restart_service
		// would trigger an approval request here. The scripted planner simulates
		// that approval was granted.
		// Act: restart the service
		api.ScriptStep{
			ExpectState: agent.StateAct,
			Decision: api.NewCallToolDecision("restart_service",
				json.RawMessage(`{"service": "api-gateway", "graceful": true}`),
				"Restart service with graceful drain"),
		},
		// Act: transition to validate
		api.ScriptStep{
			ExpectState: agent.StateAct,
			Decision:    api.NewTransitionDecision(agent.StateValidate, "Service restarted, validating recovery"),
		},
		// Validate: check metrics after restart
		api.ScriptStep{
			ExpectState: agent.StateValidate,
			Decision: api.NewCallToolDecision("get_metrics",
				json.RawMessage(`{"service": "api-gateway"}`),
				"Verify service health after restart"),
		},
		// Finish with resolution
		api.ScriptStep{
			ExpectState: agent.StateValidate,
			Decision: api.NewFinishDecision(
				"Service restarted. Error rate normalized. Incident logged.",
				json.RawMessage(`{
					"resolution": "service_restarted",
					"service": "api-gateway",
					"root_cause": "connection_pool_exhausted",
					"action_taken": "graceful_restart",
					"downtime": "3.2s",
					"post_restart_status": "healthy"
				}`)),
		},
	)

	// Create the engine
	// Note: AutoApprover is used for demo purposes. In production, you would
	// use an interactive approver or integration with an approval system.
	engine, err := api.New(
		api.WithRegistry(registry),
		api.WithPlanner(planner),
		api.WithToolEligibility(eligibility),
		api.WithTransitions(api.DefaultTransitions()),
		api.WithApprover(api.AutoApprover()),
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
	run, err := engine.Run(ctx, "High error rate detected on api-gateway service")
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

	// Show service state after incident
	fmt.Printf("\n--- Post-Incident Service State ---\n")
	svc := infra.services["api-gateway"]
	fmt.Printf("Service: %s\n", svc.Name)
	fmt.Printf("Status: %s\n", svc.Status)
	fmt.Printf("CPU: %.0f%%\n", svc.CPU)
	fmt.Printf("Memory: %.0f%%\n", svc.Memory)
	fmt.Printf("Errors/min: %d\n", svc.ErrorsPerMin)
	fmt.Printf("Restarts: %d\n", svc.Restarts)

	return nil
}
