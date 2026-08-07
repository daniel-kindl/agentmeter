package limits

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/source"
	"github.com/daniel-kindl/agentmeter/internal/store"
)

// FiveHourWindow is the length of the rolling block both agents enforce.
const FiveHourWindow = 5 * time.Hour

// SevenDayWindow is the length of the weekly allowance.
const SevenDayWindow = 7 * 24 * time.Hour

// Budgets replaces the automatic baseline with explicit token ceilings. A zero
// field keeps the automatic baseline for that window.
type Budgets struct {
	FiveHour int64
	SevenDay int64
}

// Estimate derives limit windows for every source with stored usage.
//
// Neither agent records its own limit accounting in the session logs, so the
// windows returned here measure stored tokens against the busiest comparable
// window in local history rather than against a published quota. They carry
// OriginEstimated so callers never present them as the agent's own numbers.
func Estimate(ctx context.Context, database *store.Store, now time.Time, budgets Budgets) ([]SourceLimits, error) {
	now = now.UTC()
	sources := make(map[string]*estimator)
	err := database.VisitUsageEvents(ctx, nil, func(event source.UsageEvent) error {
		state := sources[event.Source]
		if state == nil {
			state = newEstimator(now)
			sources[event.Source] = state
		}
		state.observe(event)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("estimate usage limits: %w", err)
	}

	estimated := make([]SourceLimits, 0, len(sources))
	for name, state := range sources {
		estimated = append(estimated, state.finish(name, budgets))
	}
	sort.Slice(estimated, func(i, j int) bool { return estimated[i].Source < estimated[j].Source })
	return estimated, nil
}

// block is one five-hour window of contiguous activity. A block opens at the
// first event floored to the hour and closes once five hours have passed since
// either its start or its most recent event, matching how the usage oracle
// groups session blocks.
type block struct {
	start  time.Time
	last   time.Time
	tokens int64
}

type estimator struct {
	now    time.Time
	blocks []block
	weeks  map[int64]int64
}

func newEstimator(now time.Time) *estimator {
	return &estimator{now: now, weeks: make(map[int64]int64)}
}

func (e *estimator) observe(event source.UsageEvent) {
	timestamp := event.Timestamp.UTC()
	total := event.InputTokens + event.OutputTokens +
		event.CacheCreationInputTokens + event.CacheReadInputTokens

	current := e.current()
	if current == nil || !timestamp.Before(current.start.Add(FiveHourWindow)) || !timestamp.Before(current.last.Add(FiveHourWindow)) {
		e.blocks = append(e.blocks, block{start: floorHour(timestamp), last: timestamp})
		current = e.current()
	}
	current.last = timestamp
	current.tokens += total

	e.weeks[weekIndex(e.now, timestamp)] += total
}

func (e *estimator) current() *block {
	if len(e.blocks) == 0 {
		return nil
	}
	return &e.blocks[len(e.blocks)-1]
}

func (e *estimator) finish(name string, budgets Budgets) SourceLimits {
	result := SourceLimits{Source: name, Origin: OriginEstimated, Windows: []Window{}}
	if len(e.blocks) < 2 {
		result.Message = "limited history: the comparison baseline grows as agentmeter records more usage"
	}

	var activeTokens int64
	var activeReset *time.Time
	if active := e.current(); active != nil && e.now.Before(active.start.Add(FiveHourWindow)) && e.now.Before(active.last.Add(FiveHourWindow)) {
		activeTokens = active.tokens
		reset := active.start.Add(FiveHourWindow)
		activeReset = &reset
	}
	var busiestBlock int64
	for _, candidate := range e.blocks {
		busiestBlock = max(busiestBlock, candidate.tokens)
	}
	var busiestWeek int64
	for _, tokens := range e.weeks {
		busiestWeek = max(busiestWeek, tokens)
	}

	result.Windows = append(result.Windows,
		estimatedWindow(KindFiveHour, "5-hour block", activeTokens, pick(budgets.FiveHour, busiestBlock), activeReset),
		estimatedWindow(KindSevenDay, "Rolling 7 days", e.weeks[0], pick(budgets.SevenDay, busiestWeek), nil),
	)
	return result
}

func estimatedWindow(kind Kind, label string, used, budget int64, resetsAt *time.Time) Window {
	window := Window{
		Kind:        kind,
		Label:       label,
		Utilization: utilization(used, budget),
		ResetsAt:    resetsAt,
		UsedTokens:  &used,
	}
	if budget > 0 {
		window.BudgetTokens = &budget
	}
	return window
}

// pick prefers an operator-supplied ceiling over the derived baseline.
func pick(configured, derived int64) int64 {
	if configured > 0 {
		return configured
	}
	return derived
}

func floorHour(value time.Time) time.Time {
	return time.Date(value.Year(), value.Month(), value.Day(), value.Hour(), 0, 0, 0, time.UTC)
}

// weekIndex buckets a timestamp into non-overlapping seven-day spans counted
// back from now. Index zero is the trailing seven days; later indexes are older.
func weekIndex(now, timestamp time.Time) int64 {
	elapsed := now.Sub(timestamp)
	if elapsed < 0 {
		return 0
	}
	return int64(elapsed / SevenDayWindow)
}
