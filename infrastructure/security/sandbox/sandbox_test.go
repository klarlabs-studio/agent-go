package sandbox_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/middleware"
	"github.com/felixgeelhaar/agent-go/domain/tool"
	"github.com/felixgeelhaar/agent-go/infrastructure/security/sandbox"
)

// mockTool is a simple tool for testing.
type mockTool struct {
	name        string
	annotations tool.Annotations
	execute     func(ctx context.Context, input json.RawMessage) (tool.Result, error)
}

func (m *mockTool) Name() string                  { return m.name }
func (m *mockTool) Description() string           { return "mock tool" }
func (m *mockTool) InputSchema() tool.Schema      { return tool.Schema{} }
func (m *mockTool) OutputSchema() tool.Schema     { return tool.Schema{} }
func (m *mockTool) Annotations() tool.Annotations { return m.annotations }

func (m *mockTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	if m.execute != nil {
		return m.execute(ctx, input)
	}
	return tool.Result{Output: json.RawMessage(`{"success":true}`)}, nil
}

func TestNoopSandbox(t *testing.T) {
	t.Parallel()

	t.Run("executes tool directly", func(t *testing.T) {
		t.Parallel()

		sb := sandbox.NewNoop()
		executed := false

		mockT := &mockTool{
			name: "test",
			execute: func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
				executed = true
				return tool.Result{Output: json.RawMessage(`{"result":"ok"}`)}, nil
			},
		}

		result, err := sb.Execute(context.Background(), mockT, json.RawMessage(`{}`))
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		if !executed {
			t.Error("Tool was not executed")
		}

		if string(result.Output) != `{"result":"ok"}` {
			t.Errorf("Output = %s, want %s", result.Output, `{"result":"ok"}`)
		}
	})

	t.Run("capabilities are unlimited", func(t *testing.T) {
		t.Parallel()

		sb := sandbox.NewNoop()
		caps := sb.Capabilities()

		if !caps.Network {
			t.Error("Network should be true")
		}
		if !caps.Filesystem {
			t.Error("Filesystem should be true")
		}
		if caps.MaxMemory != 0 {
			t.Errorf("MaxMemory = %d, want 0", caps.MaxMemory)
		}
		if caps.MaxExecTime != 0 {
			t.Errorf("MaxExecTime = %v, want 0", caps.MaxExecTime)
		}
	})

	t.Run("close is no-op", func(t *testing.T) {
		t.Parallel()

		sb := sandbox.NewNoop()
		if err := sb.Close(); err != nil {
			t.Errorf("Close() error = %v", err)
		}
	})
}

func TestCapabilities(t *testing.T) {
	t.Parallel()

	caps := sandbox.Capabilities{
		Network:       true,
		Filesystem:    false,
		MaxMemory:     64 * 1024 * 1024,
		MaxExecTime:   30 * time.Second,
		AllowedEnv:    []string{"PATH", "HOME"},
		ReadOnlyPaths: []string{"/etc"},
		WritePaths:    []string{"/tmp"},
	}

	// Verify JSON serialization
	data, err := json.Marshal(caps)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	var decoded sandbox.Capabilities
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}

	if decoded.Network != caps.Network {
		t.Errorf("Network = %v, want %v", decoded.Network, caps.Network)
	}
	if decoded.MaxMemory != caps.MaxMemory {
		t.Errorf("MaxMemory = %d, want %d", decoded.MaxMemory, caps.MaxMemory)
	}
}

func TestConfig(t *testing.T) {
	t.Parallel()

	t.Run("applies options", func(t *testing.T) {
		t.Parallel()

		cfg := sandbox.Config{}

		sandbox.WithMaxMemory(128 * 1024 * 1024)(&cfg)
		if cfg.MaxMemory != 128*1024*1024 {
			t.Errorf("MaxMemory = %d, want %d", cfg.MaxMemory, 128*1024*1024)
		}

		sandbox.WithMaxExecTime(60 * time.Second)(&cfg)
		if cfg.MaxExecTime != 60*time.Second {
			t.Errorf("MaxExecTime = %v, want %v", cfg.MaxExecTime, 60*time.Second)
		}

		sandbox.WithNetwork()(&cfg)
		if !cfg.AllowNetwork {
			t.Error("AllowNetwork should be true")
		}

		sandbox.WithFilesystem("/data", []string{"/etc"}, []string{"/tmp"})(&cfg)
		if !cfg.AllowFilesystem {
			t.Error("AllowFilesystem should be true")
		}
		if cfg.FSRoot != "/data" {
			t.Errorf("FSRoot = %s, want /data", cfg.FSRoot)
		}

		sandbox.WithEnv("PATH", "HOME")(&cfg)
		if len(cfg.AllowedEnv) != 2 {
			t.Errorf("AllowedEnv length = %d, want 2", len(cfg.AllowedEnv))
		}
	})
}

