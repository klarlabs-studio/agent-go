package plannerllm

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/infrastructure/planner"
)

// --- Mock Provider ---

type mockProvider struct {
	name     string
	response CompletionResponse
	err      error
	lastReq  CompletionRequest // captured for assertions
}

func (m *mockProvider) Complete(_ context.Context, req CompletionRequest) (CompletionResponse, error) {
	m.lastReq = req
	return m.response, m.err
}

func (m *mockProvider) Name() string {
	if m.name != "" {
		return m.name
	}
	return "mock"
}

// --- ParseDecisionJSON Tests ---

func TestParseDecisionJSON_CallTool(t *testing.T) {
	t.Parallel()
	input := `{"decision": "call_tool", "tool_name": "read_file", "input": {"path": "/tmp/test"}, "reason": "need to read"}`
	d, err := ParseDecisionJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Type != agent.DecisionCallTool {
		t.Errorf("expected call_tool, got %s", d.Type)
	}
	if d.CallTool.ToolName != "read_file" {
		t.Errorf("expected read_file, got %s", d.CallTool.ToolName)
	}
	if d.CallTool.Reason != "need to read" {
		t.Errorf("expected reason 'need to read', got %s", d.CallTool.Reason)
	}

	var inputMap map[string]string
	if err := json.Unmarshal(d.CallTool.Input, &inputMap); err != nil {
		t.Fatalf("failed to unmarshal input: %v", err)
	}
	if inputMap["path"] != "/tmp/test" {
		t.Errorf("expected path /tmp/test, got %s", inputMap["path"])
	}
}

func TestParseDecisionJSON_Transition(t *testing.T) {
	t.Parallel()
	input := `{"decision": "transition", "to_state": "explore", "reason": "need info"}`
	d, err := ParseDecisionJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Type != agent.DecisionTransition {
		t.Errorf("expected transition, got %s", d.Type)
	}
	if d.Transition.ToState != agent.StateExplore {
		t.Errorf("expected explore, got %s", d.Transition.ToState)
	}
}

func TestParseDecisionJSON_Finish(t *testing.T) {
	t.Parallel()
	input := `{"decision": "finish", "result": {"answer": 42}, "summary": "found the answer"}`
	d, err := ParseDecisionJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Type != agent.DecisionFinish {
		t.Errorf("expected finish, got %s", d.Type)
	}
	if d.Finish.Summary != "found the answer" {
		t.Errorf("expected summary 'found the answer', got %s", d.Finish.Summary)
	}
	if string(d.Finish.Result) != `{"answer": 42}` {
		t.Errorf("unexpected result: %s", d.Finish.Result)
	}
}

func TestParseDecisionJSON_Fail(t *testing.T) {
	t.Parallel()
	input := `{"decision": "fail", "reason": "cannot proceed"}`
	d, err := ParseDecisionJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Type != agent.DecisionFail {
		t.Errorf("expected fail, got %s", d.Type)
	}
	if d.Fail.Reason != "cannot proceed" {
		t.Errorf("expected reason 'cannot proceed', got %s", d.Fail.Reason)
	}
}

func TestParseDecisionJSON_AskHuman(t *testing.T) {
	t.Parallel()
	input := `{"decision": "ask_human", "question": "Which option?", "options": ["A", "B"]}`
	d, err := ParseDecisionJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Type != agent.DecisionAskHuman {
		t.Errorf("expected ask_human, got %s", d.Type)
	}
	if d.AskHuman.Question != "Which option?" {
		t.Errorf("expected question 'Which option?', got %s", d.AskHuman.Question)
	}
	if len(d.AskHuman.Options) != 2 || d.AskHuman.Options[0] != "A" || d.AskHuman.Options[1] != "B" {
		t.Errorf("expected options [A, B], got %v", d.AskHuman.Options)
	}
}

