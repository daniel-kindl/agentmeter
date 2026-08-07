package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// LimitSnapshot is one cached limit window as an authoritative provider
// reported it. Window kinds are opaque strings here so that persistence stays
// independent of the limits package, which reads from this store.
type LimitSnapshot struct {
	Source      string
	Kind        string
	FetchedAt   time.Time
	Utilization float64
	ResetsAt    *time.Time
}

// SaveLimitSnapshots replaces every cached window for one source.
//
// Replacement rather than upsert is deliberate: a provider that stops reporting
// a window must not leave the previous value behind to be rendered as current.
func (s *Store) SaveLimitSnapshots(ctx context.Context, sourceName string, snapshots []LimitSnapshot) error {
	if strings.TrimSpace(sourceName) == "" {
		return fmt.Errorf("save limit snapshots: source is blank")
	}
	for index, snapshot := range snapshots {
		if strings.TrimSpace(snapshot.Kind) == "" {
			return fmt.Errorf("save limit snapshot %d: window kind is blank", index)
		}
		if snapshot.Utilization < 0 {
			return fmt.Errorf("save limit snapshot %d: utilization is negative", index)
		}
		if snapshot.FetchedAt.IsZero() {
			return fmt.Errorf("save limit snapshot %d: fetch time is zero", index)
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin limit snapshot write: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, "DELETE FROM limit_snapshots WHERE source = ?", sourceName); err != nil {
		return fmt.Errorf("clear limit snapshots: %w", err)
	}
	const insert = `INSERT INTO limit_snapshots (source, window_kind, fetched_at, utilization, resets_at)
VALUES (?, ?, ?, ?, ?)`
	for index, snapshot := range snapshots {
		var resets any
		if snapshot.ResetsAt != nil {
			resets = snapshot.ResetsAt.UTC().Format(time.RFC3339Nano)
		}
		_, err := tx.ExecContext(ctx, insert,
			sourceName, snapshot.Kind, snapshot.FetchedAt.UTC().Format(time.RFC3339Nano),
			snapshot.Utilization, resets,
		)
		if err != nil {
			return fmt.Errorf("insert limit snapshot %d: %w", index, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit limit snapshot write: %w", err)
	}
	return nil
}

// LoadLimitSnapshots returns every cached window, ordered by source and kind.
func (s *Store) LoadLimitSnapshots(ctx context.Context) ([]LimitSnapshot, error) {
	const query = `SELECT source, window_kind, fetched_at, utilization, resets_at
FROM limit_snapshots ORDER BY source, window_kind`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query limit snapshots: %w", err)
	}
	defer func() { _ = rows.Close() }()

	snapshots := []LimitSnapshot{}
	for rows.Next() {
		var snapshot LimitSnapshot
		var fetchedAt string
		var resetsAt sql.NullString
		if err := rows.Scan(&snapshot.Source, &snapshot.Kind, &fetchedAt, &snapshot.Utilization, &resetsAt); err != nil {
			return nil, fmt.Errorf("scan limit snapshot: %w", err)
		}
		snapshot.FetchedAt, err = time.Parse(time.RFC3339Nano, fetchedAt)
		if err != nil {
			return nil, fmt.Errorf("parse stored limit fetch time: %w", err)
		}
		if resetsAt.Valid {
			parsed, err := time.Parse(time.RFC3339Nano, resetsAt.String)
			if err != nil {
				return nil, fmt.Errorf("parse stored limit reset time: %w", err)
			}
			snapshot.ResetsAt = &parsed
		}
		snapshots = append(snapshots, snapshot)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate limit snapshots: %w", err)
	}
	return snapshots, nil
}
