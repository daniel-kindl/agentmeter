package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// FileName is the configuration file agentmeter looks for beside its database.
const FileName = "config.json"

// Provider kinds agentmeter knows how to talk to. The kind selects the response
// shape and authentication scheme; the endpoint and credentials are data.
const (
	KindAnthropicOAuth = "anthropic-oauth"
	KindChatGPTUsage   = "chatgpt-usage"
)

// Provider describes one authoritative limit source.
//
// Endpoints are configuration rather than constants because both are
// undocumented and can move without notice. When one does, an operator should
// be able to point agentmeter at the new address instead of waiting for a
// release.
type Provider struct {
	// Source names the agent and must match the source recorded on usage
	// events, so live windows and derived ones describe the same thing.
	Source string `json:"source"`
	// Kind selects the request and response handling.
	Kind string `json:"kind"`
	// BaseURL is the API root. Empty keeps the built-in default for the kind.
	BaseURL string `json:"base_url"`
	// CredentialPath overrides where the token is read from. Empty keeps the
	// agent's standard location.
	CredentialPath string `json:"credential_path"`
	// UserAgent overrides the identifying header. Empty keeps the default,
	// which some endpoints throttle callers for omitting.
	UserAgent string `json:"user_agent"`
	// Disabled removes a provider without deleting its entry.
	Disabled bool `json:"disabled"`
}

// Limits groups the usage-limit settings.
type Limits struct {
	// Providers replaces the built-in set when non-empty.
	Providers []Provider `json:"providers"`
	// FiveHourBudget and SevenDayBudget are token ceilings for the derived
	// windows. Zero compares against the busiest window in local history.
	FiveHourBudget int64 `json:"five_hour_budget"`
	SevenDayBudget int64 `json:"seven_day_budget"`
}

// Config is the complete on-disk configuration.
type Config struct {
	Limits Limits `json:"limits"`
}

// Default returns the configuration used when no file is present.
func Default() Config {
	return Config{Limits: Limits{Providers: DefaultProviders()}}
}

// DefaultProviders is the built-in provider set.
func DefaultProviders() []Provider {
	return []Provider{
		{Source: "claude", Kind: KindAnthropicOAuth},
		{Source: "codex", Kind: KindChatGPTUsage},
	}
}

// Load reads path. A missing file is not an error: it yields the defaults, so
// running with no configuration behaves exactly as shipping intended.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path) //nolint:gosec // an operator-chosen config path
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("read %s", path)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return Default(), nil
	}

	var parsed Config
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	// An unknown key is far more likely to be a typo in a hand-written file
	// than a deliberate extension, and silently ignoring it means a setting
	// that appears to be applied but is not.
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&parsed); err != nil {
		return Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(parsed.Limits.Providers) == 0 {
		parsed.Limits.Providers = DefaultProviders()
	}
	if err := parsed.validate(); err != nil {
		return Config{}, fmt.Errorf("%s: %w", path, err)
	}
	return parsed, nil
}

func (c Config) validate() error {
	seen := make(map[string]struct{}, len(c.Limits.Providers))
	for index, candidate := range c.Limits.Providers {
		if strings.TrimSpace(candidate.Source) == "" {
			return fmt.Errorf("provider %d has no source", index)
		}
		switch candidate.Kind {
		case KindAnthropicOAuth, KindChatGPTUsage:
		default:
			return fmt.Errorf("provider %q has unknown kind %q (want %q or %q)",
				candidate.Source, candidate.Kind, KindAnthropicOAuth, KindChatGPTUsage)
		}
		if _, duplicate := seen[candidate.Source]; duplicate {
			return fmt.Errorf("provider source %q appears more than once", candidate.Source)
		}
		seen[candidate.Source] = struct{}{}
	}
	if c.Limits.FiveHourBudget < 0 || c.Limits.SevenDayBudget < 0 {
		return errors.New("token budgets cannot be negative")
	}
	return nil
}
