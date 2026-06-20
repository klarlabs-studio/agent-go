package agent

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewRun(t *testing.T) {
	run := NewRun("test-123", "test goal")

	if run.ID != "test-123" {
		t.Errorf("NewRun().ID = %q, want %q", run.ID, "test-123")
	}
	if run.Goal != "test goal" {
		t.Errorf("NewRun().Goal = %q, want %q", run.Goal, "test goal")
	}
	if run.CurrentState != StateIntake {
		t.Errorf("NewRun().CurrentState = %q, want %q", run.CurrentState, StateIntake)
	}
	if run.Status != RunStatusPending {
		t.Errorf("NewRun().Status = %q, want %q", run.Status, RunStatusPending)
	}
	if run.Vars == nil {
		t.Error("NewRun().Vars is nil, want initialized map")
	}
	if run.Evidence == nil {
		t.Error("NewRun().Evidence is nil, want initialized slice")
	}
	if run.StartTime.IsZero() {
		t.Error("NewRun().StartTime is zero, want current time")
	}
}

func TestRun_Start(t *testing.T) {
	run := NewRun("test", "goal")
	beforeStart := time.Now()
	run.Start()

	if run.Status != RunStatusRunning {
		t.Errorf("Run.Start() status = %q, want %q", run.Status, RunStatusRunning)
	}
	if run.StartTime.Before(beforeStart) {
		t.Error("Run.Start() did not update StartTime")
	}
}

func TestRun_TransitionTo(t *testing.T) {
	t.Run("non-terminal transition", func(t *testing.T) {
		run := NewRun("test", "goal")
		run.Start()
		run.TransitionTo(StateExplore)

		if run.CurrentState != StateExplore {
			t.Errorf("Run.TransitionTo() CurrentState = %q, want %q", run.CurrentState, StateExplore)
		}
		if run.Status != RunStatusRunning {
			t.Errorf("Run.TransitionTo() Status = %q, want %q", run.Status, RunStatusRunning)
		}
		if !run.EndTime.IsZero() {
			t.Error("Run.TransitionTo() to non-terminal state should not set EndTime")
		}
	})

	t.Run("terminal transition to done", func(t *testing.T) {
		run := NewRun("test", "goal")
		run.Start()
		run.TransitionTo(StateDone)

		if run.CurrentState != StateDone {
			t.Errorf("Run.TransitionTo() CurrentState = %q, want %q", run.CurrentState, StateDone)
		}
		if run.Status != RunStatusCompleted {
			t.Errorf("Run.TransitionTo() Status = %q, want %q", run.Status, RunStatusCompleted)
		}
		if run.EndTime.IsZero() {
			t.Error("Run.TransitionTo() to terminal state should set EndTime")
		}
	})

	t.Run("terminal transition to failed", func(t *testing.T) {
		run := NewRun("test", "goal")
		run.Start()
		run.TransitionTo(StateFailed)

		if run.CurrentState != StateFailed {
			t.Errorf("Run.TransitionTo() CurrentState = %q, want %q", run.CurrentState, StateFailed)
		}
		if run.Status != RunStatusFailed {
			t.Errorf("Run.TransitionTo() Status = %q, want %q", run.Status, RunStatusFailed)
		}
	})
}

func TestRun_Complete(t *testing.T) {
	run := NewRun("test", "goal")
	run.Start()
	result := json.RawMessage(`{"success": true}`)
	run.Complete(result)

	if run.Status != RunStatusCompleted {
		t.Errorf("Run.Complete() Status = %q, want %q", run.Status, RunStatusCompleted)
	}
	if run.CurrentState != StateDone {
		t.Errorf("Run.Complete() CurrentState = %q, want %q", run.CurrentState, StateDone)
	}
	if string(run.Result) != `{"success": true}` {
		t.Errorf("Run.Complete() Result = %q, want %q", run.Result, result)
	}
	if run.EndTime.IsZero() {
		t.Error("Run.Complete() should set EndTime")
	}
}

func TestRun_Fail(t *testing.T) {
	run := NewRun("test", "goal")
	run.Start()
	run.Fail("something went wrong")

	if run.Status != RunStatusFailed {
		t.Errorf("Run.Fail() Status = %q, want %q", run.Status, RunStatusFailed)
	}
	if run.CurrentState != StateFailed {
		t.Errorf("Run.Fail() CurrentState = %q, want %q", run.CurrentState, StateFailed)
	}
	if run.Error != "something went wrong" {
		t.Errorf("Run.Fail() Error = %q, want %q", run.Error, "something went wrong")
	}
	if run.EndTime.IsZero() {
		t.Error("Run.Fail() should set EndTime")
	}
}

