package limits_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/limits"
)

// stubProvider stands in for a network provider and counts its calls so the
// cache can be observed.
type stubProvider struct {
	name    string
	windows []limits.Window
	err     error

	mu    sync.Mutex
	calls int
}

func (s *stubProvider) Source() string { return s.name }

func (s *stubProvider) Fetch(context.Context, time.Time) ([]limits.Window, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return append([]limits.Window(nil), s.windows...), nil
}

func (s *stubProvider) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// enableLive turns on authoritative fetching, which is off until the operator
// asks for it. Tests that exercise providers have to opt in the same way.
func enableLive(t *testing.T, service *limits.Service) *limits.Service {
	t.Helper()
	if err := service.SetLive(context.Background(), true); err != nil {
		t.Fatalf("enable live limits: %v", err)
	}
	return service
}

func liveWindows() []limits.Window {
	return []limits.Window{
		{Kind: limits.KindFiveHour, Utilization: 91},
		{Kind: limits.KindSevenDay, Utilization: 81},
	}
}

func TestReportPrefersLiveWindowsOverEstimates(t *testing.T) {
	t.Parallel()

	database := newStore(t, event("claude", now.Add(-time.Hour), 100))
	service := enableLive(t, &limits.Service{
		Store:     database,
		Providers: []limits.Provider{&stubProvider{name: "claude", windows: liveWindows()}},
	})

	report, err := service.Report(context.Background(), now, time.UTC)
	if err != nil {
		t.Fatalf("report: %v", err)
	}

	if report.Mode != "live" {
		t.Fatalf("mode = %q, want live", report.Mode)
	}
	claude := sourceOf(t, report.Sources, "claude")
	if claude.Origin != limits.OriginLive || claude.Stale {
		t.Fatalf("claude = %+v, want a fresh live source", claude)
	}
	block := windowOf(t, claude, limits.KindFiveHour)
	if block.Utilization != 91 {
		t.Fatalf("five-hour utilization = %v, want 91", block.Utilization)
	}
	// Labels come from the limits package so a cached window reads the same
	// as a freshly fetched one.
	if block.Label != "5-hour session" {
		t.Fatalf("five-hour label = %q, want the agent's own wording", block.Label)
	}
	if block.UsedTokens != nil {
		t.Fatalf("live window carries token counts %v, want none", block.UsedTokens)
	}
}

func TestReportServesEstimatesWhenNoProviderIsConfigured(t *testing.T) {
	t.Parallel()

	database := newStore(t, event("claude", now.Add(-time.Hour), 100))
	service := &limits.Service{Store: database}

	report, err := service.Report(context.Background(), now, time.UTC)
	if err != nil {
		t.Fatalf("report: %v", err)
	}

	if report.Mode != "estimated" {
		t.Fatalf("mode = %q, want estimated", report.Mode)
	}
	if got := sourceOf(t, report.Sources, "claude").Origin; got != limits.OriginEstimated {
		t.Fatalf("origin = %q, want estimated", got)
	}
}

func TestReportReusesCachedSnapshotsWithinTheTTL(t *testing.T) {
	t.Parallel()

	database := newStore(t, event("claude", now.Add(-time.Hour), 100))
	stub := &stubProvider{name: "claude", windows: liveWindows()}
	service := enableLive(t, &limits.Service{Store: database, Providers: []limits.Provider{stub}, CacheTTL: 180 * time.Second})
	ctx := context.Background()

	if _, err := service.Report(ctx, now, time.UTC); err != nil {
		t.Fatalf("first report: %v", err)
	}
	if _, err := service.Report(ctx, now.Add(time.Minute), time.UTC); err != nil {
		t.Fatalf("second report: %v", err)
	}
	if got := stub.callCount(); got != 1 {
		t.Fatalf("provider calls = %d, want 1 within the TTL", got)
	}

	report, err := service.Report(ctx, now.Add(4*time.Minute), time.UTC)
	if err != nil {
		t.Fatalf("third report: %v", err)
	}
	if got := stub.callCount(); got != 2 {
		t.Fatalf("provider calls = %d, want a refetch past the TTL", got)
	}
	if got := sourceOf(t, report.Sources, "claude").Origin; got != limits.OriginLive {
		t.Fatalf("origin = %q, want live", got)
	}
}

