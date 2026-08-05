CREATE TABLE IF NOT EXISTS usage_events (
    dedupe_key TEXT PRIMARY KEY,
    timestamp TEXT NOT NULL,
    source TEXT NOT NULL,
    session_id TEXT NOT NULL,
    model TEXT NOT NULL,
    input_tokens INTEGER NOT NULL CHECK (input_tokens >= 0),
    output_tokens INTEGER NOT NULL CHECK (output_tokens >= 0),
    cache_creation_input_tokens INTEGER NOT NULL CHECK (cache_creation_input_tokens >= 0),
    cache_read_input_tokens INTEGER NOT NULL CHECK (cache_read_input_tokens >= 0)
);

