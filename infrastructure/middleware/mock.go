package middleware

import (
	"context"
	"encoding/json"
	"regexp"
	"sync"

	"go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/tool"
	"go.klarlabs.de/agent/infrastructure/logging"
)

// MockResponse represents a mock response for a tool.
type MockResponse struct {
	// Output is the mock output to return.
	Output json.RawMessage

	// Error is an optional error to return.
	Error error

	// Delay simulates execution time (not implemented in basic version).
	// Delay time.Duration

	// Condition is an optional function to determine if this mock applies.
	Condition func(input json.RawMessage) bool
}

// MockConfig configures the mock middleware.
type MockConfig struct {
	// Mocks maps tool names to their mock responses.
	Mocks map[string]MockResponse

	// PatternMocks maps tool name patterns (regex) to mock responses.
	PatternMocks map[string]MockResponse

	// Passthrough allows non-mocked tools to execute normally.
	// If false (default), non-mocked tools will return an error.
	Passthrough bool

	// DefaultResponse is returned for tools without specific mocks when Passthrough is false.
	DefaultResponse *MockResponse

	// OnMockHit is called when a mock is matched.
	OnMockHit func(toolName string, input json.RawMessage)
}

// Mock returns middleware that injects mock responses for tool executions.
// This is useful for testing agent behavior without calling real tools.
func Mock(cfg MockConfig) middleware.Middleware {
	// Compile pattern regexes
	compiledPatterns := make(map[*regexp.Regexp]MockResponse)
	for pattern, response := range cfg.PatternMocks {
		re, err := regexp.Compile(pattern)
		if err != nil {
			logging.Warn().
				Add(logging.Str("pattern", pattern)).
				Add(logging.ErrorField(err)).
				Msg("invalid mock pattern regex")
			continue
		}
		compiledPatterns[re] = response
	}

	return func(next middleware.Handler) middleware.Handler {
		return func(ctx context.Context, execCtx *middleware.ExecutionContext) (tool.Result, error) {
			toolName := execCtx.Tool.Name()

			// Check exact match first
			if mock, ok := cfg.Mocks[toolName]; ok {
				// Check condition if present
				if mock.Condition == nil || mock.Condition(execCtx.Input) {
					if cfg.OnMockHit != nil {
						cfg.OnMockHit(toolName, execCtx.Input)
					}

					logging.Debug().
						Add(logging.RunID(execCtx.RunID)).
						Add(logging.ToolName(toolName)).
						Msg("mock response returned")

					return tool.Result{Output: mock.Output}, mock.Error
				}
			}

			// Check pattern matches
			for pattern, mock := range compiledPatterns {
				if pattern.MatchString(toolName) {
					if mock.Condition == nil || mock.Condition(execCtx.Input) {
						if cfg.OnMockHit != nil {
							cfg.OnMockHit(toolName, execCtx.Input)
						}

						logging.Debug().
							Add(logging.RunID(execCtx.RunID)).
							Add(logging.ToolName(toolName)).
							Add(logging.Str("pattern", pattern.String())).
							Msg("mock response returned (pattern match)")

						return tool.Result{Output: mock.Output}, mock.Error
					}
				}
			}

			// No mock found
			if cfg.Passthrough {
				return next(ctx, execCtx)
			}

			// Return default response if configured
			if cfg.DefaultResponse != nil {
				logging.Debug().
					Add(logging.RunID(execCtx.RunID)).
					Add(logging.ToolName(toolName)).
					Msg("default mock response returned")

				return tool.Result{Output: cfg.DefaultResponse.Output}, cfg.DefaultResponse.Error
			}

			// Return empty result for non-mocked tools
			logging.Warn().
				Add(logging.RunID(execCtx.RunID)).
				Add(logging.ToolName(toolName)).
				Msg("no mock found for tool")

			return tool.Result{}, nil
		}
	}
}

// MockBuilder provides a fluent interface for building mock configurations.
type MockBuilder struct {
	cfg MockConfig
}

