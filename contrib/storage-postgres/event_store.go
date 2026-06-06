package postgres

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.klarlabs.de/agent/domain/event"
)

// EventStore is a PostgreSQL-backed implementation of event.Store.
type EventStore struct {
	pool        *pgxpool.Pool
	schema      string
	subscribers map[string][]chan event.Event
	mu          sync.RWMutex
}

// NewEventStore creates a new PostgreSQL event store.
func NewEventStore(pool *pgxpool.Pool, schema string) *EventStore {
	if schema == "" {
		schema = "public"
	}
	return &EventStore{
		pool:        pool,
		schema:      schema,
		subscribers: make(map[string][]chan event.Event),
	}
}

// tableName returns the fully qualified table name.
func (s *EventStore) tableName() string {
	return fmt.Sprintf("%s.events", s.schema)
}

// snapshotTableName returns the fully qualified snapshot table name.
func (s *EventStore) snapshotTableName() string {
	return fmt.Sprintf("%s.snapshots", s.schema)
}

// Append persists one or more events atomically.
func (s *EventStore) Append(ctx context.Context, events ...event.Event) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return s.wrapError(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// Get current max sequence for each run
	sequences := make(map[string]uint64)
	for _, e := range events {
		if _, ok := sequences[e.RunID]; !ok {
			var maxSeq *uint64
			err := tx.QueryRow(ctx,
				fmt.Sprintf("SELECT MAX(sequence) FROM %s WHERE run_id = $1", s.tableName()),
				e.RunID,
			).Scan(&maxSeq)
			if err != nil && !errors.Is(err, pgx.ErrNoRows) {
				return s.wrapError(err)
			}
			if maxSeq != nil {
				sequences[e.RunID] = *maxSeq
			}
		}
	}

	// Insert events with sequence numbers
	insertQuery := fmt.Sprintf(`
		INSERT INTO %s (id, run_id, type, timestamp, payload, sequence, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, s.tableName())

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

		_, err := tx.Exec(ctx, insertQuery,
			events[i].ID,
			events[i].RunID,
			string(events[i].Type),
			events[i].Timestamp,
			events[i].Payload,
			events[i].Sequence,
			events[i].Version,
		)
		if err != nil {
			return s.wrapError(err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return s.wrapError(err)
	}

	// Notify subscribers
	s.notifySubscribers(events)

	return nil
}

// LoadEvents retrieves all events for a run in sequence order.
func (s *EventStore) LoadEvents(ctx context.Context, runID string) ([]event.Event, error) {
	query := fmt.Sprintf(`
		SELECT id, run_id, type, timestamp, payload, sequence, version
		FROM %s
		WHERE run_id = $1
		ORDER BY sequence ASC
	`, s.tableName())

	rows, err := s.pool.Query(ctx, query, runID)
	if err != nil {
		return nil, s.wrapError(err)
	}
	defer rows.Close()

	return s.scanEvents(rows)
}

// LoadEventsFrom retrieves events starting from a specific sequence number.
func (s *EventStore) LoadEventsFrom(ctx context.Context, runID string, fromSeq uint64) ([]event.Event, error) {
	query := fmt.Sprintf(`
		SELECT id, run_id, type, timestamp, payload, sequence, version
		FROM %s
		WHERE run_id = $1 AND sequence >= $2
		ORDER BY sequence ASC
	`, s.tableName())

	rows, err := s.pool.Query(ctx, query, runID, fromSeq)
	if err != nil {
		return nil, s.wrapError(err)
	}
	defer rows.Close()

	return s.scanEvents(rows)
}

// Subscribe returns a channel that receives new events for a run.
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

// Query retrieves events matching the given options.
func (s *EventStore) Query(ctx context.Context, runID string, opts event.QueryOptions) ([]event.Event, error) {
	query, args := s.buildQuerySQL(runID, opts)

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, s.wrapError(err)
	}
	defer rows.Close()

	return s.scanEvents(rows)
}

// CountEvents returns the number of events for a run.
func (s *EventStore) CountEvents(ctx context.Context, runID string) (int64, error) {
	query := fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE run_id = $1`, s.tableName())

	var count int64
	err := s.pool.QueryRow(ctx, query, runID).Scan(&count)
	if err != nil {
		return 0, s.wrapError(err)
	}

	return count, nil
}

