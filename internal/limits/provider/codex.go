package provider

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"
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

// DefaultCodexUserAgent identifies the caller as the Codex CLI.
const DefaultCodexUserAgent = "codex-cli"

// NewCodex builds a provider reading credentials from the given config root.
// A blank baseURL, userAgent, or credentialPath keeps the built-in default.
func NewCodex(configRoot, baseURL, userAgent, credentialPath string) *Codex {
	return &Codex{
		BaseURL:    orDefault(baseURL, CodexBaseURL),
		UserAgent:  orDefault(userAgent, DefaultCodexUserAgent),
		Credential: CodexCredential(configRoot, credentialPath),
	}
}

// Source names the agent these windows belong to.
func (c *Codex) Source() string { return "codex" }

// sessionWindowCeiling separates the rolling session window from the weekly
// allowance. The endpoint reports each window's length rather than naming it,
// and the two are orders of magnitude apart, so any threshold between a day and
// a week distinguishes them.
const sessionWindowCeiling = 24 * 60 * 60

// codexWindow is one rate limit window. The window's own length decides which
// limit it is: an account can report its weekly allowance as the primary window
// with no secondary at all, so mapping by position would label a weekly figure
// as a five-hour one.
type codexWindow struct {
	UsedPercent        *float64 `json:"used_percent"`
	LimitWindowSeconds *int64   `json:"limit_window_seconds"`
	ResetAfterSeconds  *int64   `json:"reset_after_seconds"`
	ResetAt            *int64   `json:"reset_at"`
}

type codexRateLimit struct {
	PrimaryWindow   *codexWindow `json:"primary_window"`
	SecondaryWindow *codexWindow `json:"secondary_window"`
}

type codexUsage struct {
	RateLimit *codexRateLimit `json:"rate_limit"`
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
	if usage.RateLimit != nil {
		for _, candidate := range []*codexWindow{usage.RateLimit.PrimaryWindow, usage.RateLimit.SecondaryWindow} {
			if window, ok := codexLimitWindow(candidate, now); ok {
				windows = append(windows, window)
			}
		}
	}
	if len(windows) == 0 {
		// A plan can genuinely carry one window, but zero means the response
		// was not the shape this parses. Say so rather than render nought
		// percent used, which would read as plenty of headroom.
		return nil, errors.New("no usage windows reported for this OpenAI account")
	}
	slices.SortFunc(windows, func(a, b limits.Window) int { return strings.Compare(string(a.Kind), string(b.Kind)) })
	return windows, nil
}

func codexLimitWindow(window *codexWindow, now time.Time) (limits.Window, bool) {
	if window == nil || window.UsedPercent == nil || window.LimitWindowSeconds == nil {
		return limits.Window{}, false
	}
	kind := limits.KindSevenDay
	if *window.LimitWindowSeconds <= sessionWindowCeiling {
		kind = limits.KindFiveHour
	}
	result := limits.Window{Kind: kind, Utilization: percentage(*window.UsedPercent)}
	switch {
	case window.ResetAt != nil && *window.ResetAt > 0:
		reset := time.Unix(*window.ResetAt, 0).UTC()
		result.ResetsAt = &reset
	case window.ResetAfterSeconds != nil && *window.ResetAfterSeconds >= 0:
		reset := now.UTC().Add(time.Duration(*window.ResetAfterSeconds) * time.Second)
		result.ResetsAt = &reset
	}
	return result, true
}
