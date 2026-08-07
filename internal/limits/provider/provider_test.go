package provider_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/limits"
	"github.com/daniel-kindl/agentmeter/internal/limits/provider"
)

// syntheticToken is hand-authored. Never place a real credential in a fixture.
const syntheticToken = "synthetic-access-token"

var now = time.Date(2026, 8, 7, 12, 0, 0, 0, time.UTC)

func staticCredential(credential provider.Credential, err error) func() (provider.Credential, error) {
	return func() (provider.Credential, error) { return credential, err }
}

// serveJSON records the request it received so header expectations can be
// asserted, and replies with the given status and body.
func serveJSON(t *testing.T, status int, body string) (*httptest.Server, *http.Request) {
	t.Helper()
	var received http.Request
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		received = *request.Clone(context.Background())
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return server, &received
}

func windowOf(t *testing.T, windows []limits.Window, kind limits.Kind) limits.Window {
	t.Helper()
	for _, window := range windows {
		if window.Kind == kind {
			return window
		}
	}
	t.Fatalf("no %q window in %+v", kind, windows)
	return limits.Window{}
}

func TestClaudeFetchMapsEveryReportedWindow(t *testing.T) {
	t.Parallel()

	const body = `{
  "five_hour": {"utilization": 33.0, "resets_at": "2026-08-07T15:00:00.528743+00:00"},
  "seven_day": {"utilization": 13.0, "resets_at": "2026-08-12T00:59:59.951713+00:00"},
  "seven_day_opus": null,
  "seven_day_sonnet": {"utilization": 1.0, "resets_at": "2026-08-11T03:00:00.951719+00:00"},
  "extra_usage": {"is_enabled": false, "monthly_limit": null, "used_credits": null, "utilization": null}
}`
	server, received := serveJSON(t, http.StatusOK, body)
	claude := &provider.Claude{
		BaseURL:    server.URL,
		UserAgent:  "claude-code/1.2.3",
		Credential: staticCredential(provider.Credential{Token: syntheticToken}, nil),
	}

	windows, err := claude.Fetch(context.Background(), now)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	if claude.Source() != "claude" {
		t.Fatalf("source = %q, want claude", claude.Source())
	}
	// A null window means the plan has no such limit, so Opus is absent
	// entirely rather than reported at zero.
	if len(windows) != 3 {
		t.Fatalf("windows = %+v, want three", windows)
	}
	// Labels are assigned by the limits package, not here, so that a window
	// rendered from cache reads the same as a freshly fetched one.
	block := windowOf(t, windows, limits.KindFiveHour)
	if block.Utilization != 33 {
		t.Fatalf("five-hour = %+v", block)
	}
	wantReset := time.Date(2026, 8, 7, 15, 0, 0, 528743000, time.UTC)
	if block.ResetsAt == nil || !block.ResetsAt.Equal(wantReset) {
		t.Fatalf("five-hour reset = %v, want %v", block.ResetsAt, wantReset)
	}
	if windowOf(t, windows, limits.KindSevenDaySonnet).Utilization != 1 {
		t.Fatalf("sonnet window = %+v", windowOf(t, windows, limits.KindSevenDaySonnet))
	}

	// Anthropic rate-limits callers that do not identify as Claude Code.
	if got := received.Header.Get("User-Agent"); got != "claude-code/1.2.3" {
		t.Fatalf("User-Agent = %q, want claude-code/1.2.3", got)
	}
	if got := received.Header.Get("anthropic-beta"); got != "oauth-2025-04-20" {
		t.Fatalf("anthropic-beta = %q", got)
	}
	if got := received.Header.Get("Authorization"); got != "Bearer "+syntheticToken {
		t.Fatalf("Authorization = %q", got)
	}
	if received.URL.Path != "/api/oauth/usage" {
		t.Fatalf("path = %q, want /api/oauth/usage", received.URL.Path)
	}
}

