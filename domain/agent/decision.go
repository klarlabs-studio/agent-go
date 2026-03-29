package agent

import "encoding/json"

// DecisionType identifies the kind of decision made by the planner.
type DecisionType string

const (
	DecisionCallTool   DecisionType = "call_tool"  // Execute a tool
	DecisionTransition DecisionType = "transition" // Move to another state
	DecisionAskHuman   DecisionType = "ask_human"  // Request human input
	DecisionFinish     DecisionType = "finish"     // Complete successfully
	DecisionFail       DecisionType = "fail"       // Terminate with failure
)

// Decision represents the planner's output - exactly one of the fields is set.
type Decision struct {
	Type       DecisionType
	CallTool   *CallToolDecision
	Transition *TransitionDecision
	AskHuman   *AskHumanDecision
	Finish     *FinishDecision
	Fail       *FailDecision
}

// CallToolDecision instructs the engine to execute a tool.
type CallToolDecision struct {
	ToolName string          `json:"tool_name"`
	Input    json.RawMessage `json:"input"`
	Reason   string          `json:"reason"`
}

// TransitionDecision instructs the engine to transition to another state.
type TransitionDecision struct {
	ToState State  `json:"to_state"`
	Reason  string `json:"reason"`
}

// AskHumanDecision requests human input before proceeding.
type AskHumanDecision struct {
	Question string   `json:"question"`
	Options  []string `json:"options,omitempty"` // Optional constrained choices
}

// FinishDecision indicates successful completion.
type FinishDecision struct {
	Summary string          `json:"summary"`
	Result  json.RawMessage `json:"result,omitempty"`
}

// FailDecision indicates terminal failure.
type FailDecision struct {
	Reason string `json:"reason"`
	Err    error  `json:"-"` // Not serialized
}

// NewCallToolDecision creates a decision to execute a tool.
func NewCallToolDecision(toolName string, input json.RawMessage, reason string) Decision {
	return Decision{
		Type: DecisionCallTool,
		CallTool: &CallToolDecision{
			ToolName: toolName,
			Input:    input,
			Reason:   reason,
		},
	}
}

// NewTransitionDecision creates a decision to transition states.
func NewTransitionDecision(toState State, reason string) Decision {
	return Decision{
		Type: DecisionTransition,
		Transition: &TransitionDecision{
			ToState: toState,
			Reason:  reason,
		},
	}
}

// NewAskHumanDecision creates a decision to request human input.
func NewAskHumanDecision(question string, options ...string) Decision {
	return Decision{
		Type: DecisionAskHuman,
		AskHuman: &AskHumanDecision{
			Question: question,
			Options:  options,
		},
	}
}

// NewFinishDecision creates a decision to complete successfully.
func NewFinishDecision(summary string, result json.RawMessage) Decision {
	return Decision{
		Type: DecisionFinish,
		Finish: &FinishDecision{
			Summary: summary,
			Result:  result,
		},
	}
}

// NewFailDecision creates a decision to terminate with failure.
func NewFailDecision(reason string, err error) Decision {
	return Decision{
		Type: DecisionFail,
		Fail: &FailDecision{
			Reason: reason,
			Err:    err,
		},
	}
}

// IsTerminal returns true if the decision leads to a terminal state.
func (d Decision) IsTerminal() bool {
	return d.Type == DecisionFinish || d.Type == DecisionFail
}
