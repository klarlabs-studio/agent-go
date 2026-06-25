package postgres

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Migration represents a versioned schema migration.
type Migration struct {
	// Version is the unique monotonically increasing migration version.
	Version int

	// Description is a human-readable description of the migration.
	Description string

	// Up is the SQL to apply the migration.
	Up string

	// Down is the SQL to revert the migration.
	Down string
}

// MigrationRecord represents a row in the schema_migrations table.
type MigrationRecord struct {
	Version   int       `json:"version"`
	AppliedAt time.Time `json:"applied_at"`
}

// Migrator manages schema migrations for the PostgreSQL storage.
type Migrator struct {
	pool       *pgxpool.Pool
	schema     string
	migrations []Migration
}

// NewMigrator creates a new Migrator with the built-in migrations.
func NewMigrator(pool *pgxpool.Pool, schema string) *Migrator {
	if schema == "" {
		schema = "public"
	}
	return &Migrator{
		pool:       pool,
		schema:     schema,
		migrations: builtinMigrations(),
	}
}

// migrationsTableName returns the fully qualified migrations tracking table name.
func (m *Migrator) migrationsTableName() string {
	return fmt.Sprintf("%s.schema_migrations", m.schema)
}

// AutoMigrate applies all pending migrations in order.
// This is safe to call on every startup; already-applied migrations are skipped.
func (m *Migrator) AutoMigrate(ctx context.Context) error {
	if err := m.ensureMigrationsTable(ctx); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}

	applied, err := m.Applied(ctx)
	if err != nil {
		return fmt.Errorf("list applied migrations: %w", err)
	}

	appliedSet := make(map[int]bool, len(applied))
	for _, rec := range applied {
		appliedSet[rec.Version] = true
	}

	// Sort migrations by version ascending.
	sorted := make([]Migration, len(m.migrations))
	copy(sorted, m.migrations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Version < sorted[j].Version
	})

	for _, mig := range sorted {
		if appliedSet[mig.Version] {
			continue
		}
		if err := m.applyUp(ctx, mig); err != nil {
			return fmt.Errorf("apply migration %d (%s): %w", mig.Version, mig.Description, err)
		}
	}

	return nil
}

// Up applies a single migration by version.
func (m *Migrator) Up(ctx context.Context, version int) error {
	if err := m.ensureMigrationsTable(ctx); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}

	mig, ok := m.findMigration(version)
	if !ok {
		return fmt.Errorf("migration version %d not found", version)
	}

	return m.applyUp(ctx, mig)
}

// Down reverts a single migration by version.
func (m *Migrator) Down(ctx context.Context, version int) error {
	if err := m.ensureMigrationsTable(ctx); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}

	mig, ok := m.findMigration(version)
	if !ok {
		return fmt.Errorf("migration version %d not found", version)
	}

	if mig.Down == "" {
		return fmt.Errorf("migration version %d has no down SQL", version)
	}

	return m.applyDown(ctx, mig)
}

// Applied returns all applied migrations in version order.
func (m *Migrator) Applied(ctx context.Context) ([]MigrationRecord, error) {
	if err := m.ensureMigrationsTable(ctx); err != nil {
		return nil, fmt.Errorf("ensure migrations table: %w", err)
	}

	query := fmt.Sprintf(
		`SELECT version, applied_at FROM %s ORDER BY version ASC`,
		m.migrationsTableName(),
	)

	rows, err := m.pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query applied migrations: %w", err)
	}
	defer rows.Close()

	var records []MigrationRecord
	for rows.Next() {
		var rec MigrationRecord
		if err := rows.Scan(&rec.Version, &rec.AppliedAt); err != nil {
			return nil, fmt.Errorf("scan migration record: %w", err)
		}
		records = append(records, rec)
	}

	return records, rows.Err()
}

// Pending returns migrations that have not yet been applied.
func (m *Migrator) Pending(ctx context.Context) ([]Migration, error) {
	applied, err := m.Applied(ctx)
	if err != nil {
		return nil, err
	}

	appliedSet := make(map[int]bool, len(applied))
	for _, rec := range applied {
		appliedSet[rec.Version] = true
	}

	var pending []Migration
	for _, mig := range m.migrations {
		if !appliedSet[mig.Version] {
			pending = append(pending, mig)
		}
	}

	sort.Slice(pending, func(i, j int) bool {
		return pending[i].Version < pending[j].Version
	})

	return pending, nil
}

