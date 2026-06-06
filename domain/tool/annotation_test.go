package tool_test

import (
	"testing"

	"go.klarlabs.de/agent/domain/tool"
)

func TestRiskLevel_String(t *testing.T) {
	t.Parallel()

	tests := []struct {
		level    tool.RiskLevel
		expected string
	}{
		{tool.RiskNone, "none"},
		{tool.RiskLow, "low"},
		{tool.RiskMedium, "medium"},
		{tool.RiskHigh, "high"},
		{tool.RiskCritical, "critical"},
		{tool.RiskLevel(99), "unknown"}, // Unknown risk level
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			t.Parallel()

			if got := tt.level.String(); got != tt.expected {
				t.Errorf("RiskLevel.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDefaultAnnotations(t *testing.T) {
	t.Parallel()

	annotations := tool.DefaultAnnotations()

	if annotations.ReadOnly {
		t.Error("DefaultAnnotations().ReadOnly should be false")
	}
	if annotations.Destructive {
		t.Error("DefaultAnnotations().Destructive should be false")
	}
	if annotations.Idempotent {
		t.Error("DefaultAnnotations().Idempotent should be false")
	}
	if annotations.Cacheable {
		t.Error("DefaultAnnotations().Cacheable should be false")
	}
	if annotations.RiskLevel != tool.RiskLow {
		t.Errorf("DefaultAnnotations().RiskLevel = %v, want %v", annotations.RiskLevel, tool.RiskLow)
	}
	if annotations.RequiresApproval {
		t.Error("DefaultAnnotations().RequiresApproval should be false")
	}
}

func TestReadOnlyAnnotations(t *testing.T) {
	t.Parallel()

	annotations := tool.ReadOnlyAnnotations()

	if !annotations.ReadOnly {
		t.Error("ReadOnlyAnnotations().ReadOnly should be true")
	}
	if !annotations.Idempotent {
		t.Error("ReadOnlyAnnotations().Idempotent should be true")
	}
	if !annotations.Cacheable {
		t.Error("ReadOnlyAnnotations().Cacheable should be true")
	}
	if annotations.RiskLevel != tool.RiskNone {
		t.Errorf("ReadOnlyAnnotations().RiskLevel = %v, want %v", annotations.RiskLevel, tool.RiskNone)
	}
}

func TestDestructiveAnnotations(t *testing.T) {
	t.Parallel()

	annotations := tool.DestructiveAnnotations()

	if !annotations.Destructive {
		t.Error("DestructiveAnnotations().Destructive should be true")
	}
	if annotations.RiskLevel != tool.RiskHigh {
		t.Errorf("DestructiveAnnotations().RiskLevel = %v, want %v", annotations.RiskLevel, tool.RiskHigh)
	}
	if !annotations.RequiresApproval {
		t.Error("DestructiveAnnotations().RequiresApproval should be true")
	}
}

func TestAnnotations_ShouldRequireApproval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		annotations tool.Annotations
		expected    bool
	}{
		{
			name:        "default annotations",
			annotations: tool.DefaultAnnotations(),
			expected:    false,
		},
		{
			name: "requires approval set",
			annotations: tool.Annotations{
				RequiresApproval: true,
			},
			expected: true,
		},
		{
			name: "destructive tool",
			annotations: tool.Annotations{
				Destructive: true,
			},
			expected: true,
		},
		{
			name: "high risk tool",
			annotations: tool.Annotations{
				RiskLevel: tool.RiskHigh,
			},
			expected: true,
		},
		{
			name: "critical risk tool",
			annotations: tool.Annotations{
				RiskLevel: tool.RiskCritical,
			},
			expected: true,
		},
		{
			name: "medium risk tool without other flags",
			annotations: tool.Annotations{
				RiskLevel: tool.RiskMedium,
			},
			expected: false,
		},
		{
			name:        "read-only tool",
			annotations: tool.ReadOnlyAnnotations(),
			expected:    false,
		},
		{
			name:        "destructive annotations",
			annotations: tool.DestructiveAnnotations(),
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.annotations.ShouldRequireApproval(); got != tt.expected {
				t.Errorf("ShouldRequireApproval() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAnnotations_CanCache(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		annotations tool.Annotations
		expected    bool
	}{
		{
			name:        "default annotations",
			annotations: tool.DefaultAnnotations(),
			expected:    false,
		},
		{
			name: "cacheable only",
			annotations: tool.Annotations{
				Cacheable: true,
			},
			expected: false, // Needs ReadOnly or Idempotent
		},
		{
			name: "cacheable and read-only",
			annotations: tool.Annotations{
				Cacheable: true,
				ReadOnly:  true,
			},
			expected: true,
		},
		{
			name: "cacheable and idempotent",
			annotations: tool.Annotations{
				Cacheable:  true,
				Idempotent: true,
			},
			expected: true,
		},
		{
			name: "read-only but not cacheable",
			annotations: tool.Annotations{
				ReadOnly: true,
			},
			expected: false,
		},
		{
			name:        "read-only annotations (has all flags)",
			annotations: tool.ReadOnlyAnnotations(),
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.annotations.CanCache(); got != tt.expected {
				t.Errorf("CanCache() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestAnnotations_CanRetry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		annotations tool.Annotations
		expected    bool
	}{
		{
			name:        "default annotations",
			annotations: tool.DefaultAnnotations(),
			expected:    false,
		},
		{
			name: "idempotent tool",
			annotations: tool.Annotations{
				Idempotent: true,
			},
			expected: true,
		},
		{
			name: "read-only tool",
			annotations: tool.Annotations{
				ReadOnly: true,
			},
			expected: true,
		},
		{
			name: "idempotent and read-only",
			annotations: tool.Annotations{
				Idempotent: true,
				ReadOnly:   true,
			},
			expected: true,
		},
		{
			name:        "destructive annotations",
			annotations: tool.DestructiveAnnotations(),
			expected:    false,
		},
		{
			name:        "read-only annotations",
			annotations: tool.ReadOnlyAnnotations(),
			expected:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.annotations.CanRetry(); got != tt.expected {
				t.Errorf("CanRetry() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRiskLevel_Ordering(t *testing.T) {
	t.Parallel()

	// Verify risk levels have correct ordering
	if tool.RiskNone >= tool.RiskLow {
		t.Error("RiskNone should be less than RiskLow")
	}
	if tool.RiskLow >= tool.RiskMedium {
		t.Error("RiskLow should be less than RiskMedium")
	}
	if tool.RiskMedium >= tool.RiskHigh {
		t.Error("RiskMedium should be less than RiskHigh")
	}
	if tool.RiskHigh >= tool.RiskCritical {
		t.Error("RiskHigh should be less than RiskCritical")
	}
}
