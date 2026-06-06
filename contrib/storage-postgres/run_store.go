package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.klarlabs.de/agent/domain/agent"
	"go.klarlabs.de/agent/domain/run"
)

// RunStore is a PostgreSQL-backed implementation of run.Store.
type RunStore struct {
	pool   *pgxpool.Pool
	schema string
}

// NewRunStore creates a new PostgreSQL run store.
func NewRunStore(pool *pgxpool.Pool, schema string) *RunStore {
	if schema == "" {
		schema = "public"
	}
	return &RunStore{
		pool:   pool,
		schema: schema,
	}
}

// tableName returns the fully qualified table name.
func (s *RunStore) tableName() string {
	return fmt.Sprintf("%s.runs", s.schema)
}

// Save persists a new run.
func (s *RunStore) Save(ctx context.Context, r *agent.Run) error {
	if r.ID == "" {
		return run.ErrInvalidRunID
	}

	vars, err := json.Marshal(r.Vars)
	if err != nil {
		return fmt.Errorf("marshal vars: %w", err)
	}

	evidence, err := json.Marshal(r.Evidence)
	if err != nil {
		return fmt.Errorf("marshal evidence: %w", err)
	}

	query := fmt.Sprintf(`
		INSERT INTO %s (id, goal, current_state, vars, evidence, status, start_time, end_time, result, error)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`, s.tableName())

	var endTime *time.Time
	if !r.EndTime.IsZero() {
		endTime = &r.EndTime
	}

	_, err = s.pool.Exec(ctx, query,
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

	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			return run.ErrRunExists
		}
		return s.wrapError(err)
	}

	return nil
}

// Get retrieves a run by ID.
func (s *RunStore) Get(ctx context.Context, id string) (*agent.Run, error) {
	if id == "" {
		return nil, run.ErrInvalidRunID
	}

	query := fmt.Sprintf(`
		SELECT id, goal, current_state, vars, evidence, status, start_time, end_time, result, error
		FROM %s
		WHERE id = $1
	`, s.tableName())

	var r agent.Run
	var vars, evidence, result []byte
	var endTime *time.Time
	var currentState, status string
	var errStr *string

	err := s.pool.QueryRow(ctx, query, id).Scan(
		&r.ID,
		&r.Goal,
		&currentState,
		&vars,
		&evidence,
		&status,
		&r.StartTime,
		&endTime,
		&result,
		&errStr,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, run.ErrRunNotFound
		}
		return nil, s.wrapError(err)
	}

	r.CurrentState = agent.State(currentState)
	r.Status = agent.RunStatus(status)

	if endTime != nil {
		r.EndTime = *endTime
	}

	if errStr != nil {
		r.Error = *errStr
	}

	if len(result) > 0 {
		r.Result = result
	}

	if err := json.Unmarshal(vars, &r.Vars); err != nil {
		return nil, fmt.Errorf("unmarshal vars: %w", err)
	}

	if err := json.Unmarshal(evidence, &r.Evidence); err != nil {
		return nil, fmt.Errorf("unmarshal evidence: %w", err)
	}

	return &r, nil
}

// Update updates an existing run.
func (s *RunStore) Update(ctx context.Context, r *agent.Run) error {
	if r.ID == "" {
		return run.ErrInvalidRunID
	}

	vars, err := json.Marshal(r.Vars)
	if err != nil {
		return fmt.Errorf("marshal vars: %w", err)
	}

	evidence, err := json.Marshal(r.Evidence)
	if err != nil {
		return fmt.Errorf("marshal evidence: %w", err)
	}

	query := fmt.Sprintf(`
		UPDATE %s
		SET goal = $2,
			current_state = $3,
			vars = $4,
			evidence = $5,
			status = $6,
			start_time = $7,
			end_time = $8,
			result = $9,
			error = $10
		WHERE id = $1
	`, s.tableName())

	var endTime *time.Time
	if !r.EndTime.IsZero() {
		endTime = &r.EndTime
	}

	result, err := s.pool.Exec(ctx, query,
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

	if err != nil {
		return s.wrapError(err)
	}

	if result.RowsAffected() == 0 {
		return run.ErrRunNotFound
	}

	return nil
}

// Delete removes a run by ID.
func (s *RunStore) Delete(ctx context.Context, id string) error {
	if id == "" {
		return run.ErrInvalidRunID
	}

	query := fmt.Sprintf(`DELETE FROM %s WHERE id = $1`, s.tableName())

	result, err := s.pool.Exec(ctx, query, id)
	if err != nil {
		return s.wrapError(err)
	}

	if result.RowsAffected() == 0 {
		return run.ErrRunNotFound
	}

	return nil
}

// List returns runs matching the filter.
func (s *RunStore) List(ctx context.Context, filter run.ListFilter) ([]*agent.Run, error) {
	query, args := s.buildListQuery(filter, false)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, s.wrapError(err)
	}
	defer rows.Close()

	var runs []*agent.Run
	for rows.Next() {
		r, err := s.scanRun(rows)
		if err != nil {
			return nil, err
		}
		runs = append(runs, r)
	}

	if err := rows.Err(); err != nil {
		return nil, s.wrapError(err)
	}

	return runs, nil
}

// Count returns the number of runs matching the filter.
func (s *RunStore) Count(ctx context.Context, filter run.ListFilter) (int64, error) {
	query, args := s.buildCountQuery(filter)

	var count int64
	err := s.pool.QueryRow(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, s.wrapError(err)
	}

	return count, nil
}

