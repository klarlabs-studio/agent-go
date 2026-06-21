package plannerllm

import (
	"fmt"
	"strings"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/infrastructure/planner"
)

const maxEvidenceEntries = 10

// BuildMessages constructs the message sequence for the LLM provider.
func BuildMessages(systemPrompt string, req planner.PlanRequest) []Message {
	msgs := make([]Message, 0, 2)
	msgs = append(msgs, Message{
		Role:    "system",
		Content: systemPrompt,
	})
	msgs = append(msgs, Message{
		Role:    "user",
		Content: buildUserMessage(req),
	})
	return msgs
}

func buildUserMessage(req planner.PlanRequest) string {
	var b strings.Builder

	// Goal
	b.WriteString("## Goal\n")
	if req.Goal != "" {
		b.WriteString(req.Goal)
	} else {
		b.WriteString("(no goal specified)")
	}
	b.WriteString("\n\n")

	// Current state
	b.WriteString("## Current State\n")
	b.WriteString(string(req.CurrentState))
	b.WriteString("\n\n")

	// Allowed transitions: the only valid next states. The model must transition
	// to one of these (never to another state, and never to the current one), so
	// it does not propose a transition the state machine cannot perform.
	if len(req.AllowedTransitions) > 0 {
		b.WriteString("## Allowed Transitions\n")
		b.WriteString("If you choose to transition, the `to_state` MUST be one of:\n")
		for _, s := range req.AllowedTransitions {
			fmt.Fprintf(&b, "- %s\n", s)
		}
		b.WriteString("Do not transition to any other state, and do not transition to the current state.\n\n")
	}

	// Available tools
	if len(req.ToolDefs) > 0 {
		b.WriteString("## Available Tools\n")
		b.WriteString(formatToolList(req.ToolDefs))
		b.WriteString("\n")
	} else if len(req.AllowedTools) > 0 {
		b.WriteString("## Available Tools\n")
		for _, name := range req.AllowedTools {
			fmt.Fprintf(&b, "- %s\n", name)
		}
		b.WriteString("\n")
	}

	// Evidence
	if len(req.Evidence) > 0 {
		b.WriteString("## Evidence So Far\n")
		b.WriteString(formatEvidence(req.Evidence, maxEvidenceEntries))
		b.WriteString("\n")
	}

	// Budgets
	if len(req.Budgets.Remaining) > 0 {
		b.WriteString("## Budget Remaining\n")
		b.WriteString(formatBudgets(req.Budgets))
		b.WriteString("\n")
	}

	// One-shot feedback from the engine about the previous step (e.g. a rejected
	// transition). Placed last so it is the most recent context the model reads.
	if req.Feedback != "" {
		b.WriteString("## Important Feedback\n")
		b.WriteString(req.Feedback)
		b.WriteString("\n\n")
	}

	b.WriteString("What is your next decision? Respond with a JSON object.")
	return b.String()
}

func formatEvidence(evidence []agent.Evidence, maxEntries int) string {
	var b strings.Builder
	start := 0
	if len(evidence) > maxEntries {
		start = len(evidence) - maxEntries
		fmt.Fprintf(&b, "(%d earlier entries omitted)\n", start)
	}
	for i := start; i < len(evidence); i++ {
		e := evidence[i]
		idx := i - start + 1
		content := truncateContent(string(e.Content), 500)
		switch e.Type {
		case agent.EvidenceToolResult:
			fmt.Fprintf(&b, "[%d] tool_result from %q: %s\n", idx, e.Source, content)
		case agent.EvidenceHumanInput:
			fmt.Fprintf(&b, "[%d] human_input: %s\n", idx, content)
		default:
			fmt.Fprintf(&b, "[%d] %s from %q: %s\n", idx, e.Type, e.Source, content)
		}
	}
	return b.String()
}

func formatToolList(toolDefs []planner.ToolDef) string {
	var b strings.Builder
	for _, td := range toolDefs {
		fmt.Fprintf(&b, "- %s: %s\n", td.Name, td.Description)
		if len(td.InputSchema) > 0 && string(td.InputSchema) != "{}" && string(td.InputSchema) != "null" {
			fmt.Fprintf(&b, "  Parameters: %s\n", string(td.InputSchema))
		}
	}
	return b.String()
}

func formatBudgets(budgets policy.BudgetSnapshot) string {
	var b strings.Builder
	for name, remaining := range budgets.Remaining {
		limit := budgets.Limits[name]
		fmt.Fprintf(&b, "%s: %d/%d remaining\n", name, remaining, limit)
	}
	return b.String()
}

func truncateContent(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
