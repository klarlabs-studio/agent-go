// Package event provides domain types and interfaces for event sourcing.
package event

import (
	"encoding/json"
	"time"
)

// Event represents a domain event in the event store.
type Event struct {
	// ID is the unique identifier for this event.
	ID string `json:"id"`

	// RunID is the ID of the run this event belongs to.
	RunID string `json:"run_id"`

	// Type classifies the event.
	Type Type `json:"type"`

	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// Payload contains the event-specific data.
	Payload json.RawMessage `json:"payload"`

	// Sequence is the ordering number within the run's event stream.
	Sequence uint64 `json:"sequence"`

	// Version is the event schema version for forward compatibility.
	Version int `json:"version,omitempty"`
}

// NewEvent creates a new event with the given type and payload, stamped with
// the current wall-clock time. For deterministic timestamps (replay, fork,
// tests) use NewEventAt with an injected clock's time.
func NewEvent(runID string, eventType Type, payload any) (Event, error) {
	return NewEventAt(runID, eventType, payload, time.Now())
}

// NewEventAt creates a new event stamped with the provided timestamp. The
// engine passes its injected clock's time so an event stream is reproducible.
func NewEventAt(runID string, eventType Type, payload any, ts time.Time) (Event, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return Event{}, err
	}

	return Event{
		RunID:     runID,
		Type:      eventType,
		Timestamp: ts,
		Payload:   data,
		Version:   1,
	}, nil
}

// UnmarshalPayload decodes the event payload into the given value.
func (e *Event) UnmarshalPayload(v any) error {
	return json.Unmarshal(e.Payload, v)
}
