package store

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"testing"
)

func TestOpenCreatesCurrentSchema(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path func(*testing.T) string
	}{
		{
			name: "disk",
			path: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "agentmeter.db")
			},
		},
		{
			name: "memory",
			path: func(*testing.T) string { return ":memory:" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store, err := Open(context.Background(), tt.path(t))
			if err != nil {
				t.Fatalf("open store: %v", err)
			}
			t.Cleanup(func() {
				if err := store.Close(); err != nil {
					t.Errorf("close store: %v", err)
				}
			})

			if got := schemaVersion(t, store.db); got != SchemaVersion {
				t.Fatalf("schema version = %d, want %d", got, SchemaVersion)
			}
			assertUsageEventsTableExists(t, store.db)
			if got := store.db.Stats().MaxOpenConnections; got != 1 {
				t.Fatalf("maximum open connections = %d, want 1", got)
			}
		})
	}
}

func TestOpenUpgradesVersionZeroAndPreservesRows(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "agentmeter.db")
	db := openRawDB(t, path)
	if _, err := db.Exec(Schema); err != nil {
		t.Fatalf("create version zero schema: %v", err)
	}
	insertSyntheticRow(t, db)
	if err := db.Close(); err != nil {
		t.Fatalf("close version zero database: %v", err)
	}

	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	}()

	if got := schemaVersion(t, store.db); got != SchemaVersion {
		t.Fatalf("schema version = %d, want %d", got, SchemaVersion)
	}
	if got := usageEventCount(t, store.db); got != 1 {
		t.Fatalf("usage event count = %d, want 1", got)
	}
}

func TestOpenCurrentSchemaWithoutChangingContents(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "agentmeter.db")
	db := openRawDB(t, path)
	if _, err := db.Exec(Schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	if _, err := db.Exec("PRAGMA user_version = 1"); err != nil {
		t.Fatalf("set schema version: %v", err)
	}
	insertSyntheticRow(t, db)
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	}()

	if got := usageEventCount(t, store.db); got != 1 {
		t.Fatalf("usage event count = %d, want 1", got)
	}
}

func TestOpenRejectsFutureSchema(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "agentmeter.db")
	db := openRawDB(t, path)
	if _, err := db.Exec("PRAGMA user_version = 2"); err != nil {
		t.Fatalf("set future schema version: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}

	store, err := Open(context.Background(), path)
	if store != nil {
		_ = store.Close()
		t.Fatal("Open returned a store for a future schema")
	}
	if !errors.Is(err, ErrSchemaTooNew) {
		t.Fatalf("Open error = %v, want ErrSchemaTooNew", err)
	}
}

func TestOpenDoesNotCreateParentDirectories(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "missing", "agentmeter.db")
	store, err := Open(context.Background(), path)
	if store != nil {
		_ = store.Close()
		t.Fatal("Open returned a store with a missing parent directory")
	}
	if err == nil {
		t.Fatal("Open succeeded with a missing parent directory")
	}
}

func openRawDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open(DriverName, path)
	if err != nil {
		t.Fatalf("open raw database: %v", err)
	}
	return db
}

func schemaVersion(t *testing.T, db *sql.DB) int {
	t.Helper()
	var version int
	if err := db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("read schema version: %v", err)
	}
	return version
}

func assertUsageEventsTableExists(t *testing.T, db *sql.DB) {
	t.Helper()
	var name string
	if err := db.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = 'usage_events'`).Scan(&name); err != nil {
		t.Fatalf("find usage_events table: %v", err)
	}
}

func insertSyntheticRow(t *testing.T, db *sql.DB) {
	t.Helper()
	const query = `INSERT INTO usage_events (
dedupe_key, timestamp, source, session_id, model, input_tokens, output_tokens,
cache_creation_input_tokens, cache_read_input_tokens
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	if _, err := db.Exec(query, "session-001:event-007", "2026-01-02T03:04:05Z", "synthetic", "session-001", "example-model", 101, 23, 17, 41); err != nil {
		t.Fatalf("insert synthetic row: %v", err)
	}
}

func usageEventCount(t *testing.T, db *sql.DB) int {
	t.Helper()
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM usage_events").Scan(&count); err != nil {
		t.Fatalf("count usage events: %v", err)
	}
	return count
}
