// Package pack provides types for reusable tool collections.
package pack

import (
	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/tool"
)

// Pack is a collection of related tools with their eligibility rules.
type Pack struct {
	// Name is the unique identifier for the pack.
	Name string

	// Description explains what the pack provides.
	Description string

	// Version is the semantic version of the pack.
	Version string

	// Tools is the collection of tools in this pack.
	Tools []tool.Tool

	// Eligibility maps states to tool names allowed in that state.
	Eligibility map[agent.State][]string

	// Dependencies lists other packs this pack depends on.
	Dependencies []string

	// Metadata holds additional pack information.
	Metadata map[string]string
}

// ToolNames returns the names of all tools in the pack.
func (p *Pack) ToolNames() []string {
	names := make([]string, len(p.Tools))
	for i, t := range p.Tools {
		names[i] = t.Name()
	}
	return names
}

// GetTool returns a tool by name from the pack.
func (p *Pack) GetTool(name string) (tool.Tool, bool) {
	for _, t := range p.Tools {
		if t.Name() == name {
			return t, true
		}
	}
	return nil, false
}

// AllowedInState returns tools allowed in the given state.
func (p *Pack) AllowedInState(state agent.State) []string {
	if allowed, ok := p.Eligibility[state]; ok {
		return allowed
	}
	return nil
}

// Builder provides a fluent API for constructing packs.
type Builder struct {
	pack *Pack
}

// NewBuilder creates a new pack builder.
func NewBuilder(name string) *Builder {
	return &Builder{
		pack: &Pack{
			Name:        name,
			Tools:       make([]tool.Tool, 0),
			Eligibility: make(map[agent.State][]string),
			Metadata:    make(map[string]string),
		},
	}
}

// WithDescription sets the pack description.
func (b *Builder) WithDescription(desc string) *Builder {
	b.pack.Description = desc
	return b
}

// WithVersion sets the pack version.
func (b *Builder) WithVersion(version string) *Builder {
	b.pack.Version = version
	return b
}

// AddTool adds a tool to the pack.
func (b *Builder) AddTool(t tool.Tool) *Builder {
	b.pack.Tools = append(b.pack.Tools, t)
	return b
}

// AddTools adds multiple tools to the pack.
func (b *Builder) AddTools(tools ...tool.Tool) *Builder {
	b.pack.Tools = append(b.pack.Tools, tools...)
	return b
}

// AllowInState allows specified tools in the given state.
func (b *Builder) AllowInState(state agent.State, toolNames ...string) *Builder {
	b.pack.Eligibility[state] = append(b.pack.Eligibility[state], toolNames...)
	return b
}

// AllowAllInState allows all pack tools in the given state.
func (b *Builder) AllowAllInState(state agent.State) *Builder {
	for _, t := range b.pack.Tools {
		b.pack.Eligibility[state] = append(b.pack.Eligibility[state], t.Name())
	}
	return b
}

// WithDependency adds a dependency on another pack.
func (b *Builder) WithDependency(packName string) *Builder {
	b.pack.Dependencies = append(b.pack.Dependencies, packName)
	return b
}

// WithMetadata adds metadata to the pack.
func (b *Builder) WithMetadata(key, value string) *Builder {
	b.pack.Metadata[key] = value
	return b
}

// Build returns the constructed pack.
func (b *Builder) Build() *Pack {
	return b.pack
}
