package middleware

import (
	"context"
	"encoding/json"
	"time"

	"go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/simulation"
	"go.klarlabs.de/agent/domain/tool"
)

// Simulation returns middleware that intercepts tool execution in dry-run mode.
// In simulation mode, destructive tools are blocked and their intent is recorded.
func Simulation(cfg simulation.Config) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			// If simulation is disabled, pass through
			if !cfg.Enabled {
				return next(ctx, execCtx)
			}

			annotations := execCtx.Tool.Annotations()
			toolName := execCtx.Tool.Name()

			// Check if tool can execute normally
			canExecute := false
			blockReason := ""

			switch {
			case annotations.ReadOnly && cfg.AllowReadOnly:
				canExecute = true
			case annotations.Idempotent && cfg.AllowIdempotent:
				canExecute = true
			case annotations.ReadOnly:
				blockReason = "read-only tools not allowed in this simulation mode"
			case annotations.Destructive:
				blockReason = "destructive tools blocked in simulation mode"
			default:
				blockReason = "tool blocked in simulation mode"
			}

			// Record intent if configured
			if cfg.RecordIntents && cfg.Recorder != nil {
				intent := simulation.Intent{
					ToolName:    toolName,
					Input:       execCtx.Input,
					State:       execCtx.CurrentState,
					Timestamp:   time.Now(),
					Blocked:     !canExecute,
					BlockReason: blockReason,
					MockResult:  false,
				}

				// Check if we have a mock result
				if _, hasMock := cfg.MockResults[toolName]; hasMock {
					intent.MockResult = true
				} else if cfg.DefaultResult != nil {
					intent.MockResult = true
				}

				cfg.Recorder.Record(intent)
			}

			// If tool can execute, proceed
			if canExecute {
				return next(ctx, execCtx)
			}

			// Check for mock result
			if mockResult, ok := cfg.MockResults[toolName]; ok {
				mockResult.Cached = false
				return mockResult, nil
			}

			// Use default result if provided
			if cfg.DefaultResult != nil {
				result := *cfg.DefaultResult
				result.Cached = false
				return result, nil
			}

			// Return a simulation result
			return tool.Result{
				Output: json.RawMessage(`{"simulated": true, "tool": "` + toolName + `"}`),
				Cached: false,
			}, nil
		}
	}
}

// SimulationConfig is an alias for simulation.Config for convenience.
type SimulationConfig = simulation.Config

// SimulationIntent is an alias for simulation.Intent for convenience.
type SimulationIntent = simulation.Intent

// NewSimulationConfig creates a new simulation configuration.
func NewSimulationConfig(opts ...simulation.ConfigOption) simulation.Config {
	cfg := simulation.DefaultConfig()
	for _, opt := range opts {
		opt(&cfg)
	}
	return cfg
}

// NewMemoryIntentRecorder creates a new in-memory intent recorder.
func NewMemoryIntentRecorder() *simulation.MemoryRecorder {
	return simulation.NewMemoryRecorder()
}
