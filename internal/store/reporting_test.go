package store

import (
	"context"
	"testing"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/source"
)

func TestVisitUsageEventsFiltersAndOrders(t *testing.T) {
	t.Parallel()

	database := openTestStore(t, ":memory:")
	first := syntheticUsageEvent("first")
	first.Timestamp = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	second := syntheticUsageEvent("second")
	second.Timestamp = time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)
	if _, err := database.InsertUsageEvents(context.Background(), []source.UsageEvent{second, first}); err != nil {
		t.Fatalf("insert events: %v", err)
	}
	since := second.Timestamp
	var keys []string
	if err := database.VisitUsageEvents(context.Background(), &since, func(event source.UsageEvent) error {
		keys = append(keys, event.DedupeKey)
		return nil
	}); err != nil {
		t.Fatalf("visit events: %v", err)
	}
	if len(keys) != 1 || keys[0] != "second" {
		t.Fatalf("keys = %v, want second", keys)
	}
}
