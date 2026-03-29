// Package pattern provides pattern detection types.
package pattern

import (
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
)

// PatternType classifies patterns.
type PatternType string

const (
	// Behavioral patterns
	PatternTypeToolSequence PatternType = "tool_sequence" // Repeated tool call sequences
	PatternTypeStateLoop    PatternType = "state_loop"    // Repeated state transitions
	PatternTypeToolAffinity PatternType = "tool_affinity" // Tools frequently used together

	// Failure patterns
	PatternTypeRecurringFailure PatternType = "recurring_failure" // Same failure mode
	PatternTypeToolFailure      PatternType = "tool_failure"      // Tool consistently fails
	PatternTypeBudgetExhaustion PatternType = "budget_exhaustion" // Runs hitting budget limits

	// Performance patterns
	PatternTypeSlowTool PatternType = "slow_tool" // Tool performance degradation
	PatternTypeLongRuns PatternType = "long_runs" // Runs taking longer than expected

	// Operational patterns
	PatternTypeCostAnomaly    PatternType = "cost_anomaly"    // Unusual cost spikes
	PatternTypeTimeout        PatternType = "timeout"         // Tools consistently timing out
	PatternTypeApprovalDelay  PatternType = "approval_delay"  // Human approval bottlenecks
	PatternTypeToolPreference PatternType = "tool_preference" // Over/under-used tools
)

// ToolSequenceData captures a repeated sequence of tool calls.
type ToolSequenceData struct {
	Sequence   []string      `json:"sequence"`    // Ordered tool names
	AverageGap time.Duration `json:"average_gap"` // Average time between calls
	States     []agent.State `json:"states"`      // States where sequence occurs
}

// StateLoopData captures repeated state transitions.
type StateLoopData struct {
	Loop       []agent.State `json:"loop"`       // State sequence forming loop
	Iterations int           `json:"iterations"` // Average iterations before exit
	ExitState  agent.State   `json:"exit_state"` // How loop typically exits
}

// ToolAffinityData captures tools frequently used together.
type ToolAffinityData struct {
	Tools       []string `json:"tools"`       // Tool names that appear together
	Correlation float64  `json:"correlation"` // How often they co-occur (0.0-1.0)
}

// FailureData captures recurring failure information.
type FailureData struct {
	FailureType  string      `json:"failure_type"`
	ToolName     string      `json:"tool_name,omitempty"`
	State        agent.State `json:"state"`
	ErrorPattern string      `json:"error_pattern"` // Common substring in errors
}

// PerformanceData captures performance-related pattern data.
type PerformanceData struct {
	ToolName        string        `json:"tool_name,omitempty"`
	AverageDuration time.Duration `json:"average_duration"`
	Threshold       time.Duration `json:"threshold"` // Normal expected duration
	Deviation       float64       `json:"deviation"` // How much above threshold
}

// ToolFailureData captures tool failure pattern data.
type ToolFailureData struct {
	ToolName   string `json:"tool_name"`
	ErrorType  string `json:"error_type"`  // Classified error type
	ErrorCount int    `json:"error_count"` // Number of failures
}

// BudgetExhaustionData captures budget exhaustion pattern data.
type BudgetExhaustionData struct {
	BudgetName      string `json:"budget_name,omitempty"` // Which budget was exhausted
	ExhaustionCount int    `json:"exhaustion_count"`      // Number of exhaustions
}

// SlowToolData captures slow tool performance pattern data.
type SlowToolData struct {
	ToolName        string        `json:"tool_name"`
	AverageDuration time.Duration `json:"average_duration"`
	P90Duration     time.Duration `json:"p90_duration"` // 90th percentile
	SlowCount       int           `json:"slow_count"`   // Number of slow executions
}

// LongRunsData captures long-running runs pattern data.
type LongRunsData struct {
	AverageDuration time.Duration `json:"average_duration"`
	Threshold       time.Duration `json:"threshold"`
	LongRunCount    int           `json:"long_run_count"`
}

// CostAnomalyData captures unusual cost spikes or trends.
type CostAnomalyData struct {
	CostType     string  `json:"cost_type"`       // e.g., "tool_calls", "tokens", "api_calls"
	AverageCost  float64 `json:"average_cost"`    // Historical average
	AnomalyCost  float64 `json:"anomaly_cost"`    // Cost in anomalous runs
	Deviation    float64 `json:"deviation"`       // Standard deviations from mean
	AnomalyCount int     `json:"anomaly_count"`   // Number of anomalies detected
	TrendDir     string  `json:"trend_direction"` // "increasing", "decreasing", "stable"
}

// TimeoutData captures tools consistently timing out.
type TimeoutData struct {
	ToolName     string        `json:"tool_name"`
	TimeoutCount int           `json:"timeout_count"` // Number of timeouts
	TotalCalls   int           `json:"total_calls"`   // Total calls to the tool
	TimeoutRate  float64       `json:"timeout_rate"`  // Percentage of calls that timeout
	AvgDuration  time.Duration `json:"avg_duration"`  // Average duration before timeout
	Limit        time.Duration `json:"timeout_limit"` // Configured timeout limit
}

// ApprovalDelayData captures human approval bottlenecks.
type ApprovalDelayData struct {
	ToolName        string        `json:"tool_name,omitempty"` // Tool requiring approval
	State           agent.State   `json:"state"`               // State where approval is needed
	AverageWaitTime time.Duration `json:"average_wait_time"`   // Average time waiting for approval
	MaxWaitTime     time.Duration `json:"max_wait_time"`       // Maximum observed wait time
	PendingCount    int           `json:"pending_count"`       // Currently pending approvals
	TotalApprovals  int           `json:"total_approvals"`     // Total approvals requested
	ApprovalRate    float64       `json:"approval_rate"`       // Percentage approved vs rejected
}

// ToolPreferenceData captures over/under-used tools.
type ToolPreferenceData struct {
	ToolName        string        `json:"tool_name"`
	UsageCount      int           `json:"usage_count"`      // Number of times used
	ExpectedUsage   float64       `json:"expected_usage"`   // Expected usage based on availability
	UsageRatio      float64       `json:"usage_ratio"`      // Actual/expected ratio
	PreferenceType  string        `json:"preference_type"`  // "overused" or "underused"
	SuccessRate     float64       `json:"success_rate"`     // Success rate when used
	AvailableStates []agent.State `json:"available_states"` // States where tool is available
}
