// Package statemachine provides the statekit integration for the agent runtime.
package statemachine

import (
	"fmt"
	"strings"

	"go.klarlabs.de/statekit"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/ledger"
	"go.klarlabs.de/agent/domain/policy"
)

// Context carries run state through the state machine.
type Context struct {
	Run           *agent.Run
	Budget        *policy.Budget
	Ledger        *ledger.Ledger
	Eligibility   *policy.ToolEligibility
	Transitions   *policy.StateTransitions
	StateRegistry *agent.StateRegistry
}

// NewContext creates a new machine context.
func NewContext(run *agent.Run, budget *policy.Budget, ledger *ledger.Ledger) *Context {
	return &Context{
		Run:           run,
		Budget:        budget,
		Ledger:        ledger,
		Eligibility:   policy.NewToolEligibility(),
		Transitions:   policy.DefaultTransitions(),
		StateRegistry: agent.NewStateRegistry(),
	}
}

// CustomTransition defines a transition involving at least one custom state.
type CustomTransition struct {
	From  agent.State
	Event statekit.EventType
	To    agent.State
}

// MachineDefinition holds the full definition of the agent state machine,
// including any custom states and transitions registered at build time.
// It is used by the visualization layer to produce DOT/Mermaid output.
type MachineDefinition struct {
	Config            *statekit.MachineConfig[*Context]
	StateRegistry     *agent.StateRegistry
	CustomTransitions []CustomTransition
	// canonicalTransitions stores the default transition graph for visualization.
	canonicalTransitions []CustomTransition
}

// State IDs as StateID type for statekit.
const (
	stateIntake   statekit.StateID = statekit.StateID(agent.StateIntake)
	stateExplore  statekit.StateID = statekit.StateID(agent.StateExplore)
	stateDecide   statekit.StateID = statekit.StateID(agent.StateDecide)
	stateAct      statekit.StateID = statekit.StateID(agent.StateAct)
	stateValidate statekit.StateID = statekit.StateID(agent.StateValidate)
	stateDone     statekit.StateID = statekit.StateID(agent.StateDone)
	stateFailed   statekit.StateID = statekit.StateID(agent.StateFailed)
)

// canonicalTransitionList returns the default transition graph used by
// both machine building and visualization.
func canonicalTransitionList() []CustomTransition {
	return []CustomTransition{
		{agent.StateIntake, "EXPLORE", agent.StateExplore},
		{agent.StateIntake, "FAIL", agent.StateFailed},
		{agent.StateExplore, "DECIDE", agent.StateDecide},
		{agent.StateExplore, "FAIL", agent.StateFailed},
		{agent.StateDecide, "ACT", agent.StateAct},
		{agent.StateDecide, "DONE", agent.StateDone},
		{agent.StateDecide, "FAIL", agent.StateFailed},
		{agent.StateAct, "VALIDATE", agent.StateValidate},
		{agent.StateAct, "FAIL", agent.StateFailed},
		{agent.StateValidate, "DONE", agent.StateDone},
		{agent.StateValidate, "EXPLORE", agent.StateExplore},
		{agent.StateValidate, "FAIL", agent.StateFailed},
	}
}

// NewAgentMachine creates the canonical agent statechart.
func NewAgentMachine() (*statekit.MachineConfig[*Context], error) {
	def, err := NewAgentMachineDefinition()
	if err != nil {
		return nil, err
	}
	return def.Config, nil
}

// NewAgentMachineDefinition creates the canonical agent statechart with full
// definition metadata for visualization and introspection.
func NewAgentMachineDefinition() (*MachineDefinition, error) {
	return NewAgentMachineBuilder().Build()
}

// MachineBuilder constructs an agent state machine with optional custom states
// and transitions while preserving backward compatibility with the canonical graph.
type MachineBuilder struct {
	customStates      []agent.CustomState
	customTransitions []CustomTransition
	registry          *agent.StateRegistry
}

// NewAgentMachineBuilder creates a builder for the agent state machine.
func NewAgentMachineBuilder() *MachineBuilder {
	return &MachineBuilder{
		registry: agent.NewStateRegistry(),
	}
}

// WithCustomState registers a custom state. Returns the builder for chaining.
func (b *MachineBuilder) WithCustomState(cs agent.CustomState) *MachineBuilder {
	b.customStates = append(b.customStates, cs)
	return b
}

// WithCustomTransition adds a transition involving custom states.
// The event name is derived from the target state name in uppercase if not
// provided explicitly.
func (b *MachineBuilder) WithCustomTransition(from, to agent.State) *MachineBuilder {
	event := statekit.EventType(strings.ToUpper(string(to)))
	b.customTransitions = append(b.customTransitions, CustomTransition{
		From:  from,
		Event: event,
		To:    to,
	})
	return b
}

// WithCustomTransitionEvent adds a transition with an explicit event name.
func (b *MachineBuilder) WithCustomTransitionEvent(from agent.State, event statekit.EventType, to agent.State) *MachineBuilder {
	b.customTransitions = append(b.customTransitions, CustomTransition{
		From:  from,
		Event: event,
		To:    to,
	})
	return b
}

