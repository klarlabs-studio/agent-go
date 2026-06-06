// Package main demonstrates Horizon 3: Governed Adaptivity features.
//
// This example shows the full workflow:
// 1. Creating runs with events (simulating agent executions)
// 2. Detecting behavioral patterns across runs
// 3. Generating policy improvement suggestions
// 4. Managing proposals through the approval workflow
// 5. Exporting data for visualization
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	domainAnalytics "go.klarlabs.de/agent/domain/analytics"
	"go.klarlabs.de/agent/domain/event"
	domainInspector "go.klarlabs.de/agent/domain/inspector"
	"go.klarlabs.de/agent/domain/pattern"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/domain/proposal"
	"go.klarlabs.de/agent/domain/run"
	"go.klarlabs.de/agent/domain/suggestion"
	"go.klarlabs.de/agent/infrastructure/analytics"
	infraInspector "go.klarlabs.de/agent/infrastructure/inspector"
	infraPattern "go.klarlabs.de/agent/infrastructure/pattern"
	infraProposal "go.klarlabs.de/agent/infrastructure/proposal"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
	infraSuggestion "go.klarlabs.de/agent/infrastructure/suggestion"
)

func main() {
	ctx := context.Background()

	fmt.Println("=== Horizon 3: Governed Adaptivity Demo ===")
	fmt.Println()

	// Initialize stores
	runStore := memory.NewRunStore()
	eventStore := memory.NewEventStore()
	patternStore := memory.NewPatternStore()
	suggestionStore := memory.NewSuggestionStore()
	proposalStore := memory.NewProposalStore()
	versionStore := memory.NewPolicyVersionStore()

	// Step 1: Simulate agent runs with events
	fmt.Println("Step 1: Simulating agent runs...")
	simulateRuns(ctx, runStore, eventStore)
	fmt.Println("  Created 5 runs with various events")
	fmt.Println()

	// Step 2: Detect patterns
	fmt.Println("Step 2: Detecting patterns...")
	patterns := detectPatterns(ctx, eventStore, runStore, patternStore)
	fmt.Printf("  Detected %d patterns:\n", len(patterns))
	for _, p := range patterns {
		fmt.Printf("    - %s (confidence: %.2f, frequency: %d)\n", p.Name, p.Confidence, p.Frequency)
	}
	fmt.Println()

	// Step 3: Generate suggestions
	fmt.Println("Step 3: Generating suggestions...")
	suggestions := generateSuggestions(ctx, patterns, suggestionStore)
	fmt.Printf("  Generated %d suggestions:\n", len(suggestions))
	for _, s := range suggestions {
		fmt.Printf("    - %s (%s)\n", s.Title, s.Type)
		fmt.Printf("      Rationale: %s\n", s.Rationale)
	}
	fmt.Println()

	// Step 4: Create and manage proposal
	fmt.Println("Step 4: Managing proposal workflow...")
	manageProposal(ctx, suggestions, proposalStore, versionStore)
	fmt.Println()

	// Step 5: Export visualization data
	fmt.Println("Step 5: Exporting visualization data...")
	exportVisualization(ctx, runStore, eventStore)
	fmt.Println()

	fmt.Println("=== Demo Complete ===")
	fmt.Println()
	fmt.Println("Key Takeaways:")
	fmt.Println("  1. Patterns detected automatically from run events")
	fmt.Println("  2. Suggestions generated (never directly applied)")
	fmt.Println("  3. Human approval required for all policy changes")
	fmt.Println("  4. Full audit trail with rollback capability")
	fmt.Println("  5. Export data for visualization and analysis")
}

func simulateRuns(ctx context.Context, runStore *memory.RunStore, eventStore *memory.EventStore) {
	for i := 0; i < 5; i++ {
		runID := fmt.Sprintf("run-%d", i+1)
		r := agent.NewRun(runID, fmt.Sprintf("Process task %d", i+1))
		_ = runStore.Save(ctx, r) // Ignore error in example

		// Simulate events for each run
		simulateRunEvents(ctx, eventStore, runID, i)
	}
}

