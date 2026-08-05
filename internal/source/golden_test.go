package source_test

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/source"
)

var updateGolden = flag.Bool("update", false, "update golden test files")

func TestUsageEventGolden(t *testing.T) {
	t.Parallel()

	event := source.UsageEvent{
		Timestamp:                time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC),
		Source:                   "synthetic",
		SessionID:                "session-001",
		Model:                    "example-model",
		InputTokens:              101,
		OutputTokens:             23,
		CacheCreationInputTokens: 17,
		CacheReadInputTokens:     41,
		DedupeKey:                "session-001:event-007",
	}

	got, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		t.Fatalf("marshal UsageEvent: %v", err)
	}
	got = append(got, '\n')

	assertGolden(t, filepath.Join("..", "..", "testdata", "synthetic", "usage_event.golden"), got)
}

func assertGolden(t *testing.T, path string, got []byte) {
	t.Helper()

	if *updateGolden {
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatalf("update golden file: %v", err)
		}
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file: %v", err)
	}
	if string(got) != string(want) {
		t.Fatalf("golden mismatch for %s; run go test ./internal/source -update", path)
	}
}
