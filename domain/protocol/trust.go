package protocol

import "errors"

// TrustLevel defines trust boundaries between agents.
type TrustLevel int

const (
	// TrustNone indicates no trust — all actions require explicit approval.
	TrustNone TrustLevel = iota
	// TrustReadOnly allows read-only tool access without approval.
	TrustReadOnly
	// TrustLimited allows non-destructive tools without approval.
	TrustLimited
	// TrustFull allows all tools without approval.
	TrustFull
)

// String returns the trust level name.
func (t TrustLevel) String() string {
	switch t {
	case TrustNone:
		return "none"
	case TrustReadOnly:
		return "read_only"
	case TrustLimited:
		return "limited"
	case TrustFull:
		return "full"
	default:
		return "unknown"
	}
}

// Permission defines what one agent is allowed to do when communicating with another.
type Permission struct {
	// AllowedActions lists actions this agent may request (empty = all).
	AllowedActions []string `json:"allowed_actions,omitempty"`

	// DeniedActions lists actions this agent may NOT request.
	DeniedActions []string `json:"denied_actions,omitempty"`

	// MaxBudget limits tool calls the remote agent can make on behalf of this one.
	MaxBudget int `json:"max_budget,omitempty"`

	// RequireApproval forces approval for all requests from this agent.
	RequireApproval bool `json:"require_approval,omitempty"`
}

// TrustPolicy defines trust relationships between agents.
type TrustPolicy struct {
	// DefaultTrust is the trust level for unregistered agents.
	DefaultTrust TrustLevel `json:"default_trust"`

	// AgentTrust maps agent names to specific trust levels.
	AgentTrust map[string]TrustLevel `json:"agent_trust,omitempty"`

	// AgentPermissions maps agent names to specific permissions.
	AgentPermissions map[string]Permission `json:"agent_permissions,omitempty"`
}

// NewTrustPolicy creates a policy with the given default trust level.
func NewTrustPolicy(defaultTrust TrustLevel) *TrustPolicy {
	return &TrustPolicy{
		DefaultTrust:     defaultTrust,
		AgentTrust:       make(map[string]TrustLevel),
		AgentPermissions: make(map[string]Permission),
	}
}

// SetTrust configures the trust level for a specific agent.
func (p *TrustPolicy) SetTrust(agentName string, level TrustLevel) {
	p.AgentTrust[agentName] = level
}

// SetPermission configures permissions for a specific agent.
func (p *TrustPolicy) SetPermission(agentName string, perm Permission) {
	p.AgentPermissions[agentName] = perm
}

// TrustFor returns the trust level for the given agent.
func (p *TrustPolicy) TrustFor(agentName string) TrustLevel {
	if level, ok := p.AgentTrust[agentName]; ok {
		return level
	}
	return p.DefaultTrust
}

// PermissionFor returns the permissions for the given agent.
func (p *TrustPolicy) PermissionFor(agentName string) Permission {
	if perm, ok := p.AgentPermissions[agentName]; ok {
		return perm
	}
	return Permission{} // no restrictions
}

// IsActionAllowed checks if an agent is permitted to request the given action.
func (p *TrustPolicy) IsActionAllowed(agentName, action string) bool {
	perm := p.PermissionFor(agentName)

	// Check denied list first
	for _, denied := range perm.DeniedActions {
		if denied == action {
			return false
		}
	}

	// If allowed list is specified, action must be in it
	if len(perm.AllowedActions) > 0 {
		for _, allowed := range perm.AllowedActions {
			if allowed == action {
				return true
			}
		}
		return false
	}

	return true
}

// Errors for protocol violations.
var (
	ErrUntrustedAgent  = errors.New("agent is not trusted")
	ErrActionDenied    = errors.New("action denied by trust policy")
	ErrBudgetExhausted = errors.New("agent budget exhausted")
	ErrTimeout         = errors.New("message timeout")
	ErrNoHandler       = errors.New("no handler for action")
	ErrAgentNotFound   = errors.New("agent not found in registry")
)
