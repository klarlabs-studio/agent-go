// Package rate provides rate limiting tools for agents.
package rate

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/pack"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// LimiterStore manages rate limiters.
type LimiterStore struct {
	mu       sync.RWMutex
	limiters map[string]*rateLimiter
}

type rateLimiter struct {
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
	mu         sync.Mutex
}

func (l *rateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(l.lastRefill).Seconds()
	l.tokens = min(l.maxTokens, l.tokens+elapsed*l.refillRate)
	l.lastRefill = now
}

func (l *rateLimiter) allow(n float64) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.refill()
	if l.tokens >= n {
		l.tokens -= n
		return true
	}
	return false
}

func (l *rateLimiter) waitTime(n float64) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.refill()
	if l.tokens >= n {
		return 0
	}
	deficit := n - l.tokens
	return time.Duration(deficit/l.refillRate*1000) * time.Millisecond
}

var store = &LimiterStore{
	limiters: make(map[string]*rateLimiter),
}

// Pack returns the rate limiting tools pack.
func Pack() *pack.Pack {
	return pack.NewBuilder("rate").
		WithDescription("Rate limiting tools").
		AddTools(
			createTool(),
			allowTool(),
			waitTool(),
			checkTool(),
			statusTool(),
			resetTool(),
			deleteTool(),
			listTool(),
			slidingWindowTool(),
			fixedWindowTool(),
			throttleTool(),
			batchAllowTool(),
		).
		AllowAllInState(agent.StateExplore).
		AllowAllInState(agent.StateAct).
		Build()
}