func TestRun_PauseResume(t *testing.T) {
	run := NewRun("test", "goal")
	run.Start()
	run.Pause()

	if run.Status != RunStatusPaused {
		t.Errorf("Run.Pause() Status = %q, want %q", run.Status, RunStatusPaused)
	}

	run.Resume()
	if run.Status != RunStatusRunning {
		t.Errorf("Run.Resume() Status = %q, want %q", run.Status, RunStatusRunning)
	}
}

func TestRun_Resume_OnlyFromPaused(t *testing.T) {
	run := NewRun("test", "goal")
	run.Start()
	originalStatus := run.Status
	run.Resume() // Should not change status since not paused

	if run.Status != originalStatus {
		t.Errorf("Run.Resume() changed status from %q to %q when not paused", originalStatus, run.Status)
	}
}

func TestRun_AddEvidence(t *testing.T) {
	run := NewRun("test", "goal")
	evidence := Evidence{
		Type:      EvidenceToolResult,
		Source:    "test_tool",
		Content:   json.RawMessage(`{"data": "test"}`),
		Timestamp: time.Now(),
	}

	run.AddEvidence(evidence)

	if len(run.Evidence) != 1 {
		t.Errorf("Run.AddEvidence() evidence length = %d, want 1", len(run.Evidence))
	}
	if run.Evidence[0].Source != "test_tool" {
		t.Errorf("Run.AddEvidence() evidence source = %q, want %q", run.Evidence[0].Source, "test_tool")
	}
}

func TestRun_ConsumedToolCalls(t *testing.T) {
	run := NewRun("test", "goal")
	if got := run.ConsumedToolCalls(); got != 0 {
		t.Fatalf("fresh run consumed = %d, want 0", got)
	}

	run.AddEvidence(NewToolEvidence("read", json.RawMessage(`{}`)))
	run.AddEvidence(NewHumanEvidence(json.RawMessage(`{"q":"?"}`))) // not a tool call
	run.AddEvidence(NewToolEvidence("write", json.RawMessage(`{}`)))
	run.AddEvidence(NewSystemEvidence("note")) // not a tool call

	if got := run.ConsumedToolCalls(); got != 2 {
		t.Fatalf("ConsumedToolCalls = %d, want 2 (only tool results count)", got)
	}
}

func TestRun_Variables(t *testing.T) {
	run := NewRun("test", "goal")

	run.SetVar("key1", "value1")
	run.SetVar("key2", 42)

	val1, ok1 := run.GetVar("key1")
	if !ok1 || val1 != "value1" {
		t.Errorf("Run.GetVar(key1) = %v, %v, want %q, true", val1, ok1, "value1")
	}

	val2, ok2 := run.GetVar("key2")
	if !ok2 || val2 != 42 {
		t.Errorf("Run.GetVar(key2) = %v, %v, want %d, true", val2, ok2, 42)
	}

	_, ok3 := run.GetVar("nonexistent")
	if ok3 {
		t.Error("Run.GetVar(nonexistent) should return false")
	}
}

func TestRun_IsTerminal(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(*Run)
		expected bool
	}{
		{"pending", func(r *Run) {}, false},
		{"running", func(r *Run) { r.Start() }, false},
		{"paused", func(r *Run) { r.Start(); r.Pause() }, false},
		{"completed", func(r *Run) { r.Complete(nil) }, true},
		{"failed", func(r *Run) { r.Fail("error") }, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			run := NewRun("test", "goal")
			tt.setup(run)
			if got := run.IsTerminal(); got != tt.expected {
				t.Errorf("Run.IsTerminal() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRun_Duration(t *testing.T) {
	t.Run("running", func(t *testing.T) {
		run := NewRun("test", "goal")
		run.Start()
		time.Sleep(10 * time.Millisecond)
		duration := run.Duration()

		if duration < 10*time.Millisecond {
			t.Errorf("Run.Duration() = %v, want >= 10ms", duration)
		}
	})

	t.Run("completed", func(t *testing.T) {
		run := NewRun("test", "goal")
		run.Start()
		time.Sleep(10 * time.Millisecond)
		run.Complete(nil)
		duration1 := run.Duration()
		time.Sleep(10 * time.Millisecond)
		duration2 := run.Duration()

		if duration1 != duration2 {
			t.Errorf("Run.Duration() should be fixed after completion: %v != %v", duration1, duration2)
		}
	})
}