func simulateRunEvents(ctx context.Context, eventStore *memory.EventStore, runID string, runIndex int) {
	baseTime := time.Now().Add(-time.Duration(runIndex) * time.Hour)

	// State transition events
	events := []struct {
		eventType event.Type
		data      map[string]any
		offset    time.Duration
	}{
		{event.TypeStateTransitioned, map[string]any{"from_state": "intake", "to_state": "explore", "reason": "begin"}, 0},
		{event.TypeToolCalled, map[string]any{"tool_name": "read_file"}, time.Second},
		{event.TypeToolSucceeded, map[string]any{"tool_name": "read_file", "duration": 150000000}, 2 * time.Second},
		{event.TypeToolCalled, map[string]any{"tool_name": "analyze_content"}, 3 * time.Second},
		{event.TypeToolSucceeded, map[string]any{"tool_name": "analyze_content", "duration": 300000000}, 4 * time.Second},
		{event.TypeStateTransitioned, map[string]any{"from_state": "explore", "to_state": "decide", "reason": "gathered info"}, 5 * time.Second},
		{event.TypeStateTransitioned, map[string]any{"from_state": "decide", "to_state": "act", "reason": "ready"}, 6 * time.Second},
		{event.TypeToolCalled, map[string]any{"tool_name": "write_file"}, 7 * time.Second},
		{event.TypeToolSucceeded, map[string]any{"tool_name": "write_file", "duration": 200000000}, 8 * time.Second},
		{event.TypeStateTransitioned, map[string]any{"from_state": "act", "to_state": "validate", "reason": "action done"}, 9 * time.Second},
		{event.TypeStateTransitioned, map[string]any{"from_state": "validate", "to_state": "done", "reason": "success"}, 10 * time.Second},
	}

	// Add some failures for pattern detection (in runs 2 and 4)
	if runIndex == 1 || runIndex == 3 {
		events[8] = struct {
			eventType event.Type
			data      map[string]any
			offset    time.Duration
		}{event.TypeToolFailed, map[string]any{"tool_name": "write_file", "error": "permission denied"}, 8 * time.Second}
	}

	for _, e := range events {
		data, err := json.Marshal(e.data)
		if err != nil {
			continue // Skip events that can't be marshaled
		}
		evt, err := event.NewEvent(runID, e.eventType, data)
		if err != nil {
			continue
		}
		// Override timestamp for realistic ordering
		evt.Timestamp = baseTime.Add(e.offset)
		_ = eventStore.Append(ctx, evt) // Ignore error in example
	}
}

func detectPatterns(ctx context.Context, eventStore *memory.EventStore, runStore *memory.RunStore, patternStore *memory.PatternStore) []pattern.Pattern {
	// Create detectors
	sequenceDetector := infraPattern.NewSequenceDetector(eventStore, runStore)
	failureDetector := infraPattern.NewFailureDetector(eventStore, runStore)

	// Create composite detector
	detector := infraPattern.NewCompositeDetector(sequenceDetector, failureDetector)

	// Detect patterns
	patterns, err := detector.Detect(ctx, pattern.DetectionOptions{
		MinConfidence: 0.5,
		MinFrequency:  2,
	})
	if err != nil {
		fmt.Printf("  Error detecting patterns: %v\n", err)
		return nil
	}

	// Store patterns
	for i := range patterns {
		_ = patternStore.Save(ctx, &patterns[i]) // Ignore error in example
	}

	return patterns
}

func generateSuggestions(ctx context.Context, patterns []pattern.Pattern, suggestionStore *memory.SuggestionStore) []suggestion.Suggestion {
	// Create generators
	eligibilityGen := infraSuggestion.NewEligibilityGenerator()
	budgetGen := infraSuggestion.NewBudgetGenerator()

	// Create composite generator
	generator := infraSuggestion.NewCompositeGenerator(eligibilityGen, budgetGen)

	// Generate suggestions
	suggestions, err := generator.Generate(ctx, patterns)
	if err != nil {
		fmt.Printf("  Error generating suggestions: %v\n", err)
		return nil
	}

	// Store suggestions
	for i := range suggestions {
		_ = suggestionStore.Save(ctx, &suggestions[i]) // Ignore error in example
	}

	return suggestions
}

