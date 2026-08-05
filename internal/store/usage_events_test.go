package store

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/source"
)

func TestInsertUsageEventsStoresEveryColumn(t *testing.T) {
	t.Parallel()

	store := openTestStore(t, ":memory:")
	event := syntheticUsageEvent("session-001:event-007")
	event.Timestamp = time.Date(2026, time.January, 2, 4, 4, 5, 123456789, time.FixedZone("synthetic", 3600))

	result, err := store.InsertUsageEvents(context.Background(), []source.UsageEvent{event})
	if err != nil {
		t.Fatalf("insert usage event: %v", err)
	}
	if result != (InsertResult{Inserted: 1}) {
		t.Fatalf("insert result = %+v, want {Inserted:1 Duplicates:0}", result)
	}

	var got struct {
		dedupeKey, timestamp, source, sessionID, model string
		input, output, cacheCreation, cacheRead        int64
	}
	const query = `SELECT dedupe_key, timestamp, source, session_id, model,
input_tokens, output_tokens, cache_creation_input_tokens, cache_read_input_tokens
FROM usage_events`
	if err := store.db.QueryRow(query).Scan(
		&got.dedupeKey,
		&got.timestamp,
		&got.source,
		&got.sessionID,
		&got.model,
		&got.input,
		&got.output,
		&got.cacheCreation,
		&got.cacheRead,
	); err != nil {
		t.Fatalf("query usage event: %v", err)
	}

	if got.dedupeKey != event.DedupeKey ||
		got.timestamp != "2026-01-02T03:04:05.123456789Z" ||
		got.source != event.Source ||
		got.sessionID != event.SessionID ||
		got.model != event.Model ||
		got.input != event.InputTokens ||
		got.output != event.OutputTokens ||
		got.cacheCreation != event.CacheCreationInputTokens ||
		got.cacheRead != event.CacheReadInputTokens {
		t.Fatalf("stored usage event = %+v, want all columns from %+v", got, event)
	}
}

func TestInsertUsageEventsEmptyBatch(t *testing.T) {
	t.Parallel()

	store := openTestStore(t, ":memory:")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := store.InsertUsageEvents(ctx, nil)
	if err != nil {
		t.Fatalf("insert empty batch: %v", err)
	}
	if result != (InsertResult{}) {
		t.Fatalf("insert result = %+v, want zero result", result)
	}
}

func TestInsertUsageEventsCountsDuplicatesAcrossCalls(t *testing.T) {
	t.Parallel()

	store := openTestStore(t, ":memory:")
	event := syntheticUsageEvent("session-001:event-007")
	if _, err := store.InsertUsageEvents(context.Background(), []source.UsageEvent{event}); err != nil {
		t.Fatalf("insert first batch: %v", err)
	}

	result, err := store.InsertUsageEvents(context.Background(), []source.UsageEvent{event})
	if err != nil {
		t.Fatalf("insert duplicate batch: %v", err)
	}
	if result != (InsertResult{Duplicates: 1}) {
		t.Fatalf("insert result = %+v, want {Inserted:0 Duplicates:1}", result)
	}
	if got := usageEventCount(t, store.db); got != 1 {
		t.Fatalf("usage event count = %d, want 1", got)
	}
}

func TestInsertUsageEventsCountsDuplicatesWithinBatch(t *testing.T) {
	t.Parallel()

	store := openTestStore(t, ":memory:")
	first := syntheticUsageEvent("session-001:event-007")
	duplicate := syntheticUsageEvent(first.DedupeKey)
	duplicate.Model = "different-synthetic-model"
	second := syntheticUsageEvent("session-001:event-008")

	result, err := store.InsertUsageEvents(context.Background(), []source.UsageEvent{first, duplicate, second})
	if err != nil {
		t.Fatalf("insert batch: %v", err)
	}
	if result != (InsertResult{Inserted: 2, Duplicates: 1}) {
		t.Fatalf("insert result = %+v, want {Inserted:2 Duplicates:1}", result)
	}
	if got := usageEventCount(t, store.db); got != 2 {
		t.Fatalf("usage event count = %d, want 2", got)
	}
}

func TestInsertUsageEventsReplacesLargerDuplicate(t *testing.T) {
	t.Parallel()

	store := openTestStore(t, ":memory:")
	first := syntheticUsageEvent("message-001:request-001")
	first.MessageID = "message-001"
	first.RequestID = "request-001"
	larger := first
	larger.OutputTokens++

	if _, err := store.InsertUsageEvents(context.Background(), []source.UsageEvent{first}); err != nil {
		t.Fatalf("insert first event: %v", err)
	}
	result, err := store.InsertUsageEvents(context.Background(), []source.UsageEvent{larger})
	if err != nil {
		t.Fatalf("replace duplicate: %v", err)
	}
	if result != (InsertResult{Updated: 1}) {
		t.Fatalf("result = %+v, want one update", result)
	}

	var output int64
	if err := store.db.QueryRow("SELECT output_tokens FROM usage_events").Scan(&output); err != nil {
		t.Fatalf("read replacement: %v", err)
	}
	if output != larger.OutputTokens {
		t.Fatalf("output tokens = %d, want %d", output, larger.OutputTokens)
	}
}