func TestCodexFetchMapsPrimaryAndSecondaryWindows(t *testing.T) {
	t.Parallel()

	const body = `{"rate_limits": [
  {"limit_name": "account",
   "primary": {"used_percent": 42.5, "window_minutes": 299, "resets_in_seconds": 17940},
   "secondary": {"used_percent": 6.0, "window_minutes": 10079, "resets_in_seconds": 275281}}
]}`
	server, received := serveJSON(t, http.StatusOK, body)
	codex := &provider.Codex{
		BaseURL:    server.URL,
		UserAgent:  "codex-cli",
		Credential: staticCredential(provider.Credential{Token: syntheticToken, Account: "account-1"}, nil),
	}

	windows, err := codex.Fetch(context.Background(), now)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}

	if codex.Source() != "codex" {
		t.Fatalf("source = %q, want codex", codex.Source())
	}
	block := windowOf(t, windows, limits.KindFiveHour)
	if block.Utilization != 42.5 {
		t.Fatalf("five-hour utilization = %v, want 42.5", block.Utilization)
	}
	// The reset arrives as a duration and is resolved against the fetch time.
	if wantReset := now.Add(17940 * time.Second); block.ResetsAt == nil || !block.ResetsAt.Equal(wantReset) {
		t.Fatalf("five-hour reset = %v, want %v", block.ResetsAt, wantReset)
	}
	if week := windowOf(t, windows, limits.KindSevenDay); week.Utilization != 6 {
		t.Fatalf("weekly utilization = %v, want 6", week.Utilization)
	}
	if got := received.Header.Get("ChatGPT-Account-Id"); got != "account-1" {
		t.Fatalf("ChatGPT-Account-Id = %q", got)
	}
	if received.URL.Path != "/wham/usage" {
		t.Fatalf("path = %q, want /wham/usage", received.URL.Path)
	}
}

func TestCodexFetchAcceptsBareArrayResponse(t *testing.T) {
	t.Parallel()

	const body = `[{"primary": {"used_percent": 12.0, "resets_in_seconds": 60}}]`
	server, _ := serveJSON(t, http.StatusOK, body)
	codex := &provider.Codex{BaseURL: server.URL, Credential: staticCredential(provider.Credential{Token: syntheticToken}, nil)}

	windows, err := codex.Fetch(context.Background(), now)
	if err != nil {
		t.Fatalf("fetch: %v", err)
	}
	if len(windows) != 1 || windows[0].Utilization != 12 {
		t.Fatalf("windows = %+v", windows)
	}
}

// An unrecognized response must fail loudly. Reporting zero windows as a
// success would render as 0% used, which is worse than showing nothing.
func TestFetchRejectsUnrecognizedResponses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
		make func(baseURL string) provider.Provider
	}{
		{
			name: "claude without windows",
			body: `{"extra_usage": {"is_enabled": false}}`,
			make: func(baseURL string) provider.Provider {
				return &provider.Claude{BaseURL: baseURL, Credential: staticCredential(provider.Credential{Token: syntheticToken}, nil)}
			},
		},
		{
			name: "claude with a null window",
			body: `{"five_hour": null, "seven_day": null}`,
			make: func(baseURL string) provider.Provider {
				return &provider.Claude{BaseURL: baseURL, Credential: staticCredential(provider.Credential{Token: syntheticToken}, nil)}
			},
		},
		{
			name: "codex with an unknown shape",
			body: `{"usage": {"percent": 10}}`,
			make: func(baseURL string) provider.Provider {
				return &provider.Codex{BaseURL: baseURL, Credential: staticCredential(provider.Credential{Token: syntheticToken}, nil)}
			},
		},
		{
			name: "codex with an empty list",
			body: `{"rate_limits": []}`,
			make: func(baseURL string) provider.Provider {
				return &provider.Codex{BaseURL: baseURL, Credential: staticCredential(provider.Credential{Token: syntheticToken}, nil)}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server, _ := serveJSON(t, http.StatusOK, tt.body)
			if _, err := tt.make(server.URL).Fetch(context.Background(), now); err == nil {
				t.Fatal("Fetch accepted an unrecognized response")
			}
		})
	}
}

func TestFetchExplainsFailureStatuses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		status int
		want   string
	}{
		{name: "unauthorized", status: http.StatusUnauthorized, want: "sign in again"},
		{name: "forbidden", status: http.StatusForbidden, want: "sign in again"},
		{name: "throttled", status: http.StatusTooManyRequests, want: "rate-limited"},
		{name: "server error", status: http.StatusInternalServerError, want: "status 500"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server, _ := serveJSON(t, tt.status, `{}`)
			claude := &provider.Claude{BaseURL: server.URL, Credential: staticCredential(provider.Credential{Token: syntheticToken}, nil)}

			_, err := claude.Fetch(context.Background(), now)
			if err == nil {
				t.Fatalf("Fetch succeeded on status %d", tt.status)
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error %q does not explain %q", err, tt.want)
			}
		})
	}
}

// Provider errors are rendered on the dashboard, so a credential must never
// reach one.
func TestFetchErrorsNeverQuoteTheCredential(t *testing.T) {
	t.Parallel()

	server, _ := serveJSON(t, http.StatusUnauthorized, `{"error": "bad token"}`)
	providers := []provider.Provider{
		&provider.Claude{BaseURL: server.URL, Credential: staticCredential(provider.Credential{Token: syntheticToken}, nil)},
		&provider.Codex{BaseURL: server.URL, Credential: staticCredential(provider.Credential{Token: syntheticToken, Account: "account-1"}, nil)},
	}

	for _, candidate := range providers {
		_, err := candidate.Fetch(context.Background(), now)
		if err == nil {
			t.Fatalf("%s fetch succeeded on 401", candidate.Source())
		}
		if strings.Contains(err.Error(), syntheticToken) {
			t.Fatalf("%s error quotes the credential: %v", candidate.Source(), err)
		}
	}
}

