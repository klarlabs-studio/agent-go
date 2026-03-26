package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/event"
	"github.com/felixgeelhaar/agent-go/domain/run"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// BulkInsertRuns inserts multiple runs in a single batch operation using the
// pgx batch API. All runs are inserted atomically within a transaction.
func (s *RunStore) BulkInsertRuns(ctx context.Context, runs []*agent.Run) error {
	if len(runs) == 0 {
		return nil
	}

	// Validate all runs before starting a transaction.
	for _, r := range runs {
		if r.ID == "" {
			return run.ErrInvalidRunID
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return s.wrapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := fmt.Sprintf(`
		INSERT INTO %s (id, goal, current_state, vars, evidence, status, start_time, end_time, result, error)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, s.tableName())

	batch := &pgx.Batch{}
	for _, r := range runs {
		vars, err := json.Marshal(r.Vars)
		if err != nil {
			return fmt.Errorf("marshal vars for run %s: %w", r.ID, err)
		}

		evidence, err := json.Marshal(r.Evidence)
		if err != nil {
			return fmt.Errorf("marshal evidence for run %s: %w", r.ID, err)
		}

		var endTime any
		if !r.EndTime.IsZero() {
			endTime = r.EndTime
		}

		batch.Queue(query,
			r.ID,
			r.Goal,
			string(r.CurrentState),
			vars,
			evidence,
			string(r.Status),
			r.StartTime,
			endTime,
			r.Result,
			r.Error,
		)
	}

	br := tx.SendBatch(ctx, batch)
	for i := range runs {
		_, err := br.Exec()
		if err != nil {
			_ = br.Close()
			if isUniqueViolation(err) {
				return fmt.Errorf("run %s: %w", runs[i].ID, run.ErrRunExists)
			}
			return s.wrapError(err)
		}
	}
	if err := br.Close(); err != nil {
		return s.wrapError(err)
	}

	return tx.Commit(ctx)
}

// BulkInsertEvents inserts multiple events in a single batch operation using
// the pgx batch API. All events are inserted atomically within a transaction.
// Sequence numbers are assigned automatically.
func (s *EventStore) BulkInsertEvents(ctx context.Context, events []event.Event) error {
	if len(events) == 0 {
		return nil
	}

	// Validate all events before starting a transaction.
	for i := range events {
		if events[i].Type == "" {
			return event.ErrInvalidEvent
		}
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return s.wrapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Get current max sequences for all affected runs.
	sequences := make(map[string]uint64)
	for _, e := range events {
		if _, ok := sequences[e.RunID]; !ok {
			var maxSeq *uint64
			err := tx.QueryRow(ctx,
				fmt.Sprintf("SELECT MAX(sequence) FROM %s WHERE run_id = $1", s.tableName()),
				e.RunID,
			).Scan(&maxSeq)
			if err != nil && !isNoRows(err) {
				return s.wrapError(err)
			}
			if maxSeq != nil {
				sequences[e.RunID] = *maxSeq
			}
		}
	}

	insertQuery := fmt.Sprintf(`
		INSERT INTO %s (id, run_id, type, timestamp, payload, sequence, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, s.tableName())

	batch := &pgx.Batch{}
	for i := range events {
		if events[i].ID == "" {
			events[i].ID = uuid.New().String()
		}

		sequences[events[i].RunID]++
		events[i].Sequence = sequences[events[i].RunID]

		if events[i].Version == 0 {
			events[i].Version = 1
		}

		batch.Queue(insertQuery,
			events[i].ID,
			events[i].RunID,
			string(events[i].Type),
			events[i].Timestamp,
			events[i].Payload,
			events[i].Sequence,
			events[i].Version,
		)
	}

	br := tx.SendBatch(ctx, batch)
	for range events {
		if _, err := br.Exec(); err != nil {
			_ = br.Close()
			return s.wrapError(err)
		}
	}
	if err := br.Close(); err != nil {
		return s.wrapError(err)
	}

	if err := tx.Commit(ctx); err != nil {
		return s.wrapError(err)
	}

	// Notify subscribers after successful commit.
	s.notifySubscribers(events)

	return nil
}

// isUniqueViolation checks if the error is a PostgreSQL unique constraint violation.
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	// pgx wraps PostgreSQL error codes. Code 23505 is unique_violation.
	return containsString(err.Error(), "duplicate key") ||
		containsString(err.Error(), "23505")
}

// isNoRows checks if the error represents no rows found.
func isNoRows(err error) bool {
	return err != nil && err.Error() == "no rows in result set"
}

// containsString checks if s contains substr.
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

// searchString is a simple substring search.
func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
