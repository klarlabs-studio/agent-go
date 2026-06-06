package test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/domain/inspector"
	"go.klarlabs.de/agent/domain/pattern"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/domain/proposal"
	"go.klarlabs.de/agent/domain/run"
	infraInspector "go.klarlabs.de/agent/infrastructure/inspector"
	infraPattern "go.klarlabs.de/agent/infrastructure/pattern"
	infraProposal "go.klarlabs.de/agent/infrastructure/proposal"
	"go.klarlabs.de/agent/infrastructure/storage/memory"
	infraSuggestion "go.klarlabs.de/agent/infrastructure/suggestion"
)

// TestHorizon3_EndToEnd_FullWorkflow tests the complete governed adaptivity workflow:
// 1. Create runs with events
// 2. Detect patterns across runs
// 3. Generate suggestions from patterns
// 4. Create proposal from suggestion
// 5. Approve proposal
// 6. Apply proposal (policy changes)
// 7. Rollback proposal
func TestHorizon3_EndToEnd_FullWorkflow(t *testing.T) {
	ctx := context.Background()

	// === Setup Stores ===
	runStore := memory.NewRunStore()
	eventStore := memory.NewEventStore()
	patternStore := memory.NewPatternStore()
	suggestionStore := memory.NewSuggestionStore()
	proposalStore := memory.NewProposalStore()
	versionStore := memory.NewPolicyVersionStore()

	// === Setup Initial Policy Version ===
	initialVersion := &policy.PolicyVersion{
		Version:     0,
		CreatedAt:   time.Now(),
		Description: "Initial policy",
		Eligibility: policy.NewEligibilitySnapshot(),
		Transitions: policy.NewTransitionSnapshot(),
		Budgets:     policy.NewBudgetLimitsSnapshot(),
		Approvals:   policy.NewApprovalSnapshot(),
	}
	initialVersion.Eligibility.AddTool(agent.StateExplore, "read_file")
	initialVersion.Eligibility.AddTool(agent.StateAct, "write_file")
	initialVersion.Budgets.SetLimit("tool_calls", 100)
	if err := versionStore.Save(ctx, initialVersion); err != nil {
		t.Fatalf("failed to save initial version: %v", err)
	}

	// === Step 1: Create Runs with Events ===
	t.Log("Step 1: Creating runs with events...")

	// Create 5 runs that all use the same tool sequence: read_file -> process_data -> write_file
	for i := 0; i < 5; i++ {
		r := agent.NewRun(fmt.Sprintf("run-%d", i), "Process data files")
		r.Status = agent.RunStatusCompleted
		r.EndTime = r.StartTime.Add(time.Minute)
		if err := runStore.Save(ctx, r); err != nil {
			t.Fatalf("failed to save run %d: %v", i, err)
		}

		baseTime := time.Now().Add(-time.Hour * time.Duration(5-i))

		// Add tool call events for the sequence
		tools := []string{"read_file", "process_data", "write_file"}
		for j, toolName := range tools {
			// Tool called event
			calledPayload := event.ToolCalledPayload{
				ToolName: toolName,
				Input:    json.RawMessage(`{"file": "test.txt"}`),
				State:    agent.StateExplore,
			}
			calledEvt, err := event.NewEvent(r.ID, event.TypeToolCalled, calledPayload)
			if err != nil {
				t.Fatalf("failed to create tool called event: %v", err)
			}
			calledEvt.Timestamp = baseTime.Add(time.Duration(j*2) * time.Minute)
			if err := eventStore.Append(ctx, calledEvt); err != nil {
				t.Fatalf("failed to append tool called event: %v", err)
			}

			// Tool succeeded event
			succeededPayload := event.ToolSucceededPayload{
				ToolName: toolName,
				Output:   json.RawMessage(`{"result": "success"}`),
				Duration: 100 * time.Millisecond,
			}
			succeededEvt, err := event.NewEvent(r.ID, event.TypeToolSucceeded, succeededPayload)
			if err != nil {
				t.Fatalf("failed to create tool succeeded event: %v", err)
			}
			succeededEvt.Timestamp = baseTime.Add(time.Duration(j*2+1) * time.Minute)
			if err := eventStore.Append(ctx, succeededEvt); err != nil {
				t.Fatalf("failed to append tool succeeded event: %v", err)
			}
		}
	}

	// Verify runs created
	runs, _ := runStore.List(ctx, run.ListFilter{})
	if len(runs) != 5 {
		t.Fatalf("expected 5 runs, got %d", len(runs))
	}
	t.Logf("  Created %d runs with tool sequences", len(runs))

	// === Step 2: Detect Patterns ===
	t.Log("Step 2: Detecting patterns...")

	sequenceDetector := infraPattern.NewSequenceDetector(eventStore, runStore)
	compositeDetector := infraPattern.NewCompositeDetector(sequenceDetector)

	detectionOpts := pattern.DetectionOptions{
		MinConfidence: 0.5,
		MinFrequency:  3,
	}

	patterns, err := compositeDetector.Detect(ctx, detectionOpts)
	if err != nil {
		t.Fatalf("failed to detect patterns: %v", err)
	}

	if len(patterns) == 0 {
		t.Fatal("expected at least one pattern to be detected")
	}

	// Save detected patterns
	for i := range patterns {
		if err := patternStore.Save(ctx, &patterns[i]); err != nil {
			t.Fatalf("failed to save pattern: %v", err)
		}
	}
	t.Logf("  Detected %d patterns", len(patterns))

	// === Step 3: Generate Suggestions ===
	t.Log("Step 3: Generating suggestions...")

	eligibilityGen := infraSuggestion.NewEligibilityGenerator()
	budgetGen := infraSuggestion.NewBudgetGenerator()
	compositeGen := infraSuggestion.NewCompositeGenerator(eligibilityGen, budgetGen)

	suggestions, err := compositeGen.Generate(ctx, patterns)
	if err != nil {
		t.Fatalf("failed to generate suggestions: %v", err)
	}

	// Save suggestions
	for i := range suggestions {
		if err := suggestionStore.Save(ctx, &suggestions[i]); err != nil {
			t.Fatalf("failed to save suggestion: %v", err)
		}
	}
	t.Logf("  Generated %d suggestions", len(suggestions))

	// === Step 4: Create Proposal ===
	t.Log("Step 4: Creating proposal...")

	applier := infraProposal.NewPolicyApplier()
	workflow := infraProposal.NewWorkflowService(proposalStore, versionStore, applier)

	prop, err := workflow.CreateProposal(ctx, "Add process_data to explore state", "Based on detected patterns, process_data should be eligible in explore state", "system")
	if err != nil {
		t.Fatalf("failed to create proposal: %v", err)
	}

	// Add eligibility change based on pattern detection
	eligibilityChange, err := infraProposal.CreateEligibilityChange(agent.StateExplore, "process_data", true, "Allow process_data in explore state based on usage patterns")
	if err != nil {
		t.Fatalf("failed to create eligibility change: %v", err)
	}

	if err := workflow.AddChange(ctx, prop.ID, *eligibilityChange); err != nil {
		t.Fatalf("failed to add eligibility change: %v", err)
	}

	// Add budget increase based on usage patterns
	budgetChange, err := infraProposal.CreateBudgetChange("tool_calls", 100, 150, "Increase budget based on observed patterns")
	if err != nil {
		t.Fatalf("failed to create budget change: %v", err)
	}

	if err := workflow.AddChange(ctx, prop.ID, *budgetChange); err != nil {
		t.Fatalf("failed to add budget change: %v", err)
	}

	// Add note about patterns
	if err := workflow.AddNote(ctx, prop.ID, "system", "This proposal is based on tool_sequence pattern detected across 5 runs"); err != nil {
		t.Fatalf("failed to add note: %v", err)
	}

	t.Logf("  Created proposal: %s", prop.ID)

	// Verify proposal is in draft status
	prop, _ = proposalStore.Get(ctx, prop.ID)
	if prop.Status != proposal.ProposalStatusDraft {
		t.Errorf("expected draft status, got %s", prop.Status)
	}

	// === Step 5: Submit and Approve Proposal ===
	t.Log("Step 5: Submitting and approving proposal...")

	// Submit for review
	if err := workflow.Submit(ctx, prop.ID, "developer"); err != nil {
		t.Fatalf("failed to submit proposal: %v", err)
	}

	prop, _ = proposalStore.Get(ctx, prop.ID)
	if prop.Status != proposal.ProposalStatusPendingReview {
		t.Errorf("expected pending_review status, got %s", prop.Status)
	}
	t.Log("  Proposal submitted for review")

	// Approve with human actor
	if err := workflow.Approve(ctx, prop.ID, "tech-lead", "Changes look good, patterns are convincing"); err != nil {
		t.Fatalf("failed to approve proposal: %v", err)
	}

	prop, _ = proposalStore.Get(ctx, prop.ID)
	if prop.Status != proposal.ProposalStatusApproved {
		t.Errorf("expected approved status, got %s", prop.Status)
	}
	if prop.ApprovedBy != "tech-lead" {
		t.Errorf("expected ApprovedBy=tech-lead, got %s", prop.ApprovedBy)
	}
	t.Logf("  Proposal approved by: %s", prop.ApprovedBy)

	// === Step 6: Apply Proposal ===
	t.Log("Step 6: Applying proposal...")

	if err := workflow.Apply(ctx, prop.ID); err != nil {
		t.Fatalf("failed to apply proposal: %v", err)
	}

	prop, _ = proposalStore.Get(ctx, prop.ID)
	if prop.Status != proposal.ProposalStatusApplied {
		t.Errorf("expected applied status, got %s", prop.Status)
	}
	t.Logf("  Proposal applied, policy version changed: %d -> %d", prop.PolicyVersionBefore, prop.PolicyVersionAfter)

	// Verify policy changes were applied
	currentVersion, err := versionStore.GetCurrent(ctx)
	if err != nil {
		t.Fatalf("failed to get current version: %v", err)
	}

	if currentVersion.Version != 1 {
		t.Errorf("expected policy version 1, got %d", currentVersion.Version)
	}

	// Verify eligibility change
	exploreTools := currentVersion.Eligibility.StateTools[agent.StateExplore]
	hasProcessData := false
	for _, tool := range exploreTools {
		if tool == "process_data" {
			hasProcessData = true
			break
		}
	}
	if !hasProcessData {
		t.Error("expected process_data to be eligible in explore state")
	}
	t.Log("  Verified: process_data now eligible in explore state")

	// Verify budget change
	if currentVersion.Budgets.Limits["tool_calls"] != 150 {
		t.Errorf("expected tool_calls budget=150, got %d", currentVersion.Budgets.Limits["tool_calls"])
	}
	t.Log("  Verified: tool_calls budget increased to 150")

	// === Step 7: Rollback Proposal ===
	t.Log("Step 7: Rolling back proposal...")

	if err := workflow.Rollback(ctx, prop.ID, "Reverting due to unexpected side effects"); err != nil {
		t.Fatalf("failed to rollback proposal: %v", err)
	}

	prop, _ = proposalStore.Get(ctx, prop.ID)
	if prop.Status != proposal.ProposalStatusRolledBack {
		t.Errorf("expected rolled_back status, got %s", prop.Status)
	}
	if prop.RollbackReason != "Reverting due to unexpected side effects" {
		t.Errorf("expected rollback reason to be set")
	}
	t.Logf("  Proposal rolled back: %s", prop.RollbackReason)

	// Verify rollback created new version
	rollbackVersion, err := versionStore.GetCurrent(ctx)
	if err != nil {
		t.Fatalf("failed to get rollback version: %v", err)
	}

	if rollbackVersion.Version != 2 {
		t.Errorf("expected rollback version 2, got %d", rollbackVersion.Version)
	}
	t.Logf("  Rollback created version %d", rollbackVersion.Version)

	// Verify policy reverted to initial state
	if rollbackVersion.Budgets.Limits["tool_calls"] != 100 {
		t.Errorf("expected tool_calls budget reverted to 100, got %d", rollbackVersion.Budgets.Limits["tool_calls"])
	}
	t.Log("  Verified: tool_calls budget reverted to 100")

	// === Verify Full Audit Trail ===
	t.Log("Verifying audit trail...")

	versions, _ := versionStore.List(ctx)
	if len(versions) != 3 {
		t.Errorf("expected 3 versions (initial, applied, rollback), got %d", len(versions))
	}
	t.Logf("  Total policy versions: %d", len(versions))

	// Verify notes were recorded
	prop, _ = proposalStore.Get(ctx, prop.ID)
	if len(prop.Notes) != 1 {
		t.Errorf("expected 1 note, got %d", len(prop.Notes))
	}
	t.Logf("  Proposal has %d notes", len(prop.Notes))

	t.Log("End-to-end test completed successfully!")
}

