package limits_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/limits"
	"github.com/daniel-kindl/agentmeter/internal/source"
	"github.com/daniel-kindl/agentmeter/internal/store"
)

var now = time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)

func newStore(t *testing.T, events ...source.UsageEvent) *store.Store {
	t.Helper()
	database, err := store.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })
	if len(events) == 0 {
		return database
	}
	if _, err := database.InsertUsageEvents(context.Background(), events); err != nil {
		t.Fatalf("insert events: %v", err)
	}
	return database
}

func event(agent string, at time.Time, tokens int64) source.UsageEvent {
	return source.UsageEvent{
		Timestamp:   at,
		Source:      agent,
		SessionID:   "session",
		Model:       "model",
		InputTokens: tokens,
		DedupeKey:   fmt.Sprintf("%s:%s", agent, at.Format(time.RFC3339Nano)),
	}
}

func windowOf(t *testing.T, result limits.SourceLimits, kind limits.Kind) limits.Window {
	t.Helper()
	for _, window := range result.Windows {
		if window.Kind == kind {
			return window
		}
	}
	t.Fatalf("source %q has no %q window", result.Source, kind)
	return limits.Window{}
}

func sourceOf(t *testing.T, results []limits.SourceLimits, name string) limits.SourceLimits {
	t.Helper()
	for _, result := range results {
		if result.Source == name {
			return result
		}
	}
	t.Fatalf("no limits for source %q in %+v", name, results)
	return limits.SourceLimits{}
}

// The active block holds only the most recent contiguous five hours, while the
// baseline it is measured against comes from the busiest block in all history.
func TestEstimateMeasuresActiveBlockAgainstBusiestBlock(t *testing.T) {
	t.Parallel()

	database := newStore(t,
		event("claude", time.Date(2026, 7, 20, 9, 0, 0, 0, time.UTC), 1000),
		event("claude", time.Date(2026, 8, 1, 8, 0, 0, 0, time.UTC), 500),
		event("claude", time.Date(2026, 8, 7, 10, 15, 0, 0, time.UTC), 100),
		event("claude", time.Date(2026, 8, 7, 11, 30, 0, 0, time.UTC), 200),
	)

	results, err := limits.Estimate(context.Background(), database, now, limits.Budgets{})
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}

	claude := sourceOf(t, results, "claude")
	if claude.Origin != limits.OriginEstimated {
		t.Fatalf("origin = %q, want %q", claude.Origin, limits.OriginEstimated)
	}
	block := windowOf(t, claude, limits.KindFiveHour)
	if *block.UsedTokens != 300 || *block.BudgetTokens != 1000 {
		t.Fatalf("five-hour tokens = %d/%d, want 300/1000", *block.UsedTokens, *block.BudgetTokens)
	}
	if block.Utilization != 30 {
		t.Fatalf("five-hour utilization = %v, want 30", block.Utilization)
	}
	// The block opened at 10:15, floored to 10:00, so it resets five hours later.
	wantReset := time.Date(2026, 8, 7, 15, 0, 0, 0, time.UTC)
	if block.ResetsAt == nil || !block.ResetsAt.Equal(wantReset) {
		t.Fatalf("five-hour reset = %v, want %v", block.ResetsAt, wantReset)
	}

	// 800 tokens landed in the trailing seven days; the busiest span is the
	// 1000-token one from three weeks back.
	week := windowOf(t, claude, limits.KindSevenDay)
	if *week.UsedTokens != 800 || *week.BudgetTokens != 1000 || week.Utilization != 80 {
		t.Fatalf("seven-day = %d/%d at %v%%, want 800/1000 at 80%%", *week.UsedTokens, *week.BudgetTokens, week.Utilization)
	}
	if week.ResetsAt != nil {
		t.Fatalf("seven-day reset = %v, want nil because the real reset instant is not derivable offline", week.ResetsAt)
	}
}

// A gap of exactly five hours closes the block, so the later event starts a new one.
func TestEstimateSplitsBlocksOnFiveHourGap(t *testing.T) {
	t.Parallel()

	first := time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC)
	database := newStore(t,
		event("claude", first, 10),
		event("claude", first.Add(4*time.Hour+59*time.Minute), 20),
		event("claude", first.Add(9*time.Hour+59*time.Minute), 40),
	)

	results, err := limits.Estimate(context.Background(), database, now, limits.Budgets{})
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}

	// The first two events sit in one block worth 30 tokens. The third arrives
	// five hours after the second, so it opens a block of its own.
	block := windowOf(t, sourceOf(t, results, "claude"), limits.KindFiveHour)
	if *block.BudgetTokens != 40 {
		t.Fatalf("busiest block = %d, want 40", *block.BudgetTokens)
	}
}

