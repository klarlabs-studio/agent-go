package copilot

import (
	"encoding/json"
	"sync"
)

// SessionHandler manages tool execution for a Copilot session.
// It provides logging, metrics, and execution control.
type SessionHandler struct {
	adapter     *Adapter
	session     *Session
	executions  map[string]*Execution
	mu          sync.RWMutex
	onExecution ExecutionCallback
}

// Execution represents a single tool execution.
type Execution struct {
	ToolCallID string
	ToolName   string
	Arguments  interface{}
	Result     *ToolResult
	Error      error
	Completed  bool
}

// ExecutionCallback is called when a tool execution completes.
type ExecutionCallback func(execution *Execution)

// SessionHandlerOption configures the SessionHandler.
type SessionHandlerOption func(*SessionHandler)

// WithExecutionCallback sets a callback for execution completion.
func WithExecutionCallback(cb ExecutionCallback) SessionHandlerOption {
	return func(h *SessionHandler) {
		h.onExecution = cb
	}
}

// NewSessionHandler creates a new session handler.
func NewSessionHandler(adapter *Adapter, session *Session, opts ...SessionHandlerOption) *SessionHandler {
	h := &SessionHandler{
		adapter:    adapter,
		session:    session,
		executions: make(map[string]*Execution),
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// GetExecution retrieves an execution by tool call ID.
func (h *SessionHandler) GetExecution(toolCallID string) (*Execution, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	exec, ok := h.executions[toolCallID]
	return exec, ok
}

// GetExecutions retrieves all executions.
func (h *SessionHandler) GetExecutions() []*Execution {
	h.mu.RLock()
	defer h.mu.RUnlock()

	result := make([]*Execution, 0, len(h.executions))
	for _, exec := range h.executions {
		result = append(result, exec)
	}
	return result
}

// RecordExecution records a tool execution for tracking.
func (h *SessionHandler) RecordExecution(exec *Execution) {
	h.mu.Lock()
	h.executions[exec.ToolCallID] = exec
	h.mu.Unlock()

	if h.onExecution != nil {
		h.onExecution(exec)
	}
}

// Clear removes all recorded executions.
func (h *SessionHandler) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.executions = make(map[string]*Execution)
}

// marshalArguments converts tool arguments to JSON.
func marshalArguments(args interface{}) (json.RawMessage, error) {
	if args == nil {
		return json.RawMessage("{}"), nil
	}

	if raw, ok := args.(json.RawMessage); ok {
		return raw, nil
	}

	if s, ok := args.(string); ok {
		if json.Valid([]byte(s)) {
			return json.RawMessage(s), nil
		}
		return json.Marshal(s)
	}

	return json.Marshal(args)
}

// ToolMetrics provides metrics about tool usage.
type ToolMetrics struct {
	TotalExecutions      int
	SuccessfulExecutions int
	FailedExecutions     int
	ToolCounts           map[string]int
}

// GetMetrics calculates metrics from recorded executions.
func (h *SessionHandler) GetMetrics() ToolMetrics {
	h.mu.RLock()
	defer h.mu.RUnlock()

	metrics := ToolMetrics{
		ToolCounts: make(map[string]int),
	}

	for _, exec := range h.executions {
		if !exec.Completed {
			continue
		}
		metrics.TotalExecutions++
		if exec.Error != nil || (exec.Result != nil && exec.Result.ResultType == "error") {
			metrics.FailedExecutions++
		} else {
			metrics.SuccessfulExecutions++
		}
		metrics.ToolCounts[exec.ToolName]++
	}

	return metrics
}

// ToolStats provides statistics about a specific tool.
type ToolStats struct {
	Name       string
	Executions int
	Successes  int
	Failures   int
}

// GetToolStats returns statistics for a specific tool.
func (h *SessionHandler) GetToolStats(toolName string) ToolStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	stats := ToolStats{Name: toolName}

	for _, exec := range h.executions {
		if exec.ToolName != toolName || !exec.Completed {
			continue
		}
		stats.Executions++
		if exec.Error != nil || (exec.Result != nil && exec.Result.ResultType == "error") {
			stats.Failures++
		} else {
			stats.Successes++
		}
	}

	return stats
}
