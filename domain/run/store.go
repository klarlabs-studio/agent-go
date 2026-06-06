// Package run provides the domain interface for run persistence.
package run

import (
	"context"
	"time"

	"go.klarlabs.de/agent/domain/agent"
)

// Store defines the interface for run persistence.
// Implementations may be in-memory, PostgreSQL, or any other backend.
type Store interface {
	// Save persists a new run.
	Save(ctx context.Context, run *agent.Run) error

	// Get retrieves a run by ID.
	Get(ctx context.Context, id string) (*agent.Run, error)

	// Update updates an existing run.
	Update(ctx context.Context, run *agent.Run) error

	// Delete removes a run by ID.
	Delete(ctx context.Context, id string) error

	// List returns runs matching the filter.
	List(ctx context.Context, filter ListFilter) ([]*agent.Run, error)

	// Count returns the number of runs matching the filter.
	Count(ctx context.Context, filter ListFilter) (int64, error)
}

// ListFilter specifies criteria for listing runs.
type ListFilter struct {
	// Status filters by run status (empty means all).
	Status []agent.RunStatus

	// States filters by current state (empty means all).
	States []agent.State

	// FromTime filters runs started after this time.
	FromTime time.Time

	// ToTime filters runs started before this time.
	ToTime time.Time

	// GoalPattern filters by goal text (substring match).
	GoalPattern string

	// Limit is the maximum number of runs to return (0 = no limit).
	Limit int

	// Offset is the number of runs to skip for pagination.
	Offset int

	// OrderBy specifies the sort order.
	OrderBy OrderBy

	// Descending reverses the sort order.
	Descending bool

	// ParentRunID filters by parent run ID (for delegation hierarchy queries).
	ParentRunID string

	// TaskID filters by task ID (for cross-agent task queries).
	TaskID string
}

// OrderBy specifies how to sort run results.
type OrderBy string

const (
	// OrderByStartTime sorts by run start time.
	OrderByStartTime OrderBy = "start_time"

	// OrderByEndTime sorts by run end time.
	OrderByEndTime OrderBy = "end_time"

	// OrderByID sorts by run ID.
	OrderByID OrderBy = "id"

	// OrderByStatus sorts by run status.
	OrderByStatus OrderBy = "status"
)

// Summary provides aggregate statistics about runs.
type Summary struct {
	// TotalRuns is the total number of runs.
	TotalRuns int64

	// CompletedRuns is the number of successfully completed runs.
	CompletedRuns int64

	// FailedRuns is the number of failed runs.
	FailedRuns int64

	// RunningRuns is the number of currently running runs.
	RunningRuns int64

	// AverageDuration is the average run duration.
	AverageDuration time.Duration
}

// SummaryProvider is an optional interface for stores that support summaries.
type SummaryProvider interface {
	// Summary returns aggregate statistics.
	Summary(ctx context.Context, filter ListFilter) (Summary, error)
}
