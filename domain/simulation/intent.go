package simulation

import (
	"encoding/json"
	"sync"
	"time"

	"go.klarlabs.de/agent/domain/agent"
)

// Intent represents a tool execution intent in simulation mode.
type Intent struct {
	// ToolName is the name of the tool that would be executed.
	ToolName string `json:"tool_name"`

	// Input is the JSON input that would be passed to the tool.
	Input json.RawMessage `json:"input"`

	// State is the agent state when the tool would be called.
	State agent.State `json:"state"`

	// Timestamp is when the intent was recorded.
	Timestamp time.Time `json:"timestamp"`

	// Blocked indicates if the tool was blocked from execution.
	Blocked bool `json:"blocked"`

	// BlockReason explains why the tool was blocked (if applicable).
	BlockReason string `json:"block_reason,omitempty"`

	// MockResult indicates if a mock result was returned.
	MockResult bool `json:"mock_result"`
}

// IntentRecorder records tool execution intents.
type IntentRecorder interface {
	// Record captures a tool execution intent.
	Record(intent Intent)

	// Intents returns all recorded intents.
	Intents() []Intent

	// Clear removes all recorded intents.
	Clear()
}

// MemoryRecorder is an in-memory intent recorder.
type MemoryRecorder struct {
	intents []Intent
	mu      sync.RWMutex
}

// NewMemoryRecorder creates a new in-memory recorder.
func NewMemoryRecorder() *MemoryRecorder {
	return &MemoryRecorder{
		intents: make([]Intent, 0),
	}
}

// Record captures a tool execution intent.
func (r *MemoryRecorder) Record(intent Intent) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.intents = append(r.intents, intent)
}

// Intents returns all recorded intents.
func (r *MemoryRecorder) Intents() []Intent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]Intent, len(r.intents))
	copy(result, r.intents)
	return result
}

// Clear removes all recorded intents.
func (r *MemoryRecorder) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.intents = r.intents[:0]
}

// Len returns the number of recorded intents.
func (r *MemoryRecorder) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.intents)
}

// IntentsByTool returns intents grouped by tool name.
func (r *MemoryRecorder) IntentsByTool() map[string][]Intent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make(map[string][]Intent)
	for _, intent := range r.intents {
		result[intent.ToolName] = append(result[intent.ToolName], intent)
	}
	return result
}

// BlockedIntents returns only blocked intents.
func (r *MemoryRecorder) BlockedIntents() []Intent {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var result []Intent
	for _, intent := range r.intents {
		if intent.Blocked {
			result = append(result, intent)
		}
	}
	return result
}

// Ensure MemoryRecorder implements IntentRecorder
var _ IntentRecorder = (*MemoryRecorder)(nil)
