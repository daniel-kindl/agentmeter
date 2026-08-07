package provider

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/limits"
)

// CodexBaseURL is the host serving the usage endpoint the Codex CLI polls.
const CodexBaseURL = "https://chatgpt.com/backend-api"

// Codex reports the limit windows behind the Codex CLI's /status line.
type Codex struct {
	HTTPClient *http.Client
	BaseURL    string
	UserAgent  string
	Credential func() (Credential, error)
}

// NewCodex builds a provider reading credentials from the given config root.
func NewCodex(configRoot string) *Codex {
	return &Codex{
		BaseURL:    CodexBaseURL,
		UserAgent:  "codex-cli",
		Credential: CodexCredential(configRoot),
	}
}

// Source names the agent these windows belong to.
func (c *Codex) Source() string { return "codex" }

// codexWindow is one rate limit window. The reset arrives as a duration rather
// than an instant, so it is resolved against the fetch time.
type codexWindow struct {
	UsedPercent     *float64 `json:"used_percent"`
	WindowMinutes   *int64   `json:"window_minutes"`
	ResetsInSeconds *int64   `json:"resets_in_seconds"`
}

type codexSnapshot struct {
	Primary   *codexWindow `json:"primary"`
	Secondary *codexWindow `json:"secondary"`
}

// codexUsage holds the account's rate limit snapshots. The Codex CLI's own
// client reads them from a rate_limits field, but the endpoint is undocumented,
// so a bare array is accepted too. A shape matching neither is an error rather
// than an empty result, so an unrecognized response can never render as 0%.
type codexUsage struct {
	RateLimits []codexSnapshot
}

func (u *codexUsage) UnmarshalJSON(data []byte) error {
	var wrapped struct {
		RateLimits []codexSnapshot `json:"rate_limits"`
	}
	if err := json.Unmarshal(data, &wrapped); err == nil && wrapped.RateLimits != nil {
		u.RateLimits = wrapped.RateLimits
		return nil
	}
	var bare []codexSnapshot
	if err := json.Unmarshal(data, &bare); err != nil {
		return fmt.Errorf("decode Codex usage windows: %w", err)
	}
	u.RateLimits = bare
	return nil
}

// Fetch returns Codex's current limit windows.
func (c *Codex) Fetch(ctx context.Context, now time.Time) ([]limits.Window, error) {
	credential, err := c.Credential()
	if err != nil {
		return nil, err
	}

	var usage codexUsage
	headers := map[string]string{
		"Authorization":   "Bearer " + credential.Token,
		"User-Agent":      c.UserAgent,
		"Accept-Encoding": "identity",
	}
	if credential.Account != "" {
		headers["ChatGPT-Account-Id"] = credential.Account
	}
	if err := fetchJSON(ctx, defaultClient(c.HTTPClient), "OpenAI", c.BaseURL+"/wham/usage", headers, &usage); err != nil {
		return nil, err
	}

	windows := []limits.Window{}
	for _, snapshot := range usage.RateLimits {
		if window, ok := codexLimitWindow(limits.KindFiveHour, snapshot.Primary, now); ok {
			windows = append(windows, window)
		}
		if window, ok := codexLimitWindow(limits.KindSevenDay, snapshot.Secondary, now); ok {
			windows = append(windows, window)
		}
		// Later entries describe additional limits such as workspace caps.
		// Reporting the account's own windows is enough for the dashboard.
		if len(windows) > 0 {
			break
		}
	}
	if len(windows) == 0 {
		return nil, errors.New("no usage windows reported for this OpenAI account")
	}
	return windows, nil
}

func codexLimitWindow(kind limits.Kind, window *codexWindow, now time.Time) (limits.Window, bool) {
	if window == nil || window.UsedPercent == nil {
		return limits.Window{}, false
	}
	result := limits.Window{
		Kind:        kind,
		Utilization: percentage(*window.UsedPercent),
	}
	if window.ResetsInSeconds != nil && *window.ResetsInSeconds >= 0 {
		reset := now.UTC().Add(time.Duration(*window.ResetsInSeconds) * time.Second)
		result.ResetsAt = &reset
	}
	return result, true
}
