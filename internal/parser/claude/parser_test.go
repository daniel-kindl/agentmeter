package claude_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/parser/claude"
	"github.com/daniel-kindl/agentmeter/internal/source"
)

func TestParseFixture(t *testing.T) {
	t.Parallel()

	file, err := os.Open(filepath.Join("..", "..", "..", "testdata", "synthetic", "claude", "usage.jsonl"))
	if err != nil {
		t.Fatalf("open synthetic fixture: %v", err)
	}
	t.Cleanup(func() {
		if err := file.Close(); err != nil {
			t.Errorf("close synthetic fixture: %v", err)
		}
	})

	var got []source.UsageEvent
	stats, err := claude.Parse(context.Background(), file, claude.FileMetadata{SessionID: "session-file"}, func(event source.UsageEvent) error {
		got = append(got, event)
		return nil
	})
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}

	wantStats := claude.Stats{Lines: 4, Emitted: 3, Ignored: 1}
	if stats != wantStats {
		t.Fatalf("stats = %+v, want %+v", stats, wantStats)
	}
	want := []source.UsageEvent{
		{
			Timestamp:                time.Date(2026, time.January, 2, 8, 4, 5, 0, time.UTC),
			Source:                   "claude",
			SessionID:                "session-direct",
			Model:                    "claude-synthetic-a",
			InputTokens:              101,
			OutputTokens:             23,
			CacheCreationInputTokens: 17,
			CacheReadInputTokens:     41,
			DedupeKey:                "message-direct:request-direct",
			MessageID:                "message-direct",
			RequestID:                "request-direct",
			RateClass:                "fast",
		},
		{
			Timestamp:    time.Date(2026, time.January, 2, 3, 4, 7, 0, time.UTC),
			Source:       "claude",
			SessionID:    "session-file",
			Model:        "claude-synthetic-b",
			InputTokens:  5,
			OutputTokens: 7,
			DedupeKey:    "message-fallback:request-fallback",
			MessageID:    "message-fallback",
			RequestID:    "request-fallback",
		},
		{
			Timestamp:            time.Date(2026, time.January, 2, 3, 4, 9, 0, time.UTC),
			Source:               "claude",
			SessionID:            "session-agent",
			Model:                "claude-synthetic-c",
			InputTokens:          11,
			OutputTokens:         13,
			CacheReadInputTokens: 19,
			DedupeKey:            "message-agent:request-agent",
			MessageID:            "message-agent",
			RequestID:            "request-agent",
			Sidechain:            true,
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events mismatch\n got: %#v\nwant: %#v", got, want)
	}
}

func TestParseAcceptsAbsentClaudeIdentityAndModel(t *testing.T) {
	t.Parallel()

	line := `{"type":"assistant","timestamp":"2026-01-02T03:04:05Z","message":{"usage":{"input_tokens":1,"output_tokens":2}}}`
	var got source.UsageEvent
	stats, err := claude.Parse(context.Background(), strings.NewReader(line), claude.FileMetadata{SessionID: "session"}, func(event source.UsageEvent) error {
		got = event
		return nil
	})
	if err != nil {
		t.Fatalf("parse line: %v", err)
	}
	if stats != (claude.Stats{Lines: 1, Emitted: 1}) {
		t.Fatalf("stats = %+v, want one emitted line", stats)
	}
	if got.Model != "unknown" || got.DedupeKey != "session:1" {
		t.Fatalf("event = %+v, want unknown model and event-index fallback", got)
	}
}

func TestParseClassifiesInvalidAndIgnoredLines(t *testing.T) {
	t.Parallel()

	lines := []string{
		"",
		`{"type":"user","message":{"role":"user"}}`,
		`{"type":"assistant","message":{"id":"no-usage"}}`,
		`{not-json}`,
		validRecord("", "request", "2026-01-02T03:04:05Z", "model", 1, 2),
		validRecord("message", "", "2026-01-02T03:04:05Z", "model", 1, 2),
		validRecord("message", "request", "not-a-time", "model", 1, 2),
		validRecord("message", "request", "2026-01-02T03:04:05Z", " ", 1, 2),
		validRecord("message", "request", "2026-01-02T03:04:05Z", "model", -1, 2),
		validRecord("message", "request", "2026-01-02T03:04:05Z", "model", 1, -1),
		`{"type":"assistant","timestamp":"2026-01-02T03:04:05Z","sessionId":"session","requestId":"request","message":{"id":"message","model":"model","usage":{"output_tokens":2}}}`,
		`{"type":"future_usage","message":{"usage":{"input_tokens":1,"output_tokens":2}}}`,
		`{"type":"progress","data":{"type":"future_progress","message":{"usage":{"input_tokens":1}}}}`,
	}

	stats, err := claude.Parse(context.Background(), strings.NewReader(strings.Join(lines, "\n")), claude.FileMetadata{SessionID: "session"}, func(source.UsageEvent) error {
		return nil
	})
	if err != nil {
		t.Fatalf("parse lines: %v", err)
	}
	want := claude.Stats{Lines: 13, Ignored: 3, UnparsedLines: 10}
	if stats != want {
		t.Fatalf("stats = %+v, want %+v", stats, want)
	}
}