// ensureMigrationsTable creates the schema_migrations table if it does not exist.
func (m *Migrator) ensureMigrationsTable(ctx context.Context) error {
	// Create the target schema first so a non-public schema works without the
	// caller pre-creating it (the tracking table + migration DDL are schema-bound).
	if _, err := m.pool.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+m.quotedSchema()); err != nil {
		return fmt.Errorf("ensure schema: %w", err)
	}
	query := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			version    INTEGER PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`, m.migrationsTableName())

	_, err := m.pool.Exec(ctx, query)
	return err
}

// quotedSchema returns the schema as a sanitized SQL identifier.
func (m *Migrator) quotedSchema() string {
	return pgx.Identifier{m.schema}.Sanitize()
}

// setSearchPath scopes unqualified DDL in the migration to the target schema, so
// CREATE TABLE in a migration lands in m.schema (not whatever the session default
// search_path is — typically public).
func (m *Migrator) setSearchPath(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, "SET LOCAL search_path TO "+m.quotedSchema())
	return err
}

// applyUp runs a migration's Up SQL and records it in the migrations table.
func (m *Migrator) applyUp(ctx context.Context, mig Migration) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := m.setSearchPath(ctx, tx); err != nil {
		return fmt.Errorf("set search_path: %w", err)
	}
	if _, err := tx.Exec(ctx, mig.Up); err != nil {
		return fmt.Errorf("execute up SQL: %w", err)
	}

	insertQuery := fmt.Sprintf(
		`INSERT INTO %s (version) VALUES ($1)`,
		m.migrationsTableName(),
	)
	if _, err := tx.Exec(ctx, insertQuery, mig.Version); err != nil {
		return fmt.Errorf("record migration: %w", err)
	}

	return tx.Commit(ctx)
}

// applyDown runs a migration's Down SQL and removes it from the migrations table.
func (m *Migrator) applyDown(ctx context.Context, mig Migration) error {
	tx, err := m.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if err := m.setSearchPath(ctx, tx); err != nil {
		return fmt.Errorf("set search_path: %w", err)
	}
	if _, err := tx.Exec(ctx, mig.Down); err != nil {
		return fmt.Errorf("execute down SQL: %w", err)
	}

	deleteQuery := fmt.Sprintf(
		`DELETE FROM %s WHERE version = $1`,
		m.migrationsTableName(),
	)
	if _, err := tx.Exec(ctx, deleteQuery, mig.Version); err != nil {
		return fmt.Errorf("remove migration record: %w", err)
	}

	return tx.Commit(ctx)
}

// findMigration returns the migration with the given version.
func (m *Migrator) findMigration(version int) (Migration, bool) {
	for _, mig := range m.migrations {
		if mig.Version == version {
			return mig, true
		}
	}
	return Migration{}, false
}

// AddMigration registers a custom migration. Migrations added here will be
// included alongside the built-in ones during AutoMigrate.
func (m *Migrator) AddMigration(mig Migration) {
	m.migrations = append(m.migrations, mig)
}

// builtinMigrations returns the initial set of schema migrations for all tables.
func builtinMigrations() []Migration {
	return []Migration{
		{
			Version:     1,
			Description: "create runs table",
			Up: `
				CREATE TABLE IF NOT EXISTS runs (
					id            TEXT PRIMARY KEY,
					goal          TEXT NOT NULL,
					current_state TEXT NOT NULL,
					vars          JSONB NOT NULL DEFAULT '{}',
					evidence      JSONB NOT NULL DEFAULT '[]',
					status        TEXT NOT NULL DEFAULT 'pending',
					start_time    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
					end_time      TIMESTAMPTZ,
					result        BYTEA,
					error         TEXT
				);

				CREATE INDEX IF NOT EXISTS idx_runs_status ON runs (status);
				CREATE INDEX IF NOT EXISTS idx_runs_current_state ON runs (current_state);
				CREATE INDEX IF NOT EXISTS idx_runs_start_time ON runs (start_time);
			`,
			Down: `DROP TABLE IF EXISTS runs;`,
		},
		{
			Version:     2,
			Description: "create events and snapshots tables",
			Up: `
				CREATE TABLE IF NOT EXISTS events (
					id       TEXT PRIMARY KEY,
					run_id   TEXT NOT NULL,
					type     TEXT NOT NULL,
					timestamp TIMESTAMPTZ NOT NULL DEFAULT NOW(),
					payload  JSONB,
					sequence BIGINT NOT NULL,
					version  INTEGER NOT NULL DEFAULT 1,
					UNIQUE (run_id, sequence)
				);

				CREATE INDEX IF NOT EXISTS idx_events_run_id ON events (run_id);
				CREATE INDEX IF NOT EXISTS idx_events_type ON events (type);
				CREATE INDEX IF NOT EXISTS idx_events_run_id_sequence ON events (run_id, sequence);

				CREATE TABLE IF NOT EXISTS snapshots (
					run_id     TEXT PRIMARY KEY,
					sequence   BIGINT NOT NULL,
					data       BYTEA NOT NULL,
					created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
				);
			`,
			Down: `
				DROP TABLE IF EXISTS snapshots;
				DROP TABLE IF EXISTS events;
			`,
		},
		{
			Version:     3,
			Description: "create knowledge_vectors table",
			Up: `
				CREATE TABLE IF NOT EXISTS knowledge_vectors (
					id         TEXT PRIMARY KEY,
					embedding  REAL[] NOT NULL,
					text       TEXT NOT NULL DEFAULT '',
					metadata   JSONB NOT NULL DEFAULT '{}',
					created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
				);

				CREATE INDEX IF NOT EXISTS idx_knowledge_vectors_created_at
					ON knowledge_vectors (created_at);
			`,
			Down: `DROP TABLE IF EXISTS knowledge_vectors;`,
		},
	}
}

// MigrateSchema is a convenience function that creates a Migrator and runs
// AutoMigrate. It is intended for use during store initialization.
func MigrateSchema(ctx context.Context, pool *pgxpool.Pool, schema string) error {
	migrator := NewMigrator(pool, schema)
	return migrator.AutoMigrate(ctx)
}
