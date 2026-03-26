package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// Migration represents a versioned schema migration.
type Migration struct {
	// Version is the unique, monotonically increasing migration identifier.
	Version int

	// Description is a human-readable description of the migration.
	Description string

	// Up applies the migration.
	Up func(ctx context.Context, tx *sql.Tx) error

	// Down reverts the migration.
	Down func(ctx context.Context, tx *sql.Tx) error
}

// MigrationRecord tracks an applied migration.
type MigrationRecord struct {
	Version   int
	AppliedAt time.Time
}

// Migrator manages version-tracked schema migrations for SQLite.
type Migrator struct {
	db         *sql.DB
	migrations []Migration
}

// NewMigrator creates a new migrator with the given database connection.
func NewMigrator(db *sql.DB) *Migrator {
	return &Migrator{
		db:         db,
		migrations: DefaultMigrations(),
	}
}

// NewMigratorWithMigrations creates a new migrator with custom migrations.
// This is useful for extending the default migrations with application-specific ones.
func NewMigratorWithMigrations(db *sql.DB, migrations []Migration) *Migrator {
	return &Migrator{
		db:         db,
		migrations: migrations,
	}
}

// DefaultMigrations returns the built-in migrations for all SQLite storage tables.
func DefaultMigrations() []Migration {
	return []Migration{
		{
			Version:     1,
			Description: "Create cache_entries table",
			Up: func(ctx context.Context, tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, `
					CREATE TABLE IF NOT EXISTS cache_entries (
						key TEXT PRIMARY KEY,
						value BLOB NOT NULL,
						expires_at INTEGER
					);`)
				return err
			},
			Down: func(ctx context.Context, tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS cache_entries;`)
				return err
			},
		},
		{
			Version:     2,
			Description: "Create events table",
			Up: func(ctx context.Context, tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, `
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
					CREATE INDEX IF NOT EXISTS idx_events_run_seq ON events(run_id, sequence);`)
				return err
			},
			Down: func(ctx context.Context, tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS events;`)
				return err
			},
		},
		{
			Version:     3,
			Description: "Create runs table",
			Up: func(ctx context.Context, tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, `
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
					CREATE INDEX IF NOT EXISTS idx_runs_start_time ON runs(start_time);`)
				return err
			},
			Down: func(ctx context.Context, tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS runs;`)
				return err
			},
		},
		{
			Version:     4,
			Description: "Create knowledge_vectors table",
			Up: func(ctx context.Context, tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, `
					CREATE TABLE IF NOT EXISTS knowledge_vectors (
						id TEXT PRIMARY KEY,
						embedding BLOB NOT NULL,
						text TEXT NOT NULL,
						metadata TEXT,
						created_at DATETIME NOT NULL
					);
					CREATE INDEX IF NOT EXISTS idx_knowledge_vectors_created_at ON knowledge_vectors(created_at);`)
				return err
			},
			Down: func(ctx context.Context, tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS knowledge_vectors;`)
				return err
			},
		},
	}
}

