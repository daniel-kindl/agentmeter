package report

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/pricing"
	"github.com/daniel-kindl/agentmeter/internal/source"
	"github.com/daniel-kindl/agentmeter/internal/store"
)

// Aggregate contains the four normalized token categories and derived cost.
type Aggregate struct {
	InputTokens              int64  `json:"input_tokens"`
	OutputTokens             int64  `json:"output_tokens"`
	CacheCreationInputTokens int64  `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64  `json:"cache_read_input_tokens"`
	CostUSD                  string `json:"cost_usd"`
}

// Daily is one local-calendar-day bucket.
type Daily struct {
	Date string `json:"date"`
	Aggregate
}

// Breakdown is one source or model bucket.
type Breakdown struct {
	Name string `json:"name"`
	Aggregate
}

// Dashboard is the complete response consumed by the local web application.
type Dashboard struct {
	Range          string      `json:"range"`
	Timezone       string      `json:"timezone"`
	Totals         Aggregate   `json:"totals"`
	Daily          []Daily     `json:"daily"`
	BySource       []Breakdown `json:"by_source"`
	ByModel        []Breakdown `json:"by_model"`
	CostComplete   bool        `json:"cost_complete"`
	UnpricedModels []string    `json:"unpriced_models"`
}

type accumulator struct {
	Aggregate
	costMicroUSD int64
}

// BuildDashboard streams the requested range and aggregates it in memory by
// day, source, and model.
func BuildDashboard(ctx context.Context, database *store.Store, selectedRange string, now time.Time, location *time.Location) (Dashboard, error) {
	since, err := rangeStart(selectedRange, now, location)
	if err != nil {
		return Dashboard{}, err
	}
	result := Dashboard{Range: selectedRange, Timezone: location.String(), CostComplete: true, Daily: []Daily{}, BySource: []Breakdown{}, ByModel: []Breakdown{}, UnpricedModels: []string{}}
	var totals accumulator
	daily := make(map[string]*accumulator)
	bySource := make(map[string]*accumulator)
	byModel := make(map[string]*accumulator)
	unpriced := make(map[string]struct{})
	err = database.VisitUsageEvents(ctx, since, func(event source.UsageEvent) error {
		cost, priced := pricing.CostMicroUSD(event)
		if !priced {
			result.CostComplete = false
			unpriced[event.Model] = struct{}{}
		}
		add(&totals, event, cost)
		date := event.Timestamp.In(location).Format("2006-01-02")
		add(bucket(daily, date), event, cost)
		add(bucket(bySource, event.Source), event, cost)
		add(bucket(byModel, event.Model), event, cost)
		return nil
	})
	if err != nil {
		return Dashboard{}, fmt.Errorf("build dashboard: %w", err)
	}
	result.Totals = finish(totals)
	for key, value := range daily {
		result.Daily = append(result.Daily, Daily{Date: key, Aggregate: finish(*value)})
	}
	for key, value := range bySource {
		result.BySource = append(result.BySource, Breakdown{Name: key, Aggregate: finish(*value)})
	}
	for key, value := range byModel {
		result.ByModel = append(result.ByModel, Breakdown{Name: key, Aggregate: finish(*value)})
	}
	for model := range unpriced {
		result.UnpricedModels = append(result.UnpricedModels, model)
	}
	sort.Slice(result.Daily, func(i, j int) bool { return result.Daily[i].Date < result.Daily[j].Date })
	sort.Slice(result.BySource, func(i, j int) bool { return result.BySource[i].Name < result.BySource[j].Name })
	sort.Slice(result.ByModel, func(i, j int) bool {
		return totalTokens(result.ByModel[i].Aggregate) > totalTokens(result.ByModel[j].Aggregate)
	})
	sort.Strings(result.UnpricedModels)
	return result, nil
}

func rangeStart(selectedRange string, now time.Time, location *time.Location) (*time.Time, error) {
	if selectedRange == "all" {
		return nil, nil
	}
	days := 0
	switch selectedRange {
	case "7d":
		days = 7
	case "30d":
		days = 30
	default:
		return nil, fmt.Errorf("unsupported range %q", selectedRange)
	}
	local := now.In(location)
	start := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, location).AddDate(0, 0, -(days - 1))
	return &start, nil
}

func bucket(buckets map[string]*accumulator, key string) *accumulator {
	value := buckets[key]
	if value == nil {
		value = &accumulator{}
		buckets[key] = value
	}
	return value
}

func add(target *accumulator, event source.UsageEvent, cost int64) {
	target.InputTokens += event.InputTokens
	target.OutputTokens += event.OutputTokens
	target.CacheCreationInputTokens += event.CacheCreationInputTokens
	target.CacheReadInputTokens += event.CacheReadInputTokens
	target.costMicroUSD += cost
}

func finish(value accumulator) Aggregate {
	value.CostUSD = pricing.FormatUSD(value.costMicroUSD)
	return value.Aggregate
}

func totalTokens(value Aggregate) int64 {
	return value.InputTokens + value.OutputTokens + value.CacheCreationInputTokens + value.CacheReadInputTokens
}
