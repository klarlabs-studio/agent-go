package kubernetes_test

import (
	"context"
	"encoding/json"
	"testing"

	kubernetes "github.com/felixgeelhaar/agent-go/contrib/pack-kubernetes"
	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/tool"
)

func TestPack(t *testing.T) {
	p := kubernetes.Pack()
	if p == nil {
		t.Fatal("Pack() returned nil")
	}
	if p.Name != "kubernetes" {
		t.Errorf("expected pack name %q, got %q", "kubernetes", p.Name)
	}
	if p.Version != "0.2.0" {
		t.Errorf("expected version %q, got %q", "0.2.0", p.Version)
	}
}

func TestToolCount(t *testing.T) {
	p := kubernetes.Pack()
	want := 9 // get, describe, logs, apply, delete, exec, port_forward, get_contexts, get_namespaces
	if got := len(p.Tools); got != want {
		t.Errorf("expected %d tools, got %d", want, got)
	}
}

func TestToolNames(t *testing.T) {
	p := kubernetes.Pack()
	expected := map[string]bool{
		"kubectl_get":            true,
		"kubectl_describe":       true,
		"kubectl_logs":           true,
		"kubectl_apply":          true,
		"kubectl_delete":         true,
		"kubectl_exec":           true,
		"kubectl_port_forward":   true,
		"kubectl_get_contexts":   true,
		"kubectl_get_namespaces": true,
	}

	for _, tt := range p.Tools {
		if !expected[tt.Name()] {
			t.Errorf("unexpected tool name: %q", tt.Name())
		}
		delete(expected, tt.Name())
	}
	for name := range expected {
		t.Errorf("missing tool: %q", name)
	}
}

func TestToolsImplementInterface(t *testing.T) {
	p := kubernetes.Pack()
	for _, tt := range p.Tools {
		var _ tool.Tool = tt
		if tt.Name() == "" {
			t.Error("tool has empty name")
		}
		if tt.Description() == "" {
			t.Errorf("tool %q has empty description", tt.Name())
		}
	}
}

func TestToolAnnotations(t *testing.T) {
	p := kubernetes.Pack()

	tests := []struct {
		name            string
		wantReadOnly    bool
		wantDestructive bool
		wantApproval    bool
		wantIdempotent  bool
		wantCacheable   bool
	}{
		{name: "kubectl_get", wantReadOnly: true, wantCacheable: true},
		{name: "kubectl_describe", wantReadOnly: true},
		{name: "kubectl_logs", wantReadOnly: true},
		{name: "kubectl_apply", wantDestructive: true, wantApproval: true, wantIdempotent: true},
		{name: "kubectl_delete", wantDestructive: true, wantApproval: true},
		{name: "kubectl_exec", wantApproval: true},
		{name: "kubectl_get_contexts", wantReadOnly: true, wantCacheable: true},
		{name: "kubectl_get_namespaces", wantReadOnly: true, wantCacheable: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tt, ok := p.GetTool(tc.name)
			if !ok {
				t.Fatalf("tool %q not found", tc.name)
			}
			ann := tt.Annotations()
			if ann.ReadOnly != tc.wantReadOnly {
				t.Errorf("ReadOnly: got %v, want %v", ann.ReadOnly, tc.wantReadOnly)
			}
			if ann.Destructive != tc.wantDestructive {
				t.Errorf("Destructive: got %v, want %v", ann.Destructive, tc.wantDestructive)
			}
			if ann.RequiresApproval != tc.wantApproval {
				t.Errorf("RequiresApproval: got %v, want %v", ann.RequiresApproval, tc.wantApproval)
			}
			if ann.Idempotent != tc.wantIdempotent {
				t.Errorf("Idempotent: got %v, want %v", ann.Idempotent, tc.wantIdempotent)
			}
			if ann.Cacheable != tc.wantCacheable {
				t.Errorf("Cacheable: got %v, want %v", ann.Cacheable, tc.wantCacheable)
			}
		})
	}
}

