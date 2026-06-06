// Package task provides shared state for multi-agent task coordination.
//
// A TaskContext spans multiple agent runs in a delegation hierarchy,
// enabling agents to share variables, evidence, and artifact references.
// It is thread-safe for concurrent access by multiple agents.
//
// Usage:
//
//	tc := task.NewContext("task-1", "root-run-id")
//	tc.SetVar("api_key", "sk-...")
//	tc.AddEvidence(agent.NewToolEvidence("search", result))
//
//	// Pass to child engine
//	childEngine, _ := api.New(api.WithTaskContext(tc), ...)
package task

import (
	"context"
	"sync"

	"go.klarlabs.de/agent/domain/agent"
)

// Context spans a multi-agent task, enabling state sharing across runs.
// All methods are thread-safe.
type Context struct {
	// ID is the unique identifier for this task.
	ID string

	// RootRunID is the run that originated this task.
	RootRunID string

	mu          sync.RWMutex
	sharedVars  map[string]any
	privateVars map[string]map[string]any // runID -> vars (scoped per agent)
	evidence    []agent.Evidence
	artifactIDs []string
}

// NewContext creates a new task context.
func NewContext(id, rootRunID string) *Context {
	return &Context{
		ID:          id,
		RootRunID:   rootRunID,
		sharedVars:  make(map[string]any),
		privateVars: make(map[string]map[string]any),
	}
}

// SetPrivateVar sets a variable scoped to a specific run (not visible to other agents).
func (c *Context) SetPrivateVar(runID, key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.privateVars[runID] == nil {
		c.privateVars[runID] = make(map[string]any)
	}
	c.privateVars[runID][key] = value
}

// GetPrivateVar reads a run-scoped variable.
func (c *Context) GetPrivateVar(runID, key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if vars, ok := c.privateVars[runID]; ok {
		v, found := vars[key]
		return v, found
	}
	return nil, false
}

// PrivateVars returns a snapshot of all private variables for a run.
func (c *Context) PrivateVars(runID string) map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	vars, ok := c.privateVars[runID]
	if !ok {
		return nil
	}
	snapshot := make(map[string]any, len(vars))
	for k, v := range vars {
		snapshot[k] = v
	}
	return snapshot
}

// SetVar sets a shared variable visible to all agents in this task.
func (c *Context) SetVar(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sharedVars[key] = value
}

// GetVar reads a shared variable.
func (c *Context) GetVar(key string) (any, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.sharedVars[key]
	return v, ok
}

// Vars returns a snapshot of all shared variables.
func (c *Context) Vars() map[string]any {
	c.mu.RLock()
	defer c.mu.RUnlock()
	snapshot := make(map[string]any, len(c.sharedVars))
	for k, v := range c.sharedVars {
		snapshot[k] = v
	}
	return snapshot
}

// AddEvidence appends shared evidence visible to all agents.
func (c *Context) AddEvidence(e agent.Evidence) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.evidence = append(c.evidence, e)
}

// Evidence returns a copy of all shared evidence.
func (c *Context) Evidence() []agent.Evidence {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]agent.Evidence, len(c.evidence))
	copy(result, c.evidence)
	return result
}

// AddArtifactRef registers a shared artifact ID.
func (c *Context) AddArtifactRef(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.artifactIDs = append(c.artifactIDs, id)
}

// ArtifactRefs returns all shared artifact IDs.
func (c *Context) ArtifactRefs() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]string, len(c.artifactIDs))
	copy(result, c.artifactIDs)
	return result
}

// Context key for propagating run ID through Go context.
type contextKey struct{}

// WithRunID returns a new context with the run ID set.
func WithRunID(ctx context.Context, runID string) context.Context {
	return context.WithValue(ctx, contextKey{}, runID)
}

// RunIDFromContext extracts the run ID from the context.
// Returns empty string if not set.
func RunIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(contextKey{}).(string); ok {
		return v
	}
	return ""
}
