package store_test

import (
	"database/sql"
	"slices"
	"strings"
	"testing"

	"github.com/daniel-kindl/agentmeter/internal/store"
)

func TestSQLiteDriverRegistered(t *testing.T) {
	t.Parallel()

	if !slices.Contains(sql.Drivers(), store.DriverName) {
		t.Fatalf("registered drivers %v do not contain %q", sql.Drivers(), store.DriverName)
	}
}

func TestSchemaCreatesUsageEventsWithDedupeKey(t *testing.T) {
	t.Parallel()

	if strings.TrimSpace(store.Schema) == "" {
		t.Fatal("embedded schema is empty")
	}

	db, err := sql.Open(store.DriverName, ":memory:")
	if err != nil {
		t.Fatalf("open in-memory database: %v", err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})

	if _, err := db.Exec(store.Schema); err != nil {
		t.Fatalf("apply embedded schema: %v", err)
	}

	const insert = `INSERT INTO usage_events (
dedupe_key, timestamp, source, session_id, model, input_tokens, output_tokens,
cache_creation_input_tokens, cache_read_input_tokens
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	values := []any{"session-001:event-007", "2026-01-02T03:04:05Z", "synthetic",
		"session-001", "example-model", 101, 23, 17, 41}
	if _, err := db.Exec(insert, values...); err != nil {
		t.Fatalf("insert usage event: %v", err)
	}
	if _, err := db.Exec(insert, values...); err == nil {
		t.Fatal("duplicate dedupe_key insert succeeded")
	}
}
