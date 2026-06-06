package copilot

import (
	"context"
	"sync"

	"go.klarlabs.de/agent/domain/tool"
)

// Adapter bridges agent-go tools with Copilot SDK sessions.
// It enables bidirectional tool sharing between the two systems.
type Adapter struct {
	registry    tool.Registry
	toolFilter  ToolFilter
	contextFunc ContextFunc
	mu          sync.RWMutex
}

// ToolFilter determines which tools should be exposed to Copilot.
type ToolFilter func(tool.Tool) bool

// ContextFunc provides context for tool executions.
// This allows customizing the context passed to tools.
type ContextFunc func(invocation ToolInvocation) context.Context

// Option configures the Adapter.
type Option func(*Adapter)

// WithToolFilter sets a filter for which tools to expose.
func WithToolFilter(filter ToolFilter) Option {
	return func(a *Adapter) {
		a.toolFilter = filter
	}
}

// WithContextFunc sets a function to provide context for tool executions.
func WithContextFunc(fn ContextFunc) Option {
	return func(a *Adapter) {
		a.contextFunc = fn
	}
}

// OnlyReadOnly returns a filter that only exposes read-only tools.
func OnlyReadOnly() ToolFilter {
	return func(t tool.Tool) bool {
		return t.Annotations().ReadOnly
	}
}

// OnlyNonDestructive returns a filter that excludes destructive tools.
func OnlyNonDestructive() ToolFilter {
	return func(t tool.Tool) bool {
		return !t.Annotations().Destructive
	}
}

// OnlyWithTags returns a filter that only exposes tools with specific tags.
func OnlyWithTags(tags ...string) ToolFilter {
	tagSet := make(map[string]bool)
	for _, tag := range tags {
		tagSet[tag] = true
	}
	return func(t tool.Tool) bool {
		for _, tag := range t.Annotations().Tags {
			if tagSet[tag] {
				return true
			}
		}
		return false
	}
}

// NewAdapter creates a new Copilot adapter.
func NewAdapter(registry tool.Registry, opts ...Option) *Adapter {
	a := &Adapter{
		registry: registry,
	}

	for _, opt := range opts {
		opt(a)
	}

	return a
}

// GetCopilotTools returns Copilot SDK tools for all registered agent-go tools.
// Use this when creating a Copilot session.
func (a *Adapter) GetCopilotTools() []Tool {
	a.mu.RLock()
	defer a.mu.RUnlock()

	agentTools := a.registry.List()
	if a.toolFilter != nil {
		filtered := make([]tool.Tool, 0, len(agentTools))
		for _, t := range agentTools {
			if a.toolFilter(t) {
				filtered = append(filtered, t)
			}
		}
		agentTools = filtered
	}

	result := make([]Tool, len(agentTools))
	for i, t := range agentTools {
		result[i] = a.convertToolWithContext(t)
	}

	return result
}

// convertToolWithContext converts a tool and wraps the handler with context.
func (a *Adapter) convertToolWithContext(agentTool tool.Tool) Tool {
	baseHandler := createHandler(agentTool)

	if a.contextFunc == nil {
		return Tool{
			Name:        agentTool.Name(),
			Description: agentTool.Description(),
			Parameters:  schemaToParameters(agentTool.InputSchema()),
			Handler:     baseHandler,
		}
	}

	// Wrap handler with custom context
	wrappedHandler := func(invocation ToolInvocation) (ToolResult, error) {
		ctx := a.contextFunc(invocation)
		return a.executeWithContext(ctx, agentTool, invocation)
	}

	return Tool{
		Name:        agentTool.Name(),
		Description: agentTool.Description(),
		Parameters:  schemaToParameters(agentTool.InputSchema()),
		Handler:     wrappedHandler,
	}
}

// executeWithContext executes a tool with a specific context.
func (a *Adapter) executeWithContext(ctx context.Context, agentTool tool.Tool, invocation ToolInvocation) (ToolResult, error) {
	input, err := marshalArguments(invocation.Arguments)
	if err != nil {
		return ToolResult{
			ResultType: "error",
			Error:      "failed to marshal arguments: " + err.Error(),
		}, nil
	}

	result, err := agentTool.Execute(ctx, input)
	if err != nil {
		return ToolResult{
			ResultType: "error",
			Error:      err.Error(),
		}, nil
	}

	return ToolResult{
		TextResultForLLM: string(result.Output),
		ResultType:       "success",
	}, nil
}

// CreateSessionConfig creates a SessionConfig with the adapter's tools.
// This is a convenience method for setting up Copilot sessions.
func (a *Adapter) CreateSessionConfig(model string, streaming bool) *SessionConfig {
	return &SessionConfig{
		Model:     model,
		Streaming: streaming,
		Tools:     a.GetCopilotTools(),
	}
}

// Refresh updates the adapter's view of tools from the registry.
// Call this if tools are added or removed after adapter creation.
func (a *Adapter) Refresh() {
	// This is a no-op since we read from registry on each call,
	// but provides a clear API for users who might cache tools.
}
