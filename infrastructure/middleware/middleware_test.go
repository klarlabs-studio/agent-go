package middleware_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/cache"
	"go.klarlabs.de/agent/domain/ledger"
	domainmw "go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/policy"
	"go.klarlabs.de/agent/domain/simulation"
	"go.klarlabs.de/agent/domain/tool"
	mw "go.klarlabs.de/agent/infrastructure/middleware"
)

// mockTool implements tool.Tool for testing.
type mockTool struct {
	name        string
	annotations tool.Annotations
	handler     func(ctx context.Context, input json.RawMessage) (tool.Result, error)
}

func (m *mockTool) Name() string              { return m.name }
func (m *mockTool) Description() string       { return "mock tool" }
func (m *mockTool) InputSchema() tool.Schema  { return tool.Schema{} }
func (m *mockTool) OutputSchema() tool.Schema { return tool.Schema{} }
func (m *mockTool) Annotations() tool.Annotations {
	return m.annotations
}
func (m *mockTool) Execute(ctx context.Context, input json.RawMessage) (tool.Result, error) {
	if m.handler != nil {
		return m.handler(ctx, input)
	}
	return tool.Result{Output: json.RawMessage(`{"status":"ok"}`)}, nil
}

// mockCache implements cache.Cache for testing.
type mockCache struct {
	data map[string][]byte
}

func newMockCache() *mockCache {
	return &mockCache{data: make(map[string][]byte)}
}

func (c *mockCache) Get(_ context.Context, key string) ([]byte, bool, error) {
	val, ok := c.data[key]
	return val, ok, nil
}

func (c *mockCache) Set(_ context.Context, key string, value []byte, _ cache.SetOptions) error {
	c.data[key] = value
	return nil
}

func (c *mockCache) Delete(_ context.Context, key string) error {
	delete(c.data, key)
	return nil
}

func (c *mockCache) Exists(_ context.Context, key string) (bool, error) {
	_, ok := c.data[key]
	return ok, nil
}

func (c *mockCache) Clear(_ context.Context) error {
	c.data = make(map[string][]byte)
	return nil
}

// mockApprover implements policy.Approver for testing.
type mockApprover struct {
	approved bool
	reason   string
	err      error
}

func (a *mockApprover) Approve(_ context.Context, _ policy.ApprovalRequest) (policy.ApprovalResponse, error) {
	if a.err != nil {
		return policy.ApprovalResponse{}, a.err
	}
	return policy.ApprovalResponse{
		Approved: a.approved,
		Reason:   a.reason,
	}, nil
}

// createTestHandler creates a simple handler for testing.
func createTestHandler(result tool.Result, err error) domainmw.Handler {
	return func(_ context.Context, _ *domainmw.ExecutionContext) (tool.Result, error) {
		return result, err
	}
}

