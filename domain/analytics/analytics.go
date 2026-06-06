// Package analytics provides types for cross-run analytics.
package analytics

import (
	"context"
	"time"

	"go.klarlabs.de/agent/domain/agent"
)

// Analytics provides aggregate insights across runs.
type Analytics interface {
	// ToolUsage returns tool usage statistics.
	ToolUsage(ctx context.Context, filter Filter) ([]ToolUsageStat, error)

	// StateDistribution returns time spent in each state.
	StateDistribution(ctx context.Context, filter Filter) ([]StateTimeStat, error)

	// SuccessRate returns the success rate for runs.
	SuccessRate(ctx context.Context, filter Filter) (SuccessRateStat, error)

	// RunSummary returns summary statistics for runs.
	RunSummary(ctx context.Context, filter Filter) (RunSummary, error)
}

// Filter specifies criteria for analytics queries.
type Filter struct {
	// FromTime filters data after this time.
	FromTime time.Time

	// ToTime filters data before this time.
	ToTime time.Time

	// RunIDs filters to specific runs (empty means all).
	RunIDs []string

	// ToolNames filters to specific tools (empty means all).
	ToolNames []string

	// States filters to specific states (empty means all).
	States []agent.State

	// GroupBy specifies how to group results.
	GroupBy GroupBy

	// Limit is the maximum number of results (0 = no limit).
	Limit int
}

// GroupBy specifies how to group analytics results.
type GroupBy string

const (
	// GroupByNone returns ungrouped results.
	GroupByNone GroupBy = ""

	// GroupByDay groups results by day.
	GroupByDay GroupBy = "day"

	// GroupByWeek groups results by week.
	GroupByWeek GroupBy = "week"

	// GroupByMonth groups results by month.
	GroupByMonth GroupBy = "month"

	// GroupByRun groups results by run.
	GroupByRun GroupBy = "run"

	// GroupByTool groups results by tool.
	GroupByTool GroupBy = "tool"

	// GroupByState groups results by state.
	GroupByState GroupBy = "state"
)

// ToolUsageStat contains usage statistics for a tool.
type ToolUsageStat struct {
	// ToolName is the tool identifier.
	ToolName string `json:"tool_name"`

	// CallCount is the number of times the tool was called.
	CallCount int64 `json:"call_count"`

	// SuccessCount is the number of successful executions.
	SuccessCount int64 `json:"success_count"`

	// FailureCount is the number of failed executions.
	FailureCount int64 `json:"failure_count"`

	// CacheHitCount is the number of cache hits.
	CacheHitCount int64 `json:"cache_hit_count"`

	// TotalDuration is the total execution time.
	TotalDuration time.Duration `json:"total_duration"`

	// AverageDuration is the average execution time.
	AverageDuration time.Duration `json:"average_duration"`

	// MinDuration is the minimum execution time.
	MinDuration time.Duration `json:"min_duration"`

	// MaxDuration is the maximum execution time.
	MaxDuration time.Duration `json:"max_duration"`

	// Period is the time period for this stat (when grouped by time).
	Period time.Time `json:"period,omitempty"`
}

// StateTimeStat contains time spent in a state.
type StateTimeStat struct {
	// State is the agent state.
	State agent.State `json:"state"`

	// TotalTime is the total time spent in this state.
	TotalTime time.Duration `json:"total_time"`

	// EntryCount is the number of times this state was entered.
	EntryCount int64 `json:"entry_count"`

	// AverageTime is the average time per visit.
	AverageTime time.Duration `json:"average_time"`

	// Percentage is the percentage of total run time.
	Percentage float64 `json:"percentage"`

	// Period is the time period for this stat (when grouped by time).
	Period time.Time `json:"period,omitempty"`
}

// SuccessRateStat contains success rate statistics.
type SuccessRateStat struct {
	// TotalRuns is the total number of runs.
	TotalRuns int64 `json:"total_runs"`

	// CompletedRuns is the number of successfully completed runs.
	CompletedRuns int64 `json:"completed_runs"`

	// FailedRuns is the number of failed runs.
	FailedRuns int64 `json:"failed_runs"`

	// SuccessRate is the percentage of successful runs.
	SuccessRate float64 `json:"success_rate"`

	// AverageDuration is the average run duration.
	AverageDuration time.Duration `json:"average_duration"`

	// Period is the time period for this stat (when grouped by time).
	Period time.Time `json:"period,omitempty"`
}

// RunSummary contains aggregate run statistics.
type RunSummary struct {
	// TotalRuns is the total number of runs.
	TotalRuns int64 `json:"total_runs"`

	// CompletedRuns is the number of completed runs.
	CompletedRuns int64 `json:"completed_runs"`

	// FailedRuns is the number of failed runs.
	FailedRuns int64 `json:"failed_runs"`

	// RunningRuns is the number of running runs.
	RunningRuns int64 `json:"running_runs"`

	// TotalToolCalls is the total number of tool calls.
	TotalToolCalls int64 `json:"total_tool_calls"`

	// TotalDuration is the total duration of all runs.
	TotalDuration time.Duration `json:"total_duration"`

	// AverageDuration is the average run duration.
	AverageDuration time.Duration `json:"average_duration"`

	// AverageToolCallsPerRun is the average tool calls per run.
	AverageToolCallsPerRun float64 `json:"average_tool_calls_per_run"`
}