// NewMockBuilder creates a new mock builder.
func NewMockBuilder() *MockBuilder {
	return &MockBuilder{
		cfg: MockConfig{
			Mocks:        make(map[string]MockResponse),
			PatternMocks: make(map[string]MockResponse),
		},
	}
}

// ForTool adds a mock for a specific tool.
func (b *MockBuilder) ForTool(name string, output interface{}) *MockBuilder {
	data, _ := json.Marshal(output)
	b.cfg.Mocks[name] = MockResponse{Output: data}
	return b
}

// ForToolWithError adds a mock that returns an error.
func (b *MockBuilder) ForToolWithError(name string, err error) *MockBuilder {
	b.cfg.Mocks[name] = MockResponse{Error: err}
	return b
}

// ForToolConditional adds a mock with a condition.
func (b *MockBuilder) ForToolConditional(name string, output interface{}, condition func(json.RawMessage) bool) *MockBuilder {
	data, _ := json.Marshal(output)
	b.cfg.Mocks[name] = MockResponse{
		Output:    data,
		Condition: condition,
	}
	return b
}

// ForPattern adds a mock for tools matching a pattern.
func (b *MockBuilder) ForPattern(pattern string, output interface{}) *MockBuilder {
	data, _ := json.Marshal(output)
	b.cfg.PatternMocks[pattern] = MockResponse{Output: data}
	return b
}

// WithPassthrough allows non-mocked tools to execute.
func (b *MockBuilder) WithPassthrough() *MockBuilder {
	b.cfg.Passthrough = true
	return b
}

// WithDefaultResponse sets a default response for non-mocked tools.
func (b *MockBuilder) WithDefaultResponse(output interface{}) *MockBuilder {
	data, _ := json.Marshal(output)
	b.cfg.DefaultResponse = &MockResponse{Output: data}
	return b
}

// WithOnHit sets a callback for when mocks are hit.
func (b *MockBuilder) WithOnHit(fn func(string, json.RawMessage)) *MockBuilder {
	b.cfg.OnMockHit = fn
	return b
}

// Build creates the mock middleware.
func (b *MockBuilder) Build() middleware.Middleware {
	return Mock(b.cfg)
}

// MockRecorder records tool invocations for verification.
type MockRecorder struct {
	mu          sync.RWMutex
	invocations []ToolInvocation
}

// ToolInvocation represents a recorded tool invocation.
type ToolInvocation struct {
	ToolName string
	Input    json.RawMessage
}

// NewMockRecorder creates a new mock recorder.
func NewMockRecorder() *MockRecorder {
	return &MockRecorder{
		invocations: make([]ToolInvocation, 0),
	}
}

// Record records a tool invocation.
func (r *MockRecorder) Record(toolName string, input json.RawMessage) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.invocations = append(r.invocations, ToolInvocation{
		ToolName: toolName,
		Input:    input,
	})
}

// Invocations returns all recorded invocations.
func (r *MockRecorder) Invocations() []ToolInvocation {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]ToolInvocation, len(r.invocations))
	copy(result, r.invocations)
	return result
}

// InvocationCount returns the number of invocations.
func (r *MockRecorder) InvocationCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.invocations)
}

// InvocationsFor returns invocations for a specific tool.
func (r *MockRecorder) InvocationsFor(toolName string) []ToolInvocation {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]ToolInvocation, 0)
	for _, inv := range r.invocations {
		if inv.ToolName == toolName {
			result = append(result, inv)
		}
	}
	return result
}

// Clear clears all recorded invocations.
func (r *MockRecorder) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.invocations = r.invocations[:0]
}

// WasCalled checks if a tool was called.
func (r *MockRecorder) WasCalled(toolName string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, inv := range r.invocations {
		if inv.ToolName == toolName {
			return true
		}
	}
	return false
}

// WasCalledTimes checks if a tool was called a specific number of times.
func (r *MockRecorder) WasCalledTimes(toolName string, times int) bool {
	return len(r.InvocationsFor(toolName)) == times
}
