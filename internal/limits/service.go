package limits

import (
	"context"
	"sort"
	"sync"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/store"
)

// DefaultCacheTTL bounds how often providers are contacted. Anthropic's usage
// endpoint throttles callers well below this interval, so reloading the
// dashboard repeatedly must not turn into a request per reload.
const DefaultCacheTTL = 180 * time.Second

// Service assembles the limits report from local history and, when the
// operator has opted in, from authoritative providers.
//
// With no providers configured it never opens a socket: the report is derived
// entirely from stored usage events.
type Service struct {
	Store     *store.Store
	Providers []Provider
	Budgets   Budgets
	CacheTTL  time.Duration

	// refresh serializes provider access so that concurrent dashboard loads
	// produce one fetch rather than one each.
	refresh sync.Mutex
}

// Report returns every source's limit windows.
//
// A live window always wins over a derived one. When a provider fails, the
// last stored snapshot is served and marked stale; with no snapshot to fall
// back on the derived estimate is served and carries the provider's error, so
// the dashboard degrades to a weaker claim rather than to a blank panel.
func (s *Service) Report(ctx context.Context, now time.Time, location *time.Location) (Report, error) {
	estimated, err := Estimate(ctx, s.Store, now, s.Budgets)
	if err != nil {
		return Report{}, err
	}

	bySource := make(map[string]SourceLimits, len(estimated))
	for _, estimate := range estimated {
		bySource[estimate.Source] = estimate
	}
	for _, live := range s.live(ctx, now) {
		bySource[live.Source] = merge(bySource[live.Source], live)
	}

	report := Report{Timezone: location.String(), Sources: make([]SourceLimits, 0, len(bySource))}
	for _, limits := range bySource {
		report.Sources = append(report.Sources, limits)
	}
	sort.Slice(report.Sources, func(i, j int) bool { return report.Sources[i].Source < report.Sources[j].Source })
	report.Mode = mode(report.Sources)
	return report, nil
}

// merge folds a provider result into whatever the estimator produced for the
// same source. A failed provider keeps the estimate and contributes only its
// explanation, so a broken credential weakens the claim instead of emptying
// the panel.
func merge(estimate, live SourceLimits) SourceLimits {
	if len(live.Windows) > 0 {
		return live
	}
	if estimate.Source == "" {
		return live
	}
	if live.Message != "" {
		estimate.Message = live.Message
	}
	return estimate
}

func mode(sources []SourceLimits) string {
	var live, estimated int
	for _, candidate := range sources {
		switch candidate.Origin {
		case OriginLive:
			live++
		case OriginEstimated:
			estimated++
		case OriginUnavailable:
		}
	}
	switch {
	case live > 0 && estimated == 0:
		return string(OriginLive)
	case live > 0:
		return "mixed"
	default:
		return string(OriginEstimated)
	}
}

// live returns one result per configured provider, reading from the snapshot
// cache when it is fresh enough and fetching otherwise.
func (s *Service) live(ctx context.Context, now time.Time) []SourceLimits {
	if len(s.Providers) == 0 {
		return nil
	}
	s.refresh.Lock()
	defer s.refresh.Unlock()

	cached, err := s.Store.LoadLimitSnapshots(ctx)
	if err != nil {
		// A cache that cannot be read is not a reason to skip the fetch.
		cached = nil
	}
	byCachedSource := make(map[string][]store.LimitSnapshot, len(cached))
	for _, snapshot := range cached {
		byCachedSource[snapshot.Source] = append(byCachedSource[snapshot.Source], snapshot)
	}

	results := make([]SourceLimits, 0, len(s.Providers))
	for _, candidate := range s.Providers {
		results = append(results, s.fetchOne(ctx, candidate, byCachedSource[candidate.Source()], now))
	}
	return results
}

func (s *Service) fetchOne(ctx context.Context, candidate Provider, cached []store.LimitSnapshot, now time.Time) SourceLimits {
	name := candidate.Source()
	if fresh, ok := fromCache(name, cached, now, s.ttl()); ok {
		return fresh
	}

	windows, err := candidate.Fetch(ctx, now)
	if err != nil {
		if stale, ok := fromCache(name, cached, now, 0); ok {
			stale.Stale = true
			stale.Message = err.Error()
			return stale
		}
		return SourceLimits{Source: name, Origin: OriginUnavailable, Message: err.Error(), Windows: []Window{}}
	}

	for index := range windows {
		windows[index].Label = liveLabel(name, windows[index].Kind)
	}
	fetchedAt := now.UTC()
	s.save(ctx, name, windows, fetchedAt)
	return SourceLimits{Source: name, Origin: OriginLive, FetchedAt: &fetchedAt, Windows: windows}
}

// fromCache rebuilds a source from stored snapshots. A zero maxAge accepts any
// age, which is how a failed fetch falls back to the last known values.
func fromCache(name string, cached []store.LimitSnapshot, now time.Time, maxAge time.Duration) (SourceLimits, bool) {
	if len(cached) == 0 {
		return SourceLimits{}, false
	}
	fetchedAt := cached[0].FetchedAt
	if maxAge > 0 && now.Sub(fetchedAt) > maxAge {
		return SourceLimits{}, false
	}
	windows := make([]Window, 0, len(cached))
	for _, snapshot := range cached {
		windows = append(windows, Window{
			Kind:        Kind(snapshot.Kind),
			Label:       liveLabel(name, Kind(snapshot.Kind)),
			Utilization: snapshot.Utilization,
			ResetsAt:    snapshot.ResetsAt,
		})
	}
	return SourceLimits{Source: name, Origin: OriginLive, FetchedAt: &fetchedAt, Windows: windows}, true
}

func (s *Service) save(ctx context.Context, name string, windows []Window, fetchedAt time.Time) {
	snapshots := make([]store.LimitSnapshot, 0, len(windows))
	for _, window := range windows {
		snapshots = append(snapshots, store.LimitSnapshot{
			Kind:        string(window.Kind),
			FetchedAt:   fetchedAt,
			Utilization: window.Utilization,
			ResetsAt:    window.ResetsAt,
		})
	}
	// A cache that cannot be written costs a request on the next load and
	// nothing else, so the report is served either way.
	_ = s.Store.SaveLimitSnapshots(ctx, name, snapshots)
}

func (s *Service) ttl() time.Duration {
	if s.CacheTTL > 0 {
		return s.CacheTTL
	}
	return DefaultCacheTTL
}
