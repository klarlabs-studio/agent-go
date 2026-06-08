// Package retry provides retry logic tools for agents.
package retry

import (
	"context"
	"encoding/json"
	"math"
	"math/rand"
	"sync"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/tool"
)

// RetryState tracks retry attempts.
type RetryState struct {
	mu     sync.RWMutex
	states map[string]*retryContext
}

type retryContext struct {
	ID          string    `json:"id"`
	MaxAttempts int       `json:"max_attempts"`
	Attempts    int       `json:"attempts"`
	LastAttempt time.Time `json:"last_attempt,omitempty"`
	NextAttempt time.Time `json:"next_attempt,omitempty"`
	BaseDelay   int       `json:"base_delay_ms"`
	MaxDelay    int       `json:"max_delay_ms"`
	Multiplier  float64   `json:"multiplier"`
	Jitter      float64   `json:"jitter"`
	Success     bool      `json:"success"`
	LastError   string    `json:"last_error,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

var state = &RetryState{
	states: make(map[string]*retryContext),
}

// Pack returns the retry tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("retry").
		WithDescription("Retry logic tools").
		AddTools(
			createTool(),
			attemptTool(),
			successTool(),
			failTool(),
			resetTool(),
			statusTool(),
			shouldRetryTool(),
			nextDelayTool(),
			listTool(),
			deleteTool(),
			calculateDelayTool(),
			exponentialTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func createTool() tool.Tool {
	return tool.NewBuilder("retry_create").
		WithDescription("Create a retry context").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID          string  `json:"id"`
				MaxAttempts int     `json:"max_attempts,omitempty"`
				BaseDelay   int     `json:"base_delay_ms,omitempty"`
				MaxDelay    int     `json:"max_delay_ms,omitempty"`
				Multiplier  float64 `json:"multiplier,omitempty"`
				Jitter      float64 `json:"jitter,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			// Set defaults
			if params.MaxAttempts == 0 {
				params.MaxAttempts = 3
			}
			if params.BaseDelay == 0 {
				params.BaseDelay = 1000
			}
			if params.MaxDelay == 0 {
				params.MaxDelay = 30000
			}
			if params.Multiplier == 0 {
				params.Multiplier = 2.0
			}

			rc := &retryContext{
				ID:          params.ID,
				MaxAttempts: params.MaxAttempts,
				Attempts:    0,
				BaseDelay:   params.BaseDelay,
				MaxDelay:    params.MaxDelay,
				Multiplier:  params.Multiplier,
				Jitter:      params.Jitter,
				CreatedAt:   time.Now(),
			}

			state.mu.Lock()
			state.states[params.ID] = rc
			state.mu.Unlock()

			result := map[string]any{
				"id":           rc.ID,
				"max_attempts": rc.MaxAttempts,
				"base_delay":   rc.BaseDelay,
				"created":      true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func attemptTool() tool.Tool {
	return tool.NewBuilder("retry_attempt").
		WithDescription("Record a retry attempt").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			state.mu.Lock()
			rc, ok := state.states[params.ID]
			if !ok {
				state.mu.Unlock()
				result := map[string]any{
					"id":    params.ID,
					"error": "retry context not found",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			rc.Attempts++
			rc.LastAttempt = time.Now()

			// Calculate next delay
			delay := calculateDelay(rc.BaseDelay, rc.Attempts, rc.Multiplier, rc.MaxDelay, rc.Jitter)
			rc.NextAttempt = time.Now().Add(time.Duration(delay) * time.Millisecond)

			attempts := rc.Attempts
			maxAttempts := rc.MaxAttempts
			state.mu.Unlock()

			result := map[string]any{
				"id":           params.ID,
				"attempt":      attempts,
				"max_attempts": maxAttempts,
				"remaining":    maxAttempts - attempts,
				"next_delay":   delay,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func successTool() tool.Tool {
	return tool.NewBuilder("retry_success").
		WithDescription("Mark retry as successful").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			state.mu.Lock()
			rc, ok := state.states[params.ID]
			if ok {
				rc.Success = true
				rc.LastError = ""
			}
			state.mu.Unlock()

			result := map[string]any{
				"id":      params.ID,
				"success": ok,
			}
			if ok {
				result["attempts"] = rc.Attempts
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func failTool() tool.Tool {
	return tool.NewBuilder("retry_fail").
		WithDescription("Record a retry failure").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID    string `json:"id"`
				Error string `json:"error,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			state.mu.Lock()
			rc, ok := state.states[params.ID]
			if ok {
				rc.Success = false
				rc.LastError = params.Error
			}
			state.mu.Unlock()

			if !ok {
				result := map[string]any{
					"id":    params.ID,
					"error": "retry context not found",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			canRetry := rc.Attempts < rc.MaxAttempts
			var nextDelay int
			if canRetry {
				nextDelay = calculateDelay(rc.BaseDelay, rc.Attempts, rc.Multiplier, rc.MaxDelay, rc.Jitter)
			}

			result := map[string]any{
				"id":         params.ID,
				"attempts":   rc.Attempts,
				"can_retry":  canRetry,
				"next_delay": nextDelay,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func resetTool() tool.Tool {
	return tool.NewBuilder("retry_reset").
		WithDescription("Reset retry attempts").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			state.mu.Lock()
			rc, ok := state.states[params.ID]
			if ok {
				rc.Attempts = 0
				rc.Success = false
				rc.LastError = ""
				rc.LastAttempt = time.Time{}
				rc.NextAttempt = time.Time{}
			}
			state.mu.Unlock()

			result := map[string]any{
				"id":    params.ID,
				"reset": ok,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func statusTool() tool.Tool {
	return tool.NewBuilder("retry_status").
		WithDescription("Get retry status").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			state.mu.RLock()
			rc, ok := state.states[params.ID]
			state.mu.RUnlock()

			if !ok {
				result := map[string]any{
					"id":    params.ID,
					"found": false,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			result := map[string]any{
				"id":           rc.ID,
				"found":        true,
				"attempts":     rc.Attempts,
				"max_attempts": rc.MaxAttempts,
				"success":      rc.Success,
				"last_error":   rc.LastError,
				"can_retry":    rc.Attempts < rc.MaxAttempts && !rc.Success,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func shouldRetryTool() tool.Tool {
	return tool.NewBuilder("retry_should_retry").
		WithDescription("Check if should retry").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			state.mu.RLock()
			rc, ok := state.states[params.ID]
			state.mu.RUnlock()

			if !ok {
				result := map[string]any{
					"id":           params.ID,
					"should_retry": false,
					"reason":       "context not found",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			shouldRetry := rc.Attempts < rc.MaxAttempts && !rc.Success
			var reason string
			switch {
			case rc.Success:
				reason = "already succeeded"
			case rc.Attempts >= rc.MaxAttempts:
				reason = "max attempts reached"
			default:
				reason = "can retry"
			}

			result := map[string]any{
				"id":           params.ID,
				"should_retry": shouldRetry,
				"reason":       reason,
				"remaining":    rc.MaxAttempts - rc.Attempts,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func nextDelayTool() tool.Tool {
	return tool.NewBuilder("retry_next_delay").
		WithDescription("Get next retry delay").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			state.mu.RLock()
			rc, ok := state.states[params.ID]
			state.mu.RUnlock()

			if !ok {
				result := map[string]any{
					"id":    params.ID,
					"error": "context not found",
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			delay := calculateDelay(rc.BaseDelay, rc.Attempts+1, rc.Multiplier, rc.MaxDelay, rc.Jitter)

			result := map[string]any{
				"id":          params.ID,
				"delay_ms":    delay,
				"delay_sec":   float64(delay) / 1000,
				"for_attempt": rc.Attempts + 1,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func listTool() tool.Tool {
	return tool.NewBuilder("retry_list").
		WithDescription("List all retry contexts").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			state.mu.RLock()
			var contexts []map[string]any
			for _, rc := range state.states {
				contexts = append(contexts, map[string]any{
					"id":           rc.ID,
					"attempts":     rc.Attempts,
					"max_attempts": rc.MaxAttempts,
					"success":      rc.Success,
					"can_retry":    rc.Attempts < rc.MaxAttempts && !rc.Success,
				})
			}
			state.mu.RUnlock()

			result := map[string]any{
				"contexts": contexts,
				"count":    len(contexts),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func deleteTool() tool.Tool {
	return tool.NewBuilder("retry_delete").
		WithDescription("Delete a retry context").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			state.mu.Lock()
			_, ok := state.states[params.ID]
			if ok {
				delete(state.states, params.ID)
			}
			state.mu.Unlock()

			result := map[string]any{
				"id":      params.ID,
				"deleted": ok,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func calculateDelayTool() tool.Tool {
	return tool.NewBuilder("retry_calculate_delay").
		WithDescription("Calculate exponential backoff delay").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				BaseDelay  int     `json:"base_delay_ms"`
				Attempt    int     `json:"attempt"`
				Multiplier float64 `json:"multiplier,omitempty"`
				MaxDelay   int     `json:"max_delay_ms,omitempty"`
				Jitter     float64 `json:"jitter,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Multiplier == 0 {
				params.Multiplier = 2.0
			}
			if params.MaxDelay == 0 {
				params.MaxDelay = 30000
			}

			delay := calculateDelay(params.BaseDelay, params.Attempt, params.Multiplier, params.MaxDelay, params.Jitter)

			result := map[string]any{
				"delay_ms":  delay,
				"delay_sec": float64(delay) / 1000,
				"attempt":   params.Attempt,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func exponentialTool() tool.Tool {
	return tool.NewBuilder("retry_exponential").
		WithDescription("Get exponential backoff sequence").
		ReadOnly().
		Idempotent().
		Cacheable().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				BaseDelay   int     `json:"base_delay_ms"`
				MaxAttempts int     `json:"max_attempts"`
				Multiplier  float64 `json:"multiplier,omitempty"`
				MaxDelay    int     `json:"max_delay_ms,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			if params.Multiplier == 0 {
				params.Multiplier = 2.0
			}
			if params.MaxDelay == 0 {
				params.MaxDelay = 30000
			}
			if params.MaxAttempts == 0 {
				params.MaxAttempts = 5
			}

			var delays []int
			var totalDelay int
			for i := 1; i <= params.MaxAttempts; i++ {
				delay := calculateDelay(params.BaseDelay, i, params.Multiplier, params.MaxDelay, 0)
				delays = append(delays, delay)
				totalDelay += delay
			}

			result := map[string]any{
				"delays":       delays,
				"total_ms":     totalDelay,
				"total_sec":    float64(totalDelay) / 1000,
				"max_attempts": params.MaxAttempts,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// calculateDelay computes the delay for a given attempt with exponential backoff.
func calculateDelay(baseDelay int, attempt int, multiplier float64, maxDelay int, jitter float64) int {
	if attempt <= 0 {
		attempt = 1
	}

	delay := float64(baseDelay) * math.Pow(multiplier, float64(attempt-1))

	if jitter > 0 {
		jitterAmount := delay * jitter
		delay += (rand.Float64()*2 - 1) * jitterAmount
	}

	if delay > float64(maxDelay) {
		delay = float64(maxDelay)
	}

	if delay < 0 {
		delay = 0
	}

	return int(delay)
}
