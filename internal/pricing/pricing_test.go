package pricing_test

import (
	"testing"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/pricing"
	"github.com/daniel-kindl/agentmeter/internal/source"
)

func TestCostMicroUSD(t *testing.T) {
	t.Parallel()

	event := source.UsageEvent{Model: "claude-sonnet-4-synthetic", InputTokens: 1_000_000, OutputTokens: 100_000, CacheCreationInputTokens: 200_000, CacheReadInputTokens: 300_000}
	got, ok := pricing.CostMicroUSD(event)
	if !ok {
		t.Fatal("known model reported unknown")
	}
	if got != 5_340_000 {
		t.Fatalf("cost = %d microUSD, want 5340000", got)
	}
	if formatted := pricing.FormatUSD(got); formatted != "5.340000" {
		t.Fatalf("formatted cost = %q", formatted)
	}
}

func TestUnknownModel(t *testing.T) {
	t.Parallel()
	if _, ok := pricing.Lookup("future-model"); ok {
		t.Fatal("unknown model received pricing")
	}
}

func TestEffectiveDatedPrice(t *testing.T) {
	t.Parallel()

	intro, ok := pricing.LookupAt("claude-sonnet-5-synthetic", time.Date(2026, time.August, 31, 23, 59, 59, 0, time.UTC))
	if !ok || intro.Input != 2_000_000 || intro.Output != 10_000_000 {
		t.Fatalf("introductory rates = %+v, known = %v", intro, ok)
	}
	standard, ok := pricing.LookupAt("claude-sonnet-5-synthetic", time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC))
	if !ok || standard.Input != 3_000_000 || standard.Output != 15_000_000 {
		t.Fatalf("standard rates = %+v, known = %v", standard, ok)
	}
}
