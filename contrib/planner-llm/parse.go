package plannerllm

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/felixgeelhaar/agent-go/domain/agent"
)

var (
	ErrEmptyResponse     = errors.New("empty LLM response")
	ErrNoJSON            = errors.New("no JSON object found in response")
	ErrUnknownDecision   = errors.New("unknown decision type")
	ErrMissingToolName   = errors.New("call_tool decision requires tool_name")
	ErrMissingReason     = errors.New("fail decision requires reason")
	ErrInvalidState      = errors.New("invalid state in transition decision")
	ErrNoToolCalls       = errors.New("no tool calls in response")
	ErrInvalidToolArgs   = errors.New("invalid tool call arguments")
)

// decisionEnvelope is the intermediate JSON structure for parsing LLM decisions.
type decisionEnvelope struct {
	Decision string          `json:"decision"`
	ToolName string          `json:"tool_name,omitempty"`
	Input    json.RawMessage `json:"input,omitempty"`
	Reason   string          `json:"reason,omitempty"`
	ToState  string          `json:"to_state,omitempty"`
	Result   json.RawMessage `json:"result,omitempty"`
	Summary  string          `json:"summary,omitempty"`
	Question string          `json:"question,omitempty"`
	Options  []string        `json:"options,omitempty"`
}

// ParseDecisionJSON extracts a Decision from LLM text output containing JSON.
func ParseDecisionJSON(text string) (agent.Decision, error) {
	text = strings.TrimSpace(text)
	if text == "" {
		return agent.Decision{}, ErrEmptyResponse
	}

	jsonStr, err := extractJSON(text)
	if err != nil {
		return agent.Decision{}, err
	}

	var env decisionEnvelope
	if err := json.Unmarshal([]byte(jsonStr), &env); err != nil {
		return agent.Decision{}, fmt.Errorf("invalid JSON: %w", err)
	}

	return envelopeToDecision(env)
}

// ParseToolCalls converts native LLM tool calls into a Decision.
// Only the first tool call is used; parallel calls are a future enhancement.
func ParseToolCalls(calls []ToolCall) (agent.Decision, error) {
	if len(calls) == 0 {
		return agent.Decision{}, ErrNoToolCalls
	}

	tc := calls[0]
	var input json.RawMessage
	if tc.Function.Arguments != "" {
		if !json.Valid([]byte(tc.Function.Arguments)) {
			return agent.Decision{}, fmt.Errorf("%w: %s", ErrInvalidToolArgs, tc.Function.Name)
		}
		input = json.RawMessage(tc.Function.Arguments)
	} else {
		input = json.RawMessage(`{}`)
	}

	return agent.NewCallToolDecision(tc.Function.Name, input, "tool call from LLM"), nil
}

func envelopeToDecision(env decisionEnvelope) (agent.Decision, error) {
	switch env.Decision {
	case "call_tool":
		if env.ToolName == "" {
			return agent.Decision{}, ErrMissingToolName
		}
		input := env.Input
		if len(input) == 0 {
			input = json.RawMessage(`{}`)
		}
		return agent.NewCallToolDecision(env.ToolName, input, env.Reason), nil

	case "transition":
		state := agent.State(env.ToState)
		if !state.IsValid() {
			return agent.Decision{}, fmt.Errorf("%w: %q", ErrInvalidState, env.ToState)
		}
		return agent.NewTransitionDecision(state, env.Reason), nil

	case "finish":
		result := env.Result
		if len(result) == 0 {
			result = nil
		}
		summary := env.Summary
		if summary == "" {
			summary = env.Reason
		}
		return agent.NewFinishDecision(summary, result), nil

	case "fail":
		reason := env.Reason
		if reason == "" {
			return agent.Decision{}, ErrMissingReason
		}
		return agent.NewFailDecision(reason, nil), nil

	case "ask_human":
		question := env.Question
		if question == "" {
			question = env.Reason
		}
		return agent.NewAskHumanDecision(question, env.Options...), nil

	default:
		return agent.Decision{}, fmt.Errorf("%w: %q", ErrUnknownDecision, env.Decision)
	}
}

// extractJSON finds the first JSON object in text, handling markdown fences and leading text.
func extractJSON(text string) (string, error) {
	// Try direct parse first
	text = strings.TrimSpace(text)
	if json.Valid([]byte(text)) && strings.HasPrefix(text, "{") {
		return text, nil
	}

	// Try markdown code fence extraction
	if idx := strings.Index(text, "```json"); idx >= 0 {
		start := idx + len("```json")
		end := strings.Index(text[start:], "```")
		if end >= 0 {
			candidate := strings.TrimSpace(text[start : start+end])
			if json.Valid([]byte(candidate)) {
				return candidate, nil
			}
		}
	}
	if idx := strings.Index(text, "```"); idx >= 0 {
		start := idx + len("```")
		// Skip optional language tag on same line
		if nl := strings.Index(text[start:], "\n"); nl >= 0 {
			start += nl + 1
		}
		end := strings.Index(text[start:], "```")
		if end >= 0 {
			candidate := strings.TrimSpace(text[start : start+end])
			if json.Valid([]byte(candidate)) && strings.HasPrefix(candidate, "{") {
				return candidate, nil
			}
		}
	}

	// Try finding first { and matching }
	braceStart := strings.Index(text, "{")
	if braceStart < 0 {
		return "", ErrNoJSON
	}

	// Find matching closing brace
	depth := 0
	inString := false
	escaped := false
	for i := braceStart; i < len(text); i++ {
		ch := text[i]
		if escaped {
			escaped = false
			continue
		}
		if ch == '\\' && inString {
			escaped = true
			continue
		}
		if ch == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if ch == '{' {
			depth++
		} else if ch == '}' {
			depth--
			if depth == 0 {
				candidate := text[braceStart : i+1]
				if json.Valid([]byte(candidate)) {
					return candidate, nil
				}
				break
			}
		}
	}

	return "", ErrNoJSON
}
