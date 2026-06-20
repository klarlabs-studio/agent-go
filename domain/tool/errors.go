package tool

import "errors"

// Domain errors for the tool system.
var (
	// ErrEmptyName indicates a tool was created with an empty name.
	ErrEmptyName = errors.New("tool name cannot be empty")

	// ErrNoHandler indicates a tool was created without a handler.
	ErrNoHandler = errors.New("tool has no handler")

	// ErrToolNotFound indicates the requested tool was not found.
	ErrToolNotFound = errors.New("tool not found")

	// ErrToolExists indicates a tool with the same name already exists.
	ErrToolExists = errors.New("tool already exists")

	// ErrToolNotAllowed indicates the tool is not allowed in the current state.
	ErrToolNotAllowed = errors.New("tool not allowed in current state")

	// ErrSideEffectInNonActState indicates a side-effecting tool was invoked in
	// a state that does not permit side effects. This is a STRUCTURAL invariant
	// — it is enforced independently of tool-eligibility configuration and
	// cannot be relaxed or bypassed. Side effects are permitted only in states
	// that allow them (canonically, the act state).
	ErrSideEffectInNonActState = errors.New("side-effecting tool rejected outside a side-effecting state")

	// ErrInvalidInput indicates the input failed schema validation.
	ErrInvalidInput = errors.New("invalid tool input")

	// ErrInvalidOutput indicates the output failed schema validation.
	ErrInvalidOutput = errors.New("invalid tool output")

	// ErrApprovalRequired indicates the tool requires approval to execute.
	ErrApprovalRequired = errors.New("approval required for tool execution")

	// ErrApprovalDenied indicates approval was denied for tool execution.
	ErrApprovalDenied = errors.New("approval denied for tool execution")

	// ErrExecutionTimeout indicates the tool execution timed out.
	ErrExecutionTimeout = errors.New("tool execution timed out")
)