func TestParseRejectsNegativeOptionalTokenCounts(t *testing.T) {
	t.Parallel()

	line := `{"type":"assistant","timestamp":"2026-01-02T03:04:05Z","sessionId":"session","requestId":"request","message":{"id":"message","model":"model","usage":{"input_tokens":1,"output_tokens":2,"cache_read_input_tokens":-1}}}`
	stats, err := claude.Parse(context.Background(), strings.NewReader(line), claude.FileMetadata{}, func(source.UsageEvent) error {
		return nil
	})
	if err != nil {
		t.Fatalf("parse line: %v", err)
	}
	if stats != (claude.Stats{Lines: 1, UnparsedLines: 1}) {
		t.Fatalf("stats = %+v, want one unparsed line", stats)
	}
}

func TestParseRequiresSession(t *testing.T) {
	t.Parallel()

	line := validRecord("message", "request", "2026-01-02T03:04:05Z", "model", 1, 2)
	stats, err := claude.Parse(context.Background(), strings.NewReader(line), claude.FileMetadata{}, func(source.UsageEvent) error {
		return nil
	})
	if err != nil {
		t.Fatalf("parse line: %v", err)
	}
	if stats != (claude.Stats{Lines: 1, UnparsedLines: 1}) {
		t.Fatalf("stats = %+v, want one unparsed line", stats)
	}
}

func TestParseEmitsDuplicatesInInputOrder(t *testing.T) {
	t.Parallel()

	first := validRecord("message-1", "request-1", "2026-01-02T03:04:05Z", "model", 1, 2)
	second := validRecord("message-2", "request-2", "2026-01-02T03:04:06Z", "model", 3, 4)
	input := strings.Join([]string{first, second, first}, "\n")
	var keys []string
	stats, err := claude.Parse(context.Background(), strings.NewReader(input), claude.FileMetadata{SessionID: "session"}, func(event source.UsageEvent) error {
		keys = append(keys, event.DedupeKey)
		return nil
	})
	if err != nil {
		t.Fatalf("parse lines: %v", err)
	}
	if want := []string{"message-1:request-1", "message-2:request-2", "message-1:request-1"}; !reflect.DeepEqual(keys, want) {
		t.Fatalf("dedupe keys = %v, want %v", keys, want)
	}
	if stats != (claude.Stats{Lines: 3, Emitted: 3}) {
		t.Fatalf("stats = %+v, want three emitted lines", stats)
	}
}

func TestParseCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	stats, err := claude.Parse(ctx, strings.NewReader(validRecord("message", "request", "2026-01-02T03:04:05Z", "model", 1, 2)), claude.FileMetadata{SessionID: "session"}, func(source.UsageEvent) error {
		t.Fatal("emit called after cancellation")
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
	}
	if stats != (claude.Stats{}) {
		t.Fatalf("stats = %+v, want zero stats", stats)
	}
}

func TestParseCancellationAfterEmission(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	input := strings.Join([]string{
		validRecord("message-1", "request-1", "2026-01-02T03:04:05Z", "model", 1, 2),
		validRecord("message-2", "request-2", "2026-01-02T03:04:06Z", "model", 3, 4),
	}, "\n")
	emitted := 0
	stats, err := claude.Parse(ctx, strings.NewReader(input), claude.FileMetadata{SessionID: "session"}, func(source.UsageEvent) error {
		emitted++
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
	}
	if emitted != 1 {
		t.Fatalf("emitted = %d, want 1", emitted)
	}
	if stats != (claude.Stats{Lines: 1, Emitted: 1}) {
		t.Fatalf("stats = %+v, want one read and emitted line", stats)
	}
}

func TestParseEmitterError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("synthetic emitter failure")
	secret := "private-record-marker"
	line := validRecord(secret, "request", "2026-01-02T03:04:05Z", "model", 1, 2)
	stats, err := claude.Parse(context.Background(), strings.NewReader(line), claude.FileMetadata{SessionID: "session"}, func(source.UsageEvent) error {
		return sentinel
	})
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want emitter error", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error exposed record contents: %v", err)
	}
	if stats != (claude.Stats{Lines: 1}) {
		t.Fatalf("stats = %+v, want one read line and no emitted lines", stats)
	}
}

func TestParseLargeOrdinaryRecord(t *testing.T) {
	t.Parallel()

	line := `{"type":"user","padding":"` + strings.Repeat("x", 1024*1024) + `"}`
	stats, err := claude.Parse(context.Background(), strings.NewReader(line), claude.FileMetadata{}, func(source.UsageEvent) error {
		return nil
	})
	if err != nil {
		t.Fatalf("parse large ordinary record: %v", err)
	}
	if stats != (claude.Stats{Lines: 1, Ignored: 1}) {
		t.Fatalf("stats = %+v, want one ignored line", stats)
	}
}

func TestParseScannerLimit(t *testing.T) {
	t.Parallel()

	secret := "private-oversized-record-marker"
	line := secret + strings.Repeat("x", 64*1024*1024)
	stats, err := claude.Parse(context.Background(), strings.NewReader(line), claude.FileMetadata{}, func(source.UsageEvent) error {
		return nil
	})
	if err == nil {
		t.Fatal("parse oversized line succeeded, want scanner error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Fatalf("error exposed record contents: %v", err)
	}
	if stats != (claude.Stats{}) {
		t.Fatalf("stats = %+v, want zero complete lines", stats)
	}
}

func validRecord(messageID, requestID, timestamp, model string, input, output int64) string {
	return `{"type":"assistant","timestamp":"` + timestamp + `","requestId":"` + requestID + `","message":{"id":"` + messageID + `","model":"` + model + `","usage":{"input_tokens":` +
		intString(input) + `,"output_tokens":` + intString(output) + `}}}`
}

func intString(value int64) string {
	if value == -1 {
		return "-1"
	}
	return string(rune('0' + value))
}
