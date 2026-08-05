package store

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/source"
)

// InsertResult summarizes the outcome of inserting one batch of usage events.
type InsertResult struct {
	Inserted   int
	Duplicates int
}

// InsertUsageEvents validates and atomically inserts a batch of normalized
// usage events. Existing dedupe keys and repeated keys within events are
// counted as duplicates rather than returned as errors.
func (s *Store) InsertUsageEvents(ctx context.Context, events []source.UsageEvent) (InsertResult, error) {
	var result InsertResult
	if len(events) == 0 {
		return result, nil
	}

	for index, event := range events {
		if err := validateUsageEvent(event); err != nil {
			return result, fmt.Errorf("validate usage event %d: %w", index, err)
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return result, fmt.Errorf("begin usage event batch: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	const insert = `INSERT INTO usage_events (
dedupe_key, timestamp, source, session_id, model, input_tokens, output_tokens,
cache_creation_input_tokens, cache_read_input_tokens
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(dedupe_key) DO NOTHING`
	for index, event := range events {
		execResult, err := tx.ExecContext(ctx, insert,
			event.DedupeKey,
			event.Timestamp.UTC().Format(time.RFC3339Nano),
			event.Source,
			event.SessionID,
			event.Model,
			event.InputTokens,
			event.OutputTokens,
			event.CacheCreationInputTokens,
			event.CacheReadInputTokens,
		)
		if err != nil {
			return InsertResult{}, fmt.Errorf("insert usage event %d: %w", index, err)
		}

		rows, err := execResult.RowsAffected()
		if err != nil {
			return InsertResult{}, fmt.Errorf("read usage event %d insert result: %w", index, err)
		}
		if rows == 0 {
			result.Duplicates++
		} else {
			result.Inserted++
		}
	}

	if err := tx.Commit(); err != nil {
		return InsertResult{}, fmt.Errorf("commit usage event batch: %w", err)
	}
	return result, nil
}

func validateUsageEvent(event source.UsageEvent) error {
	if event.Timestamp.IsZero() {
		return fmt.Errorf("timestamp is zero")
	}
	if strings.TrimSpace(event.Source) == "" {
		return fmt.Errorf("source is blank")
	}
	if strings.TrimSpace(event.SessionID) == "" {
		return fmt.Errorf("session ID is blank")
	}
	if strings.TrimSpace(event.Model) == "" {
		return fmt.Errorf("model is blank")
	}
	if strings.TrimSpace(event.DedupeKey) == "" {
		return fmt.Errorf("dedupe key is blank")
	}
	if event.InputTokens < 0 {
		return fmt.Errorf("input tokens are negative")
	}
	if event.OutputTokens < 0 {
		return fmt.Errorf("output tokens are negative")
	}
	if event.CacheCreationInputTokens < 0 {
		return fmt.Errorf("cache creation input tokens are negative")
	}
	if event.CacheReadInputTokens < 0 {
		return fmt.Errorf("cache read input tokens are negative")
	}
	return nil
}
