package middleware

import (
	"context"
	"time"

	"go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/tool"
	"go.klarlabs.de/agent/infrastructure/logging"
)

// LoggingConfig configures the logging middleware.
type LoggingConfig struct {
	// LogInput logs the tool input (may contain sensitive data).
	LogInput bool
	// LogOutput logs the tool output (may be large).
	LogOutput bool
}

// Logging returns middleware that logs tool execution.
func Logging(cfg LoggingConfig) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			start := time.Now()

			// Log start
			entry := logging.Info().
				Add(logging.RunID(execCtx.RunID)).
				Add(logging.State(execCtx.CurrentState)).
				Add(logging.ToolName(execCtx.Tool.Name()))

			if cfg.LogInput && len(execCtx.Input) > 0 {
				entry = entry.Add(logging.Str("input", string(execCtx.Input)))
			}

			entry.Msg("executing tool")

			// Execute
			result, err := next(ctx, execCtx)
			duration := time.Since(start)

			// Log result
			if err != nil {
				logging.Error().
					Add(logging.RunID(execCtx.RunID)).
					Add(logging.ToolName(execCtx.Tool.Name())).
					Add(logging.ErrorField(err)).
					Add(logging.Duration(duration)).
					Msg("tool execution failed")
			} else {
				logEntry := logging.Info().
					Add(logging.RunID(execCtx.RunID)).
					Add(logging.ToolName(execCtx.Tool.Name())).
					Add(logging.Duration(duration)).
					Add(logging.Cached(result.Cached))

				if cfg.LogOutput && len(result.Output) > 0 {
					// Truncate large outputs
					output := string(result.Output)
					if len(output) > 500 {
						output = output[:500] + "..."
					}
					logEntry = logEntry.Add(logging.Str("output", output))
				}

				logEntry.Msg("tool executed")
			}

			return result, err
		}
	}
}
