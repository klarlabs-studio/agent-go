package policy_test

import (
	"context"
	"encoding/json"
	"testing"

	"go.klarlabs.de/agent/domain/policy"
)

func TestAutoApprover(t *testing.T) {
	t.Parallel()

	t.Run("creates with name", func(t *testing.T) {
		t.Parallel()

		approver := policy.NewAutoApprover("test-approver")
		if approver == nil {
			t.Fatal("NewAutoApprover returned nil")
		}
	})

	t.Run("approves all requests", func(t *testing.T) {
		t.Parallel()

		approver := policy.NewAutoApprover("auto-system")
		req := policy.ApprovalRequest{
			RunID:     "run-123",
			ToolName:  "dangerous_tool",
			Input:     json.RawMessage(`{"action": "delete"}`),
			Reason:    "test request",
			RiskLevel: "high",
		}

		resp, err := approver.Approve(context.Background(), req)
		if err != nil {
			t.Fatalf("Approve() error = %v", err)
		}
		if !resp.Approved {
			t.Error("AutoApprover should approve all requests")
		}
		if resp.Approver != "auto-system" {
			t.Errorf("Approver = %s, want auto-system", resp.Approver)
		}
		if resp.Reason != "auto-approved" {
			t.Errorf("Reason = %s, want auto-approved", resp.Reason)
		}
		if resp.Timestamp.IsZero() {
			t.Error("Timestamp should not be zero")
		}
	})
}

func TestDenyApprover(t *testing.T) {
	t.Parallel()

	t.Run("creates with reason", func(t *testing.T) {
		t.Parallel()

		approver := policy.NewDenyApprover("not allowed")
		if approver == nil {
			t.Fatal("NewDenyApprover returned nil")
		}
	})

	t.Run("denies all requests", func(t *testing.T) {
		t.Parallel()

		approver := policy.NewDenyApprover("security policy violation")
		req := policy.ApprovalRequest{
			RunID:     "run-456",
			ToolName:  "any_tool",
			Input:     json.RawMessage(`{}`),
			Reason:    "test request",
			RiskLevel: "low",
		}

		resp, err := approver.Approve(context.Background(), req)
		if err != nil {
			t.Fatalf("Approve() error = %v", err)
		}
		if resp.Approved {
			t.Error("DenyApprover should deny all requests")
		}
		if resp.Reason != "security policy violation" {
			t.Errorf("Reason = %s, want 'security policy violation'", resp.Reason)
		}
		if resp.Timestamp.IsZero() {
			t.Error("Timestamp should not be zero")
		}
	})
}

func TestDefaultApprovalPolicy(t *testing.T) {
	t.Parallel()

	p := policy.DefaultApprovalPolicy()

	if !p.RequireForDestructive {
		t.Error("RequireForDestructive should be true")
	}
	if !p.RequireForHighRisk {
		t.Error("RequireForHighRisk should be true")
	}
}

func TestApprovalPolicy_RequiresApproval(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		policy        policy.ApprovalPolicy
		toolName      string
		isDestructive bool
		isHighRisk    bool
		want          bool
	}{
		{
			name: "exempt tool",
			policy: policy.ApprovalPolicy{
				RequireForDestructive: true,
				ExemptTools:           []string{"safe_tool"},
			},
			toolName:      "safe_tool",
			isDestructive: true,
			isHighRisk:    true,
			want:          false,
		},
		{
			name: "required tool",
			policy: policy.ApprovalPolicy{
				RequireForTools: []string{"critical_tool"},
			},
			toolName:      "critical_tool",
			isDestructive: false,
			isHighRisk:    false,
			want:          true,
		},
		{
			name: "destructive tool",
			policy: policy.ApprovalPolicy{
				RequireForDestructive: true,
			},
			toolName:      "delete_tool",
			isDestructive: true,
			isHighRisk:    false,
			want:          true,
		},
		{
			name: "high risk tool",
			policy: policy.ApprovalPolicy{
				RequireForHighRisk: true,
			},
			toolName:      "risky_tool",
			isDestructive: false,
			isHighRisk:    true,
			want:          true,
		},
		{
			name: "normal tool no policy",
			policy: policy.ApprovalPolicy{
				RequireForDestructive: false,
				RequireForHighRisk:    false,
			},
			toolName:      "normal_tool",
			isDestructive: false,
			isHighRisk:    false,
			want:          false,
		},
		{
			name: "destructive but not required",
			policy: policy.ApprovalPolicy{
				RequireForDestructive: false,
			},
			toolName:      "delete_tool",
			isDestructive: true,
			isHighRisk:    false,
			want:          false,
		},
		{
			name: "not destructive with requirement",
			policy: policy.ApprovalPolicy{
				RequireForDestructive: true,
			},
			toolName:      "safe_tool",
			isDestructive: false,
			isHighRisk:    false,
			want:          false,
		},
		{
			name: "exempt overrides required",
			policy: policy.ApprovalPolicy{
				RequireForTools: []string{"special_tool"},
				ExemptTools:     []string{"special_tool"},
			},
			toolName:      "special_tool",
			isDestructive: false,
			isHighRisk:    false,
			want:          false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := tt.policy.RequiresApproval(tt.toolName, tt.isDestructive, tt.isHighRisk)
			if got != tt.want {
				t.Errorf("RequiresApproval() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestApprovalRequest_Fields(t *testing.T) {
	t.Parallel()

	req := policy.ApprovalRequest{
		RunID:     "run-789",
		ToolName:  "test_tool",
		Input:     json.RawMessage(`{"key": "value"}`),
		Reason:    "test reason",
		RiskLevel: "medium",
	}

	if req.RunID != "run-789" {
		t.Errorf("RunID = %s, want run-789", req.RunID)
	}
	if req.ToolName != "test_tool" {
		t.Errorf("ToolName = %s, want test_tool", req.ToolName)
	}
	if req.Reason != "test reason" {
		t.Errorf("Reason = %s, want 'test reason'", req.Reason)
	}
	if req.RiskLevel != "medium" {
		t.Errorf("RiskLevel = %s, want medium", req.RiskLevel)
	}
}

func TestApprovalResponse_Fields(t *testing.T) {
	t.Parallel()

	resp := policy.ApprovalResponse{
		Approved: true,
		Approver: "admin",
		Reason:   "approved by admin",
	}

	if !resp.Approved {
		t.Error("Approved should be true")
	}
	if resp.Approver != "admin" {
		t.Errorf("Approver = %s, want admin", resp.Approver)
	}
	if resp.Reason != "approved by admin" {
		t.Errorf("Reason = %s, want 'approved by admin'", resp.Reason)
	}
}
