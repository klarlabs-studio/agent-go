// Package sqlite provides SQLite-backed implementations of agent-go storage interfaces.
//
// This package offers lightweight, file-based storage suitable for development,
// testing, and single-node deployments. SQLite provides ACID compliance and
// requires no external database server.
//
// # Usage
//
//	db, err := sql.Open("sqlite3", "agent.db")
//	if err != nil {
//		return err
//	}
//
//	cache := sqlite.NewCache(db)
//	eventStore := sqlite.NewEventStore(db)
//	runStore := sqlite.NewRunStore(db)
package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/felixgeelhaar/agent-go/domain/agent"
	"github.com/felixgeelhaar/agent-go/domain/cache"
	"github.com/felixgeelhaar/agent-go/domain/event"
	"github.com/felixgeelhaar/agent-go/domain/run"
	"github.com/google/uuid"
)

// Cache is a SQLite-backed implementation of cache.Cache.
// It stores cached values in a SQLite table with optional TTL support.
type Cache struct {
	db *sql.DB
}

// NewCache creates a new SQLite cache with the given database connection.
// The caller is responsible for managing the database connection lifecycle.
func NewCache(db *sql.DB) *Cache {
	return &Cache{db: db}
}

// EnsureSchema creates the cache table if it doesn't exist.
func (c *Cache) EnsureSchema(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS cache_entries (
		key TEXT PRIMARY KEY,
		value BLOB NOT NULL,
		expires_at INTEGER  -- Unix timestamp in milliseconds, NULL = no expiry
	);`

	_, err := c.db.ExecContext(ctx, query)
	return err
}

// Get retrieves a cached value by key.
// Returns the value, whether it was found, and any error.
func (c *Cache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	query := `SELECT value, expires_at FROM cache_entries WHERE key = ?`

	var value []byte
	var expiresAt *int64

	err := c.db.QueryRowContext(ctx, query, key).Scan(&value, &expiresAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}

	// Check if expired (expires_at is in milliseconds)
	if expiresAt != nil && *expiresAt <= time.Now().UnixMilli() {
		// Delete expired entry
		_, _ = c.db.ExecContext(ctx, `DELETE FROM cache_entries WHERE key = ?`, key)
		return nil, false, nil
	}

	return value, true, nil
}

// Set stores a value with the given key and options.
func (c *Cache) Set(ctx context.Context, key string, value []byte, opts cache.SetOptions) error {
	var expiresAt *int64
	if opts.TTL > 0 {
		// Store expiry time in milliseconds for better precision
		exp := time.Now().Add(opts.TTL).UnixMilli()
		expiresAt = &exp
	}

	query := `INSERT OR REPLACE INTO cache_entries (key, value, expires_at) VALUES (?, ?, ?)`
	_, err := c.db.ExecContext(ctx, query, key, value, expiresAt)
	return err
}

// Delete removes a cached entry by key.
func (c *Cache) Delete(ctx context.Context, key string) error {
	query := `DELETE FROM cache_entries WHERE key = ?`
	_, err := c.db.ExecContext(ctx, query, key)
	return err
}

// Exists checks if a key exists in the cache.
func (c *Cache) Exists(ctx context.Context, key string) (bool, error) {
	query := `SELECT 1 FROM cache_entries WHERE key = ? AND (expires_at IS NULL OR expires_at > ?)`

	var exists int
	err := c.db.QueryRowContext(ctx, query, key, time.Now().UnixMilli()).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// Clear removes all entries from the cache.
func (c *Cache) Clear(ctx context.Context) error {
	query := `DELETE FROM cache_entries`
	_, err := c.db.ExecContext(ctx, query)
	return err
}

// Close closes the underlying database connection.
func (c *Cache) Close() error {
	return c.db.Close()
}

// EventStore is a SQLite-backed implementation of event.Store.
// It provides event sourcing capabilities with atomic append operations.
type EventStore struct {
	db          *sql.DB
	subscribers map[string][]chan event.Event
	mu          sync.RWMutex
}

// NewEventStore creates a new SQLite event store with the given database connection.
// The caller is responsible for managing the database connection lifecycle.
func NewEventStore(db *sql.DB) *EventStore {
	return &EventStore{
		db:          db,
		subscribers: make(map[string][]chan event.Event),
	}
}

// EnsureSchema creates the events table if it doesn't exist.
func (s *EventStore) EnsureSchema(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS events (
		id TEXT PRIMARY KEY,
		run_id TEXT NOT NULL,
		type TEXT NOT NULL,
		timestamp DATETIME NOT NULL,
		payload BLOB,
		sequence INTEGER NOT NULL,
		version INTEGER NOT NULL DEFAULT 1,
		UNIQUE(run_id, sequence)
	);
	CREATE INDEX IF NOT EXISTS idx_events_run_id ON events(run_id);
	CREATE INDEX IF NOT EXISTS idx_events_run_seq ON events(run_id, sequence);`

	_, err := s.db.ExecContext(ctx, query)
	return err
}

