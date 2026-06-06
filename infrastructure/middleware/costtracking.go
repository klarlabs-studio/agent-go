package middleware

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/tool"
)

// CostUnit represents a unit of cost measurement.
type CostUnit string

const (
	// CostUnitTokens measures cost in tokens (for LLM operations).
	CostUnitTokens CostUnit = "tokens"
	// CostUnitRequests measures cost in API requests.
	CostUnitRequests CostUnit = "requests"
	// CostUnitBytes measures cost in bytes transferred.
	CostUnitBytes CostUnit = "bytes"
	// CostUnitDuration measures cost in execution time.
	CostUnitDuration CostUnit = "duration_ms"
	// CostUnitCredits measures cost in service credits.
	CostUnitCredits CostUnit = "credits"
	// CostUnitDollars measures cost in US dollars.
	CostUnitDollars CostUnit = "usd"
)

// CostEntry records a single cost event.
type CostEntry struct {
	RunID     string            `json:"run_id"`
	ToolName  string            `json:"tool_name"`
	Unit      CostUnit          `json:"unit"`
	Amount    float64           `json:"amount"`
	Timestamp time.Time         `json:"timestamp"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// CostSummary provides aggregated cost information.
type CostSummary struct {
	RunID       string               `json:"run_id"`
	TotalByUnit map[CostUnit]float64 `json:"total_by_unit"`
	ByTool      map[string]ToolCost  `json:"by_tool"`
	EntryCount  int                  `json:"entry_count"`
	StartTime   time.Time            `json:"start_time"`
	EndTime     time.Time            `json:"end_time"`
}

// ToolCost contains cost information for a specific tool.
type ToolCost struct {
	ToolName    string               `json:"tool_name"`
	Invocations int                  `json:"invocations"`
	TotalByUnit map[CostUnit]float64 `json:"total_by_unit"`
}

// CostCalculator calculates costs for tool executions.
type CostCalculator func(ctx context.Context, execCtx *middleware.ExecutionContext, result tool.Result, duration time.Duration) []CostEntry

// CostTrackingConfig configures the cost tracking middleware.
type CostTrackingConfig struct {
	// Calculator computes costs for each tool execution.
	// If nil, uses default calculator (request count + duration).
	Calculator CostCalculator

	// Store receives cost entries for persistence.
	Store CostStore

	// OnCostRecorded is called after each cost entry is recorded.
	OnCostRecorded func(entry CostEntry)

	// EnabledUnits restricts which cost units are tracked.
	// If empty, all units are tracked.
	EnabledUnits []CostUnit

	// IncludeMetadata adds execution context metadata to entries.
	IncludeMetadata bool

	// TrackErrors includes failed executions in cost tracking.
	TrackErrors bool
}

// CostStore persists cost entries.
type CostStore interface {
	// Record stores a cost entry.
	Record(ctx context.Context, entry CostEntry) error

	// GetSummary returns aggregated costs for a run.
	GetSummary(ctx context.Context, runID string) (*CostSummary, error)

	// ListEntries returns cost entries for a run.
	ListEntries(ctx context.Context, runID string, limit int) ([]CostEntry, error)
}

// DefaultCostTrackingConfig returns a sensible default cost tracking configuration.
func DefaultCostTrackingConfig() CostTrackingConfig {
	return CostTrackingConfig{
		Calculator:      defaultCostCalculator,
		IncludeMetadata: true,
		TrackErrors:     false,
	}
}

// CostTrackingOption configures the cost tracking middleware.
type CostTrackingOption func(*CostTrackingConfig)

// WithCostCalculator sets the cost calculator.
func WithCostCalculator(calc CostCalculator) CostTrackingOption {
	return func(c *CostTrackingConfig) {
		c.Calculator = calc
	}
}

// WithCostStore sets the cost store.
func WithCostStore(store CostStore) CostTrackingOption {
	return func(c *CostTrackingConfig) {
		c.Store = store
	}
}

// WithCostCallback sets the callback for recorded costs.
func WithCostCallback(callback func(CostEntry)) CostTrackingOption {
	return func(c *CostTrackingConfig) {
		c.OnCostRecorded = callback
	}
}

// WithEnabledCostUnits sets which cost units to track.
func WithEnabledCostUnits(units ...CostUnit) CostTrackingOption {
	return func(c *CostTrackingConfig) {
		c.EnabledUnits = units
	}
}

// WithCostMetadata enables/disables metadata in cost entries.
func WithCostMetadata(enabled bool) CostTrackingOption {
	return func(c *CostTrackingConfig) {
		c.IncludeMetadata = enabled
	}
}

// WithErrorTracking enables/disables tracking costs for failed executions.
func WithErrorTracking(enabled bool) CostTrackingOption {
	return func(c *CostTrackingConfig) {
		c.TrackErrors = enabled
	}
}

// CostTracking returns middleware that tracks costs for tool executions.
func CostTracking(cfg CostTrackingConfig) middleware.Middleware {
	if cfg.Calculator == nil {
		cfg.Calculator = defaultCostCalculator
	}

	enabledUnits := make(map[CostUnit]bool)
	for _, unit := range cfg.EnabledUnits {
		enabledUnits[unit] = true
	}

	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			start := time.Now()

			// Execute the tool
			result, err := next(ctx, execCtx)

			duration := time.Since(start)

			// Skip tracking on error if not enabled
			if err != nil && !cfg.TrackErrors {
				return result, err
			}

			// Calculate costs
			entries := cfg.Calculator(ctx, execCtx, result, duration)

			// Filter and process entries
			for _, entry := range entries {
				// Filter by enabled units
				if len(enabledUnits) > 0 && !enabledUnits[entry.Unit] {
					continue
				}

				// Add metadata if enabled
				if cfg.IncludeMetadata {
					if entry.Metadata == nil {
						entry.Metadata = make(map[string]string)
					}
					entry.Metadata["state"] = execCtx.CurrentState.String()
					if err != nil {
						entry.Metadata["error"] = "true"
					}
				}

				// Store entry
				if cfg.Store != nil {
					_ = cfg.Store.Record(ctx, entry)
				}

				// Call callback
				if cfg.OnCostRecorded != nil {
					cfg.OnCostRecorded(entry)
				}
			}

			return result, err
		}
	}
}

// NewCostTracking creates cost tracking middleware with options.
func NewCostTracking(opts ...CostTrackingOption) middleware.Middleware {
	cfg := DefaultCostTrackingConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return CostTracking(cfg)
}

// defaultCostCalculator provides basic cost tracking.
func defaultCostCalculator(ctx context.Context, execCtx *middleware.ExecutionContext, result tool.Result, duration time.Duration) []CostEntry {
	now := time.Now()

	entries := []CostEntry{
		{
			RunID:     execCtx.RunID,
			ToolName:  execCtx.Tool.Name(),
			Unit:      CostUnitRequests,
			Amount:    1,
			Timestamp: now,
		},
		{
			RunID:     execCtx.RunID,
			ToolName:  execCtx.Tool.Name(),
			Unit:      CostUnitDuration,
			Amount:    float64(duration.Milliseconds()),
			Timestamp: now,
		},
	}

	// Track input/output size
	if len(execCtx.Input) > 0 {
		entries = append(entries, CostEntry{
			RunID:     execCtx.RunID,
			ToolName:  execCtx.Tool.Name(),
			Unit:      CostUnitBytes,
			Amount:    float64(len(execCtx.Input) + len(result.Output)),
			Timestamp: now,
		})
	}

	return entries
}

// MemoryCostStore is an in-memory implementation of CostStore.
type MemoryCostStore struct {
	mu      sync.RWMutex
	entries map[string][]CostEntry // runID -> entries
}

// NewMemoryCostStore creates a new in-memory cost store.
func NewMemoryCostStore() *MemoryCostStore {
	return &MemoryCostStore{
		entries: make(map[string][]CostEntry),
	}
}

// Record stores a cost entry.
func (s *MemoryCostStore) Record(ctx context.Context, entry CostEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.entries[entry.RunID] == nil {
		s.entries[entry.RunID] = make([]CostEntry, 0)
	}
	s.entries[entry.RunID] = append(s.entries[entry.RunID], entry)
	return nil
}

// GetSummary returns aggregated costs for a run.
func (s *MemoryCostStore) GetSummary(ctx context.Context, runID string) (*CostSummary, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := s.entries[runID]
	if len(entries) == 0 {
		return &CostSummary{
			RunID:       runID,
			TotalByUnit: make(map[CostUnit]float64),
			ByTool:      make(map[string]ToolCost),
		}, nil
	}

	summary := &CostSummary{
		RunID:       runID,
		TotalByUnit: make(map[CostUnit]float64),
		ByTool:      make(map[string]ToolCost),
		EntryCount:  len(entries),
		StartTime:   entries[0].Timestamp,
		EndTime:     entries[len(entries)-1].Timestamp,
	}

	for _, entry := range entries {
		// Aggregate by unit
		summary.TotalByUnit[entry.Unit] += entry.Amount

		// Aggregate by tool
		toolCost, ok := summary.ByTool[entry.ToolName]
		if !ok {
			toolCost = ToolCost{
				ToolName:    entry.ToolName,
				TotalByUnit: make(map[CostUnit]float64),
			}
		}
		toolCost.TotalByUnit[entry.Unit] += entry.Amount
		if entry.Unit == CostUnitRequests {
			toolCost.Invocations += int(entry.Amount)
		}
		summary.ByTool[entry.ToolName] = toolCost
	}

	return summary, nil
}

// ListEntries returns cost entries for a run.
func (s *MemoryCostStore) ListEntries(ctx context.Context, runID string, limit int) ([]CostEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries := s.entries[runID]
	if limit > 0 && limit < len(entries) {
		return entries[:limit], nil
	}
	return entries, nil
}

// Clear removes all entries for a run.
func (s *MemoryCostStore) Clear(runID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.entries, runID)
}

// ClearAll removes all entries.
func (s *MemoryCostStore) ClearAll() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.entries = make(map[string][]CostEntry)
}

// LLMCostCalculator creates a cost calculator for LLM tools.
func LLMCostCalculator(inputTokenCost, outputTokenCost float64) CostCalculator {
	return func(ctx context.Context, execCtx *middleware.ExecutionContext, result tool.Result, duration time.Duration) []CostEntry {
		now := time.Now()
		entries := make([]CostEntry, 0)

		// Try to extract token counts from result
		var tokenInfo struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
			TotalTokens  int `json:"total_tokens"`
		}

		if len(result.Output) > 0 {
			_ = json.Unmarshal(result.Output, &tokenInfo)
		}

		// Record token usage
		if tokenInfo.TotalTokens > 0 || tokenInfo.InputTokens > 0 || tokenInfo.OutputTokens > 0 {
			totalTokens := tokenInfo.TotalTokens
			if totalTokens == 0 {
				totalTokens = tokenInfo.InputTokens + tokenInfo.OutputTokens
			}

			entries = append(entries, CostEntry{
				RunID:     execCtx.RunID,
				ToolName:  execCtx.Tool.Name(),
				Unit:      CostUnitTokens,
				Amount:    float64(totalTokens),
				Timestamp: now,
			})

			// Calculate dollar cost
			cost := float64(tokenInfo.InputTokens)*inputTokenCost + float64(tokenInfo.OutputTokens)*outputTokenCost
			if cost > 0 {
				entries = append(entries, CostEntry{
					RunID:     execCtx.RunID,
					ToolName:  execCtx.Tool.Name(),
					Unit:      CostUnitDollars,
					Amount:    cost,
					Timestamp: now,
				})
			}
		}

		// Always record request and duration
		entries = append(entries,
			CostEntry{
				RunID:     execCtx.RunID,
				ToolName:  execCtx.Tool.Name(),
				Unit:      CostUnitRequests,
				Amount:    1,
				Timestamp: now,
			},
			CostEntry{
				RunID:     execCtx.RunID,
				ToolName:  execCtx.Tool.Name(),
				Unit:      CostUnitDuration,
				Amount:    float64(duration.Milliseconds()),
				Timestamp: now,
			},
		)

		return entries
	}
}

// APICallCostCalculator creates a cost calculator for API calls with per-call pricing.
func APICallCostCalculator(costPerCall float64) CostCalculator {
	return func(ctx context.Context, execCtx *middleware.ExecutionContext, result tool.Result, duration time.Duration) []CostEntry {
		now := time.Now()

		return []CostEntry{
			{
				RunID:     execCtx.RunID,
				ToolName:  execCtx.Tool.Name(),
				Unit:      CostUnitRequests,
				Amount:    1,
				Timestamp: now,
			},
			{
				RunID:     execCtx.RunID,
				ToolName:  execCtx.Tool.Name(),
				Unit:      CostUnitCredits,
				Amount:    costPerCall,
				Timestamp: now,
			},
			{
				RunID:     execCtx.RunID,
				ToolName:  execCtx.Tool.Name(),
				Unit:      CostUnitDuration,
				Amount:    float64(duration.Milliseconds()),
				Timestamp: now,
			},
		}
	}
}
