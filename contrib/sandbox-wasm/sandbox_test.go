package wasm

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/tool"
	"go.klarlabs.de/agent/infrastructure/security/sandbox"
)

func TestNewSandbox(t *testing.T) {
	sb, err := NewSandbox()
	if err != nil {
		t.Fatalf("NewSandbox: %v", err)
	}
	defer sb.Close()

	if sb.runtime == nil {
		t.Error("expected non-nil runtime")
	}
}

func TestNewSandbox_WithOptions(t *testing.T) {
	sb, err := NewSandbox(
		sandbox.WithMaxMemory(64<<20),           // 64MB
		sandbox.WithMaxExecTime(30*time.Second), // 30s
	)
	if err != nil {
		t.Fatalf("NewSandbox with options: %v", err)
	}
	defer sb.Close()

	caps := sb.Capabilities()
	if caps.MaxMemory != 64<<20 {
		t.Errorf("MaxMemory: got %d, want %d", caps.MaxMemory, 64<<20)
	}
	if caps.MaxExecTime != 30*time.Second {
		t.Errorf("MaxExecTime: got %v, want %v", caps.MaxExecTime, 30*time.Second)
	}
	if caps.Network {
		t.Error("expected network disabled by default")
	}
}

func TestSandbox_Capabilities(t *testing.T) {
	sb, err := NewSandbox(
		sandbox.WithMaxMemory(1<<20),
		sandbox.WithMaxExecTime(5*time.Second),
	)
	if err != nil {
		t.Fatalf("NewSandbox: %v", err)
	}
	defer sb.Close()

	caps := sb.Capabilities()
	if caps.Network {
		t.Error("WASM should not have network access")
	}
	if caps.MaxMemory != 1<<20 {
		t.Errorf("MaxMemory: got %d, want %d", caps.MaxMemory, 1<<20)
	}
}

// mockTool is a non-WASM tool for testing fallback execution.
type mockTool struct {
	name   string
	result tool.Result
	err    error
}

func (m *mockTool) Name() string                  { return m.name }
func (m *mockTool) Description() string           { return "mock" }
func (m *mockTool) Annotations() tool.Annotations { return tool.Annotations{} }
func (m *mockTool) InputSchema() tool.Schema      { return tool.Schema{} }
func (m *mockTool) OutputSchema() tool.Schema     { return tool.Schema{} }
func (m *mockTool) Execute(_ context.Context, _ json.RawMessage) (tool.Result, error) {
	return m.result, m.err
}

func TestSandbox_FallbackExecution(t *testing.T) {
	sb, err := NewSandbox()
	if err != nil {
		t.Fatalf("NewSandbox: %v", err)
	}
	defer sb.Close()

	// A non-WASM tool should execute directly
	mt := &mockTool{
		name:   "echo",
		result: tool.Result{Output: json.RawMessage(`{"ok":true}`)},
	}

	result, err := sb.Execute(context.Background(), mt, json.RawMessage(`{"input":"test"}`))
	if err != nil {
		t.Fatalf("Execute fallback: %v", err)
	}
	if string(result.Output) != `{"ok":true}` {
		t.Errorf("output: got %s, want {\"ok\":true}", result.Output)
	}
}

func TestSandbox_FallbackError(t *testing.T) {
	sb, err := NewSandbox()
	if err != nil {
		t.Fatalf("NewSandbox: %v", err)
	}
	defer sb.Close()

	mt := &mockTool{
		name: "failing",
		err:  context.DeadlineExceeded,
	}

	_, err = sb.Execute(context.Background(), mt, json.RawMessage(`{}`))
	if err != context.DeadlineExceeded {
		t.Errorf("expected DeadlineExceeded, got %v", err)
	}
}

func TestSandbox_Close(t *testing.T) {
	sb, err := NewSandbox()
	if err != nil {
		t.Fatalf("NewSandbox: %v", err)
	}

	if err := sb.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}

	// Close again should be safe
	if err := sb.Close(); err != nil {
		t.Errorf("double Close: %v", err)
	}
}

func TestSandbox_ContextCancellation(t *testing.T) {
	sb, err := NewSandbox(sandbox.WithMaxExecTime(50 * time.Millisecond))
	if err != nil {
		t.Fatalf("NewSandbox: %v", err)
	}
	defer sb.Close()

	// Cancel context immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	mt := &mockTool{
		name:   "cancelled",
		err:    nil,
		result: tool.Result{Output: json.RawMessage(`{}`)},
	}

	// Non-WASM fallback should still propagate cancelled context
	// (the mock doesn't check ctx, so this tests the sandbox doesn't block)
	_, _ = sb.Execute(ctx, mt, json.RawMessage(`{}`))
}

func TestSandbox_InterfaceCompliance(t *testing.T) {
	var _ sandbox.Sandbox = (*Sandbox)(nil)
}
