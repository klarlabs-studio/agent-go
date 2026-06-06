package dashboard

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/event"
	"go.klarlabs.de/agent/domain/run"
)

// mockRunStore implements run.Store for testing.
type mockRunStore struct {
	runs map[string]*agent.Run
}

func newMockRunStore() *mockRunStore {
	return &mockRunStore{
		runs: make(map[string]*agent.Run),
	}
}

func (m *mockRunStore) Save(_ context.Context, r *agent.Run) error {
	m.runs[r.ID] = r
	return nil
}

func (m *mockRunStore) Get(_ context.Context, id string) (*agent.Run, error) {
	r, ok := m.runs[id]
	if !ok {
		return nil, run.ErrRunNotFound
	}
	return r, nil
}

func (m *mockRunStore) Update(_ context.Context, r *agent.Run) error {
	m.runs[r.ID] = r
	return nil
}

func (m *mockRunStore) Delete(_ context.Context, id string) error {
	delete(m.runs, id)
	return nil
}

func (m *mockRunStore) List(_ context.Context, filter run.ListFilter) ([]*agent.Run, error) {
	var result []*agent.Run
	for _, r := range m.runs {
		// Apply status filter if specified
		if len(filter.Status) > 0 {
			matchStatus := false
			for _, s := range filter.Status {
				if r.Status == s {
					matchStatus = true
					break
				}
			}
			if !matchStatus {
				continue
			}
		}
		result = append(result, r)
	}

	// Apply limit
	if filter.Limit > 0 && len(result) > filter.Limit {
		result = result[:filter.Limit]
	}

	return result, nil
}

func (m *mockRunStore) Count(_ context.Context, filter run.ListFilter) (int64, error) {
	runs, _ := m.List(context.Background(), filter)
	return int64(len(runs)), nil
}

// mockEventStore implements event.Store for testing.
type mockEventStore struct {
	events map[string][]event.Event
	subs   map[string][]chan event.Event
}

func newMockEventStore() *mockEventStore {
	return &mockEventStore{
		events: make(map[string][]event.Event),
		subs:   make(map[string][]chan event.Event),
	}
}

func (m *mockEventStore) Append(_ context.Context, events ...event.Event) error {
	for _, evt := range events {
		m.events[evt.RunID] = append(m.events[evt.RunID], evt)
		// Notify subscribers
		for _, ch := range m.subs[evt.RunID] {
			select {
			case ch <- evt:
			default:
			}
		}
	}
	return nil
}

func (m *mockEventStore) LoadEvents(_ context.Context, runID string) ([]event.Event, error) {
	return m.events[runID], nil
}

func (m *mockEventStore) LoadEventsFrom(_ context.Context, runID string, fromSeq uint64) ([]event.Event, error) {
	events := m.events[runID]
	var result []event.Event
	for _, evt := range events {
		if evt.Sequence >= fromSeq {
			result = append(result, evt)
		}
	}
	return result, nil
}

func (m *mockEventStore) Subscribe(_ context.Context, runID string) (<-chan event.Event, error) {
	ch := make(chan event.Event, 10)
	m.subs[runID] = append(m.subs[runID], ch)
	return ch, nil
}

func TestHandleHealth(t *testing.T) {
	srv := New(Config{})

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()

	srv.handleHealth(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var health HealthStatus
	if err := json.NewDecoder(w.Body).Decode(&health); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if health.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got %s", health.Status)
	}
}

