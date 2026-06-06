// Package pack provides pack registry implementation.
package pack

import (
	"sync"

	"go.klarlabs.de/agent/domain/pack"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/domain/tool"
)

// Registry is an in-memory pack registry.
type Registry struct {
	packs map[string]*pack.Pack
	mu    sync.RWMutex
}

// NewRegistry creates a new pack registry.
func NewRegistry() *Registry {
	return &Registry{
		packs: make(map[string]*pack.Pack),
	}
}

// Register adds a pack to the registry.
func (r *Registry) Register(p *pack.Pack) error {
	if p == nil || p.Name == "" {
		return pack.ErrInvalidPack
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.packs[p.Name]; exists {
		return pack.ErrPackExists
	}

	r.packs[p.Name] = p
	return nil
}

// Get retrieves a pack by name.
func (r *Registry) Get(name string) (*pack.Pack, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, ok := r.packs[name]
	return p, ok
}

// List returns all registered packs.
func (r *Registry) List() []*pack.Pack {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]*pack.Pack, 0, len(r.packs))
	for _, p := range r.packs {
		result = append(result, p)
	}
	return result
}

// Unregister removes a pack from the registry.
func (r *Registry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.packs[name]; !exists {
		return pack.ErrPackNotFound
	}

	delete(r.packs, name)
	return nil
}

// Install installs a pack's tools into a tool registry and eligibility policy.
func (r *Registry) Install(name string, toolReg tool.Registry, eligibility *policy.ToolEligibility) error {
	r.mu.RLock()
	p, ok := r.packs[name]
	r.mu.RUnlock()

	if !ok {
		return pack.ErrPackNotFound
	}

	// Install dependencies first
	for _, depName := range p.Dependencies {
		if _, found := r.Get(depName); !found {
			return pack.ErrDependencyNotFound
		}
		if err := r.Install(depName, toolReg, eligibility); err != nil {
			return err
		}
	}

	// Register all tools
	for _, t := range p.Tools {
		// Ignore if tool already exists (from dependency)
		if toolReg.Has(t.Name()) {
			continue
		}
		if err := toolReg.Register(t); err != nil {
			return err
		}
	}

	// Configure eligibility
	for state, toolNames := range p.Eligibility {
		for _, toolName := range toolNames {
			eligibility.Allow(state, toolName)
		}
	}

	return nil
}

// InstallPack adds all tools from a pack without checking registry.
func (r *Registry) InstallPack(p *pack.Pack, toolReg tool.Registry, eligibility *policy.ToolEligibility) error {
	if p == nil {
		return pack.ErrInvalidPack
	}

	// Register all tools
	for _, t := range p.Tools {
		if toolReg.Has(t.Name()) {
			continue
		}
		if err := toolReg.Register(t); err != nil {
			return err
		}
	}

	// Configure eligibility
	for state, toolNames := range p.Eligibility {
		for _, toolName := range toolNames {
			eligibility.Allow(state, toolName)
		}
	}

	return nil
}

// Clear removes all packs from the registry.
func (r *Registry) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.packs = make(map[string]*pack.Pack)
}

// Len returns the number of registered packs.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.packs)
}

// Ensure Registry implements pack.Registry and pack.Installer
var (
	_ pack.Registry  = (*Registry)(nil)
	_ pack.Installer = (*Registry)(nil)
)