func manageProposal(ctx context.Context, suggestions []suggestion.Suggestion, proposalStore *memory.ProposalStore, versionStore *memory.PolicyVersionStore) {
	// Create initial policy version
	eligibilitySnapshot := policy.NewEligibilitySnapshot()
	eligibilitySnapshot.AddTool(agent.StateExplore, "read_file")
	eligibilitySnapshot.AddTool(agent.StateExplore, "analyze_content")
	eligibilitySnapshot.AddTool(agent.StateAct, "write_file")

	transitionSnapshot := policy.NewTransitionSnapshot()
	transitionSnapshot.AddTransition(agent.StateIntake, agent.StateExplore)
	transitionSnapshot.AddTransition(agent.StateExplore, agent.StateDecide)
	transitionSnapshot.AddTransition(agent.StateDecide, agent.StateAct)
	transitionSnapshot.AddTransition(agent.StateDecide, agent.StateDone)
	transitionSnapshot.AddTransition(agent.StateAct, agent.StateValidate)
	transitionSnapshot.AddTransition(agent.StateValidate, agent.StateDone)
	transitionSnapshot.AddTransition(agent.StateValidate, agent.StateFailed)

	budgetSnapshot := policy.NewBudgetLimitsSnapshot()
	budgetSnapshot.SetLimit("tool_calls", 100)

	initialVersion := &policy.PolicyVersion{
		Version:     0,
		CreatedAt:   time.Now(),
		Description: "Initial policy",
		Eligibility: eligibilitySnapshot,
		Transitions: transitionSnapshot,
		Budgets:     budgetSnapshot,
		Approvals:   policy.NewApprovalSnapshot(),
	}
	_ = versionStore.Save(ctx, initialVersion) // Ignore error in example
	fmt.Println("  Initial policy version: v0")

	// Create workflow service with policy applier
	applier := infraProposal.NewPolicyApplier()
	workflow := infraProposal.NewWorkflowService(proposalStore, versionStore, applier)

	// Create a proposal
	prop, err := workflow.CreateProposal(ctx, "Improve tool eligibility", "Based on detected patterns", "system")
	if err != nil {
		fmt.Printf("  Error creating proposal: %v\n", err)
		return
	}
	fmt.Printf("  Created proposal: %s\n", prop.ID)

	// Add changes based on suggestions
	changesAdded := 0
	if len(suggestions) > 0 {
		for _, s := range suggestions {
			// Create proper eligibility change
			eligibilityChange := proposal.EligibilityChange{
				State:    agent.StateExplore,
				ToolName: "list_directory",
				Allowed:  true,
			}
			change, _ := proposal.NewPolicyChange(
				proposal.ChangeTypeEligibility,
				"explore:list_directory",
				s.Rationale,
				nil,
				eligibilityChange,
			)
			if s.Type == suggestion.SuggestionTypeIncreaseBudget {
				budgetChange := proposal.BudgetChange{
					BudgetName: "tool_calls",
					OldValue:   100,
					NewValue:   150,
				}
				change, _ = proposal.NewPolicyChange(
					proposal.ChangeTypeBudget,
					"tool_calls",
					s.Rationale,
					budgetChange.OldValue,
					budgetChange,
				)
			}
			if change != nil {
				_ = workflow.AddChange(ctx, prop.ID, *change) // Ignore error in example
				changesAdded++
			}
		}
	} else {
		// Add a sample change if no suggestions
		eligibilityChange := proposal.EligibilityChange{
			State:    agent.StateExplore,
			ToolName: "list_directory",
			Allowed:  true,
		}
		change, err := proposal.NewPolicyChange(
			proposal.ChangeTypeEligibility,
			"explore:list_directory",
			"Add list_directory to explore state",
			nil,
			eligibilityChange,
		)
		if err == nil && change != nil {
			_ = workflow.AddChange(ctx, prop.ID, *change) // Ignore error in example
			changesAdded++
		}
	}
	fmt.Printf("  Added %d changes to proposal\n", changesAdded)

	// Submit for review
	err = workflow.Submit(ctx, prop.ID, "developer@example.com")
	if err != nil {
		fmt.Printf("  Error submitting proposal: %v\n", err)
		return
	}
	fmt.Println("  Submitted proposal for review")

	// Approve (simulating human approval)
	err = workflow.Approve(ctx, prop.ID, "admin@example.com", "Approved after review")
	if err != nil {
		fmt.Printf("  Error approving proposal: %v\n", err)
		return
	}
	fmt.Println("  Proposal approved by admin@example.com")

	// Apply the changes
	err = workflow.Apply(ctx, prop.ID)
	if err != nil {
		fmt.Printf("  Error applying proposal: %v\n", err)
		return
	}
	fmt.Println("  Applied proposal - policy updated to v1")

	// Show version history
	versions, _ := versionStore.List(ctx)
	fmt.Printf("  Version history: %d versions\n", len(versions))

	// Demonstrate rollback capability
	fmt.Println("  Demonstrating rollback...")
	err = workflow.Rollback(ctx, prop.ID, "Demonstrating rollback capability")
	if err != nil {
		fmt.Printf("  Error rolling back: %v\n", err)
		return
	}
	fmt.Println("  Rolled back to previous version")
}

