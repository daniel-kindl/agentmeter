package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/source"
)

// VisitUsageEvents streams stored events at or after since in chronological
// order. A nil since visits the complete history.
func (s *Store) VisitUsageEvents(ctx context.Context, since *time.Time, visit func(source.UsageEvent) error) error {
	query := `SELECT timestamp, source, session_id, model, input_tokens, output_tokens,
cache_creation_input_tokens, cache_read_input_tokens, dedupe_key, message_id, request_id,
sidechain, rate_class
FROM usage_events`
	var args []any
	if since != nil {
		query += " WHERE timestamp >= ?"
		args = append(args, since.UTC().Format(time.RFC3339Nano))
	}
	query += " ORDER BY timestamp, dedupe_key"
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query usage events: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var event source.UsageEvent
		var timestamp string
		var messageID, requestID sql.NullString
		if err := rows.Scan(
			&timestamp, &event.Source, &event.SessionID, &event.Model,
			&event.InputTokens, &event.OutputTokens, &event.CacheCreationInputTokens,
			&event.CacheReadInputTokens, &event.DedupeKey, &messageID, &requestID,
			&event.Sidechain, &event.RateClass,
		); err != nil {
			return fmt.Errorf("scan usage event: %w", err)
		}
		parsed, err := time.Parse(time.RFC3339Nano, timestamp)
		if err != nil {
			return fmt.Errorf("parse stored usage timestamp: %w", err)
		}
		event.Timestamp = parsed
		event.MessageID = messageID.String
		event.RequestID = requestID.String
		if err := visit(event); err != nil {
			return fmt.Errorf("visit usage event: %w", err)
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate usage events: %w", err)
	}
	return nil
}
