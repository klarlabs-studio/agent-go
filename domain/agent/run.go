package agent

import (
	"encoding/json"
	"time"
)

// RunStatus represents the current status of a run.
type RunStatus string

const (
	RunStatusPending   RunStatus = "pending"   // Not yet started
	RunStatusRunning   RunStatus = "running"   // Currently executing
	RunStatusPaused    RunStatus = "paused"    // Temporarily suspended
	RunStatusCompleted RunStatus = "completed" // Successfully finished
	RunStatusFailed    RunStatus = "failed"    // Terminated with error
)

// PendingQuestion represents a question awaiting human input.
type PendingQuestion struct {
	Question string    `json:"question"`
	Options  []string  `json:"options,omitempty"`
	AskedAt  time.Time `json:"asked_at"`
}

// Run represents a single execution of the agent.
// It is the aggregate root for the agent domain.
type Run struct {
	ID              string           `json:"id"`
	Goal            string           `json:"goal"`
	CurrentState    State            `json:"current_state"`
	Vars            map[string]any   `json:"vars"`
	Evidence        []Evidence       `json:"evidence"`
	Status          RunStatus        `json:"status"`
	StartTime       time.Time        `json:"start_time"`
	EndTime         time.Time        `json:"end_time,omitempty"`
	Result          json.RawMessage  `json:"result,omitempty"`
	Error           string           `json:"error,omitempty"`
	PendingQuestion *PendingQuestion `json:"pending_question,omitempty"`
	ParentRunID     string           `json:"parent_run_id,omitempty"`
	TaskID          string           `json:"task_id,omitempty"`
}

// NewRun creates a new run with the given ID and initial state.
func NewRun(id string, goal string) *Run {
	return &Run{
		ID:           id,
		Goal:         goal,
		CurrentState: StateIntake,
		Vars:         make(map[string]any),
		Evidence:     make([]Evidence, 0),
		Status:       RunStatusPending,
		StartTime:    time.Now(),
	}
}

// Start marks the run as running.
func (r *Run) Start() {
	r.Status = RunStatusRunning
	r.StartTime = time.Now()
}

// TransitionTo changes the current state.
func (r *Run) TransitionTo(state State) {
	r.CurrentState = state
	if state.IsTerminal() {
		r.EndTime = time.Now()
		if state == StateDone {
			r.Status = RunStatusCompleted
		} else {
			r.Status = RunStatusFailed
		}
	}
}

// Complete marks the run as successfully completed.
func (r *Run) Complete(result json.RawMessage) {
	r.Status = RunStatusCompleted
	r.CurrentState = StateDone
	r.EndTime = time.Now()
	r.Result = result
}

// Fail marks the run as failed with an error.
func (r *Run) Fail(err string) {
	r.Status = RunStatusFailed
	r.CurrentState = StateFailed
	r.EndTime = time.Now()
	r.Error = err
}

// Pause suspends the run for later resumption.
func (r *Run) Pause() {
	r.Status = RunStatusPaused
}

// Resume continues a paused run.
func (r *Run) Resume() {
	if r.Status == RunStatusPaused {
		r.Status = RunStatusRunning
	}
}

// AddEvidence appends evidence to the run.
func (r *Run) AddEvidence(e Evidence) {
	r.Evidence = append(r.Evidence, e)
}

// SetVar sets a variable in the run context.
func (r *Run) SetVar(key string, value any) {
	r.Vars[key] = value
}

// GetVar retrieves a variable from the run context.
func (r *Run) GetVar(key string) (any, bool) {
	v, ok := r.Vars[key]
	return v, ok
}

// IsTerminal returns true if the run has reached a terminal status.
func (r *Run) IsTerminal() bool {
	return r.Status == RunStatusCompleted || r.Status == RunStatusFailed
}

// Duration returns the duration of the run.
func (r *Run) Duration() time.Duration {
	if r.EndTime.IsZero() {
		return time.Since(r.StartTime)
	}
	return r.EndTime.Sub(r.StartTime)
}

// HasPendingQuestion returns true if the run has a pending question awaiting human input.
func (r *Run) HasPendingQuestion() bool {
	return r.PendingQuestion != nil
}

// ClearPendingQuestion removes the pending question from the run.
func (r *Run) ClearPendingQuestion() {
	r.PendingQuestion = nil
}

// AskHuman sets a pending question and pauses the run.
func (r *Run) AskHuman(question string, options []string) {
	r.PendingQuestion = &PendingQuestion{
		Question: question,
		Options:  options,
		AskedAt:  time.Now(),
	}
	r.Status = RunStatusPaused
}
