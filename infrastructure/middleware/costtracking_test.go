package middleware_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	domainmw "go.klarlabs.de/agent/domain/middleware"
	"go.klarlabs.de/agent/domain/tool"
	mw "go.klarlabs.de/agent/infrastructure/middleware"
)

var errTestCost = errors.New("test error")

func TestCostTracking_RecordsBasicCosts(t *testing.T) {
	t.Parallel()

	var recorded []mw.CostEntry
	var mu sync.Mutex

	cfg := mw.CostTrackingConfig{
		OnCostRecorded: func(entry mw.CostEntry) {
			mu.Lock()
			recorded = append(recorded, entry)
			mu.Unlock()
		},
		IncludeMetadata: true,
	}

	middleware := mw.CostTracking(cfg)

	mockT := &mockTool{name: "test_tool"}
	execCtx := &domainmw.ExecutionContext{
		RunID:        "run-123",
		CurrentState: agent.StateAct,
		Tool:         mockT,
		Input:        json.RawMessage(`{"query": "test"}`),
	}

	output := json.RawMessage(`{"result": "success"}`)
	handler := middleware(createTestHandler(tool.Result{Output: output}, nil))

	_, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if len(recorded) < 2 {
		t.Fatalf("expected at least 2 cost entries, got %d", len(recorded))
	}

	// Verify request entry
	hasRequest := false
	hasDuration := false
	hasBytes := false

	for _, entry := range recorded {
		if entry.Unit == mw.CostUnitRequests {
			hasRequest = true
			if entry.Amount != 1 {
				t.Errorf("expected request amount 1, got %f", entry.Amount)
			}
		}
		if entry.Unit == mw.CostUnitDuration {
			hasDuration = true
			if entry.Amount < 0 {
				t.Errorf("expected positive duration, got %f", entry.Amount)
			}
		}
		if entry.Unit == mw.CostUnitBytes {
			hasBytes = true
		}
	}

	if !hasRequest {
		t.Error("expected request cost entry")
	}
	if !hasDuration {
		t.Error("expected duration cost entry")
	}
	if !hasBytes {
		t.Error("expected bytes cost entry")
	}
}

func TestCostTracking_WithStore(t *testing.T) {
	t.Parallel()

	store := mw.NewMemoryCostStore()

	cfg := mw.CostTrackingConfig{
		Store:           store,
		IncludeMetadata: true,
	}

	middleware := mw.CostTracking(cfg)

	mockT := &mockTool{name: "test_tool"}
	execCtx := &domainmw.ExecutionContext{
		RunID:        "run-456",
		CurrentState: agent.StateAct,
		Tool:         mockT,
		Input:        json.RawMessage(`{}`),
	}

	handler := middleware(createTestHandler(tool.Result{Output: json.RawMessage(`{}`)}, nil))

	_, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify entries were stored
	entries, err := store.ListEntries(context.Background(), "run-456", 10)
	if err != nil {
		t.Fatalf("ListEntries failed: %v", err)
	}

	if len(entries) == 0 {
		t.Error("expected entries in store")
	}

	// Verify summary
	summary, err := store.GetSummary(context.Background(), "run-456")
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}

	if summary.RunID != "run-456" {
		t.Errorf("expected run ID run-456, got %s", summary.RunID)
	}

	if summary.TotalByUnit[mw.CostUnitRequests] < 1 {
		t.Error("expected at least 1 request in summary")
	}
}

