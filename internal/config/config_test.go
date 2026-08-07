package config_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/daniel-kindl/agentmeter/internal/config"
)

func write(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), config.FileName)
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

// Running with no configuration must behave exactly as shipping intended.
func TestLoadWithoutAFileYieldsDefaults(t *testing.T) {
	t.Parallel()

	loaded, err := config.Load(filepath.Join(t.TempDir(), "absent.json"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Limits.Providers) != 2 {
		t.Fatalf("providers = %+v, want the built-in pair", loaded.Limits.Providers)
	}
	if loaded.Limits.Providers[0].Source != "claude" || loaded.Limits.Providers[1].Source != "codex" {
		t.Fatalf("providers = %+v", loaded.Limits.Providers)
	}
}

func TestLoadEmptyFileYieldsDefaults(t *testing.T) {
	t.Parallel()

	loaded, err := config.Load(write(t, "   \n"))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Limits.Providers) != 2 {
		t.Fatalf("providers = %+v, want the built-in pair", loaded.Limits.Providers)
	}
}

// The endpoints are undocumented and can move, which is the whole reason they
// are configuration rather than constants.
func TestLoadOverridesEndpointsAndCredentials(t *testing.T) {
	t.Parallel()

	path := write(t, `{
  "limits": {
    "five_hour_budget": 1900000,
    "providers": [
      {"source": "claude", "kind": "anthropic-oauth", "base_url": "https://example.invalid/anthropic",
       "credential_path": "/tmp/synthetic.json", "user_agent": "claude-code/9.9.9"},
      {"source": "codex", "kind": "chatgpt-usage", "disabled": true}
    ]
  }
}`)

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Limits.FiveHourBudget != 1900000 {
		t.Fatalf("five-hour budget = %d", loaded.Limits.FiveHourBudget)
	}
	claude := loaded.Limits.Providers[0]
	if claude.BaseURL != "https://example.invalid/anthropic" || claude.CredentialPath != "/tmp/synthetic.json" {
		t.Fatalf("claude provider = %+v", claude)
	}
	if claude.UserAgent != "claude-code/9.9.9" {
		t.Fatalf("user agent = %q", claude.UserAgent)
	}
	if !loaded.Limits.Providers[1].Disabled {
		t.Fatal("codex provider is not disabled")
	}
}

func TestLoadRejectsBadConfiguration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		contents string
		want     string
	}{
		{
			name:     "unknown kind",
			contents: `{"limits":{"providers":[{"source":"gemini","kind":"whatever"}]}}`,
			want:     "unknown kind",
		},
		{
			name:     "blank source",
			contents: `{"limits":{"providers":[{"source":"  ","kind":"anthropic-oauth"}]}}`,
			want:     "no source",
		},
		{
			name:     "duplicate source",
			contents: `{"limits":{"providers":[{"source":"claude","kind":"anthropic-oauth"},{"source":"claude","kind":"chatgpt-usage"}]}}`,
			want:     "more than once",
		},
		{
			name:     "negative budget",
			contents: `{"limits":{"five_hour_budget":-1}}`,
			want:     "negative",
		},
		// A typo in a hand-written file is far more likely than a deliberate
		// extension, and ignoring it means a setting that looks applied but is
		// not.
		{
			name:     "misspelled key",
			contents: `{"limits":{"provider":[{"source":"claude","kind":"anthropic-oauth"}]}}`,
			want:     "unknown field",
		},
		{
			name:     "malformed JSON",
			contents: `{"limits":`,
			want:     "parse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := config.Load(write(t, tt.contents))
			if err == nil {
				t.Fatal("Load accepted invalid configuration")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q does not mention %q", err, tt.want)
			}
		})
	}
}