// TestHorizon3_EndToEnd_InspectorExports tests the inspector exports across the workflow.
func TestHorizon3_EndToEnd_InspectorExports(t *testing.T) {
	ctx := context.Background()

	// Setup stores
	runStore := memory.NewRunStore()
	eventStore := memory.NewEventStore()

	// Create a run with events
	r := agent.NewRun("inspector-test-run", "Test inspector exports")
	r.Status = agent.RunStatusCompleted
	r.EndTime = r.StartTime.Add(time.Minute)
	if err := runStore.Save(ctx, r); err != nil {
		t.Fatalf("failed to save run: %v", err)
	}

	// Add state transition events
	transitions := []struct {
		from, to agent.State
	}{
		{agent.StateIntake, agent.StateExplore},
		{agent.StateExplore, agent.StateDecide},
		{agent.StateDecide, agent.StateAct},
		{agent.StateAct, agent.StateValidate},
		{agent.StateValidate, agent.StateDone},
	}

	baseTime := time.Now().Add(-time.Minute)
	for i, tr := range transitions {
		payload := event.StateTransitionedPayload{
			FromState: tr.from,
			ToState:   tr.to,
		}
		evt, err := event.NewEvent(r.ID, event.TypeStateTransitioned, payload)
		if err != nil {
			t.Fatalf("failed to create transition event: %v", err)
		}
		evt.Timestamp = baseTime.Add(time.Duration(i*10) * time.Second)
		if err := eventStore.Append(ctx, evt); err != nil {
			t.Fatalf("failed to append transition event: %v", err)
		}
	}

	// Add tool call events
	toolPayload := event.ToolSucceededPayload{
		ToolName: "test_tool",
		Output:   json.RawMessage(`{}`),
		Duration: 50 * time.Millisecond,
	}
	toolEvt, err := event.NewEvent(r.ID, event.TypeToolSucceeded, toolPayload)
	if err != nil {
		t.Fatalf("failed to create tool event: %v", err)
	}
	if err := eventStore.Append(ctx, toolEvt); err != nil {
		t.Fatalf("failed to append tool event: %v", err)
	}

	// Create exporters
	runExporter := infraInspector.NewRunExporter(runStore, eventStore)
	stateMachineExporter := infraInspector.NewStateMachineExporter(nil, nil)
	metricsExporter := infraInspector.NewMetricsExporter(runStore, eventStore)

	// Create inspector (formatters are registered automatically)
	insp := infraInspector.NewDefaultInspector(runExporter, stateMachineExporter, metricsExporter)

	// Test JSON run export
	t.Log("Testing JSON run export...")
	jsonExport, err := insp.ExportRun(ctx, r.ID, inspector.FormatJSON)
	if err != nil {
		t.Fatalf("failed to export run as JSON: %v", err)
	}
	if len(jsonExport) == 0 {
		t.Error("JSON export should not be empty")
	}
	t.Logf("  JSON export size: %d bytes", len(jsonExport))

	// Verify JSON is valid
	var runData map[string]interface{}
	if err := json.Unmarshal(jsonExport, &runData); err != nil {
		t.Errorf("JSON export is invalid: %v", err)
	}

	// Test DOT state machine export
	t.Log("Testing DOT state machine export...")
	dotExport, err := insp.ExportStateMachine(ctx, inspector.FormatDOT)
	if err != nil {
		t.Fatalf("failed to export state machine as DOT: %v", err)
	}
	if len(dotExport) == 0 {
		t.Error("DOT export should not be empty")
	}
	t.Logf("  DOT export size: %d bytes", len(dotExport))

	// Verify DOT format
	dotStr := string(dotExport)
	if !contains(dotStr, "digraph") {
		t.Error("DOT export should contain 'digraph'")
	}
	if !contains(dotStr, "intake") {
		t.Error("DOT export should contain 'intake' state")
	}

	// Test Mermaid state machine export
	t.Log("Testing Mermaid state machine export...")
	mermaidExport, err := insp.ExportStateMachine(ctx, inspector.FormatMermaid)
	if err != nil {
		t.Fatalf("failed to export state machine as Mermaid: %v", err)
	}
	if len(mermaidExport) == 0 {
		t.Error("Mermaid export should not be empty")
	}
	t.Logf("  Mermaid export size: %d bytes", len(mermaidExport))

	// Verify Mermaid format
	mermaidStr := string(mermaidExport)
	if !contains(mermaidStr, "stateDiagram-v2") {
		t.Error("Mermaid export should contain 'stateDiagram-v2'")
	}

	// Test metrics export
	t.Log("Testing metrics export...")
	metricsFilter := inspector.MetricsFilter{
		IncludeToolMetrics:  true,
		IncludeStateMetrics: true,
	}
	metricsExportData, err := insp.ExportMetrics(ctx, metricsFilter, inspector.FormatJSON)
	if err != nil {
		t.Fatalf("failed to export metrics: %v", err)
	}
	if len(metricsExportData) == 0 {
		t.Error("Metrics export should not be empty")
	}
	t.Logf("  Metrics export size: %d bytes", len(metricsExportData))

	t.Log("Inspector exports test completed successfully!")
}