func TestCostTracking_FilterByEnabledUnits(t *testing.T) {
	t.Parallel()

	var recorded []mw.CostEntry
	var mu sync.Mutex

	cfg := mw.CostTrackingConfig{
		EnabledUnits: []mw.CostUnit{mw.CostUnitRequests}, // Only track requests
		OnCostRecorded: func(entry mw.CostEntry) {
			mu.Lock()
			recorded = append(recorded, entry)
			mu.Unlock()
		},
	}

	middleware := mw.CostTracking(cfg)

	mockT := &mockTool{name: "test_tool"}
	execCtx := &domainmw.ExecutionContext{
		RunID:        "run-123",
		CurrentState: agent.StateAct,
		Tool:         mockT,
		Input:        json.RawMessage(`{}`),
	}

	handler := middleware(createTestHandler(tool.Result{Output: json.RawMessage(`{}`)}, nil))

	_, err := handler(context.Background(), execCtx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	// Should only have request entries
	for _, entry := range recorded {
		if entry.Unit != mw.CostUnitRequests {
			t.Errorf("unexpected unit %s, expected only requests", entry.Unit)
		}
	}
}

func TestCostTracking_SkipsErrorsWhenNotEnabled(t *testing.T) {
	t.Parallel()

	var recorded []mw.CostEntry
	var mu sync.Mutex

	cfg := mw.CostTrackingConfig{
		TrackErrors: false,
		OnCostRecorded: func(entry mw.CostEntry) {
			mu.Lock()
			recorded = append(recorded, entry)
			mu.Unlock()
		},
	}

	middleware := mw.CostTracking(cfg)

	mockT := &mockTool{name: "test_tool"}
	execCtx := &domainmw.ExecutionContext{
		RunID:        "run-123",
		CurrentState: agent.StateAct,
		Tool:         mockT,
	}

	handler := middleware(createTestHandler(tool.Result{}, errTestCost))

	_, _ = handler(context.Background(), execCtx)

	mu.Lock()
	defer mu.Unlock()

	if len(recorded) != 0 {
		t.Errorf("expected no recorded entries on error, got %d", len(recorded))
	}
}

func TestCostTracking_TracksErrorsWhenEnabled(t *testing.T) {
	t.Parallel()

	var recorded []mw.CostEntry
	var mu sync.Mutex

	cfg := mw.CostTrackingConfig{
		TrackErrors: true,
		OnCostRecorded: func(entry mw.CostEntry) {
			mu.Lock()
			recorded = append(recorded, entry)
			mu.Unlock()
		},
		IncludeMetadata: true,
	}

	middleware := mw.CostTracking(cfg)

	mockT := &mockTool{name: "test_tool"}
	execCtx := &domainmw.ExecutionContext{
		RunID:        "run-123",
		CurrentState: agent.StateAct,
		Tool:         mockT,
	}

	handler := middleware(createTestHandler(tool.Result{}, errTestCost))

	_, _ = handler(context.Background(), execCtx)

	mu.Lock()
	defer mu.Unlock()

	if len(recorded) == 0 {
		t.Fatal("expected recorded entries on error when enabled")
	}

	// Verify error metadata
	hasErrorMetadata := false
	for _, entry := range recorded {
		if entry.Metadata != nil && entry.Metadata["error"] == "true" {
			hasErrorMetadata = true
			break
		}
	}

	if !hasErrorMetadata {
		t.Error("expected error metadata in entry")
	}
}

func TestMemoryCostStore_GetSummary(t *testing.T) {
	t.Parallel()

	store := mw.NewMemoryCostStore()
	ctx := context.Background()

	// Record multiple entries
	now := time.Now()
	entries := []mw.CostEntry{
		{RunID: "run-1", ToolName: "tool_a", Unit: mw.CostUnitRequests, Amount: 1, Timestamp: now},
		{RunID: "run-1", ToolName: "tool_a", Unit: mw.CostUnitDuration, Amount: 100, Timestamp: now},
		{RunID: "run-1", ToolName: "tool_b", Unit: mw.CostUnitRequests, Amount: 1, Timestamp: now.Add(time.Second)},
		{RunID: "run-1", ToolName: "tool_b", Unit: mw.CostUnitTokens, Amount: 500, Timestamp: now.Add(time.Second)},
	}

	for _, entry := range entries {
		if err := store.Record(ctx, entry); err != nil {
			t.Fatalf("Record failed: %v", err)
		}
	}

	summary, err := store.GetSummary(ctx, "run-1")
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}

	// Verify totals
	if summary.TotalByUnit[mw.CostUnitRequests] != 2 {
		t.Errorf("expected 2 requests, got %f", summary.TotalByUnit[mw.CostUnitRequests])
	}

	if summary.TotalByUnit[mw.CostUnitDuration] != 100 {
		t.Errorf("expected 100ms duration, got %f", summary.TotalByUnit[mw.CostUnitDuration])
	}

	if summary.TotalByUnit[mw.CostUnitTokens] != 500 {
		t.Errorf("expected 500 tokens, got %f", summary.TotalByUnit[mw.CostUnitTokens])
	}

	// Verify by tool
	if summary.ByTool["tool_a"].Invocations != 1 {
		t.Errorf("expected 1 invocation for tool_a, got %d", summary.ByTool["tool_a"].Invocations)
	}

	if summary.ByTool["tool_b"].Invocations != 1 {
		t.Errorf("expected 1 invocation for tool_b, got %d", summary.ByTool["tool_b"].Invocations)
	}

	if summary.EntryCount != 4 {
		t.Errorf("expected 4 entries, got %d", summary.EntryCount)
	}
}

