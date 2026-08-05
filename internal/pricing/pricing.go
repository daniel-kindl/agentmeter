package pricing

import (
	"fmt"
	"strings"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/source"
)

// Rates are micro-US-dollars charged per million tokens.
type Rates struct {
	Input       int64
	Output      int64
	CacheCreate int64
	CacheRead   int64
}

// Price is one effective-dated entry in the bundled pricing snapshot. Until is
// exclusive; a zero boundary is open-ended.
type Price struct {
	Match string
	From  time.Time
	Until time.Time
	Rates Rates
}

var sonnet5StandardPricing = time.Date(2026, time.September, 1, 0, 0, 0, 0, time.UTC)

// Snapshot is reviewed against the vendors' public pricing pages. Unknown
// models deliberately remain unpriced instead of silently inheriting a rate.
var Snapshot = []Price{
	{Match: "claude-fable-5", Rates: Rates{Input: 10_000_000, Output: 50_000_000, CacheCreate: 12_500_000, CacheRead: 1_000_000}},
	{Match: "claude-opus-5", Rates: Rates{Input: 5_000_000, Output: 25_000_000, CacheCreate: 6_250_000, CacheRead: 500_000}},
	{Match: "claude-sonnet-5", Until: sonnet5StandardPricing, Rates: Rates{Input: 2_000_000, Output: 10_000_000, CacheCreate: 2_500_000, CacheRead: 200_000}},
	{Match: "claude-sonnet-5", From: sonnet5StandardPricing, Rates: Rates{Input: 3_000_000, Output: 15_000_000, CacheCreate: 3_750_000, CacheRead: 300_000}},
	{Match: "claude-haiku-4-5", Rates: Rates{Input: 1_000_000, Output: 5_000_000, CacheCreate: 1_250_000, CacheRead: 100_000}},
	{Match: "claude-opus-4", Rates: Rates{Input: 15_000_000, Output: 75_000_000, CacheCreate: 18_750_000, CacheRead: 1_500_000}},
	{Match: "claude-sonnet-4", Rates: Rates{Input: 3_000_000, Output: 15_000_000, CacheCreate: 3_750_000, CacheRead: 300_000}},
	{Match: "claude-3-7-sonnet", Rates: Rates{Input: 3_000_000, Output: 15_000_000, CacheCreate: 3_750_000, CacheRead: 300_000}},
	{Match: "claude-3-5-sonnet", Rates: Rates{Input: 3_000_000, Output: 15_000_000, CacheCreate: 3_750_000, CacheRead: 300_000}},
	{Match: "claude-3-5-haiku", Rates: Rates{Input: 800_000, Output: 4_000_000, CacheCreate: 1_000_000, CacheRead: 80_000}},
	{Match: "claude-3-haiku", Rates: Rates{Input: 250_000, Output: 1_250_000, CacheCreate: 300_000, CacheRead: 30_000}},
	{Match: "gpt-5.2", Rates: Rates{Input: 1_750_000, Output: 14_000_000, CacheRead: 175_000}},
	{Match: "gpt-5.1", Rates: Rates{Input: 1_250_000, Output: 10_000_000, CacheRead: 125_000}},
	{Match: "gpt-5", Rates: Rates{Input: 1_250_000, Output: 10_000_000, CacheRead: 125_000}},
}

// CostMicroUSD returns an event cost in micro-US-dollars and whether its model
// is present in the bundled snapshot.
func CostMicroUSD(event source.UsageEvent) (int64, bool) {
	rates, ok := LookupAt(event.Model, event.Timestamp)
	if !ok {
		return 0, false
	}
	weighted := event.InputTokens*rates.Input +
		event.OutputTokens*rates.Output +
		event.CacheCreationInputTokens*rates.CacheCreate +
		event.CacheReadInputTokens*rates.CacheRead
	return weighted / 1_000_000, true
}

// Lookup returns the currently effective longest matching model prefix.
func Lookup(model string) (Rates, bool) {
	return LookupAt(model, time.Now())
}

// LookupAt returns the longest matching model prefix effective at the supplied
// instant. Zero timestamps use the current time for callers without event time.
func LookupAt(model string, at time.Time) (Rates, bool) {
	model = strings.ToLower(strings.TrimSpace(model))
	if at.IsZero() {
		at = time.Now()
	}
	var selected Rates
	longest := 0
	for _, entry := range Snapshot {
		if strings.HasPrefix(model, entry.Match) &&
			(entry.From.IsZero() || !at.Before(entry.From)) &&
			(entry.Until.IsZero() || at.Before(entry.Until)) &&
			len(entry.Match) > longest {
			selected = entry.Rates
			longest = len(entry.Match)
		}
	}
	return selected, longest > 0
}

// FormatUSD renders micro-US-dollars as an exact six-decimal string.
func FormatUSD(microUSD int64) string {
	whole := microUSD / 1_000_000
	fraction := microUSD % 1_000_000
	return fmt.Sprintf("%d.%06d", whole, fraction)
}