// Once five hours pass with no activity there is no active block to report.
func TestEstimateReportsNoActiveBlockAfterIdleWindow(t *testing.T) {
	t.Parallel()

	database := newStore(t, event("codex", now.Add(-6*time.Hour), 500))

	results, err := limits.Estimate(context.Background(), database, now, limits.Budgets{})
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}

	block := windowOf(t, sourceOf(t, results, "codex"), limits.KindFiveHour)
	if *block.UsedTokens != 0 || block.Utilization != 0 {
		t.Fatalf("five-hour = %d tokens at %v%%, want 0 at 0%%", *block.UsedTokens, block.Utilization)
	}
	if block.ResetsAt != nil {
		t.Fatalf("five-hour reset = %v, want nil when no block is active", block.ResetsAt)
	}
}

func TestEstimateHonorsConfiguredBudgets(t *testing.T) {
	t.Parallel()

	database := newStore(t, event("claude", now.Add(-time.Hour), 300))

	results, err := limits.Estimate(context.Background(), database, now, limits.Budgets{FiveHour: 3000, SevenDay: 8000})
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}

	claude := sourceOf(t, results, "claude")
	if block := windowOf(t, claude, limits.KindFiveHour); block.Utilization != 10 {
		t.Fatalf("five-hour utilization = %v, want 10", block.Utilization)
	}
	if week := windowOf(t, claude, limits.KindSevenDay); *week.BudgetTokens != 8000 {
		t.Fatalf("seven-day budget = %d, want 8000", *week.BudgetTokens)
	}
}

// A single block makes the source its own baseline, which reads as 100%. Say so
// rather than letting the operator read it as a real quota.
func TestEstimateFlagsLimitedHistory(t *testing.T) {
	t.Parallel()

	database := newStore(t, event("codex", now.Add(-10*time.Minute), 50))

	results, err := limits.Estimate(context.Background(), database, now, limits.Budgets{})
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}

	codex := sourceOf(t, results, "codex")
	if codex.Message == "" {
		t.Fatal("message is empty, want a limited-history explanation")
	}
	if block := windowOf(t, codex, limits.KindFiveHour); block.Utilization != 100 {
		t.Fatalf("five-hour utilization = %v, want 100", block.Utilization)
	}
}

// Several blocks inside one week still leave the weekly window as its own
// baseline, so the explanation has to cover that case too.
func TestEstimateFlagsLimitedHistoryWithOneWeekOfBlocks(t *testing.T) {
	t.Parallel()

	database := newStore(t,
		event("claude", now.Add(-100*time.Hour), 400),
		event("claude", now.Add(-50*time.Hour), 900),
		event("claude", now.Add(-time.Hour), 200),
	)

	results, err := limits.Estimate(context.Background(), database, now, limits.Budgets{})
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}

	claude := sourceOf(t, results, "claude")
	// Three separate blocks, so the five-hour comparison is real.
	if block := windowOf(t, claude, limits.KindFiveHour); block.Utilization != 200.0/900.0*100 {
		t.Fatalf("five-hour utilization = %v", block.Utilization)
	}
	// One week bucket, so the weekly comparison is not.
	if week := windowOf(t, claude, limits.KindSevenDay); week.Utilization != 100 {
		t.Fatalf("seven-day utilization = %v, want 100", week.Utilization)
	}
	if claude.Message == "" {
		t.Fatal("message is empty, want a limited-history explanation for the weekly window")
	}
}

func TestEstimateSortsSourcesAndSkipsUnusedAgents(t *testing.T) {
	t.Parallel()

	database := newStore(t,
		event("codex", now.Add(-time.Hour), 10),
		event("claude", now.Add(-time.Hour), 20),
	)

	results, err := limits.Estimate(context.Background(), database, now, limits.Budgets{})
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}

	if len(results) != 2 || results[0].Source != "claude" || results[1].Source != "codex" {
		t.Fatalf("sources = %+v, want claude then codex", results)
	}
}

func TestEstimateReturnsNothingForEmptyHistory(t *testing.T) {
	t.Parallel()

	results, err := limits.Estimate(context.Background(), newStore(t), now, limits.Budgets{})
	if err != nil {
		t.Fatalf("estimate: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("sources = %+v, want none", results)
	}
}