func TestMemoryCostStore_ListEntries_WithLimit(t *testing.T) {
	t.Parallel()

	store := mw.NewMemoryCostStore()
	ctx := context.Background()

	// Record 5 entries
	for i := 0; i < 5; i++ {
		err := store.Record(ctx, mw.CostEntry{
			RunID:     "run-1",
			ToolName:  "tool",
			Unit:      mw.CostUnitRequests,
			Amount:    1,
			Timestamp: time.Now(),
		})
		if err != nil {
			t.Fatalf("Record failed: %v", err)
		}
	}

	// List with limit
	entries, err := store.ListEntries(ctx, "run-1", 3)
	if err != nil {
		t.Fatalf("ListEntries failed: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("expected 3 entries, got %d", len(entries))
	}

	// List without limit
	entries, err = store.ListEntries(ctx, "run-1", 0)
	if err != nil {
		t.Fatalf("ListEntries failed: %v", err)
	}

	if len(entries) != 5 {
		t.Errorf("expected 5 entries, got %d", len(entries))
	}
}

func TestMemoryCostStore_Clear(t *testing.T) {
	t.Parallel()

	store := mw.NewMemoryCostStore()
	ctx := context.Background()

	err := store.Record(ctx, mw.CostEntry{
		RunID:     "run-1",
		ToolName:  "tool",
		Unit:      mw.CostUnitRequests,
		Amount:    1,
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("Record failed: %v", err)
	}

	store.Clear("run-1")

	entries, err := store.ListEntries(ctx, "run-1", 0)
	if err != nil {
		t.Fatalf("ListEntries failed: %v", err)
	}

	if len(entries) != 0 {
		t.Errorf("expected 0 entries after clear, got %d", len(entries))
	}
}

func TestMemoryCostStore_ClearAll(t *testing.T) {
	t.Parallel()

	store := mw.NewMemoryCostStore()
	ctx := context.Background()

	// Record entries for multiple runs
	for _, runID := range []string{"run-1", "run-2", "run-3"} {
		err := store.Record(ctx, mw.CostEntry{
			RunID:     runID,
			ToolName:  "tool",
			Unit:      mw.CostUnitRequests,
			Amount:    1,
			Timestamp: time.Now(),
		})
		if err != nil {
			t.Fatalf("Record failed: %v", err)
		}
	}

	store.ClearAll()

	for _, runID := range []string{"run-1", "run-2", "run-3"} {
		entries, err := store.ListEntries(ctx, runID, 0)
		if err != nil {
			t.Fatalf("ListEntries failed: %v", err)
		}
		if len(entries) != 0 {
			t.Errorf("expected 0 entries for %s after clear all, got %d", runID, len(entries))
		}
	}
}

func TestMemoryCostStore_EmptySummary(t *testing.T) {
	t.Parallel()

	store := mw.NewMemoryCostStore()
	ctx := context.Background()

	summary, err := store.GetSummary(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("GetSummary failed: %v", err)
	}

	if summary.RunID != "nonexistent" {
		t.Errorf("expected run ID nonexistent, got %s", summary.RunID)
	}

	if summary.EntryCount != 0 {
		t.Errorf("expected 0 entries, got %d", summary.EntryCount)
	}
}

func TestLLMCostCalculator(t *testing.T) {
	t.Parallel()

	inputTokenCost := 0.01  // $0.01 per input token
	outputTokenCost := 0.03 // $0.03 per output token

	calc := mw.LLMCostCalculator(inputTokenCost, outputTokenCost)

	mockT := &mockTool{name: "llm_tool"}
	execCtx := &domainmw.ExecutionContext{
		RunID:        "run-123",
		CurrentState: agent.StateAct,
		Tool:         mockT,
	}

	// Result with token info
	result := tool.Result{
		Output: json.RawMessage(`{"input_tokens": 100, "output_tokens": 50}`),
	}

	entries := calc(context.Background(), execCtx, result, 500*time.Millisecond)

	// Verify token entry
	var tokenEntry *mw.CostEntry
	var dollarEntry *mw.CostEntry
	var requestEntry *mw.CostEntry
	var durationEntry *mw.CostEntry

	for i := range entries {
		switch entries[i].Unit {
		case mw.CostUnitTokens:
			tokenEntry = &entries[i]
		case mw.CostUnitDollars:
			dollarEntry = &entries[i]
		case mw.CostUnitRequests:
			requestEntry = &entries[i]
		case mw.CostUnitDuration:
			durationEntry = &entries[i]
		}
	}

	if tokenEntry == nil {
		t.Fatal("expected token entry")
	}
	if tokenEntry.Amount != 150 {
		t.Errorf("expected 150 tokens, got %f", tokenEntry.Amount)
	}

	if dollarEntry == nil {
		t.Fatal("expected dollar entry")
	}
	expectedCost := 100*0.01 + 50*0.03 // $2.50
	if dollarEntry.Amount != expectedCost {
		t.Errorf("expected $%f, got $%f", expectedCost, dollarEntry.Amount)
	}

	if requestEntry == nil {
		t.Fatal("expected request entry")
	}
	if requestEntry.Amount != 1 {
		t.Errorf("expected 1 request, got %f", requestEntry.Amount)
	}

	if durationEntry == nil {
		t.Fatal("expected duration entry")
	}
	if durationEntry.Amount != 500 {
		t.Errorf("expected 500ms duration, got %f", durationEntry.Amount)
	}
}

func TestLLMCostCalculator_NoTokenInfo(t *testing.T) {
	t.Parallel()

	calc := mw.LLMCostCalculator(0.01, 0.03)

	mockT := &mockTool{name: "llm_tool"}
	execCtx := &domainmw.ExecutionContext{
		RunID:        "run-123",
		CurrentState: agent.StateAct,
		Tool:         mockT,
	}

	// Result without token info
	result := tool.Result{
		Output: json.RawMessage(`{"response": "hello"}`),
	}

	entries := calc(context.Background(), execCtx, result, 100*time.Millisecond)

	// Should still have request and duration
	hasRequest := false
	hasDuration := false
	hasTokens := false

	for _, entry := range entries {
		switch entry.Unit {
		case mw.CostUnitRequests:
			hasRequest = true
		case mw.CostUnitDuration:
			hasDuration = true
		case mw.CostUnitTokens:
			hasTokens = true
		}
	}

	if !hasRequest {
		t.Error("expected request entry")
	}
	if !hasDuration {
		t.Error("expected duration entry")
	}
	if hasTokens {
		t.Error("should not have token entry without token info")
	}
}

func TestAPICallCostCalculator(t *testing.T) {
	t.Parallel()

	costPerCall := 0.001 // $0.001 per call

	calc := mw.APICallCostCalculator(costPerCall)

	mockT := &mockTool{name: "api_tool"}
	execCtx := &domainmw.ExecutionContext{
		RunID:        "run-123",
		CurrentState: agent.StateAct,
		Tool:         mockT,
	}

	result := tool.Result{Output: json.RawMessage(`{}`)}

	entries := calc(context.Background(), execCtx, result, 200*time.Millisecond)

	// Verify entries
	var requestEntry, creditsEntry, durationEntry *mw.CostEntry

	for i := range entries {
		switch entries[i].Unit {
		case mw.CostUnitRequests:
			requestEntry = &entries[i]
		case mw.CostUnitCredits:
			creditsEntry = &entries[i]
		case mw.CostUnitDuration:
			durationEntry = &entries[i]
		}
	}

	if requestEntry == nil || requestEntry.Amount != 1 {
		t.Error("expected 1 request")
	}

	if creditsEntry == nil || creditsEntry.Amount != costPerCall {
		t.Errorf("expected %f credits, got %v", costPerCall, creditsEntry)
	}

	if durationEntry == nil || durationEntry.Amount != 200 {
		t.Errorf("expected 200ms duration")
	}
}

func TestNewCostTracking_WithOptions(t *testing.T) {
	t.Parallel()

	store := mw.NewMemoryCostStore()
	callbackCalled := false

	middleware := mw.NewCostTracking(
		mw.WithCostStore(store),
		mw.WithCostCallback(func(entry mw.CostEntry) {
			callbackCalled = true
		}),
		mw.WithEnabledCostUnits(mw.CostUnitRequests, mw.CostUnitDuration),
		mw.WithCostMetadata(true),
		mw.WithErrorTracking(true),
	)

	if middleware == nil {
		t.Fatal("NewCostTracking returned nil")
	}

	mockT := &mockTool{name: "test_tool"}
	execCtx := &domainmw.ExecutionContext{
		RunID:        "run-123",
		CurrentState: agent.StateAct,
		Tool:         mockT,
	}

	handler := middleware(createTestHandler(tool.Result{Output: json.RawMessage(`{}`)}, nil))
	_, _ = handler(context.Background(), execCtx)

	if !callbackCalled {
		t.Error("expected callback to be called")
	}
}

func TestDefaultCostTrackingConfig(t *testing.T) {
	t.Parallel()

	cfg := mw.DefaultCostTrackingConfig()

	if cfg.Calculator == nil {
		t.Error("expected default calculator")
	}

	if !cfg.IncludeMetadata {
		t.Error("expected IncludeMetadata to be true by default")
	}

	if cfg.TrackErrors {
		t.Error("expected TrackErrors to be false by default")
	}
}

func TestCostTracking_IncludesStateMetadata(t *testing.T) {
	t.Parallel()

	var recorded []mw.CostEntry
	var mu sync.Mutex

	cfg := mw.CostTrackingConfig{
		IncludeMetadata: true,
		OnCostRecorded: func(entry mw.CostEntry) {
			mu.Lock()
			recorded = append(recorded, entry)
			mu.Unlock()
		},
	}

	middleware := mw.CostTracking(cfg)

	mockT := &mockTool{name: "test_tool"}
	execCtx := &domainmw.ExecutionContext{
		RunID:        "run-123",
		CurrentState: agent.StateExplore,
		Tool:         mockT,
	}

	handler := middleware(createTestHandler(tool.Result{Output: json.RawMessage(`{}`)}, nil))
	_, _ = handler(context.Background(), execCtx)

	mu.Lock()
	defer mu.Unlock()

	if len(recorded) == 0 {
		t.Fatal("expected recorded entries")
	}

	// Check state metadata
	hasStateMetadata := false
	for _, entry := range recorded {
		if entry.Metadata != nil && entry.Metadata["state"] == agent.StateExplore.String() {
			hasStateMetadata = true
			break
		}
	}

	if !hasStateMetadata {
		t.Error("expected state metadata in entry")
	}
}