func TestParseDecisionJSON_MarkdownCodeFence(t *testing.T) {
	t.Parallel()
	input := "Here is my decision:\n```json\n{\"decision\": \"finish\", \"summary\": \"done\"}\n```"
	d, err := ParseDecisionJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Type != agent.DecisionFinish {
		t.Errorf("expected finish, got %s", d.Type)
	}
	if d.Finish.Summary != "done" {
		t.Errorf("expected summary 'done', got %s", d.Finish.Summary)
	}
}

func TestParseDecisionJSON_LeadingText(t *testing.T) {
	t.Parallel()
	input := `I think the best action is: {"decision": "fail", "reason": "blocked"}`
	d, err := ParseDecisionJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Type != agent.DecisionFail {
		t.Errorf("expected fail, got %s", d.Type)
	}
}

func TestParseDecisionJSON_EmptyResponse(t *testing.T) {
	t.Parallel()
	_, err := ParseDecisionJSON("")
	if !errors.Is(err, ErrEmptyResponse) {
		t.Errorf("expected ErrEmptyResponse, got %v", err)
	}
}

func TestParseDecisionJSON_InvalidJSON(t *testing.T) {
	t.Parallel()
	_, err := ParseDecisionJSON("not json at all")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseDecisionJSON_UnknownDecisionType(t *testing.T) {
	t.Parallel()
	_, err := ParseDecisionJSON(`{"decision": "teleport", "reason": "magic"}`)
	if !errors.Is(err, ErrUnknownDecision) {
		t.Errorf("expected ErrUnknownDecision, got %v", err)
	}
}

func TestParseDecisionJSON_MissingToolName(t *testing.T) {
	t.Parallel()
	_, err := ParseDecisionJSON(`{"decision": "call_tool", "input": {}}`)
	if !errors.Is(err, ErrMissingToolName) {
		t.Errorf("expected ErrMissingToolName, got %v", err)
	}
}

func TestParseDecisionJSON_InvalidState(t *testing.T) {
	t.Parallel()
	_, err := ParseDecisionJSON(`{"decision": "transition", "to_state": "flying", "reason": "up"}`)
	if !errors.Is(err, ErrInvalidState) {
		t.Errorf("expected ErrInvalidState, got %v", err)
	}
}

func TestParseDecisionJSON_MissingReason(t *testing.T) {
	t.Parallel()
	_, err := ParseDecisionJSON(`{"decision": "fail"}`)
	if !errors.Is(err, ErrMissingReason) {
		t.Errorf("expected ErrMissingReason, got %v", err)
	}
}

func TestParseDecisionJSON_CallToolNoInput(t *testing.T) {
	t.Parallel()
	d, err := ParseDecisionJSON(`{"decision": "call_tool", "tool_name": "list_files", "reason": "explore"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(d.CallTool.Input) != "{}" {
		t.Errorf("expected empty object for missing input, got %s", d.CallTool.Input)
	}
}

func TestParseDecisionJSON_FinishUsesReasonAsSummary(t *testing.T) {
	t.Parallel()
	d, err := ParseDecisionJSON(`{"decision": "finish", "reason": "all done"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Finish.Summary != "all done" {
		t.Errorf("expected summary to fall back to reason, got %s", d.Finish.Summary)
	}
}

// --- ParseToolCalls Tests ---

func TestParseToolCalls_SingleCall(t *testing.T) {
	t.Parallel()
	calls := []ToolCall{
		{
			ID:   "call_123",
			Type: "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      "read_file",
				Arguments: `{"path": "/tmp/foo"}`,
			},
		},
	}
	d, err := ParseToolCalls(calls)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Type != agent.DecisionCallTool {
		t.Errorf("expected call_tool, got %s", d.Type)
	}
	if d.CallTool.ToolName != "read_file" {
		t.Errorf("expected read_file, got %s", d.CallTool.ToolName)
	}
}

func TestParseToolCalls_InvalidArguments(t *testing.T) {
	t.Parallel()
	calls := []ToolCall{
		{
			ID:   "call_456",
			Type: "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      "bad_tool",
				Arguments: "not valid json",
			},
		},
	}
	_, err := ParseToolCalls(calls)
	if !errors.Is(err, ErrInvalidToolArgs) {
		t.Errorf("expected ErrInvalidToolArgs, got %v", err)
	}
}

func TestParseToolCalls_EmptyArguments(t *testing.T) {
	t.Parallel()
	calls := []ToolCall{
		{
			ID:   "call_789",
			Type: "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      "no_args_tool",
				Arguments: "",
			},
		},
	}
	d, err := ParseToolCalls(calls)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(d.CallTool.Input) != "{}" {
		t.Errorf("expected empty object for empty args, got %s", d.CallTool.Input)
	}
}