func TestInsertUsageEventsPrefersParentOverSidechainReplay(t *testing.T) {
	t.Parallel()

	store := openTestStore(t, ":memory:")
	replay := syntheticUsageEvent("message-001:request-replay")
	replay.MessageID = "message-001"
	replay.RequestID = "request-replay"
	replay.Sidechain = true
	replay.CacheReadInputTokens = 50_000
	parent := syntheticUsageEvent("message-001:request-parent")
	parent.MessageID = "message-001"
	parent.RequestID = "request-parent"

	if _, err := store.InsertUsageEvents(context.Background(), []source.UsageEvent{replay}); err != nil {
		t.Fatalf("insert replay: %v", err)
	}
	result, err := store.InsertUsageEvents(context.Background(), []source.UsageEvent{parent})
	if err != nil {
		t.Fatalf("replace replay: %v", err)
	}
	if result != (InsertResult{Updated: 1}) {
		t.Fatalf("result = %+v, want one update", result)
	}

	var key string
	var sidechain bool
	if err := store.db.QueryRow("SELECT dedupe_key, sidechain FROM usage_events").Scan(&key, &sidechain); err != nil {
		t.Fatalf("read parent: %v", err)
	}
	if key != parent.DedupeKey || sidechain {
		t.Fatalf("stored key = %q, sidechain = %t; want parent", key, sidechain)
	}
}

func TestInsertUsageEventsRejectsInvalidFieldsWithoutPartialWrite(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*source.UsageEvent)
	}{
		{name: "zero timestamp", mutate: func(event *source.UsageEvent) { event.Timestamp = time.Time{} }},
		{name: "blank source", mutate: func(event *source.UsageEvent) { event.Source = " \t" }},
		{name: "blank session", mutate: func(event *source.UsageEvent) { event.SessionID = " \t" }},
		{name: "blank model", mutate: func(event *source.UsageEvent) { event.Model = " \t" }},
		{name: "blank dedupe key", mutate: func(event *source.UsageEvent) { event.DedupeKey = " \t" }},
		{name: "negative input", mutate: func(event *source.UsageEvent) { event.InputTokens = -1 }},
		{name: "negative output", mutate: func(event *source.UsageEvent) { event.OutputTokens = -1 }},
		{name: "negative cache creation", mutate: func(event *source.UsageEvent) { event.CacheCreationInputTokens = -1 }},
		{name: "negative cache read", mutate: func(event *source.UsageEvent) { event.CacheReadInputTokens = -1 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			store := openTestStore(t, ":memory:")
			valid := syntheticUsageEvent("session-001:event-valid")
			invalid := syntheticUsageEvent("sensitive-event-payload")
			invalid.Source = "sensitive-source-payload"
			invalid.SessionID = "sensitive-session-payload"
			invalid.Model = "sensitive-model-payload"
			tt.mutate(&invalid)

			result, err := store.InsertUsageEvents(context.Background(), []source.UsageEvent{valid, invalid})
			if err == nil {
				t.Fatal("insert invalid batch succeeded")
			}
			if result != (InsertResult{}) {
				t.Fatalf("insert result = %+v, want zero result", result)
			}
			if strings.Contains(err.Error(), "sensitive") {
				t.Fatalf("validation error contains event contents: %v", err)
			}
			if got := usageEventCount(t, store.db); got != 0 {
				t.Fatalf("usage event count = %d, want 0", got)
			}
		})
	}
}

func TestInsertUsageEventsCanceledContextWritesNothing(t *testing.T) {
	t.Parallel()

	store := openTestStore(t, ":memory:")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result, err := store.InsertUsageEvents(ctx, []source.UsageEvent{syntheticUsageEvent("session-001:event-007")})
	if err == nil {
		t.Fatal("insert with canceled context succeeded")
	}
	if result != (InsertResult{}) {
		t.Fatalf("insert result = %+v, want zero result", result)
	}
	if got := usageEventCount(t, store.db); got != 0 {
		t.Fatalf("usage event count = %d, want 0", got)
	}
}

func TestInsertUsageEventsConstraintErrorRollsBackBatch(t *testing.T) {
	t.Parallel()

	store := openTestStore(t, ":memory:")
	const trigger = `CREATE TRIGGER reject_second_event BEFORE INSERT ON usage_events
WHEN NEW.dedupe_key = 'session-001:event-rejected'
BEGIN
    SELECT RAISE(ABORT, 'synthetic constraint failure');
END`
	if _, err := store.db.Exec(trigger); err != nil {
		t.Fatalf("create synthetic trigger: %v", err)
	}

	events := []source.UsageEvent{
		syntheticUsageEvent("session-001:event-accepted"),
		syntheticUsageEvent("session-001:event-rejected"),
	}
	result, err := store.InsertUsageEvents(context.Background(), events)
	if err == nil {
		t.Fatal("insert constraint-violating batch succeeded")
	}
	if result != (InsertResult{}) {
		t.Fatalf("insert result = %+v, want zero result", result)
	}
	if got := usageEventCount(t, store.db); got != 0 {
		t.Fatalf("usage event count = %d, want 0", got)
	}
}

func TestInsertUsageEventsPersistsAfterReopen(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "agentmeter.db")
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	if _, err := store.InsertUsageEvents(context.Background(), []source.UsageEvent{syntheticUsageEvent("session-001:event-007")}); err != nil {
		t.Fatalf("insert usage event: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	reopened, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	defer func() {
		if err := reopened.Close(); err != nil {
			t.Errorf("close reopened store: %v", err)
		}
	}()
	if got := usageEventCount(t, reopened.db); got != 1 {
		t.Fatalf("usage event count after reopen = %d, want 1", got)
	}
}

func openTestStore(t *testing.T, path string) *Store {
	t.Helper()
	store, err := Open(context.Background(), path)
	if err != nil {
		t.Fatalf("open test store: %v", err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close test store: %v", err)
		}
	})
	return store
}

func syntheticUsageEvent(dedupeKey string) source.UsageEvent {
	return source.UsageEvent{
		Timestamp:                time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
		Source:                   "synthetic",
		SessionID:                "session-001",
		Model:                    "example-model",
		InputTokens:              101,
		OutputTokens:             23,
		CacheCreationInputTokens: 17,
		CacheReadInputTokens:     41,
		DedupeKey:                dedupeKey,
	}
}
