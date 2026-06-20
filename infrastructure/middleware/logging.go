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
	// Logger is the injected structured logger. When nil, a no-op logger is
	// used — this middleware NEVER falls back to the package-level logging
	// singleton, so a configured engine's logs all flow to its injected sink.
	Logger *logging.Logger
}

// resolveLogger returns the injected logger, or a no-op logger when nil. The
// execution-path middleware NEVER falls back to the package-level logging
// singleton (logging.Get); an unconfigured engine is silent rather than
// leaking to a global sink.
func resolveLogger(l *logging.Logger) *logging.Logger {
	if l == nil {
		return logging.NewNopLogger()
	}
	return l
}

// Logging returns middleware that logs tool execution.
func Logging(cfg LoggingConfig) middleware.Middleware {
	log := resolveLogger(cfg.Logger)
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			start := time.Now()

			// Log start
			entry := log.Info().
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
				log.Error().
					Add(logging.RunID(execCtx.RunID)).
					Add(logging.ToolName(execCtx.Tool.Name())).
					Add(logging.ErrorField(err)).
					Add(logging.Duration(duration)).
					Msg("tool execution failed")
			} else {
				logEntry := log.Info().
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