func TestHandleListRuns_NoStore(t *testing.T) {
	srv := New(Config{})

	req := httptest.NewRequest(http.MethodGet, "/api/runs", nil)
	w := httptest.NewRecorder()

	srv.handleListRuns(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var runs []RunSummary
	if err := json.NewDecoder(w.Body).Decode(&runs); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(runs) != 0 {
		t.Errorf("Expected empty list, got %d runs", len(runs))
	}
}

func TestHandleListRuns_WithStore(t *testing.T) {
	store := newMockRunStore()

	// Add test runs
	run1 := agent.NewRun("run-1", "Test goal 1")
	run1.Status = agent.RunStatusRunning
	run1.CurrentState = agent.StateExplore

	run2 := agent.NewRun("run-2", "Test goal 2")
	run2.Status = agent.RunStatusCompleted
	run2.CurrentState = agent.StateDone
	run2.EndTime = time.Now()

	store.Save(context.Background(), run1)
	store.Save(context.Background(), run2)

	srv := New(Config{RunStore: store})

	req := httptest.NewRequest(http.MethodGet, "/api/runs", nil)
	w := httptest.NewRecorder()

	srv.handleListRuns(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var runs []RunSummary
	if err := json.NewDecoder(w.Body).Decode(&runs); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(runs) != 2 {
		t.Errorf("Expected 2 runs, got %d", len(runs))
	}
}

func TestHandleListRuns_WithStatusFilter(t *testing.T) {
	store := newMockRunStore()

	// Add test runs
	run1 := agent.NewRun("run-1", "Test goal 1")
	run1.Status = agent.RunStatusRunning

	run2 := agent.NewRun("run-2", "Test goal 2")
	run2.Status = agent.RunStatusCompleted

	store.Save(context.Background(), run1)
	store.Save(context.Background(), run2)

	srv := New(Config{RunStore: store})

	req := httptest.NewRequest(http.MethodGet, "/api/runs?status=running", nil)
	w := httptest.NewRecorder()

	srv.handleListRuns(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var runs []RunSummary
	if err := json.NewDecoder(w.Body).Decode(&runs); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(runs) != 1 {
		t.Errorf("Expected 1 run, got %d", len(runs))
	}

	if runs[0].Status != string(agent.RunStatusRunning) {
		t.Errorf("Expected status 'running', got %s", runs[0].Status)
	}
}

func TestHandleListRuns_MethodNotAllowed(t *testing.T) {
	srv := New(Config{})

	req := httptest.NewRequest(http.MethodPost, "/api/runs", nil)
	w := httptest.NewRecorder()

	srv.handleListRuns(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHandleGetRun_Success(t *testing.T) {
	store := newMockRunStore()

	// Add test run with evidence
	testRun := agent.NewRun("run-123", "Test goal")
	testRun.Status = agent.RunStatusRunning
	testRun.CurrentState = agent.StateExplore
	testRun.AddEvidence(agent.NewToolEvidence("test_tool", json.RawMessage(`"test content"`)))
	testRun.SetVar("key1", "value1")

	store.Save(context.Background(), testRun)

	srv := New(Config{RunStore: store})

	req := httptest.NewRequest(http.MethodGet, "/api/runs/run-123", nil)
	w := httptest.NewRecorder()

	srv.handleGetRun(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var detail RunDetail
	if err := json.NewDecoder(w.Body).Decode(&detail); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if detail.ID != "run-123" {
		t.Errorf("Expected ID 'run-123', got %s", detail.ID)
	}

	if detail.Goal != "Test goal" {
		t.Errorf("Expected goal 'Test goal', got %s", detail.Goal)
	}

	if len(detail.Evidence) != 1 {
		t.Errorf("Expected 1 evidence, got %d", len(detail.Evidence))
	}

	if len(detail.Vars) != 1 {
		t.Errorf("Expected 1 var, got %d", len(detail.Vars))
	}
}

func TestHandleGetRun_WithEvents(t *testing.T) {
	runStore := newMockRunStore()
	eventStore := newMockEventStore()

	// Add test run
	testRun := agent.NewRun("run-123", "Test goal")
	runStore.Save(context.Background(), testRun)

	// Add test events
	evt1 := event.Event{
		ID:        "evt-1",
		Type:      event.TypeRunStarted,
		RunID:     "run-123",
		Sequence:  1,
		Timestamp: time.Now(),
		Payload:   json.RawMessage(`{"goal":"Test goal"}`),
		Version:   1,
	}
	eventStore.Append(context.Background(), evt1)

	srv := New(Config{
		RunStore:   runStore,
		EventStore: eventStore,
	})

	req := httptest.NewRequest(http.MethodGet, "/api/runs/run-123", nil)
	w := httptest.NewRecorder()

	srv.handleGetRun(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var detail RunDetail
	if err := json.NewDecoder(w.Body).Decode(&detail); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(detail.Events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(detail.Events))
	}

	if detail.Events[0].Type != event.TypeRunStarted {
		t.Errorf("Expected event type 'RunStarted', got %s", detail.Events[0].Type)
	}
}

func TestHandleGetRun_NotFound(t *testing.T) {
	store := newMockRunStore()
	srv := New(Config{RunStore: store})

	req := httptest.NewRequest(http.MethodGet, "/api/runs/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.handleGetRun(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandleGetRun_NoStore(t *testing.T) {
	srv := New(Config{})

	req := httptest.NewRequest(http.MethodGet, "/api/runs/run-123", nil)
	w := httptest.NewRecorder()

	srv.handleGetRun(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

func TestHandleGetRun_MissingID(t *testing.T) {
	srv := New(Config{RunStore: newMockRunStore()})

	req := httptest.NewRequest(http.MethodGet, "/api/runs/", nil)
	w := httptest.NewRecorder()

	srv.handleGetRun(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleRunEvents_Success(t *testing.T) {
	eventStore := newMockEventStore()

	// Add test events
	evt1 := event.Event{
		ID:        "evt-1",
		Type:      event.TypeRunStarted,
		RunID:     "run-123",
		Sequence:  1,
		Timestamp: time.Now(),
		Payload:   json.RawMessage(`{"goal":"Test goal"}`),
		Version:   1,
	}
	evt2 := event.Event{
		ID:        "evt-2",
		Type:      event.TypeStateTransitioned,
		RunID:     "run-123",
		Sequence:  2,
		Timestamp: time.Now(),
		Payload:   json.RawMessage(`{"from":"intake","to":"explore"}`),
		Version:   1,
	}
	eventStore.Append(context.Background(), evt1, evt2)

	srv := New(Config{EventStore: eventStore})

	req := httptest.NewRequest(http.MethodGet, "/api/runs/events?run_id=run-123", nil)
	w := httptest.NewRecorder()

	srv.handleRunEvents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var events []event.Event
	if err := json.NewDecoder(w.Body).Decode(&events); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
	}
}

func TestHandleRunEvents_NoStore(t *testing.T) {
	srv := New(Config{})

	req := httptest.NewRequest(http.MethodGet, "/api/runs/events?run_id=run-123", nil)
	w := httptest.NewRecorder()

	srv.handleRunEvents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	var events []event.Event
	if err := json.NewDecoder(w.Body).Decode(&events); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if len(events) != 0 {
		t.Errorf("Expected empty list, got %d events", len(events))
	}
}

func TestHandleRunEvents_MissingRunID(t *testing.T) {
	srv := New(Config{EventStore: newMockEventStore()})

	req := httptest.NewRequest(http.MethodGet, "/api/runs/events", nil)
	w := httptest.NewRecorder()

	srv.handleRunEvents(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleIndex(t *testing.T) {
	srv := New(Config{})

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	srv.handleIndex(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("Expected Content-Type 'text/html; charset=utf-8', got %s", contentType)
	}

	body := w.Body.String()
	if len(body) == 0 {
		t.Error("Expected non-empty body")
	}

	if !contains(body, "agent-go Dashboard") && !contains(body, "Agent Dashboard") {
		t.Error("Expected body to contain 'agent-go Dashboard' or 'Agent Dashboard'")
	}
}

func TestHandleIndex_NotFound(t *testing.T) {
	srv := New(Config{})

	req := httptest.NewRequest(http.MethodGet, "/nonexistent", nil)
	w := httptest.NewRecorder()

	srv.handleIndex(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("Expected status 404, got %d", w.Code)
	}
}

func TestHandleWebSocket_MissingRunID(t *testing.T) {
	srv := New(Config{EventStore: newMockEventStore()})

	req := httptest.NewRequest(http.MethodGet, "/ws/", nil)
	w := httptest.NewRecorder()

	srv.handleWebSocket(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestHandleWebSocket_NoStore(t *testing.T) {
	srv := New(Config{})

	req := httptest.NewRequest(http.MethodGet, "/ws/run-123", nil)
	w := httptest.NewRecorder()

	srv.handleWebSocket(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 503, got %d", w.Code)
	}
}

func TestMiddleware_CORS(t *testing.T) {
	srv := New(Config{EnableCORS: true})

	req := httptest.NewRequest(http.MethodOptions, "/api/health", nil)
	w := httptest.NewRecorder()

	handler := srv.withMiddleware(srv.mux)
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	origin := w.Header().Get("Access-Control-Allow-Origin")
	if origin != "*" {
		t.Errorf("Expected CORS header '*', got %s", origin)
	}
}

func TestMiddleware_SecurityHeaders(t *testing.T) {
	srv := New(Config{})

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()

	handler := srv.withMiddleware(srv.mux)
	handler.ServeHTTP(w, req)

	xContentType := w.Header().Get("X-Content-Type-Options")
	if xContentType != "nosniff" {
		t.Errorf("Expected X-Content-Type-Options 'nosniff', got %s", xContentType)
	}

	xFrame := w.Header().Get("X-Frame-Options")
	if xFrame != "DENY" {
		t.Errorf("Expected X-Frame-Options 'DENY', got %s", xFrame)
	}
}

func TestServerIntegration(t *testing.T) {
	runStore := newMockRunStore()
	eventStore := newMockEventStore()

	// Create test run
	testRun := agent.NewRun("integration-test", "Integration test goal")
	testRun.Status = agent.RunStatusRunning
	runStore.Save(context.Background(), testRun)

	// Create test event
	evt := event.Event{
		ID:        "evt-integration",
		Type:      event.TypeRunStarted,
		RunID:     "integration-test",
		Sequence:  1,
		Timestamp: time.Now(),
		Payload:   json.RawMessage(`{"goal":"Integration test goal"}`),
		Version:   1,
	}
	eventStore.Append(context.Background(), evt)

	srv := New(Config{
		RunStore:   runStore,
		EventStore: eventStore,
	})

	// Test health
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	w := httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Health check failed: %d", w.Code)
	}

	// Test list runs
	req = httptest.NewRequest(http.MethodGet, "/api/runs", nil)
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("List runs failed: %d", w.Code)
	}

	// Test get run
	req = httptest.NewRequest(http.MethodGet, "/api/runs/integration-test", nil)
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Get run failed: %d", w.Code)
	}

	// Test get events
	req = httptest.NewRequest(http.MethodGet, "/api/runs/events?run_id=integration-test", nil)
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Get events failed: %d", w.Code)
	}

	// Test index
	req = httptest.NewRequest(http.MethodGet, "/", nil)
	w = httptest.NewRecorder()
	srv.mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Index failed: %d", w.Code)
	}
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || indexOf(s, substr) >= 0)
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
