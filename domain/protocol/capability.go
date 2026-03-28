package protocol

// Capability advertises what an agent can do.
// Used for capability-based routing and discovery.
type Capability struct {
	// Name is the capability identifier (e.g., "research", "code-review").
	Name string `json:"name"`

	// Description is a human-readable explanation.
	Description string `json:"description,omitempty"`

	// Actions lists specific actions this agent handles.
	Actions []string `json:"actions,omitempty"`

	// ToolNames lists tools this agent exposes for delegation.
	ToolNames []string `json:"tool_names,omitempty"`

	// MaxConcurrency is the max parallel requests this agent handles (0 = unlimited).
	MaxConcurrency int `json:"max_concurrency,omitempty"`
}

// AgentDescriptor describes an agent in the protocol registry.
type AgentDescriptor struct {
	// Name is the unique agent identifier.
	Name string `json:"name"`

	// Description is a human-readable summary.
	Description string `json:"description,omitempty"`

	// Capabilities lists what this agent can do.
	Capabilities []Capability `json:"capabilities"`

	// TrustLevel indicates the trust boundary for this agent.
	TrustLevel TrustLevel `json:"trust_level"`

	// Metadata holds additional agent-specific data.
	Metadata map[string]string `json:"metadata,omitempty"`
}

// HasCapability checks if the agent has a specific capability.
func (d AgentDescriptor) HasCapability(name string) bool {
	for _, c := range d.Capabilities {
		if c.Name == name {
			return true
		}
	}
	return false
}

// HandlesAction checks if the agent handles a specific action.
func (d AgentDescriptor) HandlesAction(action string) bool {
	for _, c := range d.Capabilities {
		for _, a := range c.Actions {
			if a == action {
				return true
			}
		}
	}
	return false
}
