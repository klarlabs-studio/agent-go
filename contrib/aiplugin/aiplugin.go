// Package aiplugin provides plugins for AI/LLM-driven state machines.
//
// These plugins build on the core plugin system to add observability,
// retry semantics, and prompt-snapshotting useful for LLM-backed
// workflows. The package is import-light: no SDK dependencies, no
// network calls. Instrumentation is your responsibility — these
// plugins record what passes through the interpreter, so feed them
// usage data via well-known event-payload keys.
//
// Conventions for event payloads:
//
//   - "prompt"            string — the prompt sent to the model
//   - "response"          string — the response received
//   - "model"             string — the model identifier
//   - "input_tokens"      int    — input token count
//   - "output_tokens"     int    — output token count
//   - "input_cost_usd"    float64 — input cost in USD
//   - "output_cost_usd"   float64 — output cost in USD
//
// All keys are optional; unknown payload shapes are skipped silently.
package aiplugin

import (
	"sync"
	"sync/atomic"

	"go.klarlabs.de/statekit/plugin"
)

// Payload keys recognized by aiplugin plugins.
const (
	KeyPrompt       = "prompt"
	KeyResponse     = "response"
	KeyModel        = "model"
	KeyInputTokens  = "input_tokens"
	KeyOutputTokens = "output_tokens"
	KeyInputCost    = "input_cost_usd"
	KeyOutputCost   = "output_cost_usd"
)

// TokenCounter counts input + output tokens and accumulates cost from
// event payloads. Safe for concurrent use.
type TokenCounter[C any] struct {
	inputTokens  atomic.Int64
	outputTokens atomic.Int64

	mu       sync.Mutex
	inputUSD float64
	outUSD   float64
}

// NewTokenCounter constructs a TokenCounter[C].
func NewTokenCounter[C any]() *TokenCounter[C] { return &TokenCounter[C]{} }

// Name implements plugin.Plugin.
func (*TokenCounter[C]) Name() string { return "ai-token-counter" }

// OnEvent implements plugin.OnEventHook. It does not modify the event.
func (t *TokenCounter[C]) OnEvent(_ plugin.Context[C], event plugin.Event) plugin.Event {
	payload, ok := event.Payload.(map[string]any)
	if !ok {
		return event
	}
	if v, ok := payload[KeyInputTokens]; ok {
		if n, ok := toInt64(v); ok {
			t.inputTokens.Add(n)
		}
	}
	if v, ok := payload[KeyOutputTokens]; ok {
		if n, ok := toInt64(v); ok {
			t.outputTokens.Add(n)
		}
	}
	if v, ok := payload[KeyInputCost]; ok {
		if f, ok := toFloat64(v); ok {
			t.mu.Lock()
			t.inputUSD += f
			t.mu.Unlock()
		}
	}
	if v, ok := payload[KeyOutputCost]; ok {
		if f, ok := toFloat64(v); ok {
			t.mu.Lock()
			t.outUSD += f
			t.mu.Unlock()
		}
	}
	return event
}

// InputTokens returns the running input-token count.
func (t *TokenCounter[C]) InputTokens() int64 { return t.inputTokens.Load() }

// OutputTokens returns the running output-token count.
func (t *TokenCounter[C]) OutputTokens() int64 { return t.outputTokens.Load() }

// TotalTokens returns the sum of input and output tokens.
func (t *TokenCounter[C]) TotalTokens() int64 {
	return t.inputTokens.Load() + t.outputTokens.Load()
}

// CostUSD returns the running total cost in USD.
func (t *TokenCounter[C]) CostUSD() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.inputUSD + t.outUSD
}

// Reset zeroes all counters.
func (t *TokenCounter[C]) Reset() {
	t.inputTokens.Store(0)
	t.outputTokens.Store(0)
	t.mu.Lock()
	t.inputUSD = 0
	t.outUSD = 0
	t.mu.Unlock()
}

// PromptSnapshot is one captured prompt/response pair from an event.
type PromptSnapshot struct {
	Model    string
	Prompt   string
	Response string
}

// PromptRecorder appends a PromptSnapshot for every event whose payload
// has a non-empty prompt or response. Useful for replay-based agent
// debugging.
type PromptRecorder[C any] struct {
	mu        sync.Mutex
	snapshots []PromptSnapshot
}

// NewPromptRecorder constructs a PromptRecorder[C].
func NewPromptRecorder[C any]() *PromptRecorder[C] { return &PromptRecorder[C]{} }

// Name implements plugin.Plugin.
func (*PromptRecorder[C]) Name() string { return "ai-prompt-recorder" }

// OnEvent implements plugin.OnEventHook. Captures prompt/response from
// the event payload if present; otherwise no-op.
func (r *PromptRecorder[C]) OnEvent(_ plugin.Context[C], event plugin.Event) plugin.Event {
	payload, ok := event.Payload.(map[string]any)
	if !ok {
		return event
	}
	prompt, _ := payload[KeyPrompt].(string)
	response, _ := payload[KeyResponse].(string)
	model, _ := payload[KeyModel].(string)
	if prompt == "" && response == "" {
		return event
	}
	r.mu.Lock()
	r.snapshots = append(r.snapshots, PromptSnapshot{
		Model:    model,
		Prompt:   prompt,
		Response: response,
	})
	r.mu.Unlock()
	return event
}

// Snapshots returns a copy of all captured snapshots.
func (r *PromptRecorder[C]) Snapshots() []PromptSnapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]PromptSnapshot, len(r.snapshots))
	copy(out, r.snapshots)
	return out
}

// Len returns the number of captured snapshots.
func (r *PromptRecorder[C]) Len() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.snapshots)
}

// Reset clears all captured snapshots.
func (r *PromptRecorder[C]) Reset() {
	r.mu.Lock()
	r.snapshots = nil
	r.mu.Unlock()
}

// toInt64 best-effort converts a JSON-decoded number to int64.
func toInt64(v any) (int64, bool) {
	switch n := v.(type) {
	case int:
		return int64(n), true
	case int32:
		return int64(n), true
	case int64:
		return n, true
	case float32:
		return int64(n), true
	case float64:
		return int64(n), true
	}
	return 0, false
}

// toFloat64 best-effort converts a JSON-decoded number to float64.
func toFloat64(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	}
	return 0, false
}
