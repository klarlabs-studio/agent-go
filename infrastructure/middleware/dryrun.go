package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/tool"
	"go.klarlabs.de/agent/infrastructure/logging"
)

// DryRunConfig configures the dry-run middleware.
type DryRunConfig struct {
	// Enabled determines if dry-run mode is active.
	Enabled bool

	// AllowReadOnly permits read-only tools to execute even in dry-run mode.
	AllowReadOnly bool

	// AllowCacheable permits cacheable tools to execute even in dry-run mode.
	AllowCacheable bool

	// MockResponses maps tool names to mock responses.
	MockResponses map[string]json.RawMessage

	// OnDryRun is called when a tool execution is skipped due to dry-run mode.
	OnDryRun func(ctx context.Context, execCtx *middleware.ExecutionContext)

	// Recorder captures all dry-run operations for later inspection.
	Recorder *DryRunRecorder

	// Logger is the injected structured logger. When nil, a no-op logger is
	// used — never the package-level logging singleton.
	Logger *logging.Logger
}

// DryRunOption configures the dry-run middleware.
type DryRunOption func(*DryRunConfig)

// WithDryRunEnabled enables or disables dry-run mode.
func WithDryRunEnabled(enabled bool) DryRunOption {
	return func(c *DryRunConfig) {
		c.Enabled = enabled
	}
}

// WithAllowReadOnly permits read-only tools in dry-run mode.
func WithAllowReadOnly(allow bool) DryRunOption {
	return func(c *DryRunConfig) {
		c.AllowReadOnly = allow
	}
}

// WithAllowCacheable permits cacheable tools in dry-run mode.
func WithAllowCacheable(allow bool) DryRunOption {
	return func(c *DryRunConfig) {
		c.AllowCacheable = allow
	}
}

// WithMockResponse sets a mock response for a specific tool.
func WithMockResponse(toolName string, response json.RawMessage) DryRunOption {
	return func(c *DryRunConfig) {
		if c.MockResponses == nil {
			c.MockResponses = make(map[string]json.RawMessage)
		}
		c.MockResponses[toolName] = response
	}
}

// WithMockResponses sets mock responses for multiple tools.
func WithMockResponses(responses map[string]json.RawMessage) DryRunOption {
	return func(c *DryRunConfig) {
		c.MockResponses = responses
	}
}

// WithDryRunCallback sets a callback for dry-run operations.
func WithDryRunCallback(fn func(ctx context.Context, execCtx *middleware.ExecutionContext)) DryRunOption {
	return func(c *DryRunConfig) {
		c.OnDryRun = fn
	}
}

// WithDryRunRecorder sets a recorder for dry-run operations.
func WithDryRunRecorder(recorder *DryRunRecorder) DryRunOption {
	return func(c *DryRunConfig) {
		c.Recorder = recorder
	}
}

// WithDryRunLogger injects the structured logger used by the middleware. When
// unset, a no-op logger is used — never the package-level logging singleton.
func WithDryRunLogger(l *logging.Logger) DryRunOption {
	return func(c *DryRunConfig) {
		c.Logger = l
	}
}

// DryRun returns middleware that prevents side-effect tools from executing.
// In dry-run mode, destructive tools return mock responses instead of executing.
func DryRun(opts ...DryRunOption) middleware.Middleware {
	cfg := DryRunConfig{
		Enabled:        true,
		AllowReadOnly:  true,
		AllowCacheable: true,
		MockResponses:  make(map[string]json.RawMessage),
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	log := resolveLogger(cfg.Logger)

	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			// If dry-run is disabled, pass through
			if !cfg.Enabled {
				return next(ctx, execCtx)
			}

			annotations := execCtx.Tool.Annotations()

			// Check if tool is allowed to execute
			shouldExecute := (cfg.AllowReadOnly && annotations.ReadOnly) ||
				(cfg.AllowCacheable && annotations.Cacheable)

			// If tool can execute, pass through
			if shouldExecute {
				return next(ctx, execCtx)
			}

			// Log dry-run
			log.Info().
				Add(logging.RunID(execCtx.RunID)).
				Add(logging.ToolName(execCtx.Tool.Name())).
				Add(logging.Str("mode", "dry-run")).
				Add(logging.Bool("destructive", annotations.Destructive)).
				Msg("tool execution skipped (dry-run mode)")

			// Call callback if provided
			if cfg.OnDryRun != nil {
				cfg.OnDryRun(ctx, execCtx)
			}

			// Record operation if recorder is set
			if cfg.Recorder != nil {
				cfg.Recorder.Record(DryRunOperation{
					RunID:     execCtx.RunID,
					State:     string(execCtx.CurrentState),
					ToolName:  execCtx.Tool.Name(),
					Input:     execCtx.Input,
					Reason:    execCtx.Reason,
					Timestamp: time.Now(),
				})
			}

			// Return mock response if available
			if mockResponse, ok := cfg.MockResponses[execCtx.Tool.Name()]; ok {
				return tool.Result{
					Output: mockResponse,
				}, nil
			}

			// Return default dry-run response
			response := dryRunResponse{
				DryRun:  true,
				Tool:    execCtx.Tool.Name(),
				Message: "Tool execution skipped in dry-run mode",
				WouldDo: execCtx.Reason,
				Input:   execCtx.Input,
				Skipped: true,
				Metadata: map[string]interface{}{
					"state":       execCtx.CurrentState,
					"destructive": annotations.Destructive,
					"risk_level":  annotations.RiskLevel,
				},
			}

			data, _ := json.Marshal(response)
			return tool.Result{Output: data}, nil
		}
	}
}