func exportVisualization(ctx context.Context, runStore *memory.RunStore, eventStore *memory.EventStore) {
	// Create analytics
	analyticsService := analytics.NewAggregator(runStore, eventStore)

	// Create eligibility and transitions for state machine export
	eligibility := policy.NewToolEligibility()
	eligibility.Allow(agent.StateExplore, "read_file")
	eligibility.Allow(agent.StateExplore, "analyze_content")
	eligibility.Allow(agent.StateAct, "write_file")

	transitions := policy.NewStateTransitions()
	transitions.Allow(agent.StateIntake, agent.StateExplore)
	transitions.Allow(agent.StateExplore, agent.StateDecide)
	transitions.Allow(agent.StateDecide, agent.StateAct)
	transitions.Allow(agent.StateDecide, agent.StateDone)
	transitions.Allow(agent.StateAct, agent.StateValidate)
	transitions.Allow(agent.StateValidate, agent.StateDone)
	transitions.Allow(agent.StateValidate, agent.StateFailed)

	// Export run as JSON using RunExporter
	runExporter := infraInspector.NewRunExporter(runStore, eventStore)
	runs, _ := runStore.List(ctx, run.ListFilter{})
	if len(runs) > 0 {
		runExport, err := runExporter.Export(ctx, runs[0].ID)
		if err == nil {
			jsonFormatter := infraInspector.NewJSONFormatter(infraInspector.WithPrettyPrint())
			jsonData, _ := jsonFormatter.Format(runExport)
			fmt.Printf("  JSON run export (%d bytes)\n", len(jsonData))
		}
	}

	// Export state machine using StateMachineExporter
	stateMachineExporter := infraInspector.NewStateMachineExporter(eligibility, transitions)
	smExport, err := stateMachineExporter.Export(ctx)
	if err == nil {
		// Export DOT
		dotFormatter := infraInspector.NewDOTFormatter()
		dotData, _ := dotFormatter.Format(smExport)
		fmt.Printf("  DOT export (%d bytes) - use with Graphviz\n", len(dotData))

		// Export Mermaid
		mermaidFormatter := infraInspector.NewMermaidFormatter()
		mermaidData, _ := mermaidFormatter.Format(smExport)
		fmt.Printf("  Mermaid export (%d bytes) - use in Markdown\n", len(mermaidData))

		// Print Mermaid diagram
		fmt.Println("\n  State Machine (Mermaid):")
		fmt.Println("  ```mermaid")
		lines := splitLines(string(mermaidData))
		for _, line := range lines {
			fmt.Printf("  %s\n", line)
		}
		fmt.Println("  ```")

		// Save exports to files (example code - permissive for readability)
		_ = os.WriteFile("state_machine.dot", dotData, 0600)     // #nosec G306
		_ = os.WriteFile("state_machine.mmd", mermaidData, 0600) // #nosec G306
		fmt.Println("\n  Saved: state_machine.dot, state_machine.mmd")
	}

	// Export metrics
	metricsExporter := infraInspector.NewMetricsExporter(runStore, eventStore)
	metricsExport, err := metricsExporter.Export(ctx, domainInspector.MetricsFilter{})
	if err == nil {
		jsonFormatter := infraInspector.NewJSONFormatter(infraInspector.WithPrettyPrint())
		metricsData, _ := jsonFormatter.Format(metricsExport)
		fmt.Printf("\n  Metrics export (%d bytes)\n", len(metricsData))
	}

	// Get analytics summary
	summary, err := analyticsService.RunSummary(ctx, domainAnalytics.Filter{})
	if err == nil {
		fmt.Printf("\n  Analytics Summary:\n")
		fmt.Printf("    Total Runs: %d\n", summary.TotalRuns)
		fmt.Printf("    Completed: %d\n", summary.CompletedRuns)
		fmt.Printf("    Failed: %d\n", summary.FailedRuns)
		if summary.TotalRuns > 0 {
			successRate := float64(summary.CompletedRuns) / float64(summary.TotalRuns) * 100
			fmt.Printf("    Success Rate: %.1f%%\n", successRate)
		}
	}
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