// ListRuns returns all run IDs with events in the store.
func (s *EventStore) ListRuns(ctx context.Context) ([]string, error) {
	query := fmt.Sprintf(`SELECT DISTINCT run_id FROM %s ORDER BY run_id`, s.tableName())

	rows, err := s.pool.Query(ctx, query)
	if err != nil {
		return nil, s.wrapError(err)
	}
	defer rows.Close()

	var runs []string
	for rows.Next() {
		var runID string
		if err := rows.Scan(&runID); err != nil {
			return nil, s.wrapError(err)
		}
		runs = append(runs, runID)
	}

	return runs, rows.Err()
}

// SaveSnapshot persists a snapshot of run state at a sequence number.
func (s *EventStore) SaveSnapshot(ctx context.Context, runID string, sequence uint64, data []byte) error {
	query := fmt.Sprintf(`
		INSERT INTO %s (run_id, sequence, data)
		VALUES ($1, $2, $3)
		ON CONFLICT (run_id) DO UPDATE SET sequence = $2, data = $3, created_at = NOW()
	`, s.snapshotTableName())

	_, err := s.pool.Exec(ctx, query, runID, sequence, data)
	if err != nil {
		return s.wrapError(err)
	}

	return nil
}

// LoadSnapshot retrieves the latest snapshot for a run.
func (s *EventStore) LoadSnapshot(ctx context.Context, runID string) ([]byte, uint64, error) {
	query := fmt.Sprintf(`
		SELECT data, sequence FROM %s WHERE run_id = $1
	`, s.snapshotTableName())

	var data []byte
	var sequence uint64

	err := s.pool.QueryRow(ctx, query, runID).Scan(&data, &sequence)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, 0, event.ErrSnapshotNotFound
		}
		return nil, 0, s.wrapError(err)
	}

	return data, sequence, nil
}

// PruneEvents removes events before a sequence number.
func (s *EventStore) PruneEvents(ctx context.Context, runID string, beforeSeq uint64) error {
	query := fmt.Sprintf(`
		DELETE FROM %s WHERE run_id = $1 AND sequence < $2
	`, s.tableName())

	_, err := s.pool.Exec(ctx, query, runID, beforeSeq)
	if err != nil {
		return s.wrapError(err)
	}

	return nil
}

// buildQuerySQL constructs the SELECT query for querying events.
func (s *EventStore) buildQuerySQL(runID string, opts event.QueryOptions) (string, []any) {
	args := []any{runID}
	argNum := 1
	conditions := []string{"run_id = $1"}

	// Filter by types
	if len(opts.Types) > 0 {
		argNum++
		types := make([]string, len(opts.Types))
		for i, t := range opts.Types {
			types[i] = string(t)
		}
		args = append(args, types)
		conditions = append(conditions, fmt.Sprintf("type = ANY($%d)", argNum))
	}

	// Filter by time range
	if opts.FromTime > 0 {
		argNum++
		args = append(args, opts.FromTime)
		conditions = append(conditions, fmt.Sprintf("EXTRACT(EPOCH FROM timestamp) >= $%d", argNum))
	}

	if opts.ToTime > 0 {
		argNum++
		args = append(args, opts.ToTime)
		conditions = append(conditions, fmt.Sprintf("EXTRACT(EPOCH FROM timestamp) <= $%d", argNum))
	}

	query := fmt.Sprintf(`
		SELECT id, run_id, type, timestamp, payload, sequence, version
		FROM %s
		WHERE %s
		ORDER BY sequence ASC
	`, s.tableName(), joinConditions(conditions))

	// Add LIMIT and OFFSET
	if opts.Limit > 0 {
		argNum++
		args = append(args, opts.Limit)
		query += fmt.Sprintf(" LIMIT $%d", argNum)
	}

	if opts.Offset > 0 {
		argNum++
		args = append(args, opts.Offset)
		query += fmt.Sprintf(" OFFSET $%d", argNum)
	}

	return query, args
}

// joinConditions joins SQL conditions with AND.
func joinConditions(conditions []string) string {
	result := ""
	for i, c := range conditions {
		if i > 0 {
			result += " AND "
		}
		result += c
	}
	return result
}

// scanEvents scans rows into Event structs.
func (s *EventStore) scanEvents(rows pgx.Rows) ([]event.Event, error) {
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

// wrapError wraps database errors with domain errors.
func (s *EventStore) wrapError(err error) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return errors.Join(event.ErrOperationTimeout, err)
	}

	return errors.Join(event.ErrConnectionFailed, err)
}

// Ensure EventStore implements event.Store, event.Querier, event.Snapshotter, event.Pruner
var (
	_ event.Store       = (*EventStore)(nil)
	_ event.Querier     = (*EventStore)(nil)
	_ event.Snapshotter = (*EventStore)(nil)
	_ event.Pruner      = (*EventStore)(nil)
)
