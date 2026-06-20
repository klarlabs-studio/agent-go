// Package tool provides the domain model for agent tools.
package tool

// RiskLevel indicates the potential impact of a tool execution.
type RiskLevel int

const (
	RiskNone     RiskLevel = iota // No risk - purely informational
	RiskLow                       // Low risk - reversible changes
	RiskMedium                    // Medium risk - may require cleanup
	RiskHigh                      // High risk - difficult to reverse
	RiskCritical                  // Critical risk - irreversible or destructive
)

// String returns the string representation of the risk level.
func (r RiskLevel) String() string {
	switch r {
	case RiskNone:
		return "none"
	case RiskLow:
		return "low"
	case RiskMedium:
		return "medium"
	case RiskHigh:
		return "high"
	case RiskCritical:
		return "critical"
	default:
		return "unknown"
	}
}

// Annotations describe tool behavior for policy enforcement, caching, and planning.
type Annotations struct {
	// ReadOnly indicates the tool has no side effects.
	ReadOnly bool `json:"read_only"`

	// Destructive indicates the tool may cause irreversible changes.
	Destructive bool `json:"destructive"`

	// Idempotent indicates multiple calls with same input yield same result.
	Idempotent bool `json:"idempotent"`

	// Cacheable indicates results can be cached.
	Cacheable bool `json:"cacheable"`

	// RiskLevel indicates the potential impact of execution.
	RiskLevel RiskLevel `json:"risk_level"`

	// RequiresApproval indicates human approval is required.
	RequiresApproval bool `json:"requires_approval"`

	// Timeout is the maximum execution time in seconds (0 = default).
	Timeout int `json:"timeout,omitempty"`

	// Sandboxed indicates the tool should execute in an isolated sandbox.
	Sandboxed bool `json:"sandboxed"`

	// Tags are arbitrary labels for categorization.
	Tags []string `json:"tags,omitempty"`
}

// DefaultAnnotations returns annotations with safe defaults.
func DefaultAnnotations() Annotations {
	return Annotations{
		ReadOnly:         false,
		Destructive:      false,
		Idempotent:       false,
		Cacheable:        false,
		RiskLevel:        RiskLow,
		RequiresApproval: false,
	}
}

// ReadOnlyAnnotations returns annotations for a read-only tool.
func ReadOnlyAnnotations() Annotations {
	return Annotations{
		ReadOnly:   true,
		Idempotent: true,
		Cacheable:  true,
		RiskLevel:  RiskNone,
	}
}

// DestructiveAnnotations returns annotations for a destructive tool.
func DestructiveAnnotations() Annotations {
	return Annotations{
		Destructive:      true,
		RiskLevel:        RiskHigh,
		RequiresApproval: true,
	}
}

// HasSideEffects reports whether executing the tool may mutate external state.
//
// This is the STRUCTURAL definition of a side effect used to enforce the
// non-negotiable invariant "side effects only in the act state". A tool has
// side effects unless it is explicitly marked ReadOnly. Destructive tools
// always have side effects, even if mislabelled ReadOnly (defensive).
//
// Unlike ShouldRequireApproval (a configurable, risk-based policy hint), this
// method is the ground truth the engine uses to reject side-effecting tools in
// non-act states. It must remain a pure function of the annotations and must
// not be widened by configuration.
func (a Annotations) HasSideEffects() bool {
	if a.Destructive {
		return true
	}
	return !a.ReadOnly
}

// ShouldRequireApproval returns true if the tool should require approval.
func (a Annotations) ShouldRequireApproval() bool {
	return a.RequiresApproval || a.Destructive || a.RiskLevel >= RiskHigh
}

// CanCache returns true if the tool result can be cached.
func (a Annotations) CanCache() bool {
	return a.Cacheable && (a.ReadOnly || a.Idempotent)
}

// CanRetry returns true if the tool can be safely retried on failure.
func (a Annotations) CanRetry() bool {
	return a.Idempotent || a.ReadOnly
}
