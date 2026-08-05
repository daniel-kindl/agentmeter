package codex_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/parser/codex"
	"github.com/daniel-kindl/agentmeter/internal/source"
)

func TestParseFixture(t *testing.T) {
	t.Parallel()

	file, err := os.Open(filepath.Join("..", "..", "..", "testdata", "synthetic", "codex", "usage.jsonl"))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	t.Cleanup(func() { _ = file.Close() })
	var got []source.UsageEvent
	stats, err := codex.Parse(context.Background(), file, codex.FileMetadata{SessionID: "session"}, func(event source.UsageEvent) error {
		got = append(got, event)
		return nil
	})
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if stats != (codex.Stats{Lines: 5, Emitted: 3, Ignored: 2}) {
		t.Fatalf("stats = %+v", stats)
	}
	want := []source.UsageEvent{
		{Timestamp: time.Date(2026, 2, 3, 13, 2, 0, 0, time.UTC), Source: "codex", SessionID: "session", Model: "gpt-synthetic", InputTokens: 70, OutputTokens: 20, CacheReadInputTokens: 30, DedupeKey: "session:3", RateClass: "fast"},
		{Timestamp: time.Date(2026, 2, 3, 8, 3, 0, 0, time.UTC), Source: "codex", SessionID: "session", Model: "gpt-synthetic", InputTokens: 40, OutputTokens: 15, CacheReadInputTokens: 20, DedupeKey: "session:4", RateClass: "fast"},
		{Timestamp: time.Date(2026, 2, 3, 8, 4, 0, 0, time.UTC), Source: "codex", SessionID: "session", Model: "gpt-synthetic", OutputTokens: 5, DedupeKey: "session:5", RateClass: "fast"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestParseLastUsageAndErrors(t *testing.T) {
	t.Parallel()

	input := strings.Join([]string{
		"{bad-json}",
		`{"timestamp":"bad","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1}}}}`,
		`{"timestamp":"2026-02-03T08:00:00Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":5,"cached_input_tokens":2,"output_tokens":1}}}}`,
	}, "\n")
	var got source.UsageEvent
	stats, err := codex.Parse(context.Background(), strings.NewReader(input), codex.FileMetadata{SessionID: "session"}, func(event source.UsageEvent) error {
		got = event
		return nil
	})
	if err != nil {
		t.Fatalf("parse input: %v", err)
	}
	if stats != (codex.Stats{Lines: 3, Emitted: 1, UnparsedLines: 2}) {
		t.Fatalf("stats = %+v", stats)
	}
	if got.Model != "unknown" || got.InputTokens != 3 || got.CacheReadInputTokens != 2 {
		t.Fatalf("event = %+v", got)
	}
}

func TestParseCancellationAndEmitterError(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := codex.Parse(ctx, strings.NewReader(""), codex.FileMetadata{}, func(source.UsageEvent) error { return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancellation error = %v", err)
	}
	sentinel := errors.New("synthetic emit failure")
	line := `{"timestamp":"2026-02-03T08:00:00Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":1}}}}`
	if _, err := codex.Parse(context.Background(), strings.NewReader(line), codex.FileMetadata{SessionID: "session"}, func(source.UsageEvent) error { return sentinel }); !errors.Is(err, sentinel) {
		t.Fatalf("emitter error = %v", err)
	}
}
