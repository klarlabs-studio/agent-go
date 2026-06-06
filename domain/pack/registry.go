package pack

import (
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/domain/tool"
)

// Registry manages a collection of packs.
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

// Installer installs packs into the runtime.
type Installer interface {
	// InstallPack adds all tools from a pack to the tool registry
	// and configures their eligibility.
	InstallPack(pack *Pack, toolReg tool.Registry, eligibility *policy.ToolEligibility) error
}
