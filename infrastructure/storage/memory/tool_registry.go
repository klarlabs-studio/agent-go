// Package memory provides in-memory storage implementations.
package memory

import (
	"sync"

	"go.klarlabs.de/agent/domain/tool"
)

// ToolRegistry is an in-memory implementation of tool.Registry.
type ToolRegistry struct {
	tools map[string]tool.Tool
	mu    sync.RWMutex
}

// NewToolRegistry creates a new in-memory tool registry.
func NewToolRegistry() *ToolRegistry {
	return &ToolRegistry{
		tools: make(map[string]tool.Tool),
	}
}

// Register adds a tool to the registry.
func (r *ToolRegistry) Register(t tool.Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[t.Name()]; exists {
		return tool.ErrToolExists
	}

	r.tools[t.Name()] = t
	return nil
}

// Get retrieves a tool by name.
func (r *ToolRegistry) Get(name string) (tool.Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	t, ok := r.tools[name]
	return t, ok
}

// List returns all registered tools.
func (r *ToolRegistry) List() []tool.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	tools := make([]tool.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

// Names returns all registered tool names.
func (r *ToolRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	return names
}

// Has checks if a tool is registered.
func (r *ToolRegistry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	_, ok := r.tools[name]
	return ok
}

// Unregister removes a tool from the registry.
func (r *ToolRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; !exists {
		return tool.ErrToolNotFound
	}

	delete(r.tools, name)
	return nil
}

// Clear removes all tools from the registry.
func (r *ToolRegistry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tools = make(map[string]tool.Tool)
}

// Count returns the number of registered tools.
func (r *ToolRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}