// EnsureSchema creates the schema_migrations tracking table.
func (m *Migrator) EnsureSchema(ctx context.Context) error {
	query := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version INTEGER PRIMARY KEY,
		description TEXT NOT NULL,
		applied_at DATETIME NOT NULL
	);`
	_, err := m.db.ExecContext(ctx, query)
	return err
}

// AutoMigrate applies all pending migrations in order.
// It creates the schema_migrations table if needed, then runs any migrations
// that have not yet been applied.
func (m *Migrator) AutoMigrate(ctx context.Context) error {
	if err := m.EnsureSchema(ctx); err != nil {
		return fmt.Errorf("ensure migration schema: %w", err)
	}

	applied, err := m.Applied(ctx)
	if err != nil {
		return fmt.Errorf("load applied migrations: %w", err)
	}

	appliedSet := make(map[int]bool, len(applied))
	for _, r := range applied {
		appliedSet[r.Version] = true
	}

	// Sort migrations by version.
	sorted := make([]Migration, len(m.migrations))
	copy(sorted, m.migrations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Version < sorted[j].Version
	})

	for _, migration := range sorted {
		if appliedSet[migration.Version] {
			continue
		}

		if err := m.applyUp(ctx, migration); err != nil {
			return fmt.Errorf("migration %d (%s): %w", migration.Version, migration.Description, err)
		}
	}

	return nil
}

// Up applies migrations up to and including the target version.
// If targetVersion is 0, all pending migrations are applied.
func (m *Migrator) Up(ctx context.Context, targetVersion int) error {
	if err := m.EnsureSchema(ctx); err != nil {
		return fmt.Errorf("ensure migration schema: %w", err)
	}

	applied, err := m.Applied(ctx)
	if err != nil {
		return err
	}

	appliedSet := make(map[int]bool, len(applied))
	for _, r := range applied {
		appliedSet[r.Version] = true
	}

	sorted := make([]Migration, len(m.migrations))
	copy(sorted, m.migrations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Version < sorted[j].Version
	})

	for _, migration := range sorted {
		if targetVersion > 0 && migration.Version > targetVersion {
			break
		}
		if appliedSet[migration.Version] {
			continue
		}
		if err := m.applyUp(ctx, migration); err != nil {
			return fmt.Errorf("migration %d (%s): %w", migration.Version, migration.Description, err)
		}
	}

	return nil
}

// Down reverts migrations down to (but not including) the target version.
// If targetVersion is 0, all migrations are reverted.
func (m *Migrator) Down(ctx context.Context, targetVersion int) error {
	if err := m.EnsureSchema(ctx); err != nil {
		return fmt.Errorf("ensure migration schema: %w", err)
	}

	applied, err := m.Applied(ctx)
	if err != nil {
		return err
	}

	appliedSet := make(map[int]bool, len(applied))
	for _, r := range applied {
		appliedSet[r.Version] = true
	}

	// Build a map of migrations by version for lookup.
	migrationMap := make(map[int]Migration, len(m.migrations))
	for _, migration := range m.migrations {
		migrationMap[migration.Version] = migration
	}

	// Sort applied migrations in reverse order for rollback.
	sort.Slice(applied, func(i, j int) bool {
		return applied[i].Version > applied[j].Version
	})

	for _, record := range applied {
		if record.Version <= targetVersion {
			break
		}

		migration, ok := migrationMap[record.Version]
		if !ok {
			return fmt.Errorf("migration %d not found in registry", record.Version)
		}

		if migration.Down == nil {
			return fmt.Errorf("migration %d has no down function", record.Version)
		}

		if err := m.applyDown(ctx, migration); err != nil {
			return fmt.Errorf("rollback migration %d (%s): %w", migration.Version, migration.Description, err)
		}
	}

	return nil
}

// Applied returns all applied migration records.
func (m *Migrator) Applied(ctx context.Context) ([]MigrationRecord, error) {
	query := `SELECT version, applied_at FROM schema_migrations ORDER BY version ASC`

	rows, err := m.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []MigrationRecord
	for rows.Next() {
		var r MigrationRecord
		if err := rows.Scan(&r.Version, &r.AppliedAt); err != nil {
			return nil, err
		}
		records = append(records, r)
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
	for _, r := range applied {
		appliedSet[r.Version] = true
	}

	var pending []Migration
	for _, migration := range m.migrations {
		if !appliedSet[migration.Version] {
			pending = append(pending, migration)
		}
	}

	sort.Slice(pending, func(i, j int) bool {
		return pending[i].Version < pending[j].Version
	})

	return pending, nil
}

// CurrentVersion returns the highest applied migration version, or 0 if none applied.
func (m *Migrator) CurrentVersion(ctx context.Context) (int, error) {
	var version *int
	err := m.db.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&version)
	if err != nil {
		return 0, err
	}
	if version == nil {
		return 0, nil
	}
	return *version, nil
}

// applyUp runs a migration's Up function within a transaction and records it.
func (m *Migrator) applyUp(ctx context.Context, migration Migration) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := migration.Up(ctx, tx); err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, description, applied_at) VALUES (?, ?, ?)`,
		migration.Version, migration.Description, time.Now(),
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// applyDown runs a migration's Down function within a transaction and removes the record.
func (m *Migrator) applyDown(ctx context.Context, migration Migration) error {
	tx, err := m.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if err := migration.Down(ctx, tx); err != nil {
		return err
	}

	_, err = tx.ExecContext(ctx,
		`DELETE FROM schema_migrations WHERE version = ?`,
		migration.Version,
	)
	if err != nil {
		return err
	}

	return tx.Commit()
}