// Summary returns aggregate statistics.
func (s *RunStore) Summary(ctx context.Context, filter run.ListFilter) (run.Summary, error) {
	whereClause, args := s.buildWhereClause(filter)

	query := fmt.Sprintf(`
		SELECT
			COUNT(*) as total,
			COUNT(*) FILTER (WHERE status = 'completed') as completed,
			COUNT(*) FILTER (WHERE status = 'failed') as failed,
			COUNT(*) FILTER (WHERE status = 'running') as running,
			COALESCE(AVG(EXTRACT(EPOCH FROM (end_time - start_time)) * 1000000000) FILTER (WHERE end_time IS NOT NULL), 0) as avg_duration_ns
		FROM %s
		%s
	`, s.tableName(), whereClause)

	var summary run.Summary
	var avgDurationNs float64

	err := s.pool.QueryRow(ctx, query, args...).Scan(
		&summary.TotalRuns,
		&summary.CompletedRuns,
		&summary.FailedRuns,
		&summary.RunningRuns,
		&avgDurationNs,
	)
	if err != nil {
		return run.Summary{}, s.wrapError(err)
	}

	summary.AverageDuration = time.Duration(avgDurationNs)

	return summary, nil
}

// buildListQuery constructs the SELECT query for listing runs.
func (s *RunStore) buildListQuery(filter run.ListFilter, countOnly bool) (string, []any) {
	whereClause, args := s.buildWhereClause(filter)

	query := fmt.Sprintf(`
		SELECT id, goal, current_state, vars, evidence, status, start_time, end_time, result, error
		FROM %s
		%s
	`, s.tableName(), whereClause)

	// Add ORDER BY
	orderBy := "start_time"
	switch filter.OrderBy {
	case run.OrderByEndTime:
		orderBy = "end_time"
	case run.OrderByID:
		orderBy = "id"
	case run.OrderByStatus:
		orderBy = "status"
	}

	direction := "ASC"
	if filter.Descending {
		direction = "DESC"
	}

	query += fmt.Sprintf(" ORDER BY %s %s NULLS LAST", orderBy, direction)

	// Add LIMIT and OFFSET
	if filter.Limit > 0 {
		args = append(args, filter.Limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}

	if filter.Offset > 0 {
		args = append(args, filter.Offset)
		query += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	return query, args
}

// buildCountQuery constructs the COUNT query.
func (s *RunStore) buildCountQuery(filter run.ListFilter) (string, []any) {
	whereClause, args := s.buildWhereClause(filter)

	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s %s`, s.tableName(), whereClause)

	return query, args
}

// buildWhereClause constructs the WHERE clause from filter.
func (s *RunStore) buildWhereClause(filter run.ListFilter) (string, []any) {
	var conditions []string
	var args []any
	argNum := 0

	// Filter by status
	if len(filter.Status) > 0 {
		argNum++
		statuses := make([]string, len(filter.Status))
		for i, status := range filter.Status {
			statuses[i] = string(status)
		}
		args = append(args, statuses)
		conditions = append(conditions, fmt.Sprintf("status = ANY($%d)", argNum))
	}

	// Filter by state
	if len(filter.States) > 0 {
		argNum++
		states := make([]string, len(filter.States))
		for i, state := range filter.States {
			states[i] = string(state)
		}
		args = append(args, states)
		conditions = append(conditions, fmt.Sprintf("current_state = ANY($%d)", argNum))
	}

	// Filter by time range
	if !filter.FromTime.IsZero() {
		argNum++
		args = append(args, filter.FromTime)
		conditions = append(conditions, fmt.Sprintf("start_time >= $%d", argNum))
	}

	if !filter.ToTime.IsZero() {
		argNum++
		args = append(args, filter.ToTime)
		conditions = append(conditions, fmt.Sprintf("start_time <= $%d", argNum))
	}

	// Filter by goal pattern
	if filter.GoalPattern != "" {
		argNum++
		args = append(args, "%"+filter.GoalPattern+"%")
		conditions = append(conditions, fmt.Sprintf("goal ILIKE $%d", argNum))
	}

	if len(conditions) == 0 {
		return "", args
	}

	return "WHERE " + strings.Join(conditions, " AND "), args
}

// scanRun scans a row into a Run struct.
func (s *RunStore) scanRun(rows pgx.Rows) (*agent.Run, error) {
	var r agent.Run
	var vars, evidence, result []byte
	var endTime *time.Time
	var currentState, status string
	var errStr *string

	err := rows.Scan(
		&r.ID,
		&r.Goal,
		&currentState,
		&vars,
		&evidence,
		&status,
		&r.StartTime,
		&endTime,
		&result,
		&errStr,
	)
	if err != nil {
		return nil, err
	}

	r.CurrentState = agent.State(currentState)
	r.Status = agent.RunStatus(status)

	if endTime != nil {
		r.EndTime = *endTime
	}

	if errStr != nil {
		r.Error = *errStr
	}

	if len(result) > 0 {
		r.Result = result
	}

	if err := json.Unmarshal(vars, &r.Vars); err != nil {
		return nil, fmt.Errorf("unmarshal vars: %w", err)
	}

	if err := json.Unmarshal(evidence, &r.Evidence); err != nil {
		return nil, fmt.Errorf("unmarshal evidence: %w", err)
	}

	return &r, nil
}

// wrapError wraps database errors with domain errors.
func (s *RunStore) wrapError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return errors.Join(run.ErrOperationTimeout, err)
	}

	return errors.Join(run.ErrConnectionFailed, err)
}

// Ensure RunStore implements run.Store and run.SummaryProvider
var (
	_ run.Store           = (*RunStore)(nil)
	_ run.SummaryProvider = (*RunStore)(nil)
)
