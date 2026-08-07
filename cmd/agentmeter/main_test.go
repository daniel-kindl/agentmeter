package main

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/daniel-kindl/agentmeter/internal/discovery"
	"github.com/daniel-kindl/agentmeter/internal/limits"
	"github.com/daniel-kindl/agentmeter/internal/scanner"
	"github.com/daniel-kindl/agentmeter/internal/store"
)

func TestRunCommands(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{name: "version", args: []string{"version"}, wantCode: 0, wantStdout: "test-version\n"},
		{name: "missing", wantCode: 2, wantStderr: usageText},
		{name: "unknown", args: []string{"other"}, wantCode: 2, wantStderr: `unknown command "other"`},
		{name: "unexpected argument", args: []string{"version", "extra"}, wantCode: 2, wantStderr: `unexpected argument "extra"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			app := application{
				stdout:  &stdout,
				stderr:  &stderr,
				version: "test-version",
				listenAndServe: func(string, http.Handler, func(net.Addr)) error {
					t.Fatal("listenAndServe called unexpectedly")
					return nil
				},
			}

			if got := app.run(tt.args); got != tt.wantCode {
				t.Errorf("exit code = %d, want %d", got, tt.wantCode)
			}
			if got := stdout.String(); got != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", got, tt.wantStdout)
			}
			if got := stderr.String(); !strings.Contains(got, tt.wantStderr) {
				t.Errorf("stderr = %q, want substring %q", got, tt.wantStderr)
			}
		})
	}
}

func TestRunScan(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := application{
		stdout:        &stdout,
		stderr:        &stderr,
		version:       "test-version",
		discoverFiles: func() ([]discovery.File, error) { return nil, nil },
	}
	if got := app.run([]string{"scan", "--db", ":memory:", "--json"}); got != 0 {
		t.Fatalf("exit code = %d, stderr = %q", got, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"inserted":0`) {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestRunServe(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	var gotAddress string
	var gotHandler http.Handler
	app := application{
		stdout:  &stdout,
		stderr:  &stderr,
		version: "test-version",
		listenAndServe: func(address string, handler http.Handler, _ func(net.Addr)) error {
			gotAddress = address
			gotHandler = handler
			return nil
		},
	}

	if got := app.run([]string{"serve", "--db", ":memory:"}); got != 0 {
		t.Fatalf("exit code = %d, want 0; stderr = %q", got, stderr.String())
	}
	if gotAddress != serveAddress {
		t.Errorf("address = %q, want %q", gotAddress, serveAddress)
	}
	if gotHandler == nil {
		t.Fatal("handler is nil")
	}
	recorder := httptest.NewRecorder()
	gotHandler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
	if recorder.Code != http.StatusOK {
		t.Errorf("handler status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func TestRunServeError(t *testing.T) {
	t.Parallel()

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	app := application{
		stdout:  &stdout,
		stderr:  &stderr,
		version: "test-version",
		listenAndServe: func(string, http.Handler, func(net.Addr)) error {
			return errors.New("synthetic listen failure")
		},
	}

	if got := app.run([]string{"serve", "--db", ":memory:"}); got != 1 {
		t.Fatalf("exit code = %d, want 1", got)
	}
	if !strings.Contains(stderr.String(), "synthetic listen failure") {
		t.Errorf("stderr = %q, want listen failure", stderr.String())
	}
}

// Serving without --live must construct no provider, so the dashboard reports
// derived estimates and the process never opens a socket to a vendor.
func TestRunServeIsOfflineByDefault(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	var gotService *limits.Service
	app := application{
		stdout:  &stdout,
		stderr:  &stderr,
		version: "test-version",
		handler: func(_ *store.Store, service *limits.Service) http.Handler {
			gotService = service
			return http.NotFoundHandler()
		},
		listenAndServe: func(string, http.Handler, func(net.Addr)) error { return nil },
	}

	if got := app.run([]string{"serve", "--db", ":memory:"}); got != 0 {
		t.Fatalf("exit code = %d, stderr = %q", got, stderr.String())
	}
	if gotService == nil {
		t.Fatal("no limit service was built")
	}
	// Providers are configured up front, but configuring one contacts nothing.
	// The switch is what must stay off.
	if gotService.Live() {
		t.Fatal("live limits are on without the flag")
	}
	if strings.Contains(stdout.String(), "live limits") {
		t.Fatalf("stdout announces live limits without the flag: %q", stdout.String())
	}
}

func TestRunServeWithLiveBuildsProvidersAndSaysSo(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	var gotService *limits.Service
	app := application{
		stdout:      &stdout,
		stderr:      &stderr,
		version:     "test-version",
		userHomeDir: func() (string, error) { return t.TempDir(), nil },
		handler: func(_ *store.Store, service *limits.Service) http.Handler {
			gotService = service
			return http.NotFoundHandler()
		},
		listenAndServe: func(string, http.Handler, func(net.Addr)) error { return nil },
	}

	if got := app.run([]string{"serve", "--db", ":memory:", "--live"}); got != 0 {
		t.Fatalf("exit code = %d, stderr = %q", got, stderr.String())
	}
	if len(gotService.Providers) != 2 {
		t.Fatalf("providers = %d, want claude and codex", len(gotService.Providers))
	}
	if !gotService.Live() {
		t.Fatal("--live did not turn the switch on")
	}
	// Leaving the machine is the one thing agentmeter does not do quietly.
	if !strings.Contains(stdout.String(), "live limits enabled") {
		t.Fatalf("stdout = %q, want a live-limits notice", stdout.String())
	}
	// The notice names the configured endpoints rather than a fixed sentence.
	if !strings.Contains(stdout.String(), "api.anthropic.com") || !strings.Contains(stdout.String(), "chatgpt.com") {
		t.Fatalf("stdout = %q, want the contacted hosts named", stdout.String())
	}
}

func TestRunServeAcceptsBudgetOverrides(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	var gotService *limits.Service
	app := application{
		stdout:  &stdout,
		stderr:  &stderr,
		version: "test-version",
		handler: func(_ *store.Store, service *limits.Service) http.Handler {
			gotService = service
			return http.NotFoundHandler()
		},
		listenAndServe: func(string, http.Handler, func(net.Addr)) error { return nil },
	}

	if got := app.run([]string{"serve", "--db", ":memory:", "--budget-5h", "1900000", "--budget-7d", "20000000"}); got != 0 {
		t.Fatalf("exit code = %d, stderr = %q", got, stderr.String())
	}
	want := limits.Budgets{FiveHour: 1900000, SevenDay: 20000000}
	if gotService.Budgets != want {
		t.Fatalf("budgets = %+v, want %+v", gotService.Budgets, want)
	}
}

func TestConfigRootsPreferEnvironmentOverrides(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "/first, /second ,")
	t.Setenv("CODEX_HOME", "/codex")

	roots := claudeConfigRoots("/home")
	if len(roots) != 2 || roots[0] != "/first" || roots[1] != "/second" {
		t.Fatalf("claude roots = %q, want the two configured directories", roots)
	}
	if got := codexRoot("/home"); got != "/codex" {
		t.Fatalf("codex root = %q, want /codex", got)
	}
}

func TestConfigRootsFallBackToHome(t *testing.T) {
	t.Setenv("CLAUDE_CONFIG_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("CODEX_HOME", "")

	if roots := claudeConfigRoots("/home"); len(roots) != 1 || roots[0] != filepath.Join("/home", ".claude") {
		t.Fatalf("claude roots = %q, want the home directory", roots)
	}
	if got := codexRoot("/home"); got != filepath.Join("/home", ".codex") {
		t.Fatalf("codex root = %q, want the home directory", got)
	}
}

// fakeAddr stands in for a bound listener address.
type fakeAddr string

func (fakeAddr) Network() string  { return "tcp" }
func (a fakeAddr) String() string { return string(a) }

// up is the single step a released binary should offer: scan, serve, open.
func TestRunUpScansServesAndOpensTheDashboard(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	var scanned bool
	var openedURL string
	app := application{
		stdout:        &stdout,
		stderr:        &stderr,
		version:       "test-version",
		discoverFiles: func() ([]discovery.File, error) { return nil, nil },
		scanFiles: func(context.Context, *store.Store, []discovery.File) (scanner.Result, error) {
			scanned = true
			return scanner.Result{}, nil
		},
		openURL: func(url string) error {
			openedURL = url
			return nil
		},
		listenAndServe: func(_ string, _ http.Handler, ready func(net.Addr)) error {
			ready(fakeAddr("127.0.0.1:7777"))
			return nil
		},
	}

	if got := app.run([]string{"up", "--db", ":memory:"}); got != 0 {
		t.Fatalf("exit code = %d, stderr = %q", got, stderr.String())
	}
	if !scanned {
		t.Error("up served without scanning first")
	}
	if openedURL != "http://127.0.0.1:7777" {
		t.Errorf("opened %q, want the bound loopback URL", openedURL)
	}
	out := stdout.String()
	if !strings.Contains(out, "scanned 0 Claude") || !strings.Contains(out, "serving on http://127.0.0.1:7777") {
		t.Errorf("stdout = %q", out)
	}
}

func TestRunUpRespectsNoOpen(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	app := application{
		stdout:        &stdout,
		stderr:        &stderr,
		version:       "test-version",
		discoverFiles: func() ([]discovery.File, error) { return nil, nil },
		openURL: func(string) error {
			t.Error("browser opened despite --no-open")
			return nil
		},
		listenAndServe: func(_ string, _ http.Handler, ready func(net.Addr)) error {
			ready(fakeAddr("127.0.0.1:7777"))
			return nil
		},
	}

	if got := app.run([]string{"up", "--db", ":memory:", "--no-open"}); got != 0 {
		t.Fatalf("exit code = %d, stderr = %q", got, stderr.String())
	}
}

// A browser that will not start is not a reason to stop serving.
func TestRunUpKeepsServingWhenTheBrowserFails(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	app := application{
		stdout:        &stdout,
		stderr:        &stderr,
		version:       "test-version",
		discoverFiles: func() ([]discovery.File, error) { return nil, nil },
		openURL:       func(string) error { return errors.New("no browser") },
		listenAndServe: func(_ string, _ http.Handler, ready func(net.Addr)) error {
			ready(fakeAddr("127.0.0.1:7777"))
			return nil
		},
	}

	if got := app.run([]string{"up", "--db", ":memory:"}); got != 0 {
		t.Fatalf("exit code = %d, want 0 despite the browser failure", got)
	}
	if !strings.Contains(stdout.String(), "serving on") {
		t.Errorf("stdout = %q, want the address printed anyway", stdout.String())
	}
	if !strings.Contains(stderr.String(), "open a browser at http://127.0.0.1:7777") {
		t.Errorf("stderr = %q, want a fallback hint", stderr.String())
	}
}

// serve is not up: it must never scan and never open a browser.
func TestRunServeNeitherScansNorOpens(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	app := application{
		stdout:  &stdout,
		stderr:  &stderr,
		version: "test-version",
		discoverFiles: func() ([]discovery.File, error) {
			t.Error("serve discovered sessions")
			return nil, nil
		},
		openURL: func(string) error {
			t.Error("serve opened a browser")
			return nil
		},
		listenAndServe: func(_ string, _ http.Handler, ready func(net.Addr)) error {
			ready(fakeAddr("127.0.0.1:7777"))
			return nil
		},
	}

	if got := app.run([]string{"serve", "--db", ":memory:"}); got != 0 {
		t.Fatalf("exit code = %d, stderr = %q", got, stderr.String())
	}
	if strings.Contains(stdout.String(), "scanned") {
		t.Errorf("stdout = %q, want no scan summary", stdout.String())
	}
}

func TestRunUpRejectsUnknownFlags(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	app := application{stdout: &stdout, stderr: &stderr, version: "test-version"}

	if got := app.run([]string{"up", "--nonsense"}); got != 2 {
		t.Fatalf("exit code = %d, want 2", got)
	}
	if !strings.Contains(stderr.String(), "usage: agentmeter up") {
		t.Errorf("stderr = %q", stderr.String())
	}
}

// serve has no --no-open flag; only up does.
func TestRunServeRejectsNoOpen(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer
	app := application{stdout: &stdout, stderr: &stderr, version: "test-version"}

	if got := app.run([]string{"serve", "--no-open"}); got != 2 {
		t.Fatalf("exit code = %d, want 2", got)
	}
}
