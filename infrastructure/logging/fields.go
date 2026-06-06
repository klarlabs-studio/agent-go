package logging

import (
	"time"

	"go.klarlabs.de/bolt"

	"go.klarlabs.de/agent/domain/agent"
)

// Field is a function that applies structured data to a log event.
type Field func(*bolt.Event) *bolt.Event

// Common field constructors for agent runtime logging.

// RunID adds a run ID field.
func RunID(id string) Field {
	return func(e *bolt.Event) *bolt.Event {
		return e.Str("run_id", id)
	}
}

// State adds a state field.
func State(s agent.State) Field {
	return func(e *bolt.Event) *bolt.Event {
		return e.Str("state", string(s))
	}
}

// FromState adds a from_state field for transitions.
func FromState(s agent.State) Field {
	return func(e *bolt.Event) *bolt.Event {
		return e.Str("from_state", string(s))
	}
}

// ToState adds a to_state field for transitions.
func ToState(s agent.State) Field {
	return func(e *bolt.Event) *bolt.Event {
		return e.Str("to_state", string(s))
	}
}

// ToolName adds a tool name field.
func ToolName(name string) Field {
	return func(e *bolt.Event) *bolt.Event {
		return e.Str("tool", name)
	}
}

// Decision adds a decision type field.
func Decision(d agent.DecisionType) Field {
	return func(e *bolt.Event) *bolt.Event {
		return e.Str("decision", string(d))
	}
}

// Duration adds a duration field in milliseconds.
func Duration(d time.Duration) Field {
	return func(e *bolt.Event) *bolt.Event {
		return e.Int64("duration_ms", d.Milliseconds())
	}
}

// DurationNs adds a duration field in nanoseconds.
func DurationNs(d time.Duration) Field {
	return func(e *bolt.Event) *bolt.Event {
		return e.Int64("duration_ns", d.Nanoseconds())
	}
}

// Cached adds a cached field.
func Cached(cached bool) Field {
	return func(e *bolt.Event) *bolt.Event {
		return e.Bool("cached", cached)
	}
}

// ErrorField adds an error field.
func ErrorField(err error) Field {
	return func(e *bolt.Event) *bolt.Event {
		if err == nil {
			return e
		}
		return e.Err(err)
	}
}

// Budget adds budget-related fields.
func Budget(name string, remaining int) Field {
	return func(e *bolt.Event) *bolt.Event {
		return e.Str("budget", name).Int("remaining", remaining)
	}
}

// Approved adds an approval status field.
func Approved(approved bool) Field {
	return func(e *bolt.Event) *bolt.Event {
		return e.Bool("approved", approved)
	}
}

// Approver adds an approver field.
func Approver(name string) Field {
	return func(e *bolt.Event) *bolt.Event {
		return e.Str("approver", name)
	}
}

// EvidenceCount adds an evidence count field.
func EvidenceCount(count int) Field {
	return func(e *bolt.Event) *bolt.Event {
		return e.Int("evidence_count", count)
	}
}

// Goal adds a goal field.
func Goal(goal string) Field {
	return func(e *bolt.Event) *bolt.Event {
		return e.Str("goal", goal)
	}
}

// Summary adds a summary field.
func Summary(summary string) Field {
	return func(e *bolt.Event) *bolt.Event {
		return e.Str("summary", summary)
	}
}

// Reason adds a reason field.
func Reason(reason string) Field {
	return func(e *bolt.Event) *bolt.Event {
		return e.Str("reason", reason)
	}
}

// Component adds a component field for categorization.
func Component(name string) Field {
	return func(e *bolt.Event) *bolt.Event {
		return e.Str("component", name)
	}
}

// Operation adds an operation field.
func Operation(op string) Field {
	return func(e *bolt.Event) *bolt.Event {
		return e.Str("operation", op)
	}
}

// Str adds a string field with custom key.
func Str(key, value string) Field {
	return func(e *bolt.Event) *bolt.Event {
		return e.Str(key, value)
	}
}

// Int adds an integer field with custom key.
func Int(key string, value int) Field {
	return func(e *bolt.Event) *bolt.Event {
		return e.Int(key, value)
	}
}

// Int64 adds an int64 field with custom key.
func Int64(key string, value int64) Field {
	return func(e *bolt.Event) *bolt.Event {
		return e.Int64(key, value)
	}
}

// Float64 adds a float64 field with custom key.
func Float64(key string, value float64) Field {
	return func(e *bolt.Event) *bolt.Event {
		return e.Float64(key, value)
	}
}

// Bool adds a boolean field with custom key.
func Bool(key string, value bool) Field {
	return func(e *bolt.Event) *bolt.Event {
		return e.Bool(key, value)
	}
}
