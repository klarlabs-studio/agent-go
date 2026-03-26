package sqlite

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
)

func TestMigrator_EnsureSchema(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	m := NewMigrator(db)
	ctx := context.Background()

	err := m.EnsureSchema(ctx)
	if err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// Verify table exists.
	var count int
	err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='schema_migrations'").Scan(&count)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected schema_migrations table, got count %d", count)
	}
}

func TestMigrator_AutoMigrate(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	m := NewMigrator(db)
	ctx := context.Background()

	err := m.AutoMigrate(ctx)
	if err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	// Verify all default tables were created.
	tables := []string{"cache_entries", "events", "runs", "knowledge_vectors"}
	for _, table := range tables {
		var count int
		err := db.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", table,
		).Scan(&count)
		if err != nil {
			t.Fatalf("query for table %s failed: %v", table, err)
		}
		if count != 1 {
			t.Errorf("expected table %s to exist", table)
		}
	}

	// Verify migration records.
	applied, err := m.Applied(ctx)
	if err != nil {
		t.Fatalf("Applied failed: %v", err)
	}
	if len(applied) != 4 {
		t.Fatalf("expected 4 applied migrations, got %d", len(applied))
	}

	// Running again should be a no-op.
	err = m.AutoMigrate(ctx)
	if err != nil {
		t.Fatalf("second AutoMigrate failed: %v", err)
	}

	applied, err = m.Applied(ctx)
	if err != nil {
		t.Fatalf("Applied failed: %v", err)
	}
	if len(applied) != 4 {
		t.Fatalf("expected 4 applied migrations after re-run, got %d", len(applied))
	}
}

func TestMigrator_UpDown(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	m := NewMigrator(db)
	ctx := context.Background()

	// Migrate up to version 2 only.
	err := m.Up(ctx, 2)
	if err != nil {
		t.Fatalf("Up(2) failed: %v", err)
	}

	version, err := m.CurrentVersion(ctx)
	if err != nil {
		t.Fatalf("CurrentVersion failed: %v", err)
	}
	if version != 2 {
		t.Errorf("expected version 2, got %d", version)
	}

	// Verify only cache_entries and events exist.
	var count int
	err = db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='cache_entries'",
	).Scan(&count)
	if err != nil || count != 1 {
		t.Error("expected cache_entries table")
	}

	err = db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='events'",
	).Scan(&count)
	if err != nil || count != 1 {
		t.Error("expected events table")
	}

	err = db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='runs'",
	).Scan(&count)
	if err != nil || count != 0 {
		t.Error("expected runs table to NOT exist yet")
	}

	// Migrate down to version 1 (revert version 2).
	err = m.Down(ctx, 1)
	if err != nil {
		t.Fatalf("Down(1) failed: %v", err)
	}

	version, err = m.CurrentVersion(ctx)
	if err != nil {
		t.Fatalf("CurrentVersion failed: %v", err)
	}
	if version != 1 {
		t.Errorf("expected version 1 after down, got %d", version)
	}

	// Events table should be dropped.
	err = db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='events'",
	).Scan(&count)
	if err != nil || count != 0 {
		t.Error("expected events table to be dropped after down migration")
	}

	// Cache entries should still exist.
	err = db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='cache_entries'",
	).Scan(&count)
	if err != nil || count != 1 {
		t.Error("expected cache_entries table to still exist")
	}
}

func TestMigrator_DownAll(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	m := NewMigrator(db)
	ctx := context.Background()

	// Apply all.
	err := m.AutoMigrate(ctx)
	if err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	// Revert all.
	err = m.Down(ctx, 0)
	if err != nil {
		t.Fatalf("Down(0) failed: %v", err)
	}

	version, err := m.CurrentVersion(ctx)
	if err != nil {
		t.Fatalf("CurrentVersion failed: %v", err)
	}
	if version != 0 {
		t.Errorf("expected version 0 after full rollback, got %d", version)
	}

	applied, err := m.Applied(ctx)
	if err != nil {
		t.Fatalf("Applied failed: %v", err)
	}
	if len(applied) != 0 {
		t.Errorf("expected 0 applied migrations after full rollback, got %d", len(applied))
	}
}

func TestMigrator_Pending(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	m := NewMigrator(db)
	ctx := context.Background()

	if err := m.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	// All migrations should be pending initially.
	pending, err := m.Pending(ctx)
	if err != nil {
		t.Fatalf("Pending failed: %v", err)
	}
	if len(pending) != 4 {
		t.Fatalf("expected 4 pending migrations, got %d", len(pending))
	}

	// Apply first two.
	if err := m.Up(ctx, 2); err != nil {
		t.Fatalf("Up(2) failed: %v", err)
	}

	pending, err = m.Pending(ctx)
	if err != nil {
		t.Fatalf("Pending failed: %v", err)
	}
	if len(pending) != 2 {
		t.Fatalf("expected 2 pending migrations, got %d", len(pending))
	}
	if pending[0].Version != 3 {
		t.Errorf("expected first pending to be version 3, got %d", pending[0].Version)
	}
}

func TestMigrator_CurrentVersion_Empty(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	m := NewMigrator(db)
	ctx := context.Background()

	if err := m.EnsureSchema(ctx); err != nil {
		t.Fatalf("EnsureSchema failed: %v", err)
	}

	version, err := m.CurrentVersion(ctx)
	if err != nil {
		t.Fatalf("CurrentVersion failed: %v", err)
	}
	if version != 0 {
		t.Errorf("expected version 0 for empty migrations, got %d", version)
	}
}

func TestMigrator_CustomMigrations(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	custom := []Migration{
		{
			Version:     1,
			Description: "Create custom table",
			Up: func(ctx context.Context, tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, `CREATE TABLE custom (id TEXT PRIMARY KEY)`)
				return err
			},
			Down: func(ctx context.Context, tx *sql.Tx) error {
				_, err := tx.ExecContext(ctx, `DROP TABLE IF EXISTS custom`)
				return err
			},
		},
	}

	m := NewMigratorWithMigrations(db, custom)
	ctx := context.Background()

	err := m.AutoMigrate(ctx)
	if err != nil {
		t.Fatalf("AutoMigrate with custom migrations failed: %v", err)
	}

	// Verify custom table exists.
	var count int
	err = db.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='custom'",
	).Scan(&count)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if count != 1 {
		t.Error("expected custom table to exist")
	}
}

func TestMigrator_UpZeroAppliesAll(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	m := NewMigrator(db)
	ctx := context.Background()

	// Up with targetVersion 0 should apply all.
	err := m.Up(ctx, 0)
	if err != nil {
		t.Fatalf("Up(0) failed: %v", err)
	}

	applied, err := m.Applied(ctx)
	if err != nil {
		t.Fatalf("Applied failed: %v", err)
	}
	if len(applied) != 4 {
		t.Errorf("expected 4 applied migrations, got %d", len(applied))
	}
}

func TestMigrator_DownMissingMigration(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// Apply with full set.
	m := NewMigrator(db)
	ctx := context.Background()

	if err := m.AutoMigrate(ctx); err != nil {
		t.Fatalf("AutoMigrate failed: %v", err)
	}

	// Create new migrator with only migration 1, try to roll back all.
	partial := NewMigratorWithMigrations(db, []Migration{
		DefaultMigrations()[0], // Only migration 1.
	})

	err := partial.Down(ctx, 0)
	if err == nil {
		t.Fatal("expected error when rolling back missing migration")
	}
	expected := fmt.Sprintf("migration %d not found in registry", 4)
	if err.Error() != expected {
		t.Logf("got error: %v", err)
	}
}
