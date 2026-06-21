package agent

import "errors"

// Domain errors for the agent runtime.
var (
	// ErrInvalidState indicates the state is not a recognized canonical state.
	ErrInvalidState = errors.New("invalid state")

	// ErrInvalidTransition indicates an attempted state transition is not allowed.
	ErrInvalidTransition = errors.New("invalid state transition")

	// ErrRunTerminated indicates an operation was attempted on a terminated run.
	ErrRunTerminated = errors.New("run already terminated")

	// ErrRunNotStarted indicates an operation requires a started run.
	ErrRunNotStarted = errors.New("run not started")

	// ErrRunPaused indicates an operation requires an active run.
	ErrRunPaused = errors.New("run is paused")

	// ErrAwaitingHumanInput indicates the run is paused awaiting human input.
	ErrAwaitingHumanInput = errors.New("run is awaiting human input")

	// ErrNoProgress indicates the run was aborted by loop detection: too many
	// consecutive steps made no progress (no state change and no new evidence),
	// e.g. the planner kept emitting self-transitions or repeats.
	ErrNoProgress = errors.New("run aborted: no progress (possible loop)")

	// ErrNoPendingQuestion indicates no pending question exists for human input.
	ErrNoPendingQuestion = errors.New("run does not have a pending question")

	// ErrInvalidHumanInput indicates the human input is not valid for the pending question.
	ErrInvalidHumanInput = errors.New("invalid human input for pending question")

	// ErrCustomStateConflict indicates a custom state name conflicts with a canonical state.
	ErrCustomStateConflict = errors.New("custom state name conflicts with canonical state")

	// ErrCustomStateDuplicate indicates a custom state with the same name is already registered.
	ErrCustomStateDuplicate = errors.New("custom state already registered")
)
