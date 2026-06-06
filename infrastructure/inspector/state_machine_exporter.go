// Package inspector provides inspector infrastructure implementations.
package inspector

import (
	"context"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/inspector"
	"go.klarlabs.de/agent/domain/policy"
)

// StateMachineExporter exports state machine data.
type StateMachineExporter struct {
	eligibility *policy.ToolEligibility
	transitions *policy.StateTransitions
}

// NewStateMachineExporter creates a new state machine exporter.
func NewStateMachineExporter(
	eligibility *policy.ToolEligibility,
	transitions *policy.StateTransitions,
) *StateMachineExporter {
	return &StateMachineExporter{
		eligibility: eligibility,
		transitions: transitions,
	}
}

// Export exports the state machine.
func (e *StateMachineExporter) Export(ctx context.Context) (*inspector.StateMachineExport, error) {
	export := &inspector.StateMachineExport{
		Initial:  agent.StateIntake,
		Terminal: []agent.State{agent.StateDone, agent.StateFailed},
	}

	// Export states
	allStates := []agent.State{
		agent.StateIntake,
		agent.StateExplore,
		agent.StateDecide,
		agent.StateAct,
		agent.StateValidate,
		agent.StateDone,
		agent.StateFailed,
	}

	for _, state := range allStates {
		stateExport := inspector.StateExport{
			Name:              state,
			Description:       getStateDescription(state),
			IsTerminal:        state == agent.StateDone || state == agent.StateFailed,
			AllowsSideEffects: state == agent.StateAct,
		}

		// Get eligible tools if eligibility is configured
		if e.eligibility != nil {
			stateExport.EligibleTools = e.eligibility.AllowedTools(state)
		}

		export.States = append(export.States, stateExport)
	}

	// Export transitions
	if e.transitions != nil {
		for _, from := range allStates {
			targets := e.transitions.AllowedTransitions(from)
			for _, to := range targets {
				export.Transitions = append(export.Transitions, inspector.StateMachineTransition{
					From:  from,
					To:    to,
					Label: getTransitionLabel(from, to),
				})
			}
		}
	} else {
		// Default transitions if none configured
		export.Transitions = getDefaultTransitions()
	}

	return export, nil
}

func getStateDescription(state agent.State) string {
	switch state {
	case agent.StateIntake:
		return "Normalize and understand the goal"
	case agent.StateExplore:
		return "Gather evidence through read-only operations"
	case agent.StateDecide:
		return "Choose the next action"
	case agent.StateAct:
		return "Perform side-effects"
	case agent.StateValidate:
		return "Confirm outcomes"
	case agent.StateDone:
		return "Terminal success state"
	case agent.StateFailed:
		return "Terminal failure state"
	default:
		return ""
	}
}

func getTransitionLabel(from, to agent.State) string {
	if to == agent.StateFailed {
		return "on error"
	}
	if to == agent.StateDone {
		return "on success"
	}
	return ""
}

func getDefaultTransitions() []inspector.StateMachineTransition {
	return []inspector.StateMachineTransition{
		{From: agent.StateIntake, To: agent.StateExplore},
		{From: agent.StateIntake, To: agent.StateDecide},
		{From: agent.StateIntake, To: agent.StateFailed, Label: "on error"},
		{From: agent.StateExplore, To: agent.StateDecide},
		{From: agent.StateExplore, To: agent.StateFailed, Label: "on error"},
		{From: agent.StateDecide, To: agent.StateAct},
		{From: agent.StateDecide, To: agent.StateExplore},
		{From: agent.StateDecide, To: agent.StateDone, Label: "on success"},
		{From: agent.StateDecide, To: agent.StateFailed, Label: "on error"},
		{From: agent.StateAct, To: agent.StateValidate},
		{From: agent.StateAct, To: agent.StateFailed, Label: "on error"},
		{From: agent.StateValidate, To: agent.StateDecide},
		{From: agent.StateValidate, To: agent.StateDone, Label: "on success"},
		{From: agent.StateValidate, To: agent.StateFailed, Label: "on error"},
	}
}

// Ensure StateMachineExporter implements inspector.StateMachineExporter
var _ inspector.StateMachineExporter = (*StateMachineExporter)(nil)