// A provider that starts failing must not blank the panel. The last known
// values stay on screen, marked stale and carrying the reason.
func TestReportFallsBackToStaleSnapshotWhenAProviderFails(t *testing.T) {
	t.Parallel()

	database := newStore(t, event("claude", now.Add(-time.Hour), 100))
	stub := &stubProvider{name: "claude", windows: liveWindows()}
	service := enableLive(t, &limits.Service{Store: database, Providers: []limits.Provider{stub}, CacheTTL: time.Minute})
	ctx := context.Background()

	if _, err := service.Report(ctx, now, time.UTC); err != nil {
		t.Fatalf("first report: %v", err)
	}
	stub.err = errors.New("Anthropic rejected the stored credentials")

	report, err := service.Report(ctx, now.Add(time.Hour), time.UTC)
	if err != nil {
		t.Fatalf("second report: %v", err)
	}

	claude := sourceOf(t, report.Sources, "claude")
	if !claude.Stale || claude.Origin != limits.OriginLive {
		t.Fatalf("claude = %+v, want stale live values", claude)
	}
	if claude.Message == "" {
		t.Fatal("stale source carries no explanation")
	}
	if windowOf(t, claude, limits.KindFiveHour).Utilization != 91 {
		t.Fatal("stale source lost its last known utilization")
	}
	if claude.FetchedAt == nil || !claude.FetchedAt.Equal(now) {
		t.Fatalf("fetch time = %v, want the original fetch at %v", claude.FetchedAt, now)
	}
}

// With nothing cached, a failing provider degrades to the derived estimate
// rather than to an empty panel.
func TestReportFallsBackToEstimateWhenAProviderFailsWithoutCache(t *testing.T) {
	t.Parallel()

	database := newStore(t, event("claude", now.Add(-time.Hour), 100))
	stub := &stubProvider{name: "claude", err: errors.New("no Claude credentials found")}
	service := enableLive(t, &limits.Service{Store: database, Providers: []limits.Provider{stub}})

	report, err := service.Report(context.Background(), now, time.UTC)
	if err != nil {
		t.Fatalf("report: %v", err)
	}

	claude := sourceOf(t, report.Sources, "claude")
	if claude.Origin != limits.OriginEstimated {
		t.Fatalf("origin = %q, want the estimate to survive", claude.Origin)
	}
	if claude.Message != "no Claude credentials found" {
		t.Fatalf("message = %q, want the provider's explanation", claude.Message)
	}
	if windowOf(t, claude, limits.KindFiveHour).UsedTokens == nil {
		t.Fatal("estimated window lost its token counts")
	}
	if report.Mode != "estimated" {
		t.Fatalf("mode = %q, want estimated", report.Mode)
	}
}

// A provider can report a source that has no stored usage at all, which is what
// happens on a fresh machine before the first scan.
func TestReportIncludesLiveSourcesWithNoStoredUsage(t *testing.T) {
	t.Parallel()

	database := newStore(t, event("claude", now.Add(-time.Hour), 100))
	service := enableLive(t, &limits.Service{
		Store: database,
		Providers: []limits.Provider{
			&stubProvider{name: "codex", windows: liveWindows()},
		},
	})

	report, err := service.Report(context.Background(), now, time.UTC)
	if err != nil {
		t.Fatalf("report: %v", err)
	}

	if len(report.Sources) != 2 {
		t.Fatalf("sources = %+v, want claude and codex", report.Sources)
	}
	if report.Mode != "mixed" {
		t.Fatalf("mode = %q, want mixed", report.Mode)
	}
	if got := sourceOf(t, report.Sources, "codex").Origin; got != limits.OriginLive {
		t.Fatalf("codex origin = %q, want live", got)
	}
}

func TestReportMarksASourceUnavailableWithNoEstimateAndNoCache(t *testing.T) {
	t.Parallel()

	service := enableLive(t, &limits.Service{
		Store:     newStore(t),
		Providers: []limits.Provider{&stubProvider{name: "codex", err: errors.New("run codex login")}},
	})

	report, err := service.Report(context.Background(), now, time.UTC)
	if err != nil {
		t.Fatalf("report: %v", err)
	}

	codex := sourceOf(t, report.Sources, "codex")
	if codex.Origin != limits.OriginUnavailable || codex.Message != "run codex login" {
		t.Fatalf("codex = %+v, want an unavailable source explaining itself", codex)
	}
}