// Build constructs the state machine configuration with all canonical and
// custom states and transitions.
func (b *MachineBuilder) Build() (*MachineDefinition, error) {
	// Register custom states in the registry
	for _, cs := range b.customStates {
		if err := b.registry.Register(cs); err != nil {
			return nil, fmt.Errorf("register custom state %q: %w", cs.Name, err)
		}
	}

	// Collect transitions FROM each canonical state to custom states.
	customFrom := make(map[agent.State][]CustomTransition)
	for _, ct := range b.customTransitions {
		customFrom[ct.From] = append(customFrom[ct.From], ct)
	}

	// If there are no custom states or transitions, use the simple canonical build.
	if len(b.customStates) == 0 && len(b.customTransitions) == 0 {
		config, err := buildCanonicalMachine()
		if err != nil {
			return nil, err
		}
		return &MachineDefinition{
			Config:               config,
			StateRegistry:        b.registry,
			CustomTransitions:    nil,
			canonicalTransitions: canonicalTransitionList(),
		}, nil
	}

	// Build with custom states. We construct the machine using a single
	// fluent chain by pre-building an statekit machine config that includes
	// custom states and transitions via the addTransitions helper approach.
	config, err := b.buildExtendedMachine(customFrom)
	if err != nil {
		return nil, err
	}

	return &MachineDefinition{
		Config:               config,
		StateRegistry:        b.registry,
		CustomTransitions:    b.customTransitions,
		canonicalTransitions: canonicalTransitionList(),
	}, nil
}

// buildCanonicalMachine builds the default state machine with no custom states.
func buildCanonicalMachine() (*statekit.MachineConfig[*Context], error) {
	return statekit.NewMachine[*Context]("agent").
		WithInitial(stateIntake).
		WithContext(&Context{}).
		WithAction("logEntry", logStateEntry).
		WithAction("recordTransition", recordTransition).
		WithGuard("canTransition", guardCanTransition).
		WithGuard("budgetAvailable", guardBudgetAvailable).
		State(stateIntake).
		OnEntry("logEntry").
		On("EXPLORE").Target(stateExplore).Guard("canTransition").Do("recordTransition").
		On("FAIL").Target(stateFailed).Do("recordTransition").
		Done().
		State(stateExplore).
		OnEntry("logEntry").
		On("DECIDE").Target(stateDecide).Guard("canTransition").Do("recordTransition").
		On("FAIL").Target(stateFailed).Do("recordTransition").
		Done().
		State(stateDecide).
		OnEntry("logEntry").
		On("ACT").Target(stateAct).Guard("canTransition").Guard("budgetAvailable").Do("recordTransition").
		On("DONE").Target(stateDone).Do("recordTransition").
		On("FAIL").Target(stateFailed).Do("recordTransition").
		Done().
		State(stateAct).
		OnEntry("logEntry").
		On("VALIDATE").Target(stateValidate).Guard("canTransition").Do("recordTransition").
		On("FAIL").Target(stateFailed).Do("recordTransition").
		Done().
		State(stateValidate).
		OnEntry("logEntry").
		On("DONE").Target(stateDone).Do("recordTransition").
		On("EXPLORE").Target(stateExplore).Guard("canTransition").Do("recordTransition").
		On("FAIL").Target(stateFailed).Do("recordTransition").
		Done().
		State(stateDone).
		Final().
		OnEntry("logEntry").
		Done().
		State(stateFailed).
		Final().
		OnEntry("logEntry").
		Done().
		Build()
}

