# Package `pack`

**Import path:** `go.klarlabs.de/agent/domain/pack`

## Overview

package pack // import "go.klarlabs.de/agent/domain/pack"

Package pack provides types for reusable tool collections.

## Full API Reference

```
package pack // import "go.klarlabs.de/agent/domain/pack"

Package pack provides types for reusable tool collections.

VARIABLES

var (
	// ErrPackNotFound is returned when a pack does not exist.
	ErrPackNotFound = errors.New("pack not found")

	// ErrPackExists is returned when a pack already exists.
	ErrPackExists = errors.New("pack already exists")

	// ErrInvalidPack is returned when a pack is invalid.
	ErrInvalidPack = errors.New("invalid pack")

	// ErrDependencyNotFound is returned when a required dependency is missing.
	ErrDependencyNotFound = errors.New("pack dependency not found")

	// ErrCircularDependency is returned when packs have circular dependencies.
	ErrCircularDependency = errors.New("circular pack dependency detected")
)
    Domain errors for pack operations.


TYPES

type Builder struct {
	// Has unexported fields.
}
    Builder provides a fluent API for constructing packs.

func NewBuilder(name string) *Builder
    NewBuilder creates a new pack builder.

func (b *Builder) AddTool(t tool.Tool) *Builder
    AddTool adds a tool to the pack.

func (b *Builder) AddTools(tools ...tool.Tool) *Builder
    AddTools adds multiple tools to the pack.

func (b *Builder) AllowAllInState(state agent.State) *Builder
    AllowAllInState allows all pack tools in the given state.

func (b *Builder) AllowInState(state agent.State, toolNames ...string) *Builder
    AllowInState allows specified tools in the given state.

func (b *Builder) Build() *Pack
    Build returns the constructed pack.

func (b *Builder) WithDependency(packName string) *Builder
    WithDependency adds a dependency on another pack.

func (b *Builder) WithDescription(desc string) *Builder
    WithDescription sets the pack description.

func (b *Builder) WithMetadata(key, value string) *Builder
    WithMetadata adds metadata to the pack.

func (b *Builder) WithVersion(version string) *Builder
    WithVersion sets the pack version.

type Installer interface {
	// InstallPack adds all tools from a pack to the tool registry
	// and configures their eligibility.
	InstallPack(pack *Pack, toolReg tool.Registry, eligibility *policy.ToolEligibility) error
}
    Installer installs packs into the runtime.

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
    Pack is a collection of related tools with their eligibility rules.

func (p *Pack) AllowedInState(state agent.State) []string
    AllowedInState returns tools allowed in the given state.

func (p *Pack) GetTool(name string) (tool.Tool, bool)
    GetTool returns a tool by name from the pack.

func (p *Pack) ToolNames() []string
    ToolNames returns the names of all tools in the pack.

type Registry interface {
	// Register adds a pack to the registry.
	Register(pack *Pack) error

	// Get retrieves a pack by name.
	Get(name string) (*Pack, bool)

	// List returns all registered packs.
	List() []*Pack

	// Unregister removes a pack from the registry.
	Unregister(name string) error

	// Install installs a pack's tools into a tool registry and eligibility policy.
	Install(name string, toolReg tool.Registry, eligibility *policy.ToolEligibility) error
}
    Registry manages a collection of packs.
```
