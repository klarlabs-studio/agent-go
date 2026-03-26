package sqlite

import (
	"database/sql"
	"fmt"
	"time"
)

// Config holds configuration for the SQLite database connection pool.
type Config struct {
	// DSN is the data source name (e.g., "file:agent.db", ":memory:").
	DSN string

	// MaxOpenConns is the maximum number of open connections to the database.
	// Default: 1 (recommended for SQLite to avoid SQLITE_BUSY errors).
	MaxOpenConns int

	// MaxIdleConns is the maximum number of idle connections in the pool.
	// Default: 1.
	MaxIdleConns int

	// ConnMaxLifetime is the maximum amount of time a connection may be reused.
	// Default: 0 (connections are reused forever).
	ConnMaxLifetime time.Duration

	// ConnMaxIdleTime is the maximum amount of time a connection may be idle.
	// Default: 0 (connections are not closed due to idle time).
	ConnMaxIdleTime time.Duration

	// JournalMode sets the SQLite journal mode (e.g., "WAL", "DELETE").
	// Default: "WAL" (recommended for concurrent read performance).
	JournalMode string

	// BusyTimeout sets the busy timeout in milliseconds.
	// Default: 5000 (5 seconds).
	BusyTimeout int

	// ForeignKeys enables foreign key constraint enforcement.
	// Default: true.
	ForeignKeys bool
}

// DefaultConfig returns a Config with sensible defaults for SQLite.
// SQLite works best with a single writer connection and WAL mode for
// concurrent reads.
func DefaultConfig(dsn string) Config {
	return Config{
		DSN:             dsn,
		MaxOpenConns:    1,
		MaxIdleConns:    1,
		ConnMaxLifetime: 0,
		ConnMaxIdleTime: 0,
		JournalMode:     "WAL",
		BusyTimeout:     5000,
		ForeignKeys:     true,
	}
}

// Open creates and configures a database connection using the provided Config.
// It applies connection pool settings and SQLite pragmas.
func Open(cfg Config) (*sql.DB, error) {
	if cfg.DSN == "" {
		return nil, fmt.Errorf("sqlite: DSN is required")
	}

	db, err := sql.Open("sqlite", cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("sqlite: open: %w", err)
	}

	// Apply connection pool settings.
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if cfg.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	}

	// Apply SQLite pragmas for optimal performance.
	if err := applyPragmas(db, cfg); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite: apply pragmas: %w", err)
	}

	// Verify the connection is working.
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("sqlite: ping: %w", err)
	}

	return db, nil
}

// applyPragmas sets SQLite-specific configuration via PRAGMA statements.
func applyPragmas(db *sql.DB, cfg Config) error {
	if cfg.JournalMode != "" {
		if _, err := db.Exec("PRAGMA journal_mode = " + cfg.JournalMode); err != nil {
			return fmt.Errorf("journal_mode: %w", err)
		}
	}

	if cfg.BusyTimeout > 0 {
		if _, err := db.Exec(fmt.Sprintf("PRAGMA busy_timeout = %d", cfg.BusyTimeout)); err != nil {
			return fmt.Errorf("busy_timeout: %w", err)
		}
	}

	if cfg.ForeignKeys {
		if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
			return fmt.Errorf("foreign_keys: %w", err)
		}
	}

	return nil
}
