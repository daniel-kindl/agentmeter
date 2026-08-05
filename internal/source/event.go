package source

import "time"

// UsageEvent is one normalized token-usage record from a supported source.
// Token counts are per-event values; a source with cumulative counters must
// compute deltas before constructing an event.
type UsageEvent struct {
	Timestamp                time.Time `json:"timestamp"`
	Source                   string    `json:"source"`
	SessionID                string    `json:"session_id"`
	Model                    string    `json:"model"`
	InputTokens              int64     `json:"input_tokens"`
	OutputTokens             int64     `json:"output_tokens"`
	CacheCreationInputTokens int64     `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64     `json:"cache_read_input_tokens"`
	DedupeKey                string    `json:"dedupe_key"`
}
