package pricing_test

import (
	"testing"

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