// Append persists one or more events atomically.
// Events are assigned sequence numbers in order of appearance.
func (s *EventStore) Append(ctx context.Context, events ...event.Event) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Get current max sequence for each run
	sequences := make(map[string]uint64)
	for _, e := range events {
		if _, ok := sequences[e.RunID]; !ok {
			var maxSeq *uint64
			err := tx.QueryRowContext(ctx,
				`SELECT MAX(sequence) FROM events WHERE run_id = ?`,
				e.RunID,
			).Scan(&maxSeq)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if maxSeq != nil {
				sequences[e.RunID] = *maxSeq
			}
		}
	}

	// Insert events with sequence numbers
	insertQuery := `
		INSERT INTO events (id, run_id, type, timestamp, payload, sequence, version)
		VALUES (?, ?, ?, ?, ?, ?, ?)`

	for i := range events {
		// Assign ID if not set
		if events[i].ID == "" {
			events[i].ID = uuid.New().String()
		}

		// Assign sequence number
		sequences[events[i].RunID]++
		events[i].Sequence = sequences[events[i].RunID]

		// Validate event
		if events[i].Type == "" {
			return event.ErrInvalidEvent
		}

		// Set default version
		if events[i].Version == 0 {
			events[i].Version = 1
		}

		_, err := tx.ExecContext(ctx, insertQuery,
			events[i].ID,
			events[i].RunID,
			string(events[i].Type),
			events[i].Timestamp,
			events[i].Payload,
			events[i].Sequence,
			events[i].Version,
		)
		if err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// Notify subscribers
	s.notifySubscribers(events)

	return nil
}

// LoadEvents retrieves all events for a run in sequence order.
func (s *EventStore) LoadEvents(ctx context.Context, runID string) ([]event.Event, error) {
	query := `
		SELECT id, run_id, type, timestamp, payload, sequence, version
		FROM events
		WHERE run_id = ?
		ORDER BY sequence ASC`

	rows, err := s.db.QueryContext(ctx, query, runID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanEvents(rows)
}

// LoadEventsFrom retrieves events starting from a specific sequence number.
// This enables incremental replay from a known checkpoint.
func (s *EventStore) LoadEventsFrom(ctx context.Context, runID string, fromSeq uint64) ([]event.Event, error) {
	query := `
		SELECT id, run_id, type, timestamp, payload, sequence, version
		FROM events
		WHERE run_id = ? AND sequence >= ?
		ORDER BY sequence ASC`

	rows, err := s.db.QueryContext(ctx, query, runID, fromSeq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanEvents(rows)
}

// Subscribe returns a channel that receives new events for a run.
// The channel is closed when the context is cancelled or the run completes.
func (s *EventStore) Subscribe(ctx context.Context, runID string) (<-chan event.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan event.Event, 100)
	s.subscribers[runID] = append(s.subscribers[runID], ch)

	// Start cleanup goroutine
	go func() {
		<-ctx.Done()
		s.unsubscribe(runID, ch)
	}()

	return ch, nil
}

// Close closes the underlying database connection.
func (s *EventStore) Close() error {
	return s.db.Close()
}

// scanEvents scans rows into Event structs.
func (s *EventStore) scanEvents(rows *sql.Rows) ([]event.Event, error) {
	var events []event.Event
	for rows.Next() {
		var e event.Event
		var eventType string

		err := rows.Scan(
			&e.ID,
			&e.RunID,
			&eventType,
			&e.Timestamp,
			&e.Payload,
			&e.Sequence,
			&e.Version,
		)
		if err != nil {
			return nil, err
		}

		e.Type = event.Type(eventType)
		events = append(events, e)
	}

	return events, rows.Err()
}

// notifySubscribers sends events to all subscribers.
func (s *EventStore) notifySubscribers(events []event.Event) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, e := range events {
		if subs, ok := s.subscribers[e.RunID]; ok {
			for _, ch := range subs {
				select {
				case ch <- e:
				default:
					// Channel full, skip
				}
			}
		}
	}
}

