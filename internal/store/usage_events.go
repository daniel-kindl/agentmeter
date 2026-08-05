package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/source"
)

// InsertResult summarizes the outcome of inserting one batch of usage events.
type InsertResult struct {
	Inserted   int
	Updated    int
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
cache_creation_input_tokens, cache_read_input_tokens, message_id, request_id, sidechain, rate_class
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULLIF(?, ''), NULLIF(?, ''), ?, ?)`
	const findExisting = `SELECT dedupe_key, input_tokens, output_tokens,
cache_creation_input_tokens, cache_read_input_tokens, sidechain
FROM usage_events
WHERE dedupe_key = ?
   OR (? <> '' AND message_id = ? AND (? = 1 OR sidechain = 1))
ORDER BY CASE WHEN dedupe_key = ? THEN 0 ELSE 1 END
LIMIT 1`
	const update = `UPDATE usage_events SET
dedupe_key = ?, timestamp = ?, source = ?, session_id = ?, model = ?,
input_tokens = ?, output_tokens = ?, cache_creation_input_tokens = ?,
cache_read_input_tokens = ?, message_id = NULLIF(?, ''), request_id = NULLIF(?, ''),
sidechain = ?, rate_class = ?
WHERE dedupe_key = ?`
	for index, event := range events {
		var existing struct {
			key                                     string
			input, output, cacheCreation, cacheRead int64
			sidechain                               bool
		}
		err := tx.QueryRowContext(ctx, findExisting,
			event.DedupeKey, event.MessageID, event.MessageID, event.Sidechain, event.DedupeKey,
		).Scan(&existing.key, &existing.input, &existing.output, &existing.cacheCreation, &existing.cacheRead, &existing.sidechain)
		switch {
		case err == sql.ErrNoRows:
			if _, err := execUsageEvent(ctx, tx, insert, event); err != nil {
				return InsertResult{}, fmt.Errorf("insert usage event %d: %w", index, err)
			}
			result.Inserted++
		case err != nil:
			return InsertResult{}, fmt.Errorf("find usage event %d duplicate: %w", index, err)
		case shouldReplace(event, existing.input, existing.output, existing.cacheCreation, existing.cacheRead, existing.sidechain):
			args := usageEventArgs(event)
			args = append(args, existing.key)
			if _, err := tx.ExecContext(ctx, update, args...); err != nil {
				return InsertResult{}, fmt.Errorf("update usage event %d: %w", index, err)
			}
			result.Updated++
		default:
			result.Duplicates++
		}
	}

	if err := tx.Commit(); err != nil {
		return InsertResult{}, fmt.Errorf("commit usage event batch: %w", err)
	}
	return result, nil
}

func execUsageEvent(ctx context.Context, tx *sql.Tx, query string, event source.UsageEvent) (sql.Result, error) {
	return tx.ExecContext(ctx, query, usageEventArgs(event)...)
}

func usageEventArgs(event source.UsageEvent) []any {
	return []any{
		event.DedupeKey,
		event.Timestamp.UTC().Format(time.RFC3339Nano),
		event.Source,
		event.SessionID,
		event.Model,
		event.InputTokens,
		event.OutputTokens,
		event.CacheCreationInputTokens,
		event.CacheReadInputTokens,
		event.MessageID,
		event.RequestID,
		event.Sidechain,
		event.RateClass,
	}
}

func shouldReplace(event source.UsageEvent, input, output, cacheCreation, cacheRead int64, sidechain bool) bool {
	if event.Sidechain != sidechain {
		return sidechain
	}
	candidateTotal := event.InputTokens + event.OutputTokens + event.CacheCreationInputTokens + event.CacheReadInputTokens
	existingTotal := input + output + cacheCreation + cacheRead
	return candidateTotal > existingTotal
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
