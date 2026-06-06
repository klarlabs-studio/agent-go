package sandbox

import (
	"context"

	"go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/tool"
)

// SandboxMiddleware creates middleware that executes tools requiring sandboxing
// in the provided sandbox. Tools are identified as needing sandboxing based on
// their annotations.
func SandboxMiddleware(sandbox Sandbox) middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			annotations := execCtx.Tool.Annotations()

			// Only sandbox tools marked as needing it or destructive tools
			if annotations.Sandboxed || annotations.Destructive {
				return sandbox.Execute(ctx, execCtx.Tool, execCtx.Input)
			}

			return next(ctx, execCtx)
		}
	}
}

// ConditionalSandboxMiddleware creates middleware that uses different sandboxes
// based on tool properties.
type ConditionalSandboxMiddleware struct {
	// ReadOnlySandbox is used for read-only tools.
	ReadOnlySandbox Sandbox

	// DefaultSandbox is used for all other sandboxed tools.
	DefaultSandbox Sandbox

	// Predicate determines if a tool should be sandboxed.
	Predicate func(t tool.Tool) bool
}

// Middleware returns the middleware handler.
func (m *ConditionalSandboxMiddleware) Middleware() middleware.Middleware {
	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			// Check if tool should be sandboxed
			if m.Predicate != nil && !m.Predicate(execCtx.Tool) {
				return next(ctx, execCtx)
			}

			annotations := execCtx.Tool.Annotations()

			// Select sandbox based on tool type
			var sb Sandbox
			if annotations.ReadOnly && m.ReadOnlySandbox != nil {
				sb = m.ReadOnlySandbox
			} else if m.DefaultSandbox != nil {
				sb = m.DefaultSandbox
			}

			if sb == nil {
				return next(ctx, execCtx)
			}

			return sb.Execute(ctx, execCtx.Tool, execCtx.Input)
		}
	}
}

// ShouldSandbox returns true if a tool should be executed in a sandbox.
func ShouldSandbox(t tool.Tool) bool {
	ann := t.Annotations()
	return ann.Sandboxed || ann.Destructive || ann.RiskLevel >= 3
}