// unsubscribe removes a subscriber channel.
func (s *EventStore) unsubscribe(runID string, ch chan event.Event) {
	s.mu.Lock()
	defer s.mu.Unlock()

	subs := s.subscribers[runID]
	for i, sub := range subs {
		if sub == ch {
			s.subscribers[runID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			break
		}
	}

	if len(s.subscribers[runID]) == 0 {
		delete(s.subscribers, runID)
	}
}

// RunStore is a SQLite-backed implementation of run.Store.
// It provides persistent storage for agent run state and history.
type RunStore struct {
	db *sql.DB
}

// NewRunStore creates a new SQLite run store with the given database connection.
// The caller is responsible for managing the database connection lifecycle.
func NewRunStore(db *sql.DB) *RunStore {
	return &RunStore{db: db}
}

// EnsureSchema creates the runs table if it doesn't exist.
func (s *RunStore) EnsureSchema(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS runs (
		id TEXT PRIMARY KEY,
		goal TEXT NOT NULL,
		current_state TEXT NOT NULL,
		vars BLOB,
		evidence BLOB,
		status TEXT NOT NULL,
		start_time DATETIME NOT NULL,
		end_time DATETIME,
		result BLOB,
		error TEXT
	);
	CREATE INDEX IF NOT EXISTS idx_runs_status ON runs(status);
	CREATE INDEX IF NOT EXISTS idx_runs_start_time ON runs(start_time);`

	_, err := s.db.ExecContext(ctx, query)
	return err
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

	query := `
		INSERT INTO runs (id, goal, current_state, vars, evidence, status, start_time, end_time, result, error)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	var endTime *time.Time
	if !r.EndTime.IsZero() {
		endTime = &r.EndTime
	}

	_, err = s.db.ExecContext(ctx, query,
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
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return run.ErrRunExists
		}
		return err
	}

	return nil
}

// Get retrieves a run by ID.
func (s *RunStore) Get(ctx context.Context, id string) (*agent.Run, error) {
	if id == "" {
		return nil, run.ErrInvalidRunID
	}

	query := `
		SELECT id, goal, current_state, vars, evidence, status, start_time, end_time, result, error
		FROM runs
		WHERE id = ?`

	var r agent.Run
	var vars, evidence, result []byte
	var endTime *time.Time
	var currentState, status string
	var errStr *string

	err := s.db.QueryRowContext(ctx, query, id).Scan(
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, run.ErrRunNotFound
		}
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

	query := `
		UPDATE runs
		SET goal = ?,
			current_state = ?,
			vars = ?,
			evidence = ?,
			status = ?,
			start_time = ?,
			end_time = ?,
			result = ?,
			error = ?
		WHERE id = ?`

	var endTime *time.Time
	if !r.EndTime.IsZero() {
		endTime = &r.EndTime
	}

	result, err := s.db.ExecContext(ctx, query,
		r.Goal,
		string(r.CurrentState),
		vars,
		evidence,
		string(r.Status),
		r.StartTime,
		endTime,
		r.Result,
		r.Error,
		r.ID,
	)

	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return run.ErrRunNotFound
	}

	return nil
}