// TestHorizon3_EndToEnd_PatternToProposal tests pattern detection leading to proposal creation.
func TestHorizon3_EndToEnd_PatternToProposal(t *testing.T) {
	ctx := context.Background()

	// Setup stores
	runStore := memory.NewRunStore()
	eventStore := memory.NewEventStore()
	proposalStore := memory.NewProposalStore()
	versionStore := memory.NewPolicyVersionStore()

	// Create initial policy
	initialPolicy := &policy.PolicyVersion{
		Version:     0,
		Eligibility: policy.NewEligibilitySnapshot(),
		Transitions: policy.NewTransitionSnapshot(),
		Budgets:     policy.NewBudgetLimitsSnapshot(),
		Approvals:   policy.NewApprovalSnapshot(),
	}
	initialPolicy.Budgets.SetLimit("tool_calls", 50)
	versionStore.Save(ctx, initialPolicy)

	// Create runs with a failure pattern - tool consistently failing
	t.Log("Creating runs with failure pattern...")
	for i := 0; i < 5; i++ {
		r := agent.NewRun(fmt.Sprintf("failure-run-%d", i), "Test failure pattern")
		r.Status = agent.RunStatusCompleted
		runStore.Save(ctx, r)

		// Add tool failure event
		failPayload := event.ToolFailedPayload{
			ToolName: "flaky_api_call",
			Error:    "connection timeout",
			Duration: 30 * time.Second,
		}
		failEvt, err := event.NewEvent(r.ID, event.TypeToolFailed, failPayload)
		if err != nil {
			t.Fatalf("failed to create event: %v", err)
		}
		eventStore.Append(ctx, failEvt)
	}

	// Detect failure patterns
	t.Log("Detecting failure patterns...")
	failureDetector := infraPattern.NewFailureDetector(eventStore, runStore)
	patterns, err := failureDetector.Detect(ctx, pattern.DetectionOptions{
		MinConfidence: 0.5,
		MinFrequency:  3,
	})
	if err != nil {
		t.Fatalf("failed to detect patterns: %v", err)
	}

	if len(patterns) == 0 {
		t.Fatal("expected failure patterns to be detected")
	}
	t.Logf("  Detected %d failure patterns", len(patterns))

	// Verify pattern type
	foundFailurePattern := false
	for _, p := range patterns {
		if p.Type == pattern.PatternTypeToolFailure {
			foundFailurePattern = true
			t.Logf("  Found tool failure pattern: %s (confidence: %.2f)", p.Name, p.Confidence)
		}
	}
	if !foundFailurePattern {
		t.Error("expected to find a tool_failure pattern")
	}

	// Create proposal based on pattern - require approval for flaky tool
	t.Log("Creating proposal based on pattern...")
	applier := infraProposal.NewPolicyApplier()
	workflow := infraProposal.NewWorkflowService(proposalStore, versionStore, applier)

	prop, _ := workflow.CreateProposal(ctx, "Require approval for flaky_api_call", "Tool has consistent failures, require human approval", "pattern-analyzer")

	approvalChange, _ := infraProposal.CreateApprovalChange("flaky_api_call", true, "Require approval due to high failure rate")
	workflow.AddChange(ctx, prop.ID, *approvalChange)

	// Link to pattern
	workflow.AddNote(ctx, prop.ID, "system", "Based on tool_failure pattern detected with 100% failure rate across 5 runs")

	// Complete workflow
	workflow.Submit(ctx, prop.ID, "developer")
	workflow.Approve(ctx, prop.ID, "sre-lead", "Good catch, this tool needs human oversight")
	workflow.Apply(ctx, prop.ID)

	// Verify approval requirement was added
	currentPolicy, _ := versionStore.GetCurrent(ctx)
	hasApproval := false
	for _, tool := range currentPolicy.Approvals.RequiredTools {
		if tool == "flaky_api_call" {
			hasApproval = true
			break
		}
	}
	if !hasApproval {
		t.Error("expected flaky_api_call to require approval")
	}
	t.Log("  Verified: flaky_api_call now requires approval")

	t.Log("Pattern to proposal test completed successfully!")
}

