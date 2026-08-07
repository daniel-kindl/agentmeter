package store

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

var fetchedAt = time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)

func openMemoryStore(t *testing.T) *Store {
	t.Helper()
	store, err := Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestSaveAndLoadLimitSnapshots(t *testing.T) {
	t.Parallel()

	store := openMemoryStore(t)
	resets := time.Date(2026, 8, 7, 15, 0, 0, 0, time.UTC)
	snapshots := []LimitSnapshot{
		{Kind: "5h", FetchedAt: fetchedAt, Utilization: 33.5, ResetsAt: &resets},
		{Kind: "7d", FetchedAt: fetchedAt, Utilization: 13},
	}
	if err := store.SaveLimitSnapshots(context.Background(), "claude", snapshots); err != nil {
		t.Fatalf("save snapshots: %v", err)
	}

	loaded, err := store.LoadLimitSnapshots(context.Background())
	if err != nil {
		t.Fatalf("load snapshots: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded %d snapshots, want 2", len(loaded))
	}
	if loaded[0].Source != "claude" || loaded[0].Kind != "5h" || loaded[0].Utilization != 33.5 {
		t.Fatalf("first snapshot = %+v", loaded[0])
	}
	if loaded[0].ResetsAt == nil || !loaded[0].ResetsAt.Equal(resets) {
		t.Fatalf("first reset = %v, want %v", loaded[0].ResetsAt, resets)
	}
	if !loaded[0].FetchedAt.Equal(fetchedAt) {
		t.Fatalf("fetch time = %v, want %v", loaded[0].FetchedAt, fetchedAt)
	}
	if loaded[1].ResetsAt != nil {
		t.Fatalf("second reset = %v, want nil", loaded[1].ResetsAt)
	}
}

// A provider that stops reporting a window must not leave the stale value
// behind for the dashboard to render as current.
func TestSaveLimitSnapshotsReplacesWindowsOfOneSource(t *testing.T) {
	t.Parallel()

	store := openMemoryStore(t)
	ctx := context.Background()
	first := []LimitSnapshot{
		{Kind: "5h", FetchedAt: fetchedAt, Utilization: 10},
		{Kind: "7d_opus", FetchedAt: fetchedAt, Utilization: 40},
	}
	if err := store.SaveLimitSnapshots(ctx, "claude", first); err != nil {
		t.Fatalf("save first snapshots: %v", err)
	}
	if err := store.SaveLimitSnapshots(ctx, "codex", first[:1]); err != nil {
		t.Fatalf("save codex snapshots: %v", err)
	}

	later := fetchedAt.Add(time.Hour)
	if err := store.SaveLimitSnapshots(ctx, "claude", []LimitSnapshot{{Kind: "5h", FetchedAt: later, Utilization: 90}}); err != nil {
		t.Fatalf("save replacement snapshots: %v", err)
	}

	loaded, err := store.LoadLimitSnapshots(ctx)
	if err != nil {
		t.Fatalf("load snapshots: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("loaded %+v, want one claude and one codex window", loaded)
	}
	if loaded[0].Source != "claude" || loaded[0].Kind != "5h" || loaded[0].Utilization != 90 {
		t.Fatalf("claude snapshot = %+v, want the replacement", loaded[0])
	}
	if loaded[1].Source != "codex" || loaded[1].Utilization != 10 {
		t.Fatalf("codex snapshot = %+v, want it untouched", loaded[1])
	}
}

func TestSaveLimitSnapshotsRejectsInvalidInput(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		source    string
		snapshots []LimitSnapshot
	}{
		{name: "blank source", source: " ", snapshots: []LimitSnapshot{{Kind: "5h", FetchedAt: fetchedAt}}},
		{name: "blank kind", source: "claude", snapshots: []LimitSnapshot{{Kind: "", FetchedAt: fetchedAt}}},
		{name: "negative utilization", source: "claude", snapshots: []LimitSnapshot{{Kind: "5h", FetchedAt: fetchedAt, Utilization: -1}}},
		{name: "zero fetch time", source: "claude", snapshots: []LimitSnapshot{{Kind: "5h"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			store := openMemoryStore(t)
			if err := store.SaveLimitSnapshots(context.Background(), tt.source, tt.snapshots); err == nil {
				t.Fatal("SaveLimitSnapshots accepted invalid input")
			}
		})
	}
}

func TestLoadLimitSnapshotsIsEmptyBeforeAnyFetch(t *testing.T) {
	t.Parallel()

	loaded, err := openMemoryStore(t).LoadLimitSnapshots(context.Background())
	if err != nil {
		t.Fatalf("load snapshots: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("loaded %+v, want none", loaded)
	}
}

func TestOpenMigratesVersionTwo(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "agentmeter.db")
	db := openRawDB(t, path)
	if _, err := db.Exec(Schema); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	// Version two predates both the snapshot cache and the settings table, so
	// drop them back to that shape.
	if _, err := db.Exec("DROP TABLE limit_snapshots; DROP TABLE settings; PRAGMA user_version = 2;"); err != nil {
		t.Fatalf("create version two schema: %v", err)
	}
	insertSyntheticRow(t, db)
	if err := db.Close(); err != nil {
		t.Fatalf("close version two database: %v", err)
	}

	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("migrate version two database: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	if got := schemaVersion(t, store.db); got != SchemaVersion {
		t.Fatalf("schema version = %d, want %d", got, SchemaVersion)
	}
	if got := usageEventCount(t, store.db); got != 1 {
		t.Fatalf("usage event count = %d, want 1", got)
	}
	if err := store.SaveLimitSnapshots(context.Background(), "claude", []LimitSnapshot{{Kind: "5h", FetchedAt: fetchedAt}}); err != nil {
		t.Fatalf("write to migrated snapshot table: %v", err)
	}
}
