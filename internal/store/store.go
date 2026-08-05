package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// ErrSchemaTooNew indicates that a database was created by a newer version of
// agentmeter and cannot be opened safely by this binary.
var ErrSchemaTooNew = errors.New("database schema is newer than supported")

// Store persists normalized usage events in SQLite.
type Store struct {
	db *sql.DB
}

// Open opens path as a SQLite database and applies any pending migrations.
// Parent directories are not created. Use ":memory:" for an in-memory store.
func Open(ctx context.Context, path string) (*Store, error) {
	db, err := sql.Open(DriverName, path)
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	if err := migrate(ctx, db); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, errors.Join(err, fmt.Errorf("close SQLite database: %w", closeErr))
		}
		return nil, err
	}

	return &Store{db: db}, nil
}

// Close releases the database connection.
func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("close SQLite database: %w", err)
	}
	return nil
}

func migrate(ctx context.Context, db *sql.DB) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin schema migration: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var version int
	if err := tx.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("read schema version: %w", err)
	}
	if version > SchemaVersion {
		return fmt.Errorf("%w: database version %d, supported version %d", ErrSchemaTooNew, version, SchemaVersion)
	}

	if version == 0 {
		if _, err := tx.ExecContext(ctx, Schema); err != nil {
			return fmt.Errorf("apply schema version 1: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "PRAGMA user_version = 1"); err != nil {
			return fmt.Errorf("record schema version 1: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit schema migration: %w", err)
	}
	return nil
}
