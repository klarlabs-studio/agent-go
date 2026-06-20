// Package ai provides primitives for LLM-driven state transitions.
//
// The package is deliberately small and dependency-free. It does not
// import any specific LLM SDK — instead it gives you the seams to wire
// in your own provider while keeping the state-machine surface
// deterministic.
//
// The two primitives are:
//
//   - Decider[C]: a function that picks the next event from a declared
//     set of candidates given the current machine context. Use this to
//     bridge LLM output into your statechart.
//
//   - Tool: a JSON-Schema-shaped description of a tool call. Use it to
//     declare a typed event payload contract that an LLM (or MCP host)
//     can fulfill, then validate inbound payloads with ValidatePayload.
//
// Determinism note: a Decider that wraps an LLM should fix temperature
// to 0 (or pass a deterministic seed) and capture the prompt and
// response in the event payload — the aiplugin.PromptRecorder picks
// these up automatically. Combined with event sourcing, this gives
// you reproducible agent runs.
package ai

import (
	"errors"
	"fmt"

	"go.klarlabs.de/statekit"
)

// Decider chooses the next event to send to a state machine, given
// the machine's current context and the set of declared candidate
// events. Implementations typically wrap an LLM call.
//
// Returning an event whose Type does not match any candidate is an
// error — Drive enforces this for safety.
type Decider[C any] func(ctx C, candidates []statekit.EventType) (statekit.Event, error)

// ErrNoMatch reports that the decider returned an event type that
// is not one of the declared candidates.
var ErrNoMatch = errors.New("decider returned event type not in candidates")

// ErrNoCandidates reports that Drive was called with an empty
// candidates list.
var ErrNoCandidates = errors.New("no candidates declared")

// Drive asks the decider to pick one event from the candidates and
// then sends it on the interpreter. The chosen event must have a Type
// that appears in the candidates list; otherwise Drive returns
// ErrNoMatch and does not send.
//
// Use Drive when an LLM is the controller for a state transition —
// e.g. "given the current order context, decide CANCEL, REFUND, or
// SHIP." Wrap your provider call in a Decider and call Drive at the
// state where the LLM owns the routing decision.
func Drive[C any](
	interp *statekit.Interpreter[C],
	decider Decider[C],
	candidates []statekit.EventType,
) (statekit.Event, error) {
	if len(candidates) == 0 {
		return statekit.Event{}, ErrNoCandidates
	}
	if interp == nil {
		return statekit.Event{}, errors.New("interpreter is nil")
	}
	if decider == nil {
		return statekit.Event{}, errors.New("decider is nil")
	}

	ctx := interp.State().Context
	chosen, err := decider(ctx, candidates)
	if err != nil {
		return statekit.Event{}, fmt.Errorf("decider failed: %w", err)
	}

	if !contains(candidates, chosen.Type) {
		return statekit.Event{}, fmt.Errorf("%w: got %q, allowed %v", ErrNoMatch, chosen.Type, candidates)
	}

	interp.Send(chosen)
	return chosen, nil
}

func contains(haystack []statekit.EventType, needle statekit.EventType) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

// -- Tool schema ------------------------------------------------------------

// Tool describes a typed event payload contract that an LLM, MCP
// host, or other agent runtime can fulfill. The Schema is a
// JSON-Schema fragment (any subset that your validator understands —
// the package itself ships only a minimal validator focused on the
// most common shapes).
//
// Pair Tool with statekit.Event whose Type matches Tool.Name; the
// LLM-provided payload becomes Event.Payload and ValidatePayload
// runs as part of your guard.
type Tool struct {
	// Name identifies the tool. Use this as the EventType when sending
	// the corresponding event.
	Name string

	// Description is a human-readable summary surfaced to the LLM.
	Description string

	// Schema is a JSON-Schema-shaped description of the expected
	// payload shape. Required keys live in Required.
	Schema map[string]any

	// Required lists payload keys that must be present.
	Required []string
}

// ValidatePayload checks the payload against the tool's Required list
// and the top-level type expectations declared in Schema. It is
// deliberately small — for full JSON-Schema validation, plug in
// santhosh-tekuri/jsonschema or equivalent in your own guard.
func (t Tool) ValidatePayload(payload map[string]any) error {
	for _, key := range t.Required {
		if _, ok := payload[key]; !ok {
			return fmt.Errorf("tool %q: missing required field %q", t.Name, key)
		}
	}
	for key, raw := range t.Schema {
		field, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		typeName, _ := field["type"].(string)
		if typeName == "" {
			continue
		}
		val, present := payload[key]
		if !present {
			continue
		}
		if err := checkType(t.Name, key, typeName, val); err != nil {
			return err
		}
	}
	return nil
}

func checkType(toolName, key, want string, val any) error {
	mismatch := func() error {
		return fmt.Errorf("tool %q: field %q expected %s, got %T", toolName, key, want, val)
	}
	switch want {
	case "string":
		if _, ok := val.(string); !ok {
			return mismatch()
		}
	case "number":
		switch val.(type) {
		case float64, float32, int, int32, int64:
		default:
			return mismatch()
		}
	case "integer":
		switch val.(type) {
		case int, int32, int64:
		default:
			return mismatch()
		}
	case "boolean":
		if _, ok := val.(bool); !ok {
			return mismatch()
		}
	case "array":
		switch val.(type) {
		case []any, []string, []int, []float64:
		default:
			return mismatch()
		}
	case "object":
		if _, ok := val.(map[string]any); !ok {
			return mismatch()
		}
	}
	return nil
}