// dryRunResponse is the default response for dry-run mode.
type dryRunResponse struct {
	DryRun   bool                   `json:"dry_run"`
	Tool     string                 `json:"tool"`
	Message  string                 `json:"message"`
	WouldDo  string                 `json:"would_do,omitempty"`
	Input    json.RawMessage        `json:"input,omitempty"`
	Skipped  bool                   `json:"skipped"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// DryRunOperation represents a captured dry-run operation.
type DryRunOperation struct {
	RunID     string          `json:"run_id"`
	State     string          `json:"state"`
	ToolName  string          `json:"tool_name"`
	Input     json.RawMessage `json:"input"`
	Reason    string          `json:"reason"`
	Timestamp time.Time       `json:"timestamp"`
}

// DryRunRecorder captures dry-run operations for later inspection.
type DryRunRecorder struct {
	operations []DryRunOperation
	mu         sync.RWMutex
}

// NewDryRunRecorder creates a new dry-run recorder.
func NewDryRunRecorder() *DryRunRecorder {
	return &DryRunRecorder{
		operations: make([]DryRunOperation, 0),
	}
}

// Record adds an operation to the recorder.
func (r *DryRunRecorder) Record(op DryRunOperation) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.operations = append(r.operations, op)
}

// Operations returns all recorded operations.
func (r *DryRunRecorder) Operations() []DryRunOperation {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]DryRunOperation, len(r.operations))
	copy(result, r.operations)
	return result
}

// OperationsByTool returns operations for a specific tool.
func (r *DryRunRecorder) OperationsByTool(toolName string) []DryRunOperation {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []DryRunOperation
	for _, op := range r.operations {
		if op.ToolName == toolName {
			result = append(result, op)
		}
	}
	return result
}

// OperationsByRun returns operations for a specific run.
func (r *DryRunRecorder) OperationsByRun(runID string) []DryRunOperation {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var result []DryRunOperation
	for _, op := range r.operations {
		if op.RunID == runID {
			result = append(result, op)
		}
	}
	return result
}

// Count returns the number of recorded operations.
func (r *DryRunRecorder) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.operations)
}

// Clear removes all recorded operations.
func (r *DryRunRecorder) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.operations = make([]DryRunOperation, 0)
}

// Summary returns a summary of dry-run operations.
func (r *DryRunRecorder) Summary() DryRunSummary {
	r.mu.RLock()
	defer r.mu.RUnlock()

	summary := DryRunSummary{
		TotalOperations: len(r.operations),
		ByTool:          make(map[string]int),
		ByState:         make(map[string]int),
	}

	for _, op := range r.operations {
		summary.ByTool[op.ToolName]++
		summary.ByState[op.State]++
	}

	return summary
}

// DryRunSummary provides an overview of dry-run operations.
type DryRunSummary struct {
	TotalOperations int            `json:"total_operations"`
	ByTool          map[string]int `json:"by_tool"`
	ByState         map[string]int `json:"by_state"`
}

// ToJSON serializes the summary to JSON.
func (s DryRunSummary) ToJSON() ([]byte, error) {
	return json.Marshal(s)
}

// DryRunReport generates a detailed report of dry-run operations.
type DryRunReport struct {
	Summary    DryRunSummary     `json:"summary"`
	Operations []DryRunOperation `json:"operations"`
	StartTime  time.Time         `json:"start_time"`
	EndTime    time.Time         `json:"end_time"`
	Duration   time.Duration     `json:"duration"`
}

// GenerateReport creates a report from the recorded operations.
func (r *DryRunRecorder) GenerateReport() DryRunReport {
	r.mu.RLock()
	defer r.mu.RUnlock()

	report := DryRunReport{
		Summary:    r.Summary(),
		Operations: make([]DryRunOperation, len(r.operations)),
	}

	copy(report.Operations, r.operations)

	if len(r.operations) > 0 {
		report.StartTime = r.operations[0].Timestamp
		report.EndTime = r.operations[len(r.operations)-1].Timestamp
		report.Duration = report.EndTime.Sub(report.StartTime)
	}

	return report
}

// PrintReport prints a human-readable report to stdout.
func (r *DryRunRecorder) PrintReport() {
	report := r.GenerateReport()

	fmt.Println("=== Dry Run Report ===")
	fmt.Printf("Total Operations: %d\n", report.Summary.TotalOperations)
	fmt.Printf("Duration: %s\n", report.Duration)
	fmt.Println()

	if len(report.Summary.ByTool) > 0 {
		fmt.Println("Operations by Tool:")
		for tool, count := range report.Summary.ByTool {
			fmt.Printf("  - %s: %d\n", tool, count)
		}
		fmt.Println()
	}

	if len(report.Summary.ByState) > 0 {
		fmt.Println("Operations by State:")
		for state, count := range report.Summary.ByState {
			fmt.Printf("  - %s: %d\n", state, count)
		}
		fmt.Println()
	}

	if len(report.Operations) > 0 {
		fmt.Println("Operations:")
		for i, op := range report.Operations {
			fmt.Printf("%d. [%s] %s in %s\n", i+1, op.Timestamp.Format(time.RFC3339), op.ToolName, op.State)
			if op.Reason != "" {
				fmt.Printf("   Reason: %s\n", op.Reason)
			}
		}
	}
}

// ConditionalDryRun returns middleware that conditionally enables dry-run mode.
// The condition function is called for each tool execution.
func ConditionalDryRun(condition func(ctx context.Context, execCtx *middleware.ExecutionContext) bool, opts ...DryRunOption) middleware.Middleware {
	dryRunMiddleware := DryRun(opts...)

	return func(next middleware.Handler) middleware.Handler {
		dryRunHandler := dryRunMiddleware(next)

		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			if condition(ctx, execCtx) {
				return dryRunHandler(ctx, execCtx)
			}
			return next(ctx, execCtx)
		}
	}
}

// DryRunFromContext checks if dry-run mode is enabled in the context.
func DryRunFromContext(ctx context.Context) bool {
	val := ctx.Value(dryRunContextKey{})
	if val == nil {
		return false
	}
	enabled, ok := val.(bool)
	return ok && enabled
}

// ContextWithDryRun returns a context with dry-run mode enabled or disabled.
func ContextWithDryRun(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, dryRunContextKey{}, enabled)
}

type dryRunContextKey struct{}

// ContextAwareDryRun returns middleware that checks context for dry-run mode.
func ContextAwareDryRun(opts ...DryRunOption) middleware.Middleware {
	// Create base config without enabled flag
	cfg := DryRunConfig{
		Enabled:        false, // Will be determined from context
		AllowReadOnly:  true,
		AllowCacheable: true,
		MockResponses:  make(map[string]json.RawMessage),
	}

	for _, opt := range opts {
		opt(&cfg)
	}

	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			// Check context for dry-run flag
			if !DryRunFromContext(ctx) {
				return next(ctx, execCtx)
			}

			// Enable dry-run for this execution
			cfg.Enabled = true
			dryRunMiddleware := DryRun(
				WithDryRunEnabled(true),
				WithAllowReadOnly(cfg.AllowReadOnly),
				WithAllowCacheable(cfg.AllowCacheable),
				WithMockResponses(cfg.MockResponses),
				WithDryRunCallback(cfg.OnDryRun),
				WithDryRunRecorder(cfg.Recorder),
				WithDryRunLogger(cfg.Logger),
			)

			handler := dryRunMiddleware(next)
			return handler(ctx, execCtx)
		}
	}
}