func TestShouldSandbox(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		annotations tool.Annotations
		want        bool
	}{
		{
			name:        "sandboxed annotation",
			annotations: tool.Annotations{Sandboxed: true},
			want:        true,
		},
		{
			name:        "destructive tool",
			annotations: tool.Annotations{Destructive: true},
			want:        true,
		},
		{
			name:        "high risk level",
			annotations: tool.Annotations{RiskLevel: 3},
			want:        true,
		},
		{
			name:        "low risk level",
			annotations: tool.Annotations{RiskLevel: 2},
			want:        false,
		},
		{
			name:        "read only",
			annotations: tool.Annotations{ReadOnly: true},
			want:        false,
		},
		{
			name:        "default",
			annotations: tool.Annotations{},
			want:        false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockT := &mockTool{
				name:        "test",
				annotations: tt.annotations,
			}

			if got := sandbox.ShouldSandbox(mockT); got != tt.want {
				t.Errorf("ShouldSandbox() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSandboxMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("sandboxes tool with Sandboxed annotation", func(t *testing.T) {
		t.Parallel()

		sb := sandbox.NewNoop()
		mw := sandbox.SandboxMiddleware(sb)

		sandboxed := false
		mockT := &mockTool{
			name:        "sandboxed_tool",
			annotations: tool.Annotations{Sandboxed: true},
			execute: func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
				sandboxed = true
				return tool.Result{Output: json.RawMessage(`{"sandboxed":true}`)}, nil
			},
		}

		execCtx := &middleware.ExecutionContext{
			Tool:  mockT,
			Input: json.RawMessage(`{}`),
		}

		nextCalled := false
		next := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
			nextCalled = true
			return tool.Result{}, nil
		}

		handler := mw(next)
		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("handler() error = %v", err)
		}

		if !sandboxed {
			t.Error("Tool was not executed via sandbox")
		}
		if nextCalled {
			t.Error("next should not be called for sandboxed tools")
		}
		if string(result.Output) != `{"sandboxed":true}` {
			t.Errorf("Output = %s, want {\"sandboxed\":true}", result.Output)
		}
	})

	t.Run("sandboxes tool with Destructive annotation", func(t *testing.T) {
		t.Parallel()

		sb := sandbox.NewNoop()
		mw := sandbox.SandboxMiddleware(sb)

		executed := false
		mockT := &mockTool{
			name:        "destructive_tool",
			annotations: tool.Annotations{Destructive: true},
			execute: func(ctx context.Context, input json.RawMessage) (tool.Result, error) {
				executed = true
				return tool.Result{Output: json.RawMessage(`{}`)}, nil
			},
		}

		execCtx := &middleware.ExecutionContext{
			Tool:  mockT,
			Input: json.RawMessage(`{}`),
		}

		nextCalled := false
		next := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
			nextCalled = true
			return tool.Result{}, nil
		}

		handler := mw(next)
		_, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("handler() error = %v", err)
		}

		if !executed {
			t.Error("Destructive tool was not executed")
		}
		if nextCalled {
			t.Error("next should not be called for destructive tools")
		}
	})

	t.Run("passes through non-sandboxed tool", func(t *testing.T) {
		t.Parallel()

		sb := sandbox.NewNoop()
		mw := sandbox.SandboxMiddleware(sb)

		mockT := &mockTool{
			name:        "normal_tool",
			annotations: tool.Annotations{}, // No sandboxing annotations
		}

		execCtx := &middleware.ExecutionContext{
			Tool:  mockT,
			Input: json.RawMessage(`{}`),
		}

		nextCalled := false
		next := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
			nextCalled = true
			return tool.Result{Output: json.RawMessage(`{"next":true}`)}, nil
		}

		handler := mw(next)
		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("handler() error = %v", err)
		}

		if !nextCalled {
			t.Error("next should be called for non-sandboxed tools")
		}
		if string(result.Output) != `{"next":true}` {
			t.Errorf("Output = %s, want {\"next\":true}", result.Output)
		}
	})
}