func TestEligibility(t *testing.T) {
	t.Parallel()

	t.Run("allows tool in permitted state", func(t *testing.T) {
		t.Parallel()

		eligibility := policy.NewToolEligibility()
		eligibility.Allow(agent.StateExplore, "read_file")

		middleware := mw.Eligibility(mw.EligibilityConfig{
			Eligibility: eligibility,
		})

		mockT := &mockTool{name: "read_file"}
		execCtx := &domainmw.ExecutionContext{
			CurrentState: agent.StateExplore,
			Tool:         mockT,
		}

		expected := tool.Result{Output: json.RawMessage(`{"status":"ok"}`)}
		handler := middleware(createTestHandler(expected, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(expected.Output) {
			t.Errorf("got output %s, want %s", result.Output, expected.Output)
		}
	})

	t.Run("blocks tool in non-permitted state", func(t *testing.T) {
		t.Parallel()

		eligibility := policy.NewToolEligibility()
		eligibility.Allow(agent.StateExplore, "read_file")

		middleware := mw.Eligibility(mw.EligibilityConfig{
			Eligibility: eligibility,
		})

		mockT := &mockTool{name: "read_file"}
		execCtx := &domainmw.ExecutionContext{
			CurrentState: agent.StateAct, // Not allowed
			Tool:         mockT,
		}

		handler := middleware(createTestHandler(tool.Result{}, nil))

		_, err := handler(context.Background(), execCtx)
		if err == nil {
			t.Fatal("expected error for blocked tool")
		}
		if !errors.Is(err, tool.ErrToolNotAllowed) {
			t.Errorf("expected ErrToolNotAllowed, got %v", err)
		}
	})

	t.Run("passes through when no eligibility configured", func(t *testing.T) {
		t.Parallel()

		middleware := mw.Eligibility(mw.EligibilityConfig{
			Eligibility: nil,
		})

		mockT := &mockTool{name: "any_tool"}
		execCtx := &domainmw.ExecutionContext{
			CurrentState: agent.StateAct,
			Tool:         mockT,
		}

		expected := tool.Result{Output: json.RawMessage(`{"passed":"through"}`)}
		handler := middleware(createTestHandler(expected, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(expected.Output) {
			t.Errorf("got output %s, want %s", result.Output, expected.Output)
		}
	})
}

func TestApproval(t *testing.T) {
	t.Parallel()

	t.Run("passes through for non-destructive tools", func(t *testing.T) {
		t.Parallel()

		middleware := mw.Approval(mw.ApprovalConfig{
			Approver: &mockApprover{approved: true},
		})

		mockT := &mockTool{
			name:        "read_file",
			annotations: tool.Annotations{ReadOnly: true},
		}
		execCtx := &domainmw.ExecutionContext{
			Tool: mockT,
		}

		expected := tool.Result{Output: json.RawMessage(`{"read":"ok"}`)}
		handler := middleware(createTestHandler(expected, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(expected.Output) {
			t.Errorf("got output %s, want %s", result.Output, expected.Output)
		}
	})

	t.Run("requires approval for destructive tools", func(t *testing.T) {
		t.Parallel()

		middleware := mw.Approval(mw.ApprovalConfig{
			Approver: &mockApprover{approved: true},
		})

		mockT := &mockTool{
			name:        "delete_file",
			annotations: tool.Annotations{Destructive: true, RiskLevel: tool.RiskHigh},
		}
		execCtx := &domainmw.ExecutionContext{
			Tool:   mockT,
			Reason: "cleanup",
		}

		expected := tool.Result{Output: json.RawMessage(`{"deleted":"ok"}`)}
		handler := middleware(createTestHandler(expected, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(expected.Output) {
			t.Errorf("got output %s, want %s", result.Output, expected.Output)
		}
	})

	t.Run("blocks when approval denied", func(t *testing.T) {
		t.Parallel()

		middleware := mw.Approval(mw.ApprovalConfig{
			Approver: &mockApprover{approved: false, reason: "not authorized"},
		})

		mockT := &mockTool{
			name:        "delete_file",
			annotations: tool.Annotations{Destructive: true, RiskLevel: tool.RiskHigh},
		}
		execCtx := &domainmw.ExecutionContext{
			Tool: mockT,
		}

		handler := middleware(createTestHandler(tool.Result{}, nil))

		_, err := handler(context.Background(), execCtx)
		if err == nil {
			t.Fatal("expected error for denied approval")
		}
		if !errors.Is(err, tool.ErrApprovalDenied) {
			t.Errorf("expected ErrApprovalDenied, got %v", err)
		}
	})

	t.Run("fails when no approver configured for destructive tool", func(t *testing.T) {
		t.Parallel()

		middleware := mw.Approval(mw.ApprovalConfig{
			Approver: nil,
		})

		mockT := &mockTool{
			name:        "delete_file",
			annotations: tool.Annotations{Destructive: true, RiskLevel: tool.RiskHigh},
		}
		execCtx := &domainmw.ExecutionContext{
			Tool: mockT,
		}

		handler := middleware(createTestHandler(tool.Result{}, nil))

		_, err := handler(context.Background(), execCtx)
		if err == nil {
			t.Fatal("expected error when no approver configured")
		}
		if !errors.Is(err, tool.ErrApprovalRequired) {
			t.Errorf("expected ErrApprovalRequired, got %v", err)
		}
	})

	t.Run("returns approver error", func(t *testing.T) {
		t.Parallel()

		expectedErr := errors.New("approval service unavailable")
		middleware := mw.Approval(mw.ApprovalConfig{
			Approver: &mockApprover{err: expectedErr},
		})

		mockT := &mockTool{
			name:        "delete_file",
			annotations: tool.Annotations{Destructive: true, RiskLevel: tool.RiskHigh},
		}
		execCtx := &domainmw.ExecutionContext{
			Tool: mockT,
		}

		handler := middleware(createTestHandler(tool.Result{}, nil))

		_, err := handler(context.Background(), execCtx)
		if err == nil {
			t.Fatal("expected error from approver")
		}
	})
}

func TestBudget(t *testing.T) {
	t.Parallel()

	t.Run("allows execution when budget available", func(t *testing.T) {
		t.Parallel()

		budget := policy.NewBudget(map[string]int{"tool_calls": 10})

		middleware := mw.Budget(mw.BudgetConfig{
			Budget:     budget,
			BudgetName: "tool_calls",
			Amount:     1,
		})

		execCtx := &domainmw.ExecutionContext{
			Tool: &mockTool{name: "test"},
		}

		expected := tool.Result{Output: json.RawMessage(`{"ok":true}`)}
		handler := middleware(createTestHandler(expected, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(expected.Output) {
			t.Errorf("got output %s, want %s", result.Output, expected.Output)
		}
	})

	t.Run("blocks when budget exceeded", func(t *testing.T) {
		t.Parallel()

		budget := policy.NewBudget(map[string]int{"tool_calls": 1})
		_ = budget.Consume("tool_calls", 1) // Exhaust budget

		middleware := mw.Budget(mw.BudgetConfig{
			Budget:     budget,
			BudgetName: "tool_calls",
			Amount:     1,
		})

		execCtx := &domainmw.ExecutionContext{
			Tool: &mockTool{name: "test"},
		}

		handler := middleware(createTestHandler(tool.Result{}, nil))

		_, err := handler(context.Background(), execCtx)
		if err == nil {
			t.Fatal("expected error for exceeded budget")
		}
		if !errors.Is(err, policy.ErrBudgetExceeded) {
			t.Errorf("expected ErrBudgetExceeded, got %v", err)
		}
	})

	t.Run("passes through when no budget configured", func(t *testing.T) {
		t.Parallel()

		middleware := mw.Budget(mw.BudgetConfig{
			Budget: nil,
		})

		execCtx := &domainmw.ExecutionContext{
			Tool: &mockTool{name: "test"},
		}

		expected := tool.Result{Output: json.RawMessage(`{"passed":"through"}`)}
		handler := middleware(createTestHandler(expected, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(expected.Output) {
			t.Errorf("got output %s, want %s", result.Output, expected.Output)
		}
	})

	t.Run("does not consume budget on error", func(t *testing.T) {
		t.Parallel()

		budget := policy.NewBudget(map[string]int{"tool_calls": 10})

		middleware := mw.Budget(mw.BudgetConfig{
			Budget:     budget,
			BudgetName: "tool_calls",
			Amount:     1,
		})

		execCtx := &domainmw.ExecutionContext{
			Tool: &mockTool{name: "test"},
		}

		handlerErr := errors.New("execution failed")
		handler := middleware(createTestHandler(tool.Result{}, handlerErr))

		_, err := handler(context.Background(), execCtx)
		if err == nil {
			t.Fatal("expected error from handler")
		}

		// Budget should not be consumed
		if budget.Remaining("tool_calls") != 10 {
			t.Errorf("budget should not be consumed on error, remaining: %d", budget.Remaining("tool_calls"))
		}
	})
}

func TestBudgetFromContext(t *testing.T) {
	t.Parallel()

	t.Run("uses budget from execution context", func(t *testing.T) {
		t.Parallel()

		budget := policy.NewBudget(map[string]int{"tool_calls": 10})

		middleware := mw.BudgetFromContext("tool_calls", 1)

		execCtx := &domainmw.ExecutionContext{
			Tool:   &mockTool{name: "test"},
			Budget: budget,
		}

		expected := tool.Result{Output: json.RawMessage(`{"ok":true}`)}
		handler := middleware(createTestHandler(expected, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(expected.Output) {
			t.Errorf("got output %s, want %s", result.Output, expected.Output)
		}
	})

	t.Run("blocks when context budget exceeded", func(t *testing.T) {
		t.Parallel()

		budget := policy.NewBudget(map[string]int{"tool_calls": 0})

		middleware := mw.BudgetFromContext("tool_calls", 1)

		execCtx := &domainmw.ExecutionContext{
			Tool:   &mockTool{name: "test"},
			Budget: budget,
		}

		handler := middleware(createTestHandler(tool.Result{}, nil))

		_, err := handler(context.Background(), execCtx)
		if err == nil {
			t.Fatal("expected error for exceeded budget")
		}
		if !errors.Is(err, policy.ErrBudgetExceeded) {
			t.Errorf("expected ErrBudgetExceeded, got %v", err)
		}
	})

	t.Run("passes through when no budget in context", func(t *testing.T) {
		t.Parallel()

		middleware := mw.BudgetFromContext("tool_calls", 1)

		execCtx := &domainmw.ExecutionContext{
			Tool:   &mockTool{name: "test"},
			Budget: nil,
		}

		expected := tool.Result{Output: json.RawMessage(`{"passed":"through"}`)}
		handler := middleware(createTestHandler(expected, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(expected.Output) {
			t.Errorf("got output %s, want %s", result.Output, expected.Output)
		}
	})
}

func TestLegacyCache(t *testing.T) {
	t.Parallel()

	t.Run("creates cache with max entries", func(t *testing.T) {
		t.Parallel()

		c := mw.NewLegacyCache(100)
		if c == nil {
			t.Fatal("NewLegacyCache() returned nil")
		}
	})

	t.Run("gets and sets values", func(t *testing.T) {
		t.Parallel()

		c := mw.NewLegacyCache(100)
		expected := tool.Result{Output: json.RawMessage(`{"value":"test"}`)}

		c.Set("key1", expected)

		result, ok := c.Get("key1")
		if !ok {
			t.Fatal("expected key to exist")
		}
		if string(result.Output) != string(expected.Output) {
			t.Errorf("got output %s, want %s", result.Output, expected.Output)
		}
	})

	t.Run("returns false for missing key", func(t *testing.T) {
		t.Parallel()

		c := mw.NewLegacyCache(100)

		_, ok := c.Get("nonexistent")
		if ok {
			t.Error("expected key to not exist")
		}
	})

	t.Run("respects max size", func(t *testing.T) {
		t.Parallel()

		c := mw.NewLegacyCache(2)

		c.Set("key1", tool.Result{})
		c.Set("key2", tool.Result{})
		c.Set("key3", tool.Result{}) // Should be ignored

		if c.Len() != 2 {
			t.Errorf("got len %d, want 2", c.Len())
		}
	})

	t.Run("clears all entries", func(t *testing.T) {
		t.Parallel()

		c := mw.NewLegacyCache(100)
		c.Set("key1", tool.Result{})
		c.Set("key2", tool.Result{})

		c.Clear()

		if c.Len() != 0 {
			t.Errorf("got len %d after clear, want 0", c.Len())
		}
	})
}

func TestCaching(t *testing.T) {
	t.Parallel()

	t.Run("caches cacheable tool results", func(t *testing.T) {
		t.Parallel()

		mockC := newMockCache()
		middleware := mw.Caching(mockC)

		mockT := &mockTool{
			name:        "cacheable_tool",
			annotations: tool.Annotations{Cacheable: true, ReadOnly: true}, // CanCache requires Cacheable AND (ReadOnly OR Idempotent)
		}
		execCtx := &domainmw.ExecutionContext{
			Tool:  mockT,
			Input: json.RawMessage(`{"query":"test"}`),
		}

		expected := tool.Result{Output: json.RawMessage(`{"result":"computed"}`)}
		callCount := 0
		handler := middleware(func(_ context.Context, _ *domainmw.ExecutionContext) (tool.Result, error) {
			callCount++
			return expected, nil
		})

		// First call - should compute
		result1, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result1.Cached {
			t.Error("first call should not be cached")
		}

		// Second call - should be cached
		result2, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !result2.Cached {
			t.Error("second call should be cached")
		}

		if callCount != 1 {
			t.Errorf("handler called %d times, expected 1", callCount)
		}
	})

	t.Run("skips caching for non-cacheable tools", func(t *testing.T) {
		t.Parallel()

		mockC := newMockCache()
		middleware := mw.Caching(mockC)

		mockT := &mockTool{
			name:        "non_cacheable_tool",
			annotations: tool.Annotations{Cacheable: false},
		}
		execCtx := &domainmw.ExecutionContext{
			Tool:  mockT,
			Input: json.RawMessage(`{"query":"test"}`),
		}

		callCount := 0
		handler := middleware(func(_ context.Context, _ *domainmw.ExecutionContext) (tool.Result, error) {
			callCount++
			return tool.Result{Output: json.RawMessage(`{"n":` + string(rune('0'+callCount)) + `}`)}, nil
		})

		// Both calls should execute
		handler(context.Background(), execCtx)
		handler(context.Background(), execCtx)

		if callCount != 2 {
			t.Errorf("handler called %d times, expected 2", callCount)
		}
	})

	t.Run("passes through when cache is nil", func(t *testing.T) {
		t.Parallel()

		middleware := mw.Caching(nil)

		mockT := &mockTool{
			name:        "tool",
			annotations: tool.Annotations{Cacheable: true, ReadOnly: true},
		}
		execCtx := &domainmw.ExecutionContext{
			Tool: mockT,
		}

		expected := tool.Result{Output: json.RawMessage(`{"result":"ok"}`)}
		handler := middleware(createTestHandler(expected, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(expected.Output) {
			t.Errorf("got output %s, want %s", result.Output, expected.Output)
		}
	})

	t.Run("uses TTL option", func(t *testing.T) {
		t.Parallel()

		mockC := newMockCache()
		middleware := mw.Caching(mockC, mw.WithCacheTTL(5*time.Minute))

		if middleware == nil {
			t.Fatal("Caching with TTL returned nil")
		}
	})
}

func TestLegacyCaching(t *testing.T) {
	t.Parallel()

	t.Run("caches with legacy cache", func(t *testing.T) {
		t.Parallel()

		legacyC := mw.NewLegacyCache(100)
		middleware := mw.LegacyCaching(legacyC)

		mockT := &mockTool{
			name:        "cacheable_tool",
			annotations: tool.Annotations{Cacheable: true, ReadOnly: true}, // CanCache requires Cacheable AND (ReadOnly OR Idempotent)
		}
		execCtx := &domainmw.ExecutionContext{
			Tool:  mockT,
			Input: json.RawMessage(`{"query":"test"}`),
		}

		callCount := 0
		handler := middleware(func(_ context.Context, _ *domainmw.ExecutionContext) (tool.Result, error) {
			callCount++
			return tool.Result{Output: json.RawMessage(`{"computed":true}`)}, nil
		})

		// First call
		result1, _ := handler(context.Background(), execCtx)
		if result1.Cached {
			t.Error("first call should not be cached")
		}

		// Second call
		result2, _ := handler(context.Background(), execCtx)
		if !result2.Cached {
			t.Error("second call should be cached")
		}

		if callCount != 1 {
			t.Errorf("handler called %d times, expected 1", callCount)
		}
	})

	t.Run("passes through when cache is nil", func(t *testing.T) {
		t.Parallel()

		middleware := mw.LegacyCaching(nil)

		mockT := &mockTool{
			name:        "tool",
			annotations: tool.Annotations{Cacheable: true, ReadOnly: true},
		}
		execCtx := &domainmw.ExecutionContext{
			Tool: mockT,
		}

		expected := tool.Result{Output: json.RawMessage(`{"ok":true}`)}
		handler := middleware(createTestHandler(expected, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(expected.Output) {
			t.Errorf("got output %s, want %s", result.Output, expected.Output)
		}
	})
}

func TestLedgerRecording(t *testing.T) {
	t.Parallel()

	t.Run("records tool call and result", func(t *testing.T) {
		t.Parallel()

		l := ledger.New("test-run-id")
		middleware := mw.LedgerRecording(mw.LedgerConfig{
			Ledger: l,
		})

		mockT := &mockTool{name: "read_file"}
		execCtx := &domainmw.ExecutionContext{
			CurrentState: agent.StateExplore,
			Tool:         mockT,
			Input:        json.RawMessage(`{"path":"/test"}`),
		}

		expected := tool.Result{Output: json.RawMessage(`{"content":"hello"}`)}
		handler := middleware(createTestHandler(expected, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(expected.Output) {
			t.Errorf("got output %s, want %s", result.Output, expected.Output)
		}

		// Verify ledger recorded entries
		entries := l.Entries()
		if len(entries) == 0 {
			t.Error("expected ledger entries")
		}
	})

	t.Run("records tool error", func(t *testing.T) {
		t.Parallel()

		l := ledger.New("test-run-id")
		middleware := mw.LedgerRecording(mw.LedgerConfig{
			Ledger: l,
		})

		mockT := &mockTool{name: "failing_tool"}
		execCtx := &domainmw.ExecutionContext{
			CurrentState: agent.StateExplore,
			Tool:         mockT,
			Input:        json.RawMessage(`{}`),
		}

		handlerErr := errors.New("tool failed")
		handler := middleware(createTestHandler(tool.Result{}, handlerErr))

		_, err := handler(context.Background(), execCtx)
		if err == nil {
			t.Fatal("expected error from handler")
		}

		// Verify ledger recorded error
		entries := l.Entries()
		if len(entries) == 0 {
			t.Error("expected ledger entries for error")
		}
	})

	t.Run("passes through when no ledger configured", func(t *testing.T) {
		t.Parallel()

		middleware := mw.LedgerRecording(mw.LedgerConfig{
			Ledger: nil,
		})

		mockT := &mockTool{name: "tool"}
		execCtx := &domainmw.ExecutionContext{
			Tool: mockT,
		}

		expected := tool.Result{Output: json.RawMessage(`{"ok":true}`)}
		handler := middleware(createTestHandler(expected, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(expected.Output) {
			t.Errorf("got output %s, want %s", result.Output, expected.Output)
		}
	})
}

func TestLogging(t *testing.T) {
	t.Parallel()

	t.Run("logs tool execution without panic", func(t *testing.T) {
		t.Parallel()

		middleware := mw.Logging(mw.LoggingConfig{
			LogInput:  true,
			LogOutput: true,
		})

		mockT := &mockTool{name: "test_tool"}
		execCtx := &domainmw.ExecutionContext{
			RunID:        "run-123",
			CurrentState: agent.StateExplore,
			Tool:         mockT,
			Input:        json.RawMessage(`{"key":"value"}`),
		}

		expected := tool.Result{Output: json.RawMessage(`{"result":"success"}`)}
		handler := middleware(createTestHandler(expected, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(expected.Output) {
			t.Errorf("got output %s, want %s", result.Output, expected.Output)
		}
	})

	t.Run("logs errors without panic", func(t *testing.T) {
		t.Parallel()

		middleware := mw.Logging(mw.LoggingConfig{
			LogInput:  false,
			LogOutput: false,
		})

		mockT := &mockTool{name: "failing_tool"}
		execCtx := &domainmw.ExecutionContext{
			RunID:        "run-456",
			CurrentState: agent.StateAct,
			Tool:         mockT,
		}

		handlerErr := errors.New("execution failed")
		handler := middleware(createTestHandler(tool.Result{}, handlerErr))

		_, err := handler(context.Background(), execCtx)
		if err == nil {
			t.Fatal("expected error from handler")
		}
	})
}

func TestSimulation(t *testing.T) {
	t.Parallel()

	t.Run("passes through when simulation disabled", func(t *testing.T) {
		t.Parallel()

		cfg := simulation.Config{Enabled: false}
		middleware := mw.Simulation(cfg)

		mockT := &mockTool{
			name:        "destructive_tool",
			annotations: tool.Annotations{Destructive: true},
		}
		execCtx := &domainmw.ExecutionContext{
			Tool: mockT,
		}

		expected := tool.Result{Output: json.RawMessage(`{"executed":true}`)}
		handler := middleware(createTestHandler(expected, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(expected.Output) {
			t.Errorf("got output %s, want %s", result.Output, expected.Output)
		}
	})

	t.Run("allows read-only tools when configured", func(t *testing.T) {
		t.Parallel()

		cfg := simulation.Config{
			Enabled:       true,
			AllowReadOnly: true,
		}
		middleware := mw.Simulation(cfg)

		mockT := &mockTool{
			name:        "read_tool",
			annotations: tool.Annotations{ReadOnly: true},
		}
		execCtx := &domainmw.ExecutionContext{
			Tool: mockT,
		}

		expected := tool.Result{Output: json.RawMessage(`{"read":"data"}`)}
		handler := middleware(createTestHandler(expected, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(expected.Output) {
			t.Errorf("got output %s, want %s", result.Output, expected.Output)
		}
	})

	t.Run("blocks destructive tools and returns simulation result", func(t *testing.T) {
		t.Parallel()

		cfg := simulation.Config{
			Enabled:       true,
			AllowReadOnly: true,
		}
		middleware := mw.Simulation(cfg)

		mockT := &mockTool{
			name:        "delete_tool",
			annotations: tool.Annotations{Destructive: true},
		}
		execCtx := &domainmw.ExecutionContext{
			Tool: mockT,
		}

		handler := middleware(createTestHandler(tool.Result{Output: json.RawMessage(`{"should":"not execute"}`)}, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should return simulation result, not the handler result
		if result.Output == nil {
			t.Error("expected simulation result")
		}
	})

	t.Run("returns mock result when configured", func(t *testing.T) {
		t.Parallel()

		mockResult := tool.Result{Output: json.RawMessage(`{"mocked":true}`)}
		cfg := simulation.Config{
			Enabled:     true,
			MockResults: map[string]tool.Result{"test_tool": mockResult},
		}
		middleware := mw.Simulation(cfg)

		mockT := &mockTool{
			name:        "test_tool",
			annotations: tool.Annotations{},
		}
		execCtx := &domainmw.ExecutionContext{
			Tool: mockT,
		}

		handler := middleware(createTestHandler(tool.Result{}, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(mockResult.Output) {
			t.Errorf("got output %s, want %s", result.Output, mockResult.Output)
		}
	})

	t.Run("returns default result when configured", func(t *testing.T) {
		t.Parallel()

		defaultResult := tool.Result{Output: json.RawMessage(`{"default":true}`)}
		cfg := simulation.Config{
			Enabled:       true,
			DefaultResult: &defaultResult,
		}
		middleware := mw.Simulation(cfg)

		mockT := &mockTool{
			name:        "unmocked_tool",
			annotations: tool.Annotations{},
		}
		execCtx := &domainmw.ExecutionContext{
			Tool: mockT,
		}

		handler := middleware(createTestHandler(tool.Result{}, nil))

		result, err := handler(context.Background(), execCtx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if string(result.Output) != string(defaultResult.Output) {
			t.Errorf("got output %s, want %s", result.Output, defaultResult.Output)
		}
	})

	t.Run("records intents when configured", func(t *testing.T) {
		t.Parallel()

		recorder := simulation.NewMemoryRecorder()
		cfg := simulation.Config{
			Enabled:       true,
			RecordIntents: true,
			Recorder:      recorder,
			AllowReadOnly: true,
		}
		middleware := mw.Simulation(cfg)

		mockT := &mockTool{
			name:        "tracked_tool",
			annotations: tool.Annotations{Destructive: true},
		}
		execCtx := &domainmw.ExecutionContext{
			CurrentState: agent.StateAct,
			Tool:         mockT,
			Input:        json.RawMessage(`{"action":"test"}`),
		}

		handler := middleware(createTestHandler(tool.Result{}, nil))
		handler(context.Background(), execCtx)

		intents := recorder.Intents()
		if len(intents) == 0 {
			t.Error("expected recorded intents")
		}
		if intents[0].ToolName != "tracked_tool" {
			t.Errorf("got tool name %s, want tracked_tool", intents[0].ToolName)
		}
		if !intents[0].Blocked {
			t.Error("expected intent to be blocked")
		}
	})
}

func TestNewSimulationConfig(t *testing.T) {
	t.Parallel()

	t.Run("creates default config", func(t *testing.T) {
		t.Parallel()

		cfg := mw.NewSimulationConfig()

		if !cfg.Enabled {
			t.Error("expected simulation to be enabled by default")
		}
		if !cfg.AllowReadOnly {
			t.Error("expected AllowReadOnly to be true by default")
		}
	})

	t.Run("applies options", func(t *testing.T) {
		t.Parallel()

		recorder := simulation.NewMemoryRecorder()
		mockResult := tool.Result{Output: json.RawMessage(`{"mock":true}`)}

		cfg := mw.NewSimulationConfig(
			simulation.WithRecorder(recorder),
			simulation.WithMockResult("test", mockResult),
			simulation.WithAllowIdempotent(true),
		)

		if cfg.Recorder != recorder {
			t.Error("recorder not set")
		}
		if _, ok := cfg.MockResults["test"]; !ok {
			t.Error("mock result not set")
		}
		if !cfg.AllowIdempotent {
			t.Error("AllowIdempotent not set")
		}
	})
}

func TestNewMemoryIntentRecorder(t *testing.T) {
	t.Parallel()

	t.Run("creates memory recorder", func(t *testing.T) {
		t.Parallel()

		recorder := mw.NewMemoryIntentRecorder()
		if recorder == nil {
			t.Fatal("NewMemoryIntentRecorder() returned nil")
		}
	})
}
