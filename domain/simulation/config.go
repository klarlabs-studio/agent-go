// Package simulation provides types for dry-run and simulation mode.
package simulation

import (
	"go.klarlabs.de/agent/domain/tool"
)

// Config configures simulation mode behavior.
type Config struct {
	// Enabled turns on simulation mode (no side effects).
	Enabled bool

	// RecordIntents captures tool execution intents.
	RecordIntents bool

	// MockResults provides predefined results for specific tools.
	// Key is tool name, value is the mock result to return.
	MockResults map[string]tool.Result

	// DefaultResult is returned for tools without specific mocks.
	// If nil, the tool executes normally (only non-destructive tools).
	DefaultResult *tool.Result

	// AllowReadOnly permits read-only tools to execute normally.
	AllowReadOnly bool

	// AllowIdempotent permits idempotent tools to execute normally.
	AllowIdempotent bool

	// Recorder receives intent records (optional).
	Recorder IntentRecorder
}

// DefaultConfig returns a Config with sensible defaults for dry-run mode.
func DefaultConfig() Config {
	return Config{
		Enabled:         true,
		RecordIntents:   true,
		MockResults:     make(map[string]tool.Result),
		AllowReadOnly:   true,
		AllowIdempotent: false,
	}
}

// ConfigOption configures simulation mode.
type ConfigOption func(*Config)

// WithMockResult sets a mock result for a specific tool.
func WithMockResult(toolName string, result tool.Result) ConfigOption {
	return func(c *Config) {
		if c.MockResults == nil {
			c.MockResults = make(map[string]tool.Result)
		}
		c.MockResults[toolName] = result
	}
}

// WithDefaultResult sets the default result for unmocked tools.
func WithDefaultResult(result tool.Result) ConfigOption {
	return func(c *Config) {
		c.DefaultResult = &result
	}
}

// WithAllowReadOnly permits read-only tools to execute.
func WithAllowReadOnly(allow bool) ConfigOption {
	return func(c *Config) {
		c.AllowReadOnly = allow
	}
}

// WithAllowIdempotent permits idempotent tools to execute.
func WithAllowIdempotent(allow bool) ConfigOption {
	return func(c *Config) {
		c.AllowIdempotent = allow
	}
}

// WithRecorder sets the intent recorder.
func WithRecorder(recorder IntentRecorder) ConfigOption {
	return func(c *Config) {
		c.Recorder = recorder
		c.RecordIntents = true
	}
}
