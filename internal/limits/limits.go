package limits

import (
	"context"
	"time"
)

// Provider fetches authoritative limit windows for one agent.
//
// Implementations leave the machine and read local agent credentials, so they
// are constructed only when the operator opts in. A provider returns labelled
// nothing: labels belong to this package so that a window rendered from cache
// reads identically to one just fetched.
type Provider interface {
	// Source names the agent, matching the source column of usage events.
	Source() string
	// Fetch returns the agent's current windows. now stamps windows whose
	// reset the vendor reports as a duration rather than an instant.
	Fetch(ctx context.Context, now time.Time) ([]Window, error)
}

// Kind identifies one usage limit window.
type Kind string

// The window kinds agentmeter can report. Providers may omit any of them.
const (
	KindFiveHour       Kind = "5h"
	KindSevenDay       Kind = "7d"
	KindSevenDayOpus   Kind = "7d_opus"
	KindSevenDaySonnet Kind = "7d_sonnet"
)

// Origin records where a window's numbers came from. Estimated windows are
// derived locally and are never presented as the agent's own accounting.
type Origin string

// The origins a source's windows can carry.
const (
	OriginEstimated   Origin = "estimated"
	OriginLive        Origin = "live"
	OriginUnavailable Origin = "unavailable"
)

// Window is one limit window's current state.
//
// Utilization is a percentage in the range [0, 100]. Live windows report it
// directly. Estimated windows compute it from UsedTokens and BudgetTokens,
// which are nil for live windows because providers report no token counts.
type Window struct {
	Kind         Kind       `json:"kind"`
	Label        string     `json:"label"`
	Utilization  float64    `json:"utilization"`
	ResetsAt     *time.Time `json:"resets_at"`
	UsedTokens   *int64     `json:"used_tokens"`
	BudgetTokens *int64     `json:"budget_tokens"`
}

// SourceLimits groups every window reported for one agent.
type SourceLimits struct {
	Source    string     `json:"source"`
	Origin    Origin     `json:"origin"`
	FetchedAt *time.Time `json:"fetched_at"`
	Stale     bool       `json:"stale"`
	Message   string     `json:"message"`
	Windows   []Window   `json:"windows"`
}

// Report is the complete limits response consumed by the local web application.
//
// Live and Configurable describe the switch rather than the readings: the page
// needs to know whether authoritative fetching is on, and whether it can be
// turned on at all, to render the control honestly.
type Report struct {
	Mode         string         `json:"mode"`
	Timezone     string         `json:"timezone"`
	Live         bool           `json:"live"`
	Configurable bool           `json:"configurable"`
	Sources      []SourceLimits `json:"sources"`
}

func utilization(used, budget int64) float64 {
	if budget <= 0 {
		return 0
	}
	return min(float64(used)/float64(budget)*100, 100)
}

// liveLabel names a window the way the agent that enforces it does, so the
// dashboard matches what /usage and /status show.
func liveLabel(sourceName string, kind Kind) string {
	claude := map[Kind]string{
		KindFiveHour:       "5-hour session",
		KindSevenDay:       "Weekly (all models)",
		KindSevenDayOpus:   "Weekly (Opus)",
		KindSevenDaySonnet: "Weekly (Sonnet)",
	}
	codex := map[Kind]string{
		KindFiveHour: "5-hour limit",
		KindSevenDay: "Weekly limit",
	}
	bySource := map[string]map[Kind]string{"claude": claude, "codex": codex}
	if label, ok := bySource[sourceName][kind]; ok {
		return label
	}
	return string(kind)
}