// TestHorizon3_EndToEnd_MultipleProposals tests handling multiple proposals.
func TestHorizon3_EndToEnd_MultipleProposals(t *testing.T) {
	ctx := context.Background()

	// Setup
	proposalStore := memory.NewProposalStore()
	versionStore := memory.NewPolicyVersionStore()
	applier := infraProposal.NewPolicyApplier()
	workflow := infraProposal.NewWorkflowService(proposalStore, versionStore, applier)

	// Initial policy
	versionStore.Save(ctx, &policy.PolicyVersion{
		Version:     0,
		Eligibility: policy.NewEligibilitySnapshot(),
		Transitions: policy.NewTransitionSnapshot(),
		Budgets:     policy.NewBudgetLimitsSnapshot(),
		Approvals:   policy.NewApprovalSnapshot(),
	})

	// Create and process multiple proposals
	t.Log("Creating multiple proposals...")

	proposals := []struct {
		title       string
		budgetName  string
		oldVal      int
		newVal      int
		shouldApply bool
	}{
		{"Increase tool_calls budget", "tool_calls", 0, 100, true},
		{"Increase api_calls budget", "api_calls", 0, 50, true},
		{"Reduce tool_calls budget", "tool_calls", 100, 75, false}, // Will be rejected
	}

	for i, pDef := range proposals {
		prop, _ := workflow.CreateProposal(ctx, pDef.title, "Test proposal", "creator")
		change, _ := infraProposal.CreateBudgetChange(pDef.budgetName, pDef.oldVal, pDef.newVal, pDef.title)
		workflow.AddChange(ctx, prop.ID, *change)
		workflow.Submit(ctx, prop.ID, "submitter")

		if pDef.shouldApply {
			workflow.Approve(ctx, prop.ID, "approver", "approved")
			if err := workflow.Apply(ctx, prop.ID); err != nil {
				t.Fatalf("failed to apply proposal %d: %v", i, err)
			}
			t.Logf("  Applied proposal %d: %s", i+1, pDef.title)
		} else {
			workflow.Reject(ctx, prop.ID, "rejector", "Budget reduction not approved")
			t.Logf("  Rejected proposal %d: %s", i+1, pDef.title)
		}
	}

	// Verify final state
	currentPolicy, _ := versionStore.GetCurrent(ctx)
	t.Logf("Final policy version: %d", currentPolicy.Version)

	if currentPolicy.Budgets.Limits["tool_calls"] != 100 {
		t.Errorf("expected tool_calls=100, got %d", currentPolicy.Budgets.Limits["tool_calls"])
	}
	if currentPolicy.Budgets.Limits["api_calls"] != 50 {
		t.Errorf("expected api_calls=50, got %d", currentPolicy.Budgets.Limits["api_calls"])
	}

	// List all proposals
	allProposals, _ := proposalStore.List(ctx, proposal.ListFilter{})
	t.Logf("Total proposals: %d", len(allProposals))

	applied := 0
	rejected := 0
	for _, p := range allProposals {
		switch p.Status {
		case proposal.ProposalStatusApplied:
			applied++
		case proposal.ProposalStatusRejected:
			rejected++
		}
	}
	t.Logf("  Applied: %d, Rejected: %d", applied, rejected)

	if applied != 2 {
		t.Errorf("expected 2 applied proposals, got %d", applied)
	}
	if rejected != 1 {
		t.Errorf("expected 1 rejected proposal, got %d", rejected)
	}

	t.Log("Multiple proposals test completed successfully!")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
