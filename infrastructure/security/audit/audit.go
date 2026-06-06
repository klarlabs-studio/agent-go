// Package audit provides security audit logging.
package audit

import (
	"context"
	"encoding/json"
	"io"
	"sync"
	"time"

	"go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/tool"
)

// Event represents a security audit event.
type Event struct {
	Timestamp   time.Time              `json:"timestamp"`
	EventType   EventType              `json:"event_type"`
	RunID       string                 `json:"run_id,omitempty"`
	ToolName    string                 `json:"tool_name,omitempty"`
	AgentState  string                 `json:"agent_state,omitempty"`
	Success     bool                   `json:"success"`
	Error       string                 `json:"error,omitempty"`
	Duration    time.Duration          `json:"duration_ns,omitempty"`
	InputHash   string                 `json:"input_hash,omitempty"`
	OutputHash  string                 `json:"output_hash,omitempty"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
	UserID      string                 `json:"user_id,omitempty"`
	ClientIP    string                 `json:"client_ip,omitempty"`
}

// EventType categorizes audit events.
type EventType string

const (
	EventToolExecution      EventType = "tool_execution"
	EventToolApproval       EventType = "tool_approval"
	EventToolRejection      EventType = "tool_rejection"
	EventStateTransition    EventType = "state_transition"
	EventValidationFailure  EventType = "validation_failure"
	EventAuthorizationCheck EventType = "authorization_check"
	EventSecretAccess       EventType = "secret_access"
	EventPolicyViolation    EventType = "policy_violation"
	EventBudgetExceeded     EventType = "budget_exceeded"
	EventRunStart           EventType = "run_start"
	EventRunComplete        EventType = "run_complete"
	EventRunFailed          EventType = "run_failed"
)

// Logger defines the interface for audit logging.
type Logger interface {
	// Log records an audit event.
	Log(ctx context.Context, event Event) error

	// Query retrieves events matching the filter.
	Query(ctx context.Context, filter Filter) ([]Event, error)

	// Close releases resources.
	Close() error
}

// Filter specifies criteria for querying events.
type Filter struct {
	StartTime  time.Time
	EndTime    time.Time
	EventTypes []EventType
	RunID      string
	ToolName   string
	Success    *bool
	Limit      int
}

// MemoryLogger implements Logger using in-memory storage.
type MemoryLogger struct {
	mu     sync.RWMutex
	events []Event
	maxLen int
}

// MemoryLoggerOption configures the memory logger.
type MemoryLoggerOption func(*MemoryLogger)

// WithMaxEvents sets the maximum number of events to retain.
func WithMaxEvents(max int) MemoryLoggerOption {
	return func(l *MemoryLogger) {
		l.maxLen = max
	}
}

// NewMemoryLogger creates a new in-memory audit logger.
func NewMemoryLogger(opts ...MemoryLoggerOption) *MemoryLogger {
	l := &MemoryLogger{
		events: make([]Event, 0),
		maxLen: 10000,
	}
	for _, opt := range opts {
		opt(l)
	}
	return l
}

// Log records an event.
func (l *MemoryLogger) Log(ctx context.Context, event Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	l.events = append(l.events, event)

	// Trim if exceeding max length
	if l.maxLen > 0 && len(l.events) > l.maxLen {
		l.events = l.events[len(l.events)-l.maxLen:]
	}

	return nil
}

// Query retrieves events matching the filter.
func (l *MemoryLogger) Query(ctx context.Context, filter Filter) ([]Event, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var result []Event
	for _, event := range l.events {
		if !matchesFilter(event, filter) {
			continue
		}
		result = append(result, event)
		if filter.Limit > 0 && len(result) >= filter.Limit {
			break
		}
	}
	return result, nil
}

// Close releases resources.
func (l *MemoryLogger) Close() error {
	return nil
}

// Events returns all events (for testing).
func (l *MemoryLogger) Events() []Event {
	l.mu.RLock()
	defer l.mu.RUnlock()
	result := make([]Event, len(l.events))
	copy(result, l.events)
	return result
}

func matchesFilter(event Event, filter Filter) bool {
	if !filter.StartTime.IsZero() && event.Timestamp.Before(filter.StartTime) {
		return false
	}
	if !filter.EndTime.IsZero() && event.Timestamp.After(filter.EndTime) {
		return false
	}
	if len(filter.EventTypes) > 0 {
		found := false
		for _, t := range filter.EventTypes {
			if event.EventType == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	if filter.RunID != "" && event.RunID != filter.RunID {
		return false
	}
	if filter.ToolName != "" && event.ToolName != filter.ToolName {
		return false
	}
	if filter.Success != nil && event.Success != *filter.Success {
		return false
	}
	return true
}

// JSONLogger writes events as JSON to an io.Writer.
type JSONLogger struct {
	mu      sync.Mutex
	writer  io.Writer
	encoder *json.Encoder
}

// NewJSONLogger creates a new JSON audit logger.
func NewJSONLogger(writer io.Writer) *JSONLogger {
	return &JSONLogger{
		writer:  writer,
		encoder: json.NewEncoder(writer),
	}
}

// Log records an event as JSON.
func (l *JSONLogger) Log(ctx context.Context, event Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	return l.encoder.Encode(event)
}

// Query is not supported by JSONLogger.
func (l *JSONLogger) Query(ctx context.Context, filter Filter) ([]Event, error) {
	return nil, nil
}

// Close releases resources.
func (l *JSONLogger) Close() error {
	if closer, ok := l.writer.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// MultiLogger logs to multiple loggers.
type MultiLogger struct {
	loggers []Logger
}

// NewMultiLogger creates a logger that writes to multiple loggers.
func NewMultiLogger(loggers ...Logger) *MultiLogger {
	return &MultiLogger{loggers: loggers}
}

// Log records an event to all loggers.
func (l *MultiLogger) Log(ctx context.Context, event Event) error {
	var firstErr error
	for _, logger := range l.loggers {
		if err := logger.Log(ctx, event); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// Query queries the first logger that supports it.
func (l *MultiLogger) Query(ctx context.Context, filter Filter) ([]Event, error) {
	for _, logger := range l.loggers {
		events, err := logger.Query(ctx, filter)
		if err == nil && events != nil {
			return events, nil
		}
	}
	return nil, nil
}

// Close closes all loggers.
func (l *MultiLogger) Close() error {
	var firstErr error
	for _, logger := range l.loggers {
		if err := logger.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// AuditMiddleware creates middleware that logs tool executions.
func AuditMiddleware(logger Logger) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			start := time.Now()

			result, err := next(ctx, execCtx)

			event := Event{
				Timestamp:  start,
				EventType:  EventToolExecution,
				ToolName:   execCtx.Tool.Name(),
				AgentState: string(execCtx.CurrentState),
				Duration:   time.Since(start),
				Success:    err == nil,
				Annotations: map[string]interface{}{
					"read_only":   execCtx.Tool.Annotations().ReadOnly,
					"destructive": execCtx.Tool.Annotations().Destructive,
				},
			}

			if err != nil {
				event.Error = err.Error()
			}

			// Log asynchronously to not block execution
			go func() { _ = logger.Log(ctx, event) }()

			return result, err
		}
	}
}
