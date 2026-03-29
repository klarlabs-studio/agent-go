package inspector

import (
	"encoding/json"
	"fmt"

	"github.com/felixgeelhaar/agent-go/domain/inspector"
)

// XStateFormatter formats state machine data as XState JSON.
// XState is a popular JavaScript library for state machines.
// This format can be visualized at https://stately.ai/viz
type XStateFormatter struct {
	machineID string
	version   string
	pretty    bool
}

// XStateFormatterOption configures the XState formatter.
type XStateFormatterOption func(*XStateFormatter)

// WithMachineID sets the machine ID.
func WithMachineID(id string) XStateFormatterOption {
	return func(f *XStateFormatter) {
		f.machineID = id
	}
}

// WithVersion sets the machine version.
func WithVersion(version string) XStateFormatterOption {
	return func(f *XStateFormatter) {
		f.version = version
	}
}

// WithXStatePretty enables pretty-printed JSON output.
func WithXStatePretty() XStateFormatterOption {
	return func(f *XStateFormatter) {
		f.pretty = true
	}
}

// NewXStateFormatter creates a new XState formatter.
func NewXStateFormatter(opts ...XStateFormatterOption) *XStateFormatter {
	f := &XStateFormatter{
		machineID: "agentMachine",
		version:   "5.0",
		pretty:    true,
	}
	for _, opt := range opts {
		opt(f)
	}
	return f
}

// XStateMachine represents an XState machine definition.
type XStateMachine struct {
	ID      string                 `json:"id"`
	Version string                 `json:"version,omitempty"`
	Initial string                 `json:"initial"`
	Context map[string]any         `json:"context,omitempty"`
	States  map[string]XStateState `json:"states"`
}

// XStateState represents a state in XState format.
type XStateState struct {
	Type        string                      `json:"type,omitempty"`
	Description string                      `json:"description,omitempty"`
	Entry       []string                    `json:"entry,omitempty"`
	Exit        []string                    `json:"exit,omitempty"`
	On          map[string]XStateTransition `json:"on,omitempty"`
	Meta        map[string]any              `json:"meta,omitempty"`
}

// XStateTransition represents a transition in XState format.
type XStateTransition struct {
	Target  string   `json:"target,omitempty"`
	Actions []string `json:"actions,omitempty"`
	Guards  []string `json:"guard,omitempty"`
}

// Format formats the data as XState JSON.
func (f *XStateFormatter) Format(data any) ([]byte, error) {
	smExport, ok := data.(*inspector.StateMachineExport)
	if !ok {
		return nil, fmt.Errorf("XState formatter requires StateMachineExport, got %T", data)
	}

	machine := f.buildMachine(smExport)

	if f.pretty {
		return json.MarshalIndent(machine, "", "  ")
	}
	return json.Marshal(machine)
}

func (f *XStateFormatter) buildMachine(data *inspector.StateMachineExport) XStateMachine {
	machine := XStateMachine{
		ID:      f.machineID,
		Version: f.version,
		Initial: string(data.Initial),
		States:  make(map[string]XStateState),
		Context: map[string]any{},
	}

	// Build state definitions
	stateMap := make(map[string]inspector.StateExport)
	for _, s := range data.States {
		stateMap[string(s.Name)] = s
	}

	// Group transitions by source state
	transitionsByState := make(map[string][]inspector.StateMachineTransition)
	for _, t := range data.Transitions {
		from := string(t.From)
		transitionsByState[from] = append(transitionsByState[from], t)
	}

	// Build each state
	for _, s := range data.States {
		stateName := string(s.Name)
		state := XStateState{
			Description: s.Description,
			Meta:        make(map[string]any),
		}

		// Mark terminal states
		if s.IsTerminal {
			state.Type = "final"
		}

		// Add metadata
		state.Meta["allowsSideEffects"] = s.AllowsSideEffects
		if len(s.EligibleTools) > 0 {
			state.Meta["eligibleTools"] = s.EligibleTools
		}

		// Add transitions
		if transitions, ok := transitionsByState[stateName]; ok && len(transitions) > 0 {
			state.On = make(map[string]XStateTransition)
			for _, t := range transitions {
				eventName := f.buildEventName(t)
				state.On[eventName] = XStateTransition{
					Target: string(t.To),
				}
			}
		}

		machine.States[stateName] = state
	}

	return machine
}

func (f *XStateFormatter) buildEventName(t inspector.StateMachineTransition) string {
	if t.Label != "" {
		return t.Label
	}
	// Generate a default event name based on the transition
	return fmt.Sprintf("TO_%s", t.To)
}

// FormatType returns the format type.
func (f *XStateFormatter) FormatType() inspector.ExportFormat {
	return inspector.FormatXState
}

// Ensure XStateFormatter implements inspector.Formatter
var _ inspector.Formatter = (*XStateFormatter)(nil)

// XStateVisualizerURL returns a URL to visualize the machine at stately.ai.
func XStateVisualizerURL() string {
	return "https://stately.ai/viz"
}

// XStateConfig represents additional XState v5 configuration.
type XStateConfig struct {
	// Types for better TypeScript inference (v5 feature).
	Types struct {
		Context string `json:"context,omitempty"`
		Events  string `json:"events,omitempty"`
	} `json:"types,omitempty"`

	// Predictable action arguments for v5 compatibility.
	PredictableActionArguments bool `json:"predictableActionArguments,omitempty"`

	// Preserve action order for v5.
	PreserveActionOrder bool `json:"preserveActionOrder,omitempty"`
}