// buildExtendedMachine builds the state machine with custom states and transitions.
// It uses a dynamic guard to route custom transitions through the policy layer.
func (b *MachineBuilder) buildExtendedMachine(customFrom map[agent.State][]CustomTransition) (*statekit.MachineConfig[*Context], error) {
	mb := statekit.NewMachine[*Context]("agent").
		WithInitial(stateIntake).
		WithContext(&Context{}).
		WithAction("logEntry", logStateEntry).
		WithAction("recordTransition", recordTransition).
		WithGuard("canTransition", guardCanTransition).
		WithGuard("budgetAvailable", guardBudgetAvailable)

	// Build intake state
	intakeState := mb.State(stateIntake).OnEntry("logEntry").
		On("EXPLORE").Target(stateExplore).Guard("canTransition").Do("recordTransition").
		On("FAIL").Target(stateFailed).Do("recordTransition")
	for _, ct := range customFrom[agent.StateIntake] {
		intakeState = intakeState.On(ct.Event).Target(statekit.StateID(ct.To)).Guard("canTransition").Do("recordTransition")
	}
	mb2 := intakeState.Done()

	// Build explore state
	exploreState := mb2.State(stateExplore).OnEntry("logEntry").
		On("DECIDE").Target(stateDecide).Guard("canTransition").Do("recordTransition").
		On("FAIL").Target(stateFailed).Do("recordTransition")
	for _, ct := range customFrom[agent.StateExplore] {
		exploreState = exploreState.On(ct.Event).Target(statekit.StateID(ct.To)).Guard("canTransition").Do("recordTransition")
	}
	mb3 := exploreState.Done()

	// Build decide state
	decideState := mb3.State(stateDecide).OnEntry("logEntry").
		On("ACT").Target(stateAct).Guard("canTransition").Guard("budgetAvailable").Do("recordTransition").
		On("DONE").Target(stateDone).Do("recordTransition").
		On("FAIL").Target(stateFailed).Do("recordTransition")
	for _, ct := range customFrom[agent.StateDecide] {
		decideState = decideState.On(ct.Event).Target(statekit.StateID(ct.To)).Guard("canTransition").Do("recordTransition")
	}
	mb4 := decideState.Done()

	// Build act state
	actState := mb4.State(stateAct).OnEntry("logEntry").
		On("VALIDATE").Target(stateValidate).Guard("canTransition").Do("recordTransition").
		On("FAIL").Target(stateFailed).Do("recordTransition")
	for _, ct := range customFrom[agent.StateAct] {
		actState = actState.On(ct.Event).Target(statekit.StateID(ct.To)).Guard("canTransition").Do("recordTransition")
	}
	mb5 := actState.Done()

	// Build validate state
	validateState := mb5.State(stateValidate).OnEntry("logEntry").
		On("DONE").Target(stateDone).Do("recordTransition").
		On("EXPLORE").Target(stateExplore).Guard("canTransition").Do("recordTransition").
		On("FAIL").Target(stateFailed).Do("recordTransition")
	for _, ct := range customFrom[agent.StateValidate] {
		validateState = validateState.On(ct.Event).Target(statekit.StateID(ct.To)).Guard("canTransition").Do("recordTransition")
	}
	mb6 := validateState.Done()

	// Build custom states
	current := mb6
	for _, cs := range b.customStates {
		stateID := statekit.StateID(cs.Name)
		transitions := customFrom[cs.Name]
		hasTransitions := len(transitions) > 0 || !cs.Terminal

		if !hasTransitions {
			// Terminal state with no outbound transitions
			if cs.Terminal {
				current = current.State(stateID).Final().OnEntry("logEntry").Done()
			} else {
				current = current.State(stateID).OnEntry("logEntry").Done()
			}
			continue
		}

		// State has outbound transitions - build with TransitionBuilder
		var tb *statekit.TransitionBuilder[*Context]

		if cs.Terminal {
			sb := current.State(stateID).Final().OnEntry("logEntry")
			// Terminal states with custom transitions - add first transition
			// to switch from StateBuilder to TransitionBuilder
			if len(transitions) > 0 {
				first := transitions[0]
				tb = sb.On(first.Event).Target(statekit.StateID(first.To)).Guard("canTransition").Do("recordTransition")
				transitions = transitions[1:]
			}
		} else {
			sb := current.State(stateID).OnEntry("logEntry")
			// Non-terminal: always has at least a FAIL transition
			if len(transitions) > 0 {
				first := transitions[0]
				tb = sb.On(first.Event).Target(statekit.StateID(first.To)).Guard("canTransition").Do("recordTransition")
				transitions = transitions[1:]
			} else {
				// Only FAIL transition
				tb = sb.On("FAIL").Target(stateFailed).Do("recordTransition")
				current = tb.Done()
				continue
			}
		}

		// Add remaining transitions
		for _, ct := range transitions {
			tb = tb.On(ct.Event).Target(statekit.StateID(ct.To)).Guard("canTransition").Do("recordTransition")
		}

		// Non-terminal custom states can always fail
		if !cs.Terminal {
			tb = tb.On("FAIL").Target(stateFailed).Do("recordTransition")
		}

		current = tb.Done()
	}

	// Terminal states
	return current.
		State(stateDone).Final().OnEntry("logEntry").Done().
		State(stateFailed).Final().OnEntry("logEntry").Done().
		Build()
}

// EventForTransition returns the event type for a state transition.
// For canonical states, well-known event names are returned (e.g. EXPLORE, DECIDE).
// For custom states, the event name is the uppercase state name, matching the
// convention used by MachineBuilder.WithCustomTransition.
func EventForTransition(to agent.State) statekit.EventType {
	switch to {
	case agent.StateExplore:
		return "EXPLORE"
	case agent.StateDecide:
		return "DECIDE"
	case agent.StateAct:
		return "ACT"
	case agent.StateValidate:
		return "VALIDATE"
	case agent.StateDone:
		return "DONE"
	case agent.StateFailed:
		return "FAIL"
	default:
		return statekit.EventType(strings.ToUpper(string(to)))
	}
}

// StateFromMachine converts the machine state ID to domain State.
func StateFromMachine(stateID statekit.StateID) agent.State {
	return agent.State(stateID)
}