func TestParseToolCalls_NoCalls(t *testing.T) {
	t.Parallel()
	_, err := ParseToolCalls(nil)
	if !errors.Is(err, ErrNoToolCalls) {
		t.Errorf("expected ErrNoToolCalls, got %v", err)
	}
}

func TestParseToolCalls_MultipleCalls_UsesFirst(t *testing.T) {
	t.Parallel()
	calls := []ToolCall{
		{
			ID:   "call_1",
			Type: "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{Name: "first_tool", Arguments: `{}`},
		},
		{
			ID:   "call_2",
			Type: "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{Name: "second_tool", Arguments: `{}`},
		},
	}
	d, err := ParseToolCalls(calls)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.CallTool.ToolName != "first_tool" {
		t.Errorf("expected first_tool, got %s", d.CallTool.ToolName)
	}
}

// --- BuildMessages Tests ---

func TestBuildMessages_FullRequest(t *testing.T) {
	t.Parallel()

	budget := policy.NewBudget(map[string]int{"tool_calls": 100})
	_ = budget.Consume("tool_calls", 5)

	req := planner.PlanRequest{
		RunID:        "run-123",
		Goal:         "Find all security vulnerabilities",
		CurrentState: agent.StateExplore,
		Evidence: []agent.Evidence{
			agent.NewToolEvidence("scan_file", json.RawMessage(`{"findings": 3}`)),
		},
		AllowedTools: []string{"read_file", "scan_file"},
		ToolDefs: []planner.ToolDef{
			{Name: "read_file", Description: "Read a file", InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`)},
			{Name: "scan_file", Description: "Scan for issues", InputSchema: json.RawMessage(`{}`)},
		},
		Budgets: budget.Snapshot(),
		Vars:    map[string]any{"target": "/src"},
	}

	msgs := BuildMessages("You are a test agent.", req)

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	// System message
	if msgs[0].Role != "system" {
		t.Errorf("expected system role, got %s", msgs[0].Role)
	}
	if msgs[0].Content != "You are a test agent." {
		t.Errorf("unexpected system prompt: %s", msgs[0].Content)
	}

	// User message should contain all sections
	user := msgs[1].Content
	assertContains(t, user, "## Goal")
	assertContains(t, user, "Find all security vulnerabilities")
	assertContains(t, user, "## Current State")
	assertContains(t, user, "explore")
	assertContains(t, user, "## Available Tools")
	assertContains(t, user, "read_file: Read a file")
	assertContains(t, user, "scan_file: Scan for issues")
	assertContains(t, user, "## Evidence So Far")
	assertContains(t, user, "tool_result")
	assertContains(t, user, "## Budget Remaining")
	assertContains(t, user, "tool_calls")
	assertContains(t, user, "What is your next decision?")
}

func TestBuildMessages_MinimalRequest(t *testing.T) {
	t.Parallel()

	req := planner.PlanRequest{
		CurrentState: agent.StateIntake,
	}

	msgs := BuildMessages(DefaultSystemPrompt, req)

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	user := msgs[1].Content
	assertContains(t, user, "## Goal")
	assertContains(t, user, "(no goal specified)")
	assertContains(t, user, "## Current State")
	assertContains(t, user, "intake")
	assertNotContains(t, user, "## Available Tools")
	assertNotContains(t, user, "## Evidence So Far")
	assertNotContains(t, user, "## Budget Remaining")
}

func TestBuildMessages_ToolDefsOnly(t *testing.T) {
	t.Parallel()

	req := planner.PlanRequest{
		CurrentState: agent.StateExplore,
		AllowedTools: []string{"read_file"},
		// No ToolDefs — should fall back to names only
	}

	msgs := BuildMessages("sys", req)
	user := msgs[1].Content
	assertContains(t, user, "## Available Tools")
	assertContains(t, user, "- read_file")
}

func TestBuildMessages_EvidenceTruncation(t *testing.T) {
	t.Parallel()

	evidence := make([]agent.Evidence, 15)
	for i := range evidence {
		evidence[i] = agent.NewToolEvidence("tool", json.RawMessage(`{"i": `+string(rune('0'+i%10))+`}`))
	}

	req := planner.PlanRequest{
		CurrentState: agent.StateExplore,
		Evidence:     evidence,
	}

	msgs := BuildMessages("sys", req)
	user := msgs[1].Content
	assertContains(t, user, "earlier entries omitted")
}

// --- LLMPlanner.Plan Tests ---

func TestLLMPlanner_Plan_TextResponse(t *testing.T) {
	t.Parallel()

	provider := &mockProvider{
		response: CompletionResponse{
			ID:    "resp-1",
			Model: "test-model",
			Message: Message{
				Role:    "assistant",
				Content: `{"decision": "transition", "to_state": "explore", "reason": "gathering info"}`,
			},
		},
	}

	p := NewPlanner(Config{
		Provider: provider,
		Model:    "test-model",
	})

	req := planner.PlanRequest{
		Goal:         "test goal",
		CurrentState: agent.StateIntake,
	}

	d, err := p.Plan(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Type != agent.DecisionTransition {
		t.Errorf("expected transition, got %s", d.Type)
	}
	if d.Transition.ToState != agent.StateExplore {
		t.Errorf("expected explore, got %s", d.Transition.ToState)
	}
}

func TestLLMPlanner_Plan_ToolCallResponse(t *testing.T) {
	t.Parallel()

	provider := &mockProvider{
		response: CompletionResponse{
			ID:    "resp-2",
			Model: "test-model",
			Message: Message{
				Role: "assistant",
				ToolCalls: []ToolCall{
					{
						ID:   "tc-1",
						Type: "function",
						Function: struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						}{
							Name:      "read_file",
							Arguments: `{"path": "/tmp/test.txt"}`,
						},
					},
				},
			},
		},
	}

	p := NewPlanner(Config{
		Provider: provider,
		Model:    "test-model",
	})

	req := planner.PlanRequest{
		Goal:         "read a file",
		CurrentState: agent.StateExplore,
		ToolDefs: []planner.ToolDef{
			{Name: "read_file", Description: "Read a file", InputSchema: json.RawMessage(`{"type":"object"}`)},
		},
	}

	d, err := p.Plan(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if d.Type != agent.DecisionCallTool {
		t.Errorf("expected call_tool, got %s", d.Type)
	}
	if d.CallTool.ToolName != "read_file" {
		t.Errorf("expected read_file, got %s", d.CallTool.ToolName)
	}
}

func TestLLMPlanner_Plan_ProviderError(t *testing.T) {
	t.Parallel()

	provider := &mockProvider{
		err: errors.New("API rate limit exceeded"),
	}

	p := NewPlanner(Config{
		Provider: provider,
		Model:    "test-model",
	})

	req := planner.PlanRequest{
		Goal:         "test goal",
		CurrentState: agent.StateIntake,
	}

	_, err := p.Plan(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from provider")
	}
	assertContains(t, err.Error(), "LLM completion failed")
	assertContains(t, err.Error(), "API rate limit exceeded")
}

func TestLLMPlanner_Plan_ToolsSentInRequest(t *testing.T) {
	t.Parallel()

	provider := &mockProvider{
		response: CompletionResponse{
			Message: Message{
				Role:    "assistant",
				Content: `{"decision": "finish", "summary": "done"}`,
			},
		},
	}

	p := NewPlanner(Config{
		Provider: provider,
		Model:    "test-model",
	})

	req := planner.PlanRequest{
		Goal:         "test",
		CurrentState: agent.StateDecide,
		ToolDefs: []planner.ToolDef{
			{Name: "tool_a", Description: "Tool A", InputSchema: json.RawMessage(`{"type":"object"}`)},
			{Name: "tool_b", Description: "Tool B", InputSchema: json.RawMessage(`{}`)},
		},
	}

	_, err := p.Plan(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify tools were passed to the provider
	if len(provider.lastReq.Tools) != 2 {
		t.Fatalf("expected 2 tools in request, got %d", len(provider.lastReq.Tools))
	}
	if provider.lastReq.Tools[0].Function.Name != "tool_a" {
		t.Errorf("expected tool_a, got %s", provider.lastReq.Tools[0].Function.Name)
	}
	if provider.lastReq.Tools[1].Function.Name != "tool_b" {
		t.Errorf("expected tool_b, got %s", provider.lastReq.Tools[1].Function.Name)
	}
}

func TestLLMPlanner_Plan_NoToolDefsNoToolsInRequest(t *testing.T) {
	t.Parallel()

	provider := &mockProvider{
		response: CompletionResponse{
			Message: Message{
				Role:    "assistant",
				Content: `{"decision": "finish", "summary": "done"}`,
			},
		},
	}

	p := NewPlanner(Config{
		Provider: provider,
		Model:    "test-model",
	})

	req := planner.PlanRequest{
		CurrentState: agent.StateDecide,
	}

	_, err := p.Plan(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(provider.lastReq.Tools) != 0 {
		t.Errorf("expected no tools, got %d", len(provider.lastReq.Tools))
	}
}

func TestLLMPlanner_Plan_ToolCallPriorityOverText(t *testing.T) {
	t.Parallel()

	// Provider returns both text content and tool calls — tool calls should win
	provider := &mockProvider{
		response: CompletionResponse{
			Message: Message{
				Role:    "assistant",
				Content: `{"decision": "fail", "reason": "this should be ignored"}`,
				ToolCalls: []ToolCall{
					{
						ID:   "tc-1",
						Type: "function",
						Function: struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						}{
							Name:      "write_file",
							Arguments: `{"path": "/out"}`,
						},
					},
				},
			},
		},
	}

	p := NewPlanner(Config{Provider: provider, Model: "m"})

	d, err := p.Plan(context.Background(), planner.PlanRequest{CurrentState: agent.StateAct})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should use tool call, not the text JSON
	if d.Type != agent.DecisionCallTool {
		t.Errorf("expected call_tool (from tool calls), got %s", d.Type)
	}
	if d.CallTool.ToolName != "write_file" {
		t.Errorf("expected write_file, got %s", d.CallTool.ToolName)
	}
}

func TestLLMPlanner_Plan_ConfigDefaults(t *testing.T) {
	t.Parallel()

	provider := &mockProvider{
		response: CompletionResponse{
			Message: Message{
				Role:    "assistant",
				Content: `{"decision": "finish", "summary": "ok"}`,
			},
		},
	}

	p := NewPlanner(Config{Provider: provider})

	_, err := p.Plan(context.Background(), planner.PlanRequest{CurrentState: agent.StateDecide})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check defaults were applied
	if provider.lastReq.Temperature != 0.7 {
		t.Errorf("expected default temperature 0.7, got %f", provider.lastReq.Temperature)
	}
	if provider.lastReq.MaxTokens != 4096 {
		t.Errorf("expected default max tokens 4096, got %d", provider.lastReq.MaxTokens)
	}
}

// --- extractJSON Tests ---

func TestExtractJSON_DirectJSON(t *testing.T) {
	t.Parallel()
	result, err := extractJSON(`{"key": "value"}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != `{"key": "value"}` {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestExtractJSON_MarkdownFence(t *testing.T) {
	t.Parallel()
	input := "```json\n{\"key\": \"value\"}\n```"
	result, err := extractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != `{"key": "value"}` {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestExtractJSON_PlainFence(t *testing.T) {
	t.Parallel()
	input := "```\n{\"key\": \"value\"}\n```"
	result, err := extractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != `{"key": "value"}` {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestExtractJSON_EmbeddedInText(t *testing.T) {
	t.Parallel()
	input := `Based on my analysis, {"decision": "finish", "summary": "done"} is the answer.`
	result, err := extractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != `{"decision": "finish", "summary": "done"}` {
		t.Errorf("unexpected result: %s", result)
	}
}

func TestExtractJSON_NoJSON(t *testing.T) {
	t.Parallel()
	_, err := extractJSON("no json here at all")
	if !errors.Is(err, ErrNoJSON) {
		t.Errorf("expected ErrNoJSON, got %v", err)
	}
}

func TestExtractJSON_NestedBraces(t *testing.T) {
	t.Parallel()
	input := `Here: {"outer": {"inner": "value"}, "key": "val"}`
	result, err := extractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal([]byte(result), &m); err != nil {
		t.Fatalf("result is not valid JSON: %v", err)
	}
	if _, ok := m["outer"]; !ok {
		t.Error("expected 'outer' key in result")
	}
}

func TestExtractJSON_StringWithBraces(t *testing.T) {
	t.Parallel()
	// JSON containing braces inside string values
	input := `{"message": "use {name} as placeholder"}`
	result, err := extractJSON(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != input {
		t.Errorf("unexpected result: %s", result)
	}
}

// --- formatEvidence Tests ---

func TestFormatEvidence_WithinLimit(t *testing.T) {
	t.Parallel()
	evidence := []agent.Evidence{
		agent.NewToolEvidence("scan", json.RawMessage(`{"count": 5}`)),
		agent.NewHumanEvidence(json.RawMessage(`{"answer": "yes"}`)),
	}
	result := formatEvidence(evidence, 10)
	assertContains(t, result, `[1] tool_result from "scan"`)
	assertContains(t, result, `[2] human_input`)
	assertNotContains(t, result, "omitted")
}

func TestFormatEvidence_ExceedsLimit(t *testing.T) {
	t.Parallel()
	evidence := make([]agent.Evidence, 5)
	for i := range evidence {
		evidence[i] = agent.NewToolEvidence("tool", json.RawMessage(`{}`))
	}
	result := formatEvidence(evidence, 3)
	assertContains(t, result, "2 earlier entries omitted")
	assertContains(t, result, "[1]")
	assertContains(t, result, "[3]")
}

// --- Planner interface compliance ---

func TestLLMPlanner_ImplementsPlannerInterface(t *testing.T) {
	t.Parallel()
	// Compile-time check via var _ above; this test documents the fact
	var p planner.Planner = NewPlanner(Config{Provider: &mockProvider{}})
	if p == nil {
		t.Error("LLMPlanner should implement planner.Planner")
	}
}

// --- helpers ---

func assertContains(t *testing.T, s, substr string) {
	t.Helper()
	if len(s) == 0 || len(substr) == 0 {
		t.Errorf("assertContains: empty string or substring")
		return
	}
	found := false
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected %q to contain %q", truncateForTest(s, 200), substr)
	}
}

func assertNotContains(t *testing.T, s, substr string) {
	t.Helper()
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			t.Errorf("expected %q to NOT contain %q", truncateForTest(s, 200), substr)
			return
		}
	}
}

func truncateForTest(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
