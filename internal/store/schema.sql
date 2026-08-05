CREATE TABLE IF NOT EXISTS usage_events (
    dedupe_key TEXT PRIMARY KEY,
    timestamp TEXT NOT NULL,
    source TEXT NOT NULL,
    session_id TEXT NOT NULL,
    model TEXT NOT NULL,
    input_tokens INTEGER NOT NULL CHECK (input_tokens >= 0),
    output_tokens INTEGER NOT NULL CHECK (output_tokens >= 0),
    cache_creation_input_tokens INTEGER NOT NULL CHECK (cache_creation_input_tokens >= 0),
    cache_read_input_tokens INTEGER NOT NULL CHECK (cache_read_input_tokens >= 0),
    message_id TEXT,
    request_id TEXT,
    sidechain INTEGER NOT NULL DEFAULT 0 CHECK (sidechain IN (0, 1)),
    rate_class TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS usage_events_timestamp_idx ON usage_events(timestamp);
CREATE INDEX IF NOT EXISTS usage_events_source_idx ON usage_events(source);
CREATE INDEX IF NOT EXISTS usage_events_model_idx ON usage_events(model);
CREATE INDEX IF NOT EXISTS usage_events_message_id_idx ON usage_events(message_id);