func TestFetchSurfacesCredentialErrorsWithoutCallingOut(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("provider contacted the endpoint without a credential")
	}))
	t.Cleanup(server.Close)

	claude := &provider.Claude{
		BaseURL:    server.URL,
		Credential: staticCredential(provider.Credential{}, os.ErrNotExist),
	}
	if _, err := claude.Fetch(context.Background(), now); err == nil {
		t.Fatal("Fetch succeeded without a credential")
	}
}

func TestClaudeCredentialPrefersTheEnvironment(t *testing.T) {
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", syntheticToken)

	credential, err := provider.ClaudeCredential([]string{t.TempDir()})()
	if err != nil {
		t.Fatalf("load credential: %v", err)
	}
	if credential.Token != syntheticToken {
		t.Fatalf("token = %q, want the environment value", credential.Token)
	}
}

func TestClaudeCredentialReadsTheCredentialsFile(t *testing.T) {
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "")

	root := t.TempDir()
	expires := time.Now().Add(time.Hour).UnixMilli()
	writeFile(t, filepath.Join(root, ".credentials.json"),
		`{"claudeAiOauth":{"accessToken":"`+syntheticToken+`","refreshToken":"synthetic-refresh","expiresAt":`+itoa(expires)+`}}`)

	credential, err := provider.ClaudeCredential([]string{t.TempDir(), root})()
	if err != nil {
		t.Fatalf("load credential: %v", err)
	}
	if credential.Token != syntheticToken {
		t.Fatalf("token = %q", credential.Token)
	}
}

// agentmeter does not perform a refresh grant, so an expired token is reported
// rather than silently renewed by rewriting the operator's credentials file.
func TestClaudeCredentialReportsExpiry(t *testing.T) {
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "")

	root := t.TempDir()
	expired := time.Now().Add(-time.Hour).UnixMilli()
	writeFile(t, filepath.Join(root, ".credentials.json"),
		`{"claudeAiOauth":{"accessToken":"`+syntheticToken+`","expiresAt":`+itoa(expired)+`}}`)

	_, err := provider.ClaudeCredential([]string{root})()
	if err == nil || !strings.Contains(err.Error(), "expired") {
		t.Fatalf("error = %v, want an expiry explanation", err)
	}
}

func TestClaudeCredentialNamesTheDirectoriesItSearched(t *testing.T) {
	t.Setenv("CLAUDE_CODE_OAUTH_TOKEN", "")

	root := t.TempDir()
	_, err := provider.ClaudeCredential([]string{root})()
	if err == nil || !strings.Contains(err.Error(), root) {
		t.Fatalf("error = %v, want it to name %q", err, root)
	}
}

func TestCodexCredentialReadsTokenAndAccount(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "auth.json"),
		`{"auth_mode":"chatgpt","OPENAI_API_KEY":null,"tokens":{"id_token":"synthetic-id","access_token":"`+
			syntheticToken+`","refresh_token":"synthetic-refresh","account_id":"account-1"},"last_refresh":"2026-08-07T11:00:00Z"}`)

	credential, err := provider.CodexCredential(root)()
	if err != nil {
		t.Fatalf("load credential: %v", err)
	}
	if credential.Token != syntheticToken || credential.Account != "account-1" {
		t.Fatalf("credential = %+v", credential)
	}
}

// An API-key-only auth.json runs Codex fine but carries no subscription for the
// usage endpoint to report on.
func TestCodexCredentialRejectsAPIKeyOnlyAuth(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, filepath.Join(root, "auth.json"), `{"auth_mode":"apiKey","OPENAI_API_KEY":"synthetic-api-key","tokens":null}`)

	_, err := provider.CodexCredential(root)()
	if err == nil || !strings.Contains(err.Error(), "codex login") {
		t.Fatalf("error = %v, want sign-in guidance", err)
	}
}

func TestCodexCredentialReportsMissingFile(t *testing.T) {
	t.Parallel()

	_, err := provider.CodexCredential(t.TempDir())()
	if err == nil || !strings.Contains(err.Error(), "codex login") {
		t.Fatalf("error = %v, want sign-in guidance", err)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func itoa(value int64) string {
	return strconv.FormatInt(value, 10)
}