// Concurrent dashboard loads must produce one fetch, not one each.
func TestReportSerializesConcurrentRefreshes(t *testing.T) {
	t.Parallel()

	database := newStore(t, event("claude", now.Add(-time.Hour), 100))
	stub := &stubProvider{name: "claude", windows: liveWindows()}
	service := enableLive(t, &limits.Service{Store: database, Providers: []limits.Provider{stub}, CacheTTL: time.Hour})

	var group sync.WaitGroup
	for range 8 {
		group.Add(1)
		go func() {
			defer group.Done()
			if _, err := service.Report(context.Background(), now, time.UTC); err != nil {
				t.Errorf("report: %v", err)
			}
		}()
	}
	group.Wait()

	if got := stub.callCount(); got != 1 {
		t.Fatalf("provider calls = %d, want 1", got)
	}
}

// Providers exist as soon as they are configured, but configuring one must not
// contact anything. Only the switch does that.
func TestReportContactsNoProviderUntilLiveIsOn(t *testing.T) {
	t.Parallel()

	database := newStore(t, event("claude", now.Add(-time.Hour), 100))
	stub := &stubProvider{name: "claude", windows: liveWindows()}
	service := &limits.Service{Store: database, Providers: []limits.Provider{stub}}

	report, err := service.Report(context.Background(), now, time.UTC)
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if stub.callCount() != 0 {
		t.Fatalf("provider was called %d times with live off", stub.callCount())
	}
	if report.Live {
		t.Fatal("report says live is on before it was enabled")
	}
	// The page still needs to know the switch exists.
	if !report.Configurable {
		t.Fatal("report says the switch is unavailable despite a configured provider")
	}
	if got := sourceOf(t, report.Sources, "claude").Origin; got != limits.OriginEstimated {
		t.Fatalf("origin = %q, want estimated", got)
	}
}

func TestSetLiveSurvivesANewService(t *testing.T) {
	t.Parallel()

	database := newStore(t, event("claude", now.Add(-time.Hour), 100))
	stub := &stubProvider{name: "claude", windows: liveWindows()}
	enableLive(t, &limits.Service{Store: database, Providers: []limits.Provider{stub}})

	// A restart builds a fresh service against the same database. The choice
	// made in the dashboard has to come back with it.
	restarted := &limits.Service{Store: database, Providers: []limits.Provider{stub}}
	if err := restarted.RestoreLive(context.Background(), false); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if !restarted.Live() {
		t.Fatal("the stored live preference was not restored")
	}

	report, err := restarted.Report(context.Background(), now, time.UTC)
	if err != nil {
		t.Fatalf("report: %v", err)
	}
	if got := sourceOf(t, report.Sources, "claude").Origin; got != limits.OriginLive {
		t.Fatalf("origin = %q, want live", got)
	}
}

// Turning the switch off must stop the fetching, not merely relabel it.
func TestSetLiveOffStopsContactingProviders(t *testing.T) {
	t.Parallel()

	database := newStore(t, event("claude", now.Add(-time.Hour), 100))
	stub := &stubProvider{name: "claude", windows: liveWindows()}
	service := enableLive(t, &limits.Service{Store: database, Providers: []limits.Provider{stub}})
	if _, err := service.Report(context.Background(), now, time.UTC); err != nil {
		t.Fatalf("first report: %v", err)
	}
	before := stub.callCount()

	if err := service.SetLive(context.Background(), false); err != nil {
		t.Fatalf("disable: %v", err)
	}
	report, err := service.Report(context.Background(), now.Add(time.Hour), time.UTC)
	if err != nil {
		t.Fatalf("second report: %v", err)
	}
	if stub.callCount() != before {
		t.Fatalf("provider called %d more times after the switch was turned off", stub.callCount()-before)
	}
	if got := sourceOf(t, report.Sources, "claude").Origin; got != limits.OriginEstimated {
		t.Fatalf("origin = %q, want estimated once live is off", got)
	}
}

// A command-line --live is the more explicit instruction for the run it is
// given in, so it wins over a stored preference of off.
func TestRestoreLiveLetsTheFlagWinOverStoredOff(t *testing.T) {
	t.Parallel()

	database := newStore(t)
	stub := &stubProvider{name: "claude", windows: liveWindows()}
	service := &limits.Service{Store: database, Providers: []limits.Provider{stub}}
	if err := service.SetLive(context.Background(), false); err != nil {
		t.Fatalf("store off: %v", err)
	}

	restarted := &limits.Service{Store: database, Providers: []limits.Provider{stub}}
	if err := restarted.RestoreLive(context.Background(), true); err != nil {
		t.Fatalf("restore: %v", err)
	}
	if !restarted.Live() {
		t.Fatal("--live did not override the stored preference")
	}
}
