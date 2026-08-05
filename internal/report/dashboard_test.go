package report_test

import (
	"context"
	"testing"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/report"
	"github.com/daniel-kindl/agentmeter/internal/source"
	"github.com/daniel-kindl/agentmeter/internal/store"
)

func TestBuildDashboard(t *testing.T) {
	t.Parallel()

	database, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	events := []source.UsageEvent{
		{Timestamp: time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC), Source: "claude", SessionID: "a", Model: "claude-sonnet-4", InputTokens: 1_000_000, DedupeKey: "a"},
		{Timestamp: time.Date(2026, 1, 3, 12, 0, 0, 0, time.UTC), Source: "codex", SessionID: "b", Model: "future-model", OutputTokens: 5, DedupeKey: "b"},
	}
	if _, err := database.InsertUsageEvents(context.Background(), events); err != nil {
		t.Fatalf("insert events: %v", err)
	}
	got, err := report.BuildDashboard(context.Background(), database, "7d", time.Date(2026, 1, 3, 18, 0, 0, 0, time.UTC), time.UTC)
	if err != nil {
		t.Fatalf("build dashboard: %v", err)
	}
	if got.Totals.InputTokens != 1_000_000 || got.Totals.OutputTokens != 5 || got.Totals.CostUSD != "3.000000" {
		t.Fatalf("totals = %+v", got.Totals)
	}
	if got.CostComplete || len(got.UnpricedModels) != 1 || got.UnpricedModels[0] != "future-model" {
		t.Fatalf("pricing status = complete:%t models:%v", got.CostComplete, got.UnpricedModels)
	}
	if len(got.Daily) != 2 || len(got.BySource) != 2 || len(got.ByModel) != 2 {
		t.Fatalf("dashboard breakdowns = %+v", got)
	}
}
