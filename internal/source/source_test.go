package source_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/source"
)

func TestUsageEventJSONFields(t *testing.T) {
	t.Parallel()

	timestamp := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)
	tests := []struct {
		name  string
		event source.UsageEvent
		want  map[string]any
	}{
		{
			name: "populated event",
			event: source.UsageEvent{
				Timestamp:                timestamp,
				Source:                   "synthetic",
				SessionID:                "session-001",
				Model:                    "example-model",
				InputTokens:              101,
				OutputTokens:             23,
				CacheCreationInputTokens: 17,
				CacheReadInputTokens:     41,
				DedupeKey:                "session-001:event-007",
				MessageID:                "message-007",
				RequestID:                "request-007",
				Sidechain:                true,
				RateClass:                "fast",
			},
			want: map[string]any{
				"timestamp":                   timestamp.Format(time.RFC3339),
				"source":                      "synthetic",
				"session_id":                  "session-001",
				"model":                       "example-model",
				"input_tokens":                float64(101),
				"output_tokens":               float64(23),
				"cache_creation_input_tokens": float64(17),
				"cache_read_input_tokens":     float64(41),
				"dedupe_key":                  "session-001:event-007",
				"message_id":                  "message-007",
				"request_id":                  "request-007",
				"sidechain":                   true,
				"rate_class":                  "fast",
			},
		},
		{
			name:  "zero values remain explicit",
			event: source.UsageEvent{},
			want: map[string]any{
				"timestamp":                   "0001-01-01T00:00:00Z",
				"source":                      "",
				"session_id":                  "",
				"model":                       "",
				"input_tokens":                float64(0),
				"output_tokens":               float64(0),
				"cache_creation_input_tokens": float64(0),
				"cache_read_input_tokens":     float64(0),
				"dedupe_key":                  "",
				"message_id":                  "",
				"request_id":                  "",
				"sidechain":                   false,
				"rate_class":                  "",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			encoded, err := json.Marshal(tt.event)
			if err != nil {
				t.Fatalf("marshal UsageEvent: %v", err)
			}

			var got map[string]any
			if err := json.Unmarshal(encoded, &got); err != nil {
				t.Fatalf("unmarshal UsageEvent JSON: %v", err)
			}

			if !mapsEqual(got, tt.want) {
				t.Fatalf("JSON fields mismatch\n got: %#v\nwant: %#v", got, tt.want)
			}
		})
	}
}

func mapsEqual(got, want map[string]any) bool {
	if len(got) != len(want) {
		return false
	}
	for key, wantValue := range want {
		if got[key] != wantValue {
			return false
		}
	}
	return true
}
