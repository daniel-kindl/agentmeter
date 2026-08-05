package pricing

import (
	"fmt"
	"strings"

	"github.com/daniel-kindl/agentmeter/internal/source"
)

// Rates are micro-US-dollars charged per million tokens.
type Rates struct {
	Input       int64
	Output      int64
	CacheCreate int64
	CacheRead   int64
}

// Snapshot is reviewed against the vendors' public pricing pages. Unknown
// models deliberately remain unpriced instead of silently inheriting a rate.
var Snapshot = []struct {
	Match string
	Rates Rates
}{
	{"claude-opus-4", Rates{Input: 15_000_000, Output: 75_000_000, CacheCreate: 18_750_000, CacheRead: 1_500_000}},
	{"claude-sonnet-4", Rates{Input: 3_000_000, Output: 15_000_000, CacheCreate: 3_750_000, CacheRead: 300_000}},
	{"claude-3-7-sonnet", Rates{Input: 3_000_000, Output: 15_000_000, CacheCreate: 3_750_000, CacheRead: 300_000}},
	{"claude-3-5-sonnet", Rates{Input: 3_000_000, Output: 15_000_000, CacheCreate: 3_750_000, CacheRead: 300_000}},
	{"claude-3-5-haiku", Rates{Input: 800_000, Output: 4_000_000, CacheCreate: 1_000_000, CacheRead: 80_000}},
	{"claude-3-haiku", Rates{Input: 250_000, Output: 1_250_000, CacheCreate: 300_000, CacheRead: 30_000}},
	{"gpt-5.2", Rates{Input: 1_750_000, Output: 14_000_000, CacheRead: 175_000}},
	{"gpt-5.1", Rates{Input: 1_250_000, Output: 10_000_000, CacheRead: 125_000}},
	{"gpt-5", Rates{Input: 1_250_000, Output: 10_000_000, CacheRead: 125_000}},
}

// CostMicroUSD returns an event cost in micro-US-dollars and whether its model
// is present in the bundled snapshot.
func CostMicroUSD(event source.UsageEvent) (int64, bool) {
	rates, ok := Lookup(event.Model)
	if !ok {
		return 0, false
	}
	weighted := event.InputTokens*rates.Input +
		event.OutputTokens*rates.Output +
		event.CacheCreationInputTokens*rates.CacheCreate +
		event.CacheReadInputTokens*rates.CacheRead
	return weighted / 1_000_000, true
}

// Lookup returns the longest matching model prefix from the snapshot.
func Lookup(model string) (Rates, bool) {
	model = strings.ToLower(strings.TrimSpace(model))
	var selected Rates
	longest := 0
	for _, entry := range Snapshot {
		if strings.HasPrefix(model, entry.Match) && len(entry.Match) > longest {
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
