package notification

import (
	"encoding/json"
	"testing"
	"time"
)

// TestNewEvent_CreatesEventsWithCorrectTypeTimestampAndPayload tests that NewEvent
// creates events with correct type, timestamp, and marshaled payload for all event types.
func TestNewEvent_CreatesEventsWithCorrectTypeTimestampAndPayload(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		eventType   EventType
		payload     any
		wantPayload string // Expected JSON payload
	}{
		{
			name:      "run.started with full payload",
			eventType: EventRunStarted,
			payload: RunStartedPayload{
				Goal: "Process files",
				Vars: map[string]any{"path": "/tmp"},
			},
			wantPayload: `{"goal":"Process files","vars":{"path":"/tmp"}}`,
		},
		{
			name:      "run.started without vars",
			eventType: EventRunStarted,
			payload: RunStartedPayload{
				Goal: "Simple goal",
			},
			wantPayload: `{"goal":"Simple goal"}`,
		},
		{
			name:      "run.completed",
			eventType: EventRunCompleted,
			payload: RunCompletedPayload{
				Result:   json.RawMessage(`{"status":"success"}`),
				Duration: 5000 * time.Millisecond,
				Steps:    10,
			},
			wantPayload: `{"result":{"status":"success"},"duration_ms":5000000000,"steps":10}`,
		},
		{
			name:      "run.failed",
			eventType: EventRunFailed,
			payload: RunFailedPayload{
				Error:    "budget exhausted",
				State:    "explore",
				Duration: 2500 * time.Millisecond,
			},
			wantPayload: `{"error":"budget exhausted","state":"explore","duration_ms":2500000000}`,
		},
		{
			name:        "run.paused",
			eventType:   EventRunPaused,
			payload:     struct{}{}, // Empty payload
			wantPayload: `{}`,
		},
		{
			name:      "state.changed with reason",
			eventType: EventStateChanged,
			payload: StateChangedPayload{
				FromState: "intake",
				ToState:   "explore",
				Reason:    "ready to gather evidence",
			},
			wantPayload: `{"from_state":"intake","to_state":"explore","reason":"ready to gather evidence"}`,
		},
		{
			name:      "state.changed without reason",
			eventType: EventStateChanged,
			payload: StateChangedPayload{
				FromState: "explore",
				ToState:   "decide",
			},
			wantPayload: `{"from_state":"explore","to_state":"decide"}`,
		},
		{
			name:      "tool.started with input",
			eventType: EventToolStarted,
			payload: ToolStartedPayload{
				ToolName: "read_file",
				Input:    json.RawMessage(`{"path":"/data/input.txt"}`),
				State:    "explore",
			},
			wantPayload: `{"tool_name":"read_file","input":{"path":"/data/input.txt"},"state":"explore"}`,
		},
		{
			name:      "tool.started without input",
			eventType: EventToolStarted,
			payload: ToolStartedPayload{
				ToolName: "list_files",
				State:    "explore",
			},
			wantPayload: `{"tool_name":"list_files","state":"explore"}`,
		},
		{
			name:      "tool.completed with output and cached",
			eventType: EventToolCompleted,
			payload: ToolCompletedPayload{
				ToolName: "read_file",
				Output:   json.RawMessage(`{"content":"file data"}`),
				Duration: 100 * time.Millisecond,
				Cached:   true,
			},
			wantPayload: `{"tool_name":"read_file","output":{"content":"file data"},"duration_ms":100000000,"cached":true}`,
		},
		{
			name:      "tool.completed without output or cached flag",
			eventType: EventToolCompleted,
			payload: ToolCompletedPayload{
				ToolName: "delete_file",
				Duration: 50 * time.Millisecond,
			},
			wantPayload: `{"tool_name":"delete_file","duration_ms":50000000}`,
		},
		{
			name:      "tool.failed",
			eventType: EventToolFailed,
			payload: ToolFailedPayload{
				ToolName: "write_file",
				Error:    "permission denied",
			},
			wantPayload: `{"tool_name":"write_file","error":"permission denied"}`,
		},
		{
			name:      "approval.needed",
			eventType: EventApprovalNeeded,
			payload: ApprovalNeededPayload{
				ToolName:  "delete_file",
				Input:     json.RawMessage(`{"path":"/data/important.txt"}`),
				RiskLevel: "high",
			},
			wantPayload: `{"tool_name":"delete_file","input":{"path":"/data/important.txt"},"risk_level":"high"}`,
		},
		{
			name:      "budget.warning",
			eventType: EventBudgetWarning,
			payload: BudgetWarningPayload{
				BudgetName: "tool_calls",
				Remaining:  10,
				Limit:      100,
				Percentage: 10,
			},
			wantPayload: `{"budget_name":"tool_calls","remaining":10,"limit":100,"percentage":10}`,
		},
		{
			name:      "budget.exhausted",
			eventType: EventBudgetExhausted,
			payload: BudgetExhaustedPayload{
				BudgetName: "max_steps",
			},
			wantPayload: `{"budget_name":"max_steps"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, err := NewEvent("event-123", tt.eventType, "run-456", tt.payload)
			if err != nil {
				t.Fatalf("NewEvent() error = %v, want nil", err)
			}

			// Verify ID
			if event.ID != "event-123" {
				t.Errorf("event.ID = %q, want %q", event.ID, "event-123")
			}

			// Verify Type
			if event.Type != tt.eventType {
				t.Errorf("event.Type = %q, want %q", event.Type, tt.eventType)
			}

			// Verify RunID
			if event.RunID != "run-456" {
				t.Errorf("event.RunID = %q, want %q", event.RunID, "run-456")
			}

			// Verify Timestamp is recent
			if event.Timestamp.Before(now) || event.Timestamp.After(time.Now().Add(time.Second)) {
				t.Errorf("event.Timestamp = %v, want recent timestamp near %v", event.Timestamp, now)
			}

			// Verify Payload marshaling
			if string(event.Payload) != tt.wantPayload {
				t.Errorf("event.Payload = %s, want %s", string(event.Payload), tt.wantPayload)
			}
		})
	}
}

// TestNewEvent_ErrorOnInvalidPayload tests that NewEvent returns error for unmarshallable payloads.
func TestNewEvent_ErrorOnInvalidPayload(t *testing.T) {
	// Functions cannot be marshaled to JSON
	invalidPayload := struct {
		Fn func()
	}{
		Fn: func() {},
	}

	event, err := NewEvent("event-123", EventRunStarted, "run-456", invalidPayload)
	if err == nil {
		t.Fatal("NewEvent() with invalid payload succeeded, want error")
	}
	if event != nil {
		t.Errorf("NewEvent() returned non-nil event on error: %+v", event)
	}
}

// TestDecodePayload_Roundtrips tests that DecodePayload correctly unmarshals
// all typed payload structs that were marshaled by NewEvent.
func TestDecodePayload_Roundtrips(t *testing.T) {
	tests := []struct {
		name        string
		eventType   EventType
		payload     any
		decodeInto  any
		wantDecoded any
	}{
		{
			name:      "RunStartedPayload with vars",
			eventType: EventRunStarted,
			payload: RunStartedPayload{
				Goal: "Process files",
				Vars: map[string]any{"path": "/tmp", "count": float64(5)},
			},
			decodeInto: &RunStartedPayload{},
			wantDecoded: &RunStartedPayload{
				Goal: "Process files",
				Vars: map[string]any{"path": "/tmp", "count": float64(5)},
			},
		},
		{
			name:      "RunStartedPayload without vars",
			eventType: EventRunStarted,
			payload: RunStartedPayload{
				Goal: "Simple goal",
			},
			decodeInto: &RunStartedPayload{},
			wantDecoded: &RunStartedPayload{
				Goal: "Simple goal",
			},
		},
		{
			name:      "RunCompletedPayload",
			eventType: EventRunCompleted,
			payload: RunCompletedPayload{
				Result:   json.RawMessage(`{"status":"ok"}`),
				Duration: 5000000000,
				Steps:    10,
			},
			decodeInto: &RunCompletedPayload{},
			wantDecoded: &RunCompletedPayload{
				Result:   json.RawMessage(`{"status":"ok"}`),
				Duration: 5000000000,
				Steps:    10,
			},
		},
		{
			name:      "RunFailedPayload",
			eventType: EventRunFailed,
			payload: RunFailedPayload{
				Error:    "budget exhausted",
				State:    "explore",
				Duration: 2500000000,
			},
			decodeInto: &RunFailedPayload{},
			wantDecoded: &RunFailedPayload{
				Error:    "budget exhausted",
				State:    "explore",
				Duration: 2500000000,
			},
		},
		{
			name:        "RunPausedPayload (empty struct)",
			eventType:   EventRunPaused,
			payload:     struct{}{},
			decodeInto:  &struct{}{},
			wantDecoded: &struct{}{},
		},
		{
			name:      "StateChangedPayload with reason",
			eventType: EventStateChanged,
			payload: StateChangedPayload{
				FromState: "intake",
				ToState:   "explore",
				Reason:    "ready",
			},
			decodeInto: &StateChangedPayload{},
			wantDecoded: &StateChangedPayload{
				FromState: "intake",
				ToState:   "explore",
				Reason:    "ready",
			},
		},
		{
			name:      "StateChangedPayload without reason",
			eventType: EventStateChanged,
			payload: StateChangedPayload{
				FromState: "explore",
				ToState:   "decide",
			},
			decodeInto: &StateChangedPayload{},
			wantDecoded: &StateChangedPayload{
				FromState: "explore",
				ToState:   "decide",
			},
		},
		{
			name:      "ToolStartedPayload with input",
			eventType: EventToolStarted,
			payload: ToolStartedPayload{
				ToolName: "read_file",
				Input:    json.RawMessage(`{"path":"/data/input.txt"}`),
				State:    "explore",
			},
			decodeInto: &ToolStartedPayload{},
			wantDecoded: &ToolStartedPayload{
				ToolName: "read_file",
				Input:    json.RawMessage(`{"path":"/data/input.txt"}`),
				State:    "explore",
			},
		},
		{
			name:      "ToolStartedPayload without input",
			eventType: EventToolStarted,
			payload: ToolStartedPayload{
				ToolName: "list_files",
				State:    "explore",
			},
			decodeInto: &ToolStartedPayload{},
			wantDecoded: &ToolStartedPayload{
				ToolName: "list_files",
				State:    "explore",
			},
		},
		{
			name:      "ToolCompletedPayload with output and cached",
			eventType: EventToolCompleted,
			payload: ToolCompletedPayload{
				ToolName: "read_file",
				Output:   json.RawMessage(`{"content":"data"}`),
				Duration: 100000000,
				Cached:   true,
			},
			decodeInto: &ToolCompletedPayload{},
			wantDecoded: &ToolCompletedPayload{
				ToolName: "read_file",
				Output:   json.RawMessage(`{"content":"data"}`),
				Duration: 100000000,
				Cached:   true,
			},
		},
		{
			name:      "ToolCompletedPayload without output or cached",
			eventType: EventToolCompleted,
			payload: ToolCompletedPayload{
				ToolName: "delete_file",
				Duration: 50000000,
			},
			decodeInto: &ToolCompletedPayload{},
			wantDecoded: &ToolCompletedPayload{
				ToolName: "delete_file",
				Duration: 50000000,
			},
		},
		{
			name:      "ToolFailedPayload",
			eventType: EventToolFailed,
			payload: ToolFailedPayload{
				ToolName: "write_file",
				Error:    "permission denied",
			},
			decodeInto: &ToolFailedPayload{},
			wantDecoded: &ToolFailedPayload{
				ToolName: "write_file",
				Error:    "permission denied",
			},
		},
		{
			name:      "ApprovalNeededPayload",
			eventType: EventApprovalNeeded,
			payload: ApprovalNeededPayload{
				ToolName:  "delete_file",
				Input:     json.RawMessage(`{"path":"/data/important.txt"}`),
				RiskLevel: "high",
			},
			decodeInto: &ApprovalNeededPayload{},
			wantDecoded: &ApprovalNeededPayload{
				ToolName:  "delete_file",
				Input:     json.RawMessage(`{"path":"/data/important.txt"}`),
				RiskLevel: "high",
			},
		},
		{
			name:      "BudgetWarningPayload",
			eventType: EventBudgetWarning,
			payload: BudgetWarningPayload{
				BudgetName: "tool_calls",
				Remaining:  10,
				Limit:      100,
				Percentage: 10,
			},
			decodeInto: &BudgetWarningPayload{},
			wantDecoded: &BudgetWarningPayload{
				BudgetName: "tool_calls",
				Remaining:  10,
				Limit:      100,
				Percentage: 10,
			},
		},
		{
			name:      "BudgetExhaustedPayload",
			eventType: EventBudgetExhausted,
			payload: BudgetExhaustedPayload{
				BudgetName: "max_steps",
			},
			decodeInto: &BudgetExhaustedPayload{},
			wantDecoded: &BudgetExhaustedPayload{
				BudgetName: "max_steps",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create event with payload
			event, err := NewEvent("event-123", tt.eventType, "run-456", tt.payload)
			if err != nil {
				t.Fatalf("NewEvent() error = %v, want nil", err)
			}

			// Decode payload
			err = event.DecodePayload(tt.decodeInto)
			if err != nil {
				t.Fatalf("DecodePayload() error = %v, want nil", err)
			}

			// Marshal both to JSON for comparison to handle json.RawMessage and map equality
			gotJSON, err := json.Marshal(tt.decodeInto)
			if err != nil {
				t.Fatalf("json.Marshal(decoded) error = %v", err)
			}

			wantJSON, err := json.Marshal(tt.wantDecoded)
			if err != nil {
				t.Fatalf("json.Marshal(want) error = %v", err)
			}

			if string(gotJSON) != string(wantJSON) {
				t.Errorf("DecodePayload() result = %s, want %s", string(gotJSON), string(wantJSON))
			}
		})
	}
}

// TestDecodePayload_EdgeCases tests edge cases for DecodePayload.
func TestDecodePayload_EdgeCases(t *testing.T) {
	tests := []struct {
		name       string
		event      *Event
		decodeInto any
		wantErr    bool
	}{
		{
			name: "nil payload returns no error",
			event: &Event{
				ID:        "event-123",
				Type:      EventRunStarted,
				Timestamp: time.Now(),
				RunID:     "run-456",
				Payload:   nil,
			},
			decodeInto: &RunStartedPayload{},
			wantErr:    false,
		},
		{
			name: "empty payload (empty JSON object)",
			event: &Event{
				ID:        "event-123",
				Type:      EventRunPaused,
				Timestamp: time.Now(),
				RunID:     "run-456",
				Payload:   json.RawMessage(`{}`),
			},
			decodeInto: &struct{}{},
			wantErr:    false,
		},
		{
			name: "invalid JSON in payload",
			event: &Event{
				ID:        "event-123",
				Type:      EventRunStarted,
				Timestamp: time.Now(),
				RunID:     "run-456",
				Payload:   json.RawMessage(`{invalid json`),
			},
			decodeInto: &RunStartedPayload{},
			wantErr:    true,
		},
		{
			name: "mismatched types - number to string",
			event: &Event{
				ID:        "event-123",
				Type:      EventToolFailed,
				Timestamp: time.Now(),
				RunID:     "run-456",
				Payload:   json.RawMessage(`{"tool_name":123,"error":"err"}`),
			},
			decodeInto: &ToolFailedPayload{},
			wantErr:    true,
		},
		{
			name: "extra fields in payload (should be ignored)",
			event: &Event{
				ID:        "event-123",
				Type:      EventToolFailed,
				Timestamp: time.Now(),
				RunID:     "run-456",
				Payload:   json.RawMessage(`{"tool_name":"test","error":"err","extra_field":"ignored"}`),
			},
			decodeInto: &ToolFailedPayload{},
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.event.DecodePayload(tt.decodeInto)
			if (err != nil) != tt.wantErr {
				t.Errorf("DecodePayload() error = %v, wantErr %v", err, tt.wantErr)
			}

			// For successful nil payload case, verify the struct wasn't modified
			if tt.name == "nil payload returns no error" {
				payload := tt.decodeInto.(*RunStartedPayload)
				if payload.Goal != "" || payload.Vars != nil {
					t.Errorf("DecodePayload() modified struct with nil payload: %+v", payload)
				}
			}
		})
	}
}

// TestEvent_JSONSerialization tests that Event can be marshaled and unmarshaled to/from JSON.
func TestEvent_JSONSerialization(t *testing.T) {
	original := &Event{
		ID:        "event-123",
		Type:      EventStateChanged,
		Timestamp: time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC),
		RunID:     "run-456",
		Payload:   json.RawMessage(`{"from_state":"intake","to_state":"explore"}`),
	}

	// Marshal to JSON
	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	// Unmarshal from JSON
	var decoded Event
	err = json.Unmarshal(data, &decoded)
	if err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	// Verify fields
	if decoded.ID != original.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, original.ID)
	}
	if decoded.Type != original.Type {
		t.Errorf("Type = %q, want %q", decoded.Type, original.Type)
	}
	if !decoded.Timestamp.Equal(original.Timestamp) {
		t.Errorf("Timestamp = %v, want %v", decoded.Timestamp, original.Timestamp)
	}
	if decoded.RunID != original.RunID {
		t.Errorf("RunID = %q, want %q", decoded.RunID, original.RunID)
	}
	if string(decoded.Payload) != string(original.Payload) {
		t.Errorf("Payload = %s, want %s", string(decoded.Payload), string(original.Payload))
	}
}

// TestFilterByType tests that FilterByType creates a filter that only allows specified event types.
func TestFilterByType(t *testing.T) {
	tests := []struct {
		name         string
		allowedTypes []EventType
		testEvents   []struct {
			eventType EventType
			want      bool
		}
	}{
		{
			name:         "single type filter",
			allowedTypes: []EventType{EventRunStarted},
			testEvents: []struct {
				eventType EventType
				want      bool
			}{
				{EventRunStarted, true},
				{EventRunCompleted, false},
				{EventRunFailed, false},
				{EventToolStarted, false},
			},
		},
		{
			name:         "multiple types filter",
			allowedTypes: []EventType{EventRunStarted, EventRunCompleted, EventRunFailed},
			testEvents: []struct {
				eventType EventType
				want      bool
			}{
				{EventRunStarted, true},
				{EventRunCompleted, true},
				{EventRunFailed, true},
				{EventToolStarted, false},
				{EventToolCompleted, false},
				{EventStateChanged, false},
			},
		},
		{
			name:         "all event types",
			allowedTypes: []EventType{EventRunStarted, EventRunCompleted, EventRunFailed, EventRunPaused, EventStateChanged, EventToolStarted, EventToolCompleted, EventToolFailed, EventApprovalNeeded, EventBudgetWarning, EventBudgetExhausted},
			testEvents: []struct {
				eventType EventType
				want      bool
			}{
				{EventRunStarted, true},
				{EventRunCompleted, true},
				{EventRunFailed, true},
				{EventRunPaused, true},
				{EventStateChanged, true},
				{EventToolStarted, true},
				{EventToolCompleted, true},
				{EventToolFailed, true},
				{EventApprovalNeeded, true},
				{EventBudgetWarning, true},
				{EventBudgetExhausted, true},
			},
		},
		{
			name:         "empty filter (allows nothing)",
			allowedTypes: []EventType{},
			testEvents: []struct {
				eventType EventType
				want      bool
			}{
				{EventRunStarted, false},
				{EventRunCompleted, false},
				{EventToolStarted, false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := FilterByType(tt.allowedTypes...)

			for _, te := range tt.testEvents {
				event := &Event{
					ID:        "event-123",
					Type:      te.eventType,
					Timestamp: time.Now(),
					RunID:     "run-456",
					Payload:   json.RawMessage(`{}`),
				}

				got := filter(event)
				if got != te.want {
					t.Errorf("FilterByType()(%v) = %v, want %v", te.eventType, got, te.want)
				}
			}
		})
	}
}

// TestFilterByRunID tests that FilterByRunID creates a filter that only allows events for specific run IDs.
func TestFilterByRunID(t *testing.T) {
	tests := []struct {
		name          string
		allowedRunIDs []string
		testRunIDs    []struct {
			runID string
			want  bool
		}
	}{
		{
			name:          "single run ID",
			allowedRunIDs: []string{"run-123"},
			testRunIDs: []struct {
				runID string
				want  bool
			}{
				{"run-123", true},
				{"run-456", false},
				{"run-789", false},
				{"", false},
			},
		},
		{
			name:          "multiple run IDs",
			allowedRunIDs: []string{"run-123", "run-456", "run-789"},
			testRunIDs: []struct {
				runID string
				want  bool
			}{
				{"run-123", true},
				{"run-456", true},
				{"run-789", true},
				{"run-000", false},
				{"", false},
			},
		},
		{
			name:          "empty run ID in allowed list",
			allowedRunIDs: []string{"run-123", ""},
			testRunIDs: []struct {
				runID string
				want  bool
			}{
				{"run-123", true},
				{"", true},
				{"run-456", false},
			},
		},
		{
			name:          "empty filter (allows nothing)",
			allowedRunIDs: []string{},
			testRunIDs: []struct {
				runID string
				want  bool
			}{
				{"run-123", false},
				{"run-456", false},
				{"", false},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := FilterByRunID(tt.allowedRunIDs...)

			for _, tr := range tt.testRunIDs {
				event := &Event{
					ID:        "event-123",
					Type:      EventRunStarted,
					Timestamp: time.Now(),
					RunID:     tr.runID,
					Payload:   json.RawMessage(`{}`),
				}

				got := filter(event)
				if got != tr.want {
					t.Errorf("FilterByRunID()(%q) = %v, want %v", tr.runID, got, tr.want)
				}
			}
		})
	}
}

// TestCombineFilters tests that CombineFilters creates a filter requiring all provided filters to pass.
func TestCombineFilters(t *testing.T) {
	tests := []struct {
		name    string
		filters []EventFilter
		events  []struct {
			event *Event
			want  bool
		}
	}{
		{
			name: "single filter - type only",
			filters: []EventFilter{
				FilterByType(EventRunStarted, EventRunCompleted),
			},
			events: []struct {
				event *Event
				want  bool
			}{
				{
					event: &Event{Type: EventRunStarted, RunID: "run-123"},
					want:  true,
				},
				{
					event: &Event{Type: EventRunCompleted, RunID: "run-123"},
					want:  true,
				},
				{
					event: &Event{Type: EventRunFailed, RunID: "run-123"},
					want:  false,
				},
			},
		},
		{
			name: "two filters - type AND runID",
			filters: []EventFilter{
				FilterByType(EventRunStarted, EventRunCompleted),
				FilterByRunID("run-123", "run-456"),
			},
			events: []struct {
				event *Event
				want  bool
			}{
				{
					event: &Event{Type: EventRunStarted, RunID: "run-123"},
					want:  true,
				},
				{
					event: &Event{Type: EventRunCompleted, RunID: "run-456"},
					want:  true,
				},
				{
					event: &Event{Type: EventRunStarted, RunID: "run-789"},
					want:  false, // Wrong runID
				},
				{
					event: &Event{Type: EventRunFailed, RunID: "run-123"},
					want:  false, // Wrong type
				},
				{
					event: &Event{Type: EventRunFailed, RunID: "run-789"},
					want:  false, // Wrong type and runID
				},
			},
		},
		{
			name: "three filters - complex AND",
			filters: []EventFilter{
				FilterByType(EventToolStarted, EventToolCompleted),
				FilterByRunID("run-123"),
				func(event *Event) bool {
					// Custom filter: only allow events with specific ID prefix
					return len(event.ID) > 0 && event.ID[0] == 'e'
				},
			},
			events: []struct {
				event *Event
				want  bool
			}{
				{
					event: &Event{ID: "event-1", Type: EventToolStarted, RunID: "run-123"},
					want:  true, // All filters pass
				},
				{
					event: &Event{ID: "xevent-1", Type: EventToolStarted, RunID: "run-123"},
					want:  false, // ID doesn't start with 'e'
				},
				{
					event: &Event{ID: "event-1", Type: EventRunStarted, RunID: "run-123"},
					want:  false, // Wrong type
				},
				{
					event: &Event{ID: "event-1", Type: EventToolStarted, RunID: "run-456"},
					want:  false, // Wrong runID
				},
			},
		},
		{
			name:    "no filters - allows everything",
			filters: []EventFilter{},
			events: []struct {
				event *Event
				want  bool
			}{
				{
					event: &Event{Type: EventRunStarted, RunID: "run-123"},
					want:  true,
				},
				{
					event: &Event{Type: EventRunFailed, RunID: "run-456"},
					want:  true,
				},
			},
		},
		{
			name: "filter that always returns false",
			filters: []EventFilter{
				func(event *Event) bool { return false },
			},
			events: []struct {
				event *Event
				want  bool
			}{
				{
					event: &Event{Type: EventRunStarted, RunID: "run-123"},
					want:  false,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			combined := CombineFilters(tt.filters...)

			for i, te := range tt.events {
				got := combined(te.event)
				if got != te.want {
					t.Errorf("CombineFilters() event[%d] = %v, want %v (Type: %v, RunID: %v, ID: %v)",
						i, got, te.want, te.event.Type, te.event.RunID, te.event.ID)
				}
			}
		})
	}
}

// TestCombineFilters_ShortCircuit tests that CombineFilters short-circuits on first false result.
func TestCombineFilters_ShortCircuit(t *testing.T) {
	var firstCalled, secondCalled bool

	firstFilter := func(event *Event) bool {
		firstCalled = true
		return false // First filter fails
	}

	secondFilter := func(event *Event) bool {
		secondCalled = true
		return true
	}

	combined := CombineFilters(firstFilter, secondFilter)
	event := &Event{Type: EventRunStarted, RunID: "run-123"}

	result := combined(event)

	if result {
		t.Error("CombineFilters() = true, want false")
	}
	if !firstCalled {
		t.Error("First filter was not called")
	}
	if secondCalled {
		t.Error("Second filter was called despite first filter returning false (should short-circuit)")
	}
}

// TestFilterByType_Idempotent tests that FilterByType produces consistent results.
func TestFilterByType_Idempotent(t *testing.T) {
	filter := FilterByType(EventRunStarted, EventToolStarted)
	event := &Event{
		ID:        "event-123",
		Type:      EventRunStarted,
		Timestamp: time.Now(),
		RunID:     "run-456",
		Payload:   json.RawMessage(`{}`),
	}

	// Call filter multiple times
	for i := 0; i < 5; i++ {
		if !filter(event) {
			t.Errorf("FilterByType() call %d returned false, want true", i+1)
		}
	}
}

// TestFilterByRunID_Idempotent tests that FilterByRunID produces consistent results.
func TestFilterByRunID_Idempotent(t *testing.T) {
	filter := FilterByRunID("run-123", "run-456")
	event := &Event{
		ID:        "event-123",
		Type:      EventRunStarted,
		Timestamp: time.Now(),
		RunID:     "run-123",
		Payload:   json.RawMessage(`{}`),
	}

	// Call filter multiple times
	for i := 0; i < 5; i++ {
		if !filter(event) {
			t.Errorf("FilterByRunID() call %d returned false, want true", i+1)
		}
	}
}