func createTool() tool.Tool {
	return tool.NewBuilder("rate_create").
		WithDescription("Create a rate limiter").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID    string  `json:"id"`
				Rate  float64 `json:"rate"`            // requests per second
				Burst float64 `json:"burst,omitempty"` // max burst size
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			burst := params.Burst
			if burst <= 0 {
				burst = params.Rate
			}

			limiter := &rateLimiter{
				tokens:     burst,
				maxTokens:  burst,
				refillRate: params.Rate,
				lastRefill: time.Now(),
			}

			store.mu.Lock()
			store.limiters[params.ID] = limiter
			store.mu.Unlock()

			result := map[string]any{
				"id":      params.ID,
				"rate":    params.Rate,
				"burst":   burst,
				"created": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func allowTool() tool.Tool {
	return tool.NewBuilder("rate_allow").
		WithDescription("Check if request is allowed by rate limiter").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID    string  `json:"id"`
				Count float64 `json:"count,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			count := params.Count
			if count <= 0 {
				count = 1
			}

			store.mu.RLock()
			limiter, ok := store.limiters[params.ID]
			store.mu.RUnlock()

			if !ok {
				result := map[string]any{
					"error": "limiter not found",
					"id":    params.ID,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			allowed := limiter.allow(count)

			limiter.mu.Lock()
			remaining := limiter.tokens
			limiter.mu.Unlock()

			result := map[string]any{
				"id":        params.ID,
				"allowed":   allowed,
				"remaining": remaining,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func waitTool() tool.Tool {
	return tool.NewBuilder("rate_wait").
		WithDescription("Wait until request is allowed").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID      string  `json:"id"`
				Count   float64 `json:"count,omitempty"`
				Timeout int     `json:"timeout_ms,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			count := params.Count
			if count <= 0 {
				count = 1
			}

			store.mu.RLock()
			limiter, ok := store.limiters[params.ID]
			store.mu.RUnlock()

			if !ok {
				result := map[string]any{
					"error": "limiter not found",
					"id":    params.ID,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			timeout := time.Duration(params.Timeout) * time.Millisecond
			if timeout <= 0 {
				timeout = 30 * time.Second
			}

			start := time.Now()
			deadline := start.Add(timeout)

			for time.Now().Before(deadline) {
				if limiter.allow(count) {
					result := map[string]any{
						"id":      params.ID,
						"allowed": true,
						"waited":  time.Since(start).Milliseconds(),
					}
					output, _ := json.Marshal(result)
					return tool.Result{Output: output}, nil
				}

				waitTime := limiter.waitTime(count)
				if waitTime > time.Until(deadline) {
					break
				}

				time.Sleep(min(waitTime, 100*time.Millisecond))
			}

			result := map[string]any{
				"id":      params.ID,
				"allowed": false,
				"timeout": true,
				"waited":  time.Since(start).Milliseconds(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func checkTool() tool.Tool {
	return tool.NewBuilder("rate_check").
		WithDescription("Check rate limit status without consuming tokens").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID    string  `json:"id"`
				Count float64 `json:"count,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			count := params.Count
			if count <= 0 {
				count = 1
			}

			store.mu.RLock()
			limiter, ok := store.limiters[params.ID]
			store.mu.RUnlock()

			if !ok {
				result := map[string]any{
					"error": "limiter not found",
					"id":    params.ID,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			limiter.mu.Lock()
			limiter.refill()
			available := limiter.tokens
			wouldAllow := available >= count
			waitTime := time.Duration(0)
			if !wouldAllow {
				deficit := count - available
				waitTime = time.Duration(deficit/limiter.refillRate*1000) * time.Millisecond
			}
			limiter.mu.Unlock()

			result := map[string]any{
				"id":           params.ID,
				"would_allow":  wouldAllow,
				"available":    available,
				"wait_time_ms": waitTime.Milliseconds(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func statusTool() tool.Tool {
	return tool.NewBuilder("rate_status").
		WithDescription("Get rate limiter status").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			store.mu.RLock()
			limiter, ok := store.limiters[params.ID]
			store.mu.RUnlock()

			if !ok {
				result := map[string]any{
					"error": "limiter not found",
					"id":    params.ID,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			limiter.mu.Lock()
			limiter.refill()
			result := map[string]any{
				"id":          params.ID,
				"tokens":      limiter.tokens,
				"max_tokens":  limiter.maxTokens,
				"refill_rate": limiter.refillRate,
				"utilization": 1 - (limiter.tokens / limiter.maxTokens),
			}
			limiter.mu.Unlock()

			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func resetTool() tool.Tool {
	return tool.NewBuilder("rate_reset").
		WithDescription("Reset rate limiter to full capacity").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			store.mu.RLock()
			limiter, ok := store.limiters[params.ID]
			store.mu.RUnlock()

			if !ok {
				result := map[string]any{
					"error": "limiter not found",
					"id":    params.ID,
				}
				output, _ := json.Marshal(result)
				return tool.Result{Output: output}, nil
			}

			limiter.mu.Lock()
			limiter.tokens = limiter.maxTokens
			limiter.lastRefill = time.Now()
			limiter.mu.Unlock()

			result := map[string]any{
				"id":    params.ID,
				"reset": true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func deleteTool() tool.Tool {
	return tool.NewBuilder("rate_delete").
		WithDescription("Delete a rate limiter").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID string `json:"id"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			store.mu.Lock()
			_, existed := store.limiters[params.ID]
			delete(store.limiters, params.ID)
			store.mu.Unlock()

			result := map[string]any{
				"id":      params.ID,
				"deleted": existed,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func listTool() tool.Tool {
	return tool.NewBuilder("rate_list").
		WithDescription("List all rate limiters").
		ReadOnly().
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			store.mu.RLock()
			var limiters []map[string]any
			for id, limiter := range store.limiters {
				limiter.mu.Lock()
				limiter.refill()
				limiters = append(limiters, map[string]any{
					"id":          id,
					"tokens":      limiter.tokens,
					"max_tokens":  limiter.maxTokens,
					"refill_rate": limiter.refillRate,
				})
				limiter.mu.Unlock()
			}
			store.mu.RUnlock()

			result := map[string]any{
				"limiters": limiters,
				"count":    len(limiters),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// Sliding window rate limiter data
type slidingWindow struct {
	mu       sync.Mutex
	window   time.Duration
	limit    int
	requests []time.Time
}

var slidingWindows = struct {
	mu      sync.RWMutex
	windows map[string]*slidingWindow
}{
	windows: make(map[string]*slidingWindow),
}

func slidingWindowTool() tool.Tool {
	return tool.NewBuilder("rate_sliding_window").
		WithDescription("Create/use a sliding window rate limiter").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID      string `json:"id"`
				Window  int    `json:"window_seconds,omitempty"`
				Limit   int    `json:"limit,omitempty"`
				Consume bool   `json:"consume,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			slidingWindows.mu.Lock()
			sw, ok := slidingWindows.windows[params.ID]
			if !ok {
				window := time.Duration(params.Window) * time.Second
				if window <= 0 {
					window = time.Minute
				}
				limit := params.Limit
				if limit <= 0 {
					limit = 60
				}
				sw = &slidingWindow{
					window:   window,
					limit:    limit,
					requests: []time.Time{},
				}
				slidingWindows.windows[params.ID] = sw
			}
			slidingWindows.mu.Unlock()

			sw.mu.Lock()
			now := time.Now()
			cutoff := now.Add(-sw.window)

			// Remove old requests
			var valid []time.Time
			for _, t := range sw.requests {
				if t.After(cutoff) {
					valid = append(valid, t)
				}
			}
			sw.requests = valid

			count := len(sw.requests)
			allowed := count < sw.limit

			if params.Consume && allowed {
				sw.requests = append(sw.requests, now)
				count++
			}

			remaining := sw.limit - count
			if remaining < 0 {
				remaining = 0
			}
			sw.mu.Unlock()

			result := map[string]any{
				"id":        params.ID,
				"allowed":   allowed,
				"remaining": remaining,
				"count":     count,
				"limit":     sw.limit,
				"window":    sw.window.Seconds(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

// Fixed window rate limiter data
type fixedWindow struct {
	mu          sync.Mutex
	window      time.Duration
	limit       int
	count       int
	windowStart time.Time
}

var fixedWindows = struct {
	mu      sync.RWMutex
	windows map[string]*fixedWindow
}{
	windows: make(map[string]*fixedWindow),
}

func fixedWindowTool() tool.Tool {
	return tool.NewBuilder("rate_fixed_window").
		WithDescription("Create/use a fixed window rate limiter").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID      string `json:"id"`
				Window  int    `json:"window_seconds,omitempty"`
				Limit   int    `json:"limit,omitempty"`
				Consume bool   `json:"consume,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			fixedWindows.mu.Lock()
			fw, ok := fixedWindows.windows[params.ID]
			if !ok {
				window := time.Duration(params.Window) * time.Second
				if window <= 0 {
					window = time.Minute
				}
				limit := params.Limit
				if limit <= 0 {
					limit = 60
				}
				fw = &fixedWindow{
					window:      window,
					limit:       limit,
					count:       0,
					windowStart: time.Now(),
				}
				fixedWindows.windows[params.ID] = fw
			}
			fixedWindows.mu.Unlock()

			fw.mu.Lock()
			now := time.Now()

			// Check if window has expired
			if now.Sub(fw.windowStart) >= fw.window {
				fw.count = 0
				fw.windowStart = now
			}

			allowed := fw.count < fw.limit
			if params.Consume && allowed {
				fw.count++
			}

			remaining := fw.limit - fw.count
			if remaining < 0 {
				remaining = 0
			}

			resetIn := fw.window - now.Sub(fw.windowStart)
			fw.mu.Unlock()

			result := map[string]any{
				"id":          params.ID,
				"allowed":     allowed,
				"remaining":   remaining,
				"count":       fw.count,
				"limit":       fw.limit,
				"reset_in_ms": resetIn.Milliseconds(),
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func throttleTool() tool.Tool {
	return tool.NewBuilder("rate_throttle").
		WithDescription("Throttle requests with delay").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				ID          string `json:"id"`
				MinInterval int    `json:"min_interval_ms,omitempty"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			minInterval := time.Duration(params.MinInterval) * time.Millisecond
			if minInterval <= 0 {
				minInterval = 100 * time.Millisecond
			}

			// Convert to token bucket rate
			rate := 1000.0 / float64(params.MinInterval)

			store.mu.Lock()
			limiter, ok := store.limiters[params.ID]
			if !ok {
				limiter = &rateLimiter{
					tokens:     1,
					maxTokens:  1,
					refillRate: rate,
					lastRefill: time.Now(),
				}
				store.limiters[params.ID] = limiter
			}
			store.mu.Unlock()

			// Wait if needed
			waited := time.Duration(0)
			for !limiter.allow(1) {
				time.Sleep(10 * time.Millisecond)
				waited += 10 * time.Millisecond
				if waited > 5*time.Second {
					break
				}
			}

			result := map[string]any{
				"id":           params.ID,
				"min_interval": minInterval.Milliseconds(),
				"waited_ms":    waited.Milliseconds(),
				"allowed":      true,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}

func batchAllowTool() tool.Tool {
	return tool.NewBuilder("rate_batch_allow").
		WithDescription("Check multiple rate limiters at once").
		WithHandler(func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
			var params struct {
				IDs []string `json:"ids"`
			}
			if err := json.Unmarshal(input, &params); err != nil {
				return tool.Result{}, err
			}

			store.mu.RLock()
			results := make(map[string]bool)
			allAllowed := true

			for _, id := range params.IDs {
				limiter, ok := store.limiters[id]
				if !ok {
					results[id] = false
					allAllowed = false
					continue
				}

				allowed := limiter.allow(1)
				results[id] = allowed
				if !allowed {
					allAllowed = false
				}
			}
			store.mu.RUnlock()

			result := map[string]any{
				"results":     results,
				"all_allowed": allAllowed,
			}
			output, _ := json.Marshal(result)
			return tool.Result{Output: output}, nil
		}).
		MustBuild()
}
