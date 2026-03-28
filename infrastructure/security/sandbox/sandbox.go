// Package sandbox provides tool execution sandboxing capabilities.
//
// Sandboxing is opt-in. By default, the engine uses no sandbox and tools
// execute directly in the host process. Use [NewNoop] for an explicit no-op
// sandbox, or implement the [Sandbox] interface for custom isolation (e.g.,
// WASM, containers, or OS-level namespacing).
//
// The sandbox interface is designed to be pluggable:
//
//	// No isolation (default)
//	sb := sandbox.NewNoop()
//
//	// Custom isolation (implement Sandbox interface)
//	sb := wasm.NewSandbox(sandbox.WithMaxMemory(64<<20), sandbox.WithMaxExecTime(30*time.Second))
//
// Capabilities advertise what a sandbox permits, allowing the engine and
// planner to make informed decisions about tool eligibility.
package sandbox

import (
	"context"
	"encoding/json"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/tool"
)

// Sandbox defines the interface for executing tools in an isolated environment.
type Sandbox interface {
	// Execute runs a tool in the sandbox with the given input.
	Execute(ctx context.Context, t tool.Tool, input json.RawMessage) (tool.Result, error)

	// Capabilities returns what the sandbox allows.
	Capabilities() Capabilities

	// Close releases sandbox resources.
	Close() error
}

// Capabilities describes what a sandbox allows.
type Capabilities struct {
	// Network indicates if network access is allowed.
	Network bool `json:"network"`

	// Filesystem indicates if filesystem access is allowed.
	Filesystem bool `json:"filesystem"`

	// MaxMemory is the maximum memory in bytes (0 means unlimited).
	MaxMemory int64 `json:"max_memory"`

	// MaxExecTime is the maximum execution time (0 means unlimited).
	MaxExecTime time.Duration `json:"max_exec_time"`

	// AllowedEnv lists allowed environment variables.
	AllowedEnv []string `json:"allowed_env,omitempty"`

	// ReadOnlyPaths lists paths that can only be read.
	ReadOnlyPaths []string `json:"read_only_paths,omitempty"`

	// WritePaths lists paths that can be written.
	WritePaths []string `json:"write_paths,omitempty"`
}

// ExecutionResult contains detailed execution information.
type ExecutionResult struct {
	// Result is the tool execution result.
	Result tool.Result

	// Duration is how long execution took.
	Duration time.Duration

	// MemoryUsed is peak memory usage in bytes.
	MemoryUsed int64

	// ExitCode is the WASM exit code (if applicable).
	ExitCode int
}

// Option configures a sandbox.
type Option func(*Config)

// Config holds common sandbox configuration.
type Config struct {
	// MaxMemory is the maximum memory in bytes.
	MaxMemory int64

	// MaxExecTime is the maximum execution time.
	MaxExecTime time.Duration

	// AllowNetwork enables network access.
	AllowNetwork bool

	// AllowFilesystem enables filesystem access.
	AllowFilesystem bool

	// FSRoot is the root directory for filesystem access.
	FSRoot string

	// ReadOnlyPaths are paths that can only be read.
	ReadOnlyPaths []string

	// WritePaths are paths that can be written.
	WritePaths []string

	// AllowedEnv are environment variables passed to sandboxed code.
	AllowedEnv []string
}

// WithMaxMemory sets the maximum memory limit.
func WithMaxMemory(bytes int64) Option {
	return func(c *Config) {
		c.MaxMemory = bytes
	}
}

// WithMaxExecTime sets the maximum execution time.
func WithMaxExecTime(d time.Duration) Option {
	return func(c *Config) {
		c.MaxExecTime = d
	}
}

// WithNetwork enables network access in the sandbox.
func WithNetwork() Option {
	return func(c *Config) {
		c.AllowNetwork = true
	}
}

// WithFilesystem enables filesystem access in the sandbox.
func WithFilesystem(root string, readOnly, writePaths []string) Option {
	return func(c *Config) {
		c.AllowFilesystem = true
		c.FSRoot = root
		c.ReadOnlyPaths = readOnly
		c.WritePaths = writePaths
	}
}

// WithEnv specifies allowed environment variables.
func WithEnv(vars ...string) Option {
	return func(c *Config) {
		c.AllowedEnv = vars
	}
}
