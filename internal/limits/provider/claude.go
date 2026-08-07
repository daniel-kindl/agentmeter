package provider

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/limits"
)

// ClaudeBaseURL is the host serving Claude's OAuth usage endpoint.
const ClaudeBaseURL = "https://api.anthropic.com"

// claudeBetaHeader opts into the OAuth surface that serves usage utilization.
const claudeBetaHeader = "oauth-2025-04-20"

// Claude reports the account limit windows behind Claude Code's /usage command.
//
// The endpoint is undocumented and the beta header is versioned, so a future
// Claude Code release can retire either without notice. Failures surface as an
// unavailable source rather than as wrong numbers.
type Claude struct {
	HTTPClient *http.Client
	BaseURL    string
	// UserAgent must identify Claude Code. Anthropic rate-limits the endpoint
	// aggressively for callers that do not.
	UserAgent  string
	Credential func() (Credential, error)
}

// NewClaude builds a provider reading credentials from the given config roots.
func NewClaude(configRoots []string, version string) *Claude {
	return &Claude{
		BaseURL:    ClaudeBaseURL,
		UserAgent:  "claude-code/" + version,
		Credential: ClaudeCredential(configRoots),
	}
}

// Source names the agent these windows belong to.
func (c *Claude) Source() string { return "claude" }

type claudeWindow struct {
	Utilization *float64 `json:"utilization"`
	ResetsAt    *string  `json:"resets_at"`
}

type claudeUsage struct {
	FiveHour       *claudeWindow `json:"five_hour"`
	SevenDay       *claudeWindow `json:"seven_day"`
	SevenDayOpus   *claudeWindow `json:"seven_day_opus"`
	SevenDaySonnet *claudeWindow `json:"seven_day_sonnet"`
}

// Fetch returns Claude's current limit windows.
func (c *Claude) Fetch(ctx context.Context, _ time.Time) ([]limits.Window, error) {
	credential, err := c.Credential()
	if err != nil {
		return nil, err
	}

	var usage claudeUsage
	headers := map[string]string{
		"Authorization":   "Bearer " + credential.Token,
		"anthropic-beta":  claudeBetaHeader,
		"User-Agent":      c.UserAgent,
		"Accept-Encoding": "identity",
	}
	if err := fetchJSON(ctx, defaultClient(c.HTTPClient), "Anthropic", c.BaseURL+"/api/oauth/usage", headers, &usage); err != nil {
		return nil, err
	}

	windows := []limits.Window{}
	for _, candidate := range []struct {
		kind   limits.Kind
		label  string
		window *claudeWindow
	}{
		{limits.KindFiveHour, "5-hour session", usage.FiveHour},
		{limits.KindSevenDay, "Weekly (all models)", usage.SevenDay},
		{limits.KindSevenDayOpus, "Weekly (Opus)", usage.SevenDayOpus},
		{limits.KindSevenDaySonnet, "Weekly (Sonnet)", usage.SevenDaySonnet},
	} {
		// A null window means the plan has no such limit, which is different
		// from a limit sitting at zero. Omit it rather than reporting 0%.
		if candidate.window == nil || candidate.window.Utilization == nil {
			continue
		}
		windows = append(windows, limits.Window{
			Kind:        candidate.kind,
			Label:       candidate.label,
			Utilization: percentage(*candidate.window.Utilization),
			ResetsAt:    parseReset(candidate.window.ResetsAt),
		})
	}
	if len(windows) == 0 {
		return nil, errors.New("no usage windows reported for this Anthropic account")
	}
	return windows, nil
}

func parseReset(value *string) *time.Time {
	if value == nil {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, *value)
	if err != nil {
		return nil
	}
	utc := parsed.UTC()
	return &utc
}
