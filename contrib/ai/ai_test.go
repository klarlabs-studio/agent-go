package ai_test

import (
	"errors"
	"testing"

	"go.klarlabs.de/statekit"
	"go.klarlabs.de/statekit/ai"
)

type ctx struct {
	Choice string
}

func newDecisionMachine(t *testing.T) *statekit.Interpreter[ctx] {
	t.Helper()
	machine, err := statekit.NewMachine[ctx]("decision").
		WithInitial("waiting").
		State("waiting").
		On("APPROVE").Target("approved").
		On("REJECT").Target("rejected").
		On("ESCALATE").Target("escalated").
		Done().
		State("approved").Final().Done().
		State("rejected").Final().Done().
		State("escalated").Final().Done().
		Build()
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	interp := statekit.NewInterpreter(machine)
	t.Cleanup(func() { _ = interp.Close() })
	interp.Start()
	return interp
}

func TestDrive_PicksAndSends(t *testing.T) {
	t.Parallel()
	interp := newDecisionMachine(t)

	decider := func(_ ctx, _ []statekit.EventType) (statekit.Event, error) {
		return statekit.Event{Type: "APPROVE"}, nil
	}

	got, err := ai.Drive(interp, decider, []statekit.EventType{"APPROVE", "REJECT", "ESCALATE"})
	if err != nil {
		t.Fatalf("Drive: %v", err)
	}
	if got.Type != "APPROVE" {
		t.Errorf("chosen = %q, want APPROVE", got.Type)
	}
	if got := string(interp.State().Value); got != "approved" {
		t.Errorf("state = %q, want approved", got)
	}
}

func TestDrive_EmptyCandidates(t *testing.T) {
	t.Parallel()
	interp := newDecisionMachine(t)

	_, err := ai.Drive(interp, func(_ ctx, _ []statekit.EventType) (statekit.Event, error) {
		return statekit.Event{Type: "APPROVE"}, nil
	}, nil)
	if !errors.Is(err, ai.ErrNoCandidates) {
		t.Errorf("expected ErrNoCandidates, got %v", err)
	}
}

func TestDrive_RejectsOutOfSet(t *testing.T) {
	t.Parallel()
	interp := newDecisionMachine(t)

	decider := func(_ ctx, _ []statekit.EventType) (statekit.Event, error) {
		return statekit.Event{Type: "DELETE"}, nil // not in candidates
	}

	_, err := ai.Drive(interp, decider, []statekit.EventType{"APPROVE", "REJECT"})
	if !errors.Is(err, ai.ErrNoMatch) {
		t.Errorf("expected ErrNoMatch, got %v", err)
	}
	if got := string(interp.State().Value); got != "waiting" {
		t.Errorf("state should be unchanged on rejection; got %q", got)
	}
}

func TestDrive_DeciderError(t *testing.T) {
	t.Parallel()
	interp := newDecisionMachine(t)

	sentinel := errors.New("upstream LLM failure")
	decider := func(_ ctx, _ []statekit.EventType) (statekit.Event, error) {
		return statekit.Event{}, sentinel
	}

	_, err := ai.Drive(interp, decider, []statekit.EventType{"APPROVE"})
	if !errors.Is(err, sentinel) {
		t.Errorf("expected wrapped sentinel error, got %v", err)
	}
}

func TestDrive_NilArguments(t *testing.T) {
	t.Parallel()

	if _, err := ai.Drive[ctx](nil, nil, []statekit.EventType{"X"}); err == nil {
		t.Error("expected error for nil interpreter")
	}

	interp := newDecisionMachine(t)
	if _, err := ai.Drive(interp, nil, []statekit.EventType{"X"}); err == nil {
		t.Error("expected error for nil decider")
	}
}

func TestTool_ValidatePayload_RequiredField(t *testing.T) {
	t.Parallel()
	tool := ai.Tool{
		Name:     "send_email",
		Required: []string{"to", "subject"},
		Schema: map[string]any{
			"to":      map[string]any{"type": "string"},
			"subject": map[string]any{"type": "string"},
		},
	}

	err := tool.ValidatePayload(map[string]any{"to": "alice@example.com"})
	if err == nil {
		t.Fatal("expected error for missing 'subject'")
	}
}

func TestTool_ValidatePayload_TypeMismatch(t *testing.T) {
	t.Parallel()
	tool := ai.Tool{
		Name: "set_count",
		Schema: map[string]any{
			"count": map[string]any{"type": "integer"},
		},
	}

	err := tool.ValidatePayload(map[string]any{"count": "not a number"})
	if err == nil {
		t.Error("expected type-mismatch error")
	}
}

func TestTool_ValidatePayload_AcceptsValidShapes(t *testing.T) {
	t.Parallel()
	tool := ai.Tool{
		Name: "complex",
		Schema: map[string]any{
			"name":   map[string]any{"type": "string"},
			"count":  map[string]any{"type": "integer"},
			"active": map[string]any{"type": "boolean"},
			"tags":   map[string]any{"type": "array"},
			"meta":   map[string]any{"type": "object"},
		},
	}

	if err := tool.ValidatePayload(map[string]any{
		"name":   "x",
		"count":  42,
		"active": true,
		"tags":   []string{"a", "b"},
		"meta":   map[string]any{"k": "v"},
	}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTool_ValidatePayload_NumberAcceptsFloat(t *testing.T) {
	t.Parallel()
	tool := ai.Tool{
		Name:   "price",
		Schema: map[string]any{"amount": map[string]any{"type": "number"}},
	}
	if err := tool.ValidatePayload(map[string]any{"amount": 9.99}); err != nil {
		t.Errorf("number should accept float64: %v", err)
	}
	if err := tool.ValidatePayload(map[string]any{"amount": 10}); err != nil {
		t.Errorf("number should accept int: %v", err)
	}
	if err := tool.ValidatePayload(map[string]any{"amount": "10"}); err == nil {
		t.Error("number should reject string")
	}
}