// Delete removes a run by ID.
func (s *RunStore) Delete(ctx context.Context, id string) error {
	if id == "" {
		return run.ErrInvalidRunID
	}

	query := `DELETE FROM runs WHERE id = ?`

	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return run.ErrRunNotFound
	}

	return nil
}

// List returns runs matching the filter.
func (s *RunStore) List(ctx context.Context, filter run.ListFilter) ([]*agent.Run, error) {
	query, args := s.buildListQuery(filter)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
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
		return nil, err
	}

	return runs, nil
}

// Count returns the number of runs matching the filter.
func (s *RunStore) Count(ctx context.Context, filter run.ListFilter) (int64, error) {
	query, args := s.buildCountQuery(filter)

	var count int64
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// Close closes the underlying database connection.
func (s *RunStore) Close() error {
	return s.db.Close()
}

// buildListQuery constructs the SELECT query for listing runs.
func (s *RunStore) buildListQuery(filter run.ListFilter) (string, []any) {
	whereClause, args := s.buildWhereClause(filter)

	query := `
		SELECT id, goal, current_state, vars, evidence, status, start_time, end_time, result, error
		FROM runs
		` + whereClause

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

	query += fmt.Sprintf(" ORDER BY %s %s", orderBy, direction)

	// Add LIMIT and OFFSET
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

	return query, args
}

// buildCountQuery constructs the COUNT query.
func (s *RunStore) buildCountQuery(filter run.ListFilter) (string, []any) {
	whereClause, args := s.buildWhereClause(filter)
	query := `SELECT COUNT(*) FROM runs ` + whereClause
	return query, args
}

// buildWhereClause constructs the WHERE clause from filter.
func (s *RunStore) buildWhereClause(filter run.ListFilter) (string, []any) {
	var conditions []string
	var args []any

	// Filter by status
	if len(filter.Status) > 0 {
		placeholders := make([]string, len(filter.Status))
		for i, status := range filter.Status {
			args = append(args, string(status))
			placeholders[i] = "?"
		}
		conditions = append(conditions, fmt.Sprintf("status IN (%s)", strings.Join(placeholders, ", ")))
	}

	// Filter by state
	if len(filter.States) > 0 {
		placeholders := make([]string, len(filter.States))
		for i, state := range filter.States {
			args = append(args, string(state))
			placeholders[i] = "?"
		}
		conditions = append(conditions, fmt.Sprintf("current_state IN (%s)", strings.Join(placeholders, ", ")))
	}

	// Filter by time range
	if !filter.FromTime.IsZero() {
		args = append(args, filter.FromTime)
		conditions = append(conditions, "start_time >= ?")
	}

	if !filter.ToTime.IsZero() {
		args = append(args, filter.ToTime)
		conditions = append(conditions, "start_time <= ?")
	}

	// Filter by goal pattern (SQLite LIKE is case-insensitive by default)
	if filter.GoalPattern != "" {
		args = append(args, "%"+filter.GoalPattern+"%")
		conditions = append(conditions, "goal LIKE ?")
	}

	if len(conditions) == 0 {
		return "", args
	}

	return "WHERE " + strings.Join(conditions, " AND "), args
}

// scanRun scans a row into a Run struct.
func (s *RunStore) scanRun(rows *sql.Rows) (*agent.Run, error) {
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

// Ensure interfaces are implemented.
var (
	_ cache.Cache = (*Cache)(nil)
	_ event.Store = (*EventStore)(nil)
	_ run.Store   = (*RunStore)(nil)
)