func TestConditionalSandboxMiddleware(t *testing.T) {
	t.Parallel()

	t.Run("uses ReadOnlySandbox for read-only tools", func(t *testing.T) {
		t.Parallel()

		readOnlySandboxUsed := false
		defaultSandboxUsed := false

		readOnlySb := &trackingSandbox{
			noop:    sandbox.NewNoop(),
			tracker: &readOnlySandboxUsed,
		}
		defaultSb := &trackingSandbox{
			noop:    sandbox.NewNoop(),
			tracker: &defaultSandboxUsed,
		}

		cond := &sandbox.ConditionalSandboxMiddleware{
			ReadOnlySandbox: readOnlySb,
			DefaultSandbox:  defaultSb,
			Predicate:       func(t tool.Tool) bool { return true }, // Always sandbox
		}

		mw := cond.Middleware()

		mockT := &mockTool{
			name:        "readonly_tool",
			annotations: tool.Annotations{ReadOnly: true},
		}

		execCtx := &middleware.ExecutionContext{
			Tool:  mockT,
			Input: json.RawMessage(`{}`),
		}

		next := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
			return tool.Result{}, nil
		}

		handler := mw(next)
		_, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("handler() error = %v", err)
		}

		if !readOnlySandboxUsed {
			t.Error("ReadOnlySandbox should be used for read-only tools")
		}
		if defaultSandboxUsed {
			t.Error("DefaultSandbox should not be used for read-only tools")
		}
	})

	t.Run("uses DefaultSandbox for non-read-only tools", func(t *testing.T) {
		t.Parallel()

		readOnlySandboxUsed := false
		defaultSandboxUsed := false

		readOnlySb := &trackingSandbox{
			noop:    sandbox.NewNoop(),
			tracker: &readOnlySandboxUsed,
		}
		defaultSb := &trackingSandbox{
			noop:    sandbox.NewNoop(),
			tracker: &defaultSandboxUsed,
		}

		cond := &sandbox.ConditionalSandboxMiddleware{
			ReadOnlySandbox: readOnlySb,
			DefaultSandbox:  defaultSb,
			Predicate:       func(t tool.Tool) bool { return true },
		}

		mw := cond.Middleware()

		mockT := &mockTool{
			name:        "write_tool",
			annotations: tool.Annotations{ReadOnly: false},
		}

		execCtx := &middleware.ExecutionContext{
			Tool:  mockT,
			Input: json.RawMessage(`{}`),
		}

		next := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
			return tool.Result{}, nil
		}

		handler := mw(next)
		_, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("handler() error = %v", err)
		}

		if readOnlySandboxUsed {
			t.Error("ReadOnlySandbox should not be used for non-read-only tools")
		}
		if !defaultSandboxUsed {
			t.Error("DefaultSandbox should be used for non-read-only tools")
		}
	})

	t.Run("skips sandboxing when predicate returns false", func(t *testing.T) {
		t.Parallel()

		sandboxUsed := false
		sb := &trackingSandbox{
			noop:    sandbox.NewNoop(),
			tracker: &sandboxUsed,
		}

		cond := &sandbox.ConditionalSandboxMiddleware{
			DefaultSandbox: sb,
			Predicate:      func(t tool.Tool) bool { return false }, // Never sandbox
		}

		mw := cond.Middleware()

		mockT := &mockTool{
			name:        "any_tool",
			annotations: tool.Annotations{},
		}

		execCtx := &middleware.ExecutionContext{
			Tool:  mockT,
			Input: json.RawMessage(`{}`),
		}

		nextCalled := false
		next := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
			nextCalled = true
			return tool.Result{Output: json.RawMessage(`{}`)}, nil
		}

		handler := mw(next)
		_, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("handler() error = %v", err)
		}

		if sandboxUsed {
			t.Error("Sandbox should not be used when predicate returns false")
		}
		if !nextCalled {
			t.Error("next should be called when predicate returns false")
		}
	})

	t.Run("passes through when no sandbox configured", func(t *testing.T) {
		t.Parallel()

		cond := &sandbox.ConditionalSandboxMiddleware{
			// No sandboxes configured
			Predicate: func(t tool.Tool) bool { return true },
		}

		mw := cond.Middleware()

		mockT := &mockTool{
			name:        "any_tool",
			annotations: tool.Annotations{},
		}

		execCtx := &middleware.ExecutionContext{
			Tool:  mockT,
			Input: json.RawMessage(`{}`),
		}

		nextCalled := false
		next := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
			nextCalled = true
			return tool.Result{Output: json.RawMessage(`{}`)}, nil
		}

		handler := mw(next)
		_, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("handler() error = %v", err)
		}

		if !nextCalled {
			t.Error("next should be called when no sandbox is configured")
		}
	})

	t.Run("nil predicate sandboxes all tools", func(t *testing.T) {
		t.Parallel()

		sandboxUsed := false
		sb := &trackingSandbox{
			noop:    sandbox.NewNoop(),
			tracker: &sandboxUsed,
		}

		cond := &sandbox.ConditionalSandboxMiddleware{
			DefaultSandbox: sb,
			Predicate:      nil, // No predicate means always evaluate
		}

		mw := cond.Middleware()

		mockT := &mockTool{
			name:        "any_tool",
			annotations: tool.Annotations{},
		}

		execCtx := &middleware.ExecutionContext{
			Tool:  mockT,
			Input: json.RawMessage(`{}`),
		}

		next := func(ctx context.Context, ec *middleware.ExecutionContext) (tool.Result, error) {
			return tool.Result{}, nil
		}

		handler := mw(next)
		_, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("handler() error = %v", err)
		}

		if !sandboxUsed {
			t.Error("Sandbox should be used when predicate is nil")
		}
	})
}

// trackingSandbox wraps a sandbox and tracks whether it was used.
type trackingSandbox struct {
	noop    *sandbox.NoopSandbox
	tracker *bool
}

func (s *trackingSandbox) Execute(ctx context.Context, t tool.Tool, input json.RawMessage) (tool.Result, error) {
	*s.tracker = true
	return s.noop.Execute(ctx, t, input)
}

func (s *trackingSandbox) Capabilities() sandbox.Capabilities {
	return s.noop.Capabilities()
}

func (s *trackingSandbox) Close() error {
	return s.noop.Close()
}
