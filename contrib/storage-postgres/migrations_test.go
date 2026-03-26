package postgres

import (
	"sort"
	"testing"
)

func TestBuiltinMigrations_VersionsAreUnique(t *testing.T) {
	migrations := builtinMigrations()
	seen := make(map[int]bool)

	for _, m := range migrations {
		if seen[m.Version] {
			t.Errorf("duplicate migration version: %d", m.Version)
		}
		seen[m.Version] = true
	}
}

func TestBuiltinMigrations_VersionsArePositive(t *testing.T) {
	migrations := builtinMigrations()

	for _, m := range migrations {
		if m.Version <= 0 {
			t.Errorf("migration version must be positive, got %d", m.Version)
		}
	}
}

func TestBuiltinMigrations_AllHaveDescription(t *testing.T) {
	migrations := builtinMigrations()

	for _, m := range migrations {
		if m.Description == "" {
			t.Errorf("migration %d has empty description", m.Version)
		}
	}
}

func TestBuiltinMigrations_AllHaveUpSQL(t *testing.T) {
	migrations := builtinMigrations()

	for _, m := range migrations {
		if m.Up == "" {
			t.Errorf("migration %d has empty Up SQL", m.Version)
		}
	}
}

func TestBuiltinMigrations_AllHaveDownSQL(t *testing.T) {
	migrations := builtinMigrations()

	for _, m := range migrations {
		if m.Down == "" {
			t.Errorf("migration %d has empty Down SQL", m.Version)
		}
	}
}

func TestBuiltinMigrations_VersionOrder(t *testing.T) {
	migrations := builtinMigrations()

	sorted := make([]Migration, len(migrations))
	copy(sorted, migrations)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Version < sorted[j].Version
	})

	// Verify versions are sequential starting from 1.
	for i, m := range sorted {
		expected := i + 1
		if m.Version != expected {
			t.Errorf("migration at index %d has version %d, expected %d", i, m.Version, expected)
		}
	}
}

func TestBuiltinMigrations_Count(t *testing.T) {
	migrations := builtinMigrations()

	// We expect exactly 3 built-in migrations: runs, events/snapshots, knowledge_vectors.
	if len(migrations) != 3 {
		t.Errorf("expected 3 built-in migrations, got %d", len(migrations))
	}
}

func TestNewMigrator_DefaultSchema(t *testing.T) {
	m := NewMigrator(nil, "")
	if m.schema != "public" {
		t.Errorf("schema = %q, want %q", m.schema, "public")
	}
}

func TestNewMigrator_CustomSchema(t *testing.T) {
	m := NewMigrator(nil, "agent")
	if m.schema != "agent" {
		t.Errorf("schema = %q, want %q", m.schema, "agent")
	}
}

func TestMigrator_migrationsTableName(t *testing.T) {
	tests := []struct {
		name     string
		schema   string
		expected string
	}{
		{"default", "public", "public.schema_migrations"},
		{"custom", "myschema", "myschema.schema_migrations"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMigrator(nil, tt.schema)
			got := m.migrationsTableName()
			if got != tt.expected {
				t.Errorf("migrationsTableName() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestMigrator_findMigration(t *testing.T) {
	m := NewMigrator(nil, "public")

	t.Run("find existing migration", func(t *testing.T) {
		mig, ok := m.findMigration(1)
		if !ok {
			t.Fatal("expected to find migration version 1")
		}
		if mig.Version != 1 {
			t.Errorf("version = %d, want 1", mig.Version)
		}
	})

	t.Run("find non-existing migration", func(t *testing.T) {
		_, ok := m.findMigration(999)
		if ok {
			t.Error("expected not to find migration version 999")
		}
	})
}

func TestMigrator_AddMigration(t *testing.T) {
	m := NewMigrator(nil, "public")
	initialCount := len(m.migrations)

	custom := Migration{
		Version:     100,
		Description: "custom migration",
		Up:          "CREATE TABLE custom (id TEXT);",
		Down:        "DROP TABLE custom;",
	}

	m.AddMigration(custom)

	if len(m.migrations) != initialCount+1 {
		t.Errorf("migration count = %d, want %d", len(m.migrations), initialCount+1)
	}

	mig, ok := m.findMigration(100)
	if !ok {
		t.Fatal("expected to find custom migration")
	}
	if mig.Description != "custom migration" {
		t.Errorf("description = %q, want %q", mig.Description, "custom migration")
	}
}

func TestBuiltinMigrations_RunsTableUp_ContainsExpectedColumns(t *testing.T) {
	migrations := builtinMigrations()
	runsUp := migrations[0].Up

	expectedColumns := []string{
		"id", "goal", "current_state", "vars", "evidence",
		"status", "start_time", "end_time", "result", "error",
	}

	for _, col := range expectedColumns {
		if !containsString(runsUp, col) {
			t.Errorf("runs migration Up SQL missing column %q", col)
		}
	}
}

func TestBuiltinMigrations_EventsTableUp_ContainsExpectedColumns(t *testing.T) {
	migrations := builtinMigrations()
	eventsUp := migrations[1].Up

	expectedColumns := []string{
		"id", "run_id", "type", "timestamp", "payload", "sequence", "version",
	}

	for _, col := range expectedColumns {
		if !containsString(eventsUp, col) {
			t.Errorf("events migration Up SQL missing column %q", col)
		}
	}
}

func TestBuiltinMigrations_KnowledgeTableUp_ContainsExpectedColumns(t *testing.T) {
	migrations := builtinMigrations()
	knowledgeUp := migrations[2].Up

	expectedColumns := []string{
		"id", "embedding", "text", "metadata", "created_at",
	}

	for _, col := range expectedColumns {
		if !containsString(knowledgeUp, col) {
			t.Errorf("knowledge migration Up SQL missing column %q", col)
		}
	}
}
