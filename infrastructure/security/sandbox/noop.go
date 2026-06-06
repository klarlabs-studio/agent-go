package sandbox

import (
	"context"
	"encoding/json"

	"go.klarlabs.de/agent/domain/tool"
)

// NoopSandbox provides no isolation - it executes tools directly.
// Use this when sandboxing is not needed or for testing.
type NoopSandbox struct{}

// NewNoop creates a new no-op sandbox.
func NewNoop() *NoopSandbox {
	return &NoopSandbox{}
}

// Execute runs the tool directly without any isolation.
func (s *NoopSandbox) Execute(ctx context.Context, t tool.Tool, input json.RawMessage) (tool.Result, error) {
	return t.Execute(ctx, input)
}

// Capabilities returns unlimited capabilities.
func (s *NoopSandbox) Capabilities() Capabilities {
	return Capabilities{
		Network:     true,
		Filesystem:  true,
		MaxMemory:   0, // unlimited
		MaxExecTime: 0, // unlimited
	}
}

// Close is a no-op.
func (s *NoopSandbox) Close() error {
	return nil
}
