package sqlite

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig("test.db")

	if cfg.DSN != "test.db" {
		t.Errorf("expected DSN test.db, got %s", cfg.DSN)
	}
	if cfg.MaxOpenConns != 1 {
		t.Errorf("expected MaxOpenConns 1, got %d", cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns != 1 {
		t.Errorf("expected MaxIdleConns 1, got %d", cfg.MaxIdleConns)
	}
	if cfg.JournalMode != "WAL" {
		t.Errorf("expected JournalMode WAL, got %s", cfg.JournalMode)
	}
	if cfg.BusyTimeout != 5000 {
		t.Errorf("expected BusyTimeout 5000, got %d", cfg.BusyTimeout)
	}
	if !cfg.ForeignKeys {
		t.Error("expected ForeignKeys to be true")
	}
}

func TestOpen_EmptyDSN(t *testing.T) {
	_, err := Open(Config{})
	if err == nil {
		t.Fatal("expected error for empty DSN")
	}
}

func TestOpen_InMemory(t *testing.T) {
	cfg := DefaultConfig(":memory:")
	db, err := Open(cfg)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	// Verify the connection works.
	err = db.Ping()
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}

func TestOpen_CustomPoolSettings(t *testing.T) {
	cfg := Config{
		DSN:             ":memory:",
		MaxOpenConns:    5,
		MaxIdleConns:    2,
		ConnMaxLifetime: 30 * time.Minute,
		ConnMaxIdleTime: 5 * time.Minute,
		JournalMode:     "WAL",
		BusyTimeout:     3000,
		ForeignKeys:     true,
	}

	db, err := Open(cfg)
	if err != nil {
		t.Fatalf("Open with custom settings failed: %v", err)
	}
	defer db.Close()

	// Verify connection is functional.
	var result int
	err = db.QueryRow("SELECT 1").Scan(&result)
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if result != 1 {
		t.Errorf("expected 1, got %d", result)
	}
}

func TestOpen_NoPragmas(t *testing.T) {
	// Config with no journal mode and no busy timeout.
	cfg := Config{
		DSN: ":memory:",
	}

	db, err := Open(cfg)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		t.Fatalf("Ping failed: %v", err)
	}
}