func TestEligibility(t *testing.T) {
	p := kubernetes.Pack()

	readOnlyTools := []string{
		"kubectl_get", "kubectl_describe", "kubectl_logs",
		"kubectl_get_contexts", "kubectl_get_namespaces",
	}

	// Explore state: read-only tools
	explore := p.AllowedInState(agent.StateExplore)
	for _, name := range readOnlyTools {
		if !contains(explore, name) {
			t.Errorf("expected %q allowed in explore state", name)
		}
	}

	// Act state: all tools except port_forward should be usable
	act := p.AllowedInState(agent.StateAct)
	actTools := append(readOnlyTools, "kubectl_apply", "kubectl_delete", "kubectl_exec")
	for _, name := range actTools {
		if !contains(act, name) {
			t.Errorf("expected %q allowed in act state", name)
		}
	}

	// Validate state: same as explore (read-only)
	validate := p.AllowedInState(agent.StateValidate)
	for _, name := range readOnlyTools {
		if !contains(validate, name) {
			t.Errorf("expected %q allowed in validate state", name)
		}
	}

	// Intake state: no tools
	intake := p.AllowedInState(agent.StateIntake)
	if len(intake) != 0 {
		t.Errorf("expected no tools in intake state, got %v", intake)
	}
}

func TestAllToolsHaveHandlers(t *testing.T) {
	p := kubernetes.Pack()
	for _, tt := range p.Tools {
		// Calling Execute with nil input should NOT return ErrNoHandler.
		// It may fail for other reasons (invalid input, kubectl not found),
		// but the handler must be registered.
		ctx := context.Background()
		_, err := tt.Execute(ctx, json.RawMessage(`{}`))
		if err != nil && err.Error() == "tool has no handler" {
			t.Errorf("tool %q has no handler registered", tt.Name())
		}
	}
}

func TestPortForwardReturnsError(t *testing.T) {
	p := kubernetes.Pack()
	tt, ok := p.GetTool("kubectl_port_forward")
	if !ok {
		t.Fatal("kubectl_port_forward tool not found")
	}

	ctx := context.Background()
	result, err := tt.Execute(ctx, json.RawMessage(`{}`))
	if err == nil {
		t.Fatal("expected error from port_forward, got nil")
	}
	if result.Output == nil {
		t.Fatal("expected non-nil output with error details")
	}

	var msg map[string]string
	if jsonErr := json.Unmarshal(result.Output, &msg); jsonErr != nil {
		t.Fatalf("failed to parse port_forward output: %v", jsonErr)
	}
	if msg["error"] != "unsupported" {
		t.Errorf("expected error field %q, got %q", "unsupported", msg["error"])
	}
}

func TestHandlerInputValidation(t *testing.T) {
	p := kubernetes.Pack()
	ctx := context.Background()

	tests := []struct {
		toolName string
		input    string
		wantErr  string
	}{
		// kubectl_get requires "resource"
		{toolName: "kubectl_get", input: `{}`, wantErr: "resource is required"},
		// kubectl_describe requires "resource" and "name"
		{toolName: "kubectl_describe", input: `{}`, wantErr: "resource is required"},
		{toolName: "kubectl_describe", input: `{"resource":"pods"}`, wantErr: "name is required"},
		// kubectl_logs requires "pod"
		{toolName: "kubectl_logs", input: `{}`, wantErr: "pod is required"},
		// kubectl_apply requires "manifest"
		{toolName: "kubectl_apply", input: `{}`, wantErr: "manifest is required"},
		// kubectl_delete requires "resource" and "name"
		{toolName: "kubectl_delete", input: `{}`, wantErr: "resource is required"},
		{toolName: "kubectl_delete", input: `{"resource":"pods"}`, wantErr: "name is required"},
		// kubectl_exec requires "pod" and "command"
		{toolName: "kubectl_exec", input: `{}`, wantErr: "pod is required"},
		{toolName: "kubectl_exec", input: `{"pod":"mypod"}`, wantErr: "command is required"},
		// Invalid JSON
		{toolName: "kubectl_get", input: `{invalid`, wantErr: "parsing input"},
	}

	for _, tc := range tests {
		t.Run(tc.toolName+"/"+tc.wantErr, func(t *testing.T) {
			tt, ok := p.GetTool(tc.toolName)
			if !ok {
				t.Fatalf("tool %q not found", tc.toolName)
			}
			_, err := tt.Execute(ctx, json.RawMessage(tc.input))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !containsString(err.Error(), tc.wantErr) {
				t.Errorf("expected error containing %q, got %q", tc.wantErr, err.Error())
			}
		})
	}
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) && searchString(haystack, needle)
}

func searchString(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
