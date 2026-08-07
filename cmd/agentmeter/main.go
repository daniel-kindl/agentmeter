// Command agentmeter provides the agentmeter command-line interface.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/config"
	"github.com/daniel-kindl/agentmeter/internal/discovery"
	"github.com/daniel-kindl/agentmeter/internal/limits"
	"github.com/daniel-kindl/agentmeter/internal/limits/provider"
	"github.com/daniel-kindl/agentmeter/internal/scanner"
	"github.com/daniel-kindl/agentmeter/internal/store"
	internalweb "github.com/daniel-kindl/agentmeter/internal/web"
)

const (
	serveAddress = "127.0.0.1:7777"
	usageText    = "usage: agentmeter <up|scan|serve|version>"
)

var version = "dev"

type application struct {
	stdout        io.Writer
	stderr        io.Writer
	version       string
	userConfigDir func() (string, error)
	userHomeDir   func() (string, error)
	openStore     func(context.Context, string) (*store.Store, error)
	discoverFiles func() ([]discovery.File, error)
	scanFiles     func(context.Context, *store.Store, []discovery.File) (scanner.Result, error)
	handler       func(*store.Store, *limits.Service) http.Handler
	// listenAndServe reports the bound address through ready before it starts
	// serving, so a caller can open a browser without racing the listener.
	listenAndServe func(address string, handler http.Handler, ready func(net.Addr)) error
	openURL        func(string) error
}

func main() {
	app := application{stdout: os.Stdout, stderr: os.Stderr, version: version}
	os.Exit(app.run(os.Args[1:]))
}

func (app application) run(args []string) int {
	app = app.withDefaults()
	if len(args) == 0 {
		if !writef(app.stderr, "%s\n", usageText) {
			return 1
		}
		return 2
	}

	switch args[0] {
	case "up":
		return app.runUp(args[1:])
	case "scan":
		return app.runScan(args[1:])
	case "serve":
		return app.runServe(args[1:])
	case "version":
		if !app.parseNoArguments("version", args[1:]) {
			return 2
		}
		if !writef(app.stdout, "%s\n", app.version) {
			return 1
		}
		return 0
	default:
		if !writef(app.stderr, "unknown command %q\n%s\n", args[0], usageText) {
			return 1
		}
		return 2
	}
}

func (app application) withDefaults() application {
	if app.userConfigDir == nil {
		app.userConfigDir = os.UserConfigDir
	}
	if app.userHomeDir == nil {
		app.userHomeDir = os.UserHomeDir
	}
	if app.openStore == nil {
		app.openStore = store.Open
	}
	if app.scanFiles == nil {
		app.scanFiles = scanner.Scan
	}
	if app.handler == nil {
		app.handler = internalweb.Handler
	}
	if app.discoverFiles == nil {
		app.discoverFiles = app.defaultFiles
	}
	if app.listenAndServe == nil {
		app.listenAndServe = listenAndServe
	}
	if app.openURL == nil {
		app.openURL = openInBrowser
	}
	return app
}

// listenAndServe binds before serving so that ready fires only once the socket
// is accepting connections. A browser opened from ready cannot arrive early.
func listenAndServe(address string, handler http.Handler, ready func(net.Addr)) error {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return err
	}
	if ready != nil {
		ready(listener.Addr())
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	return server.Serve(listener)
}

// openInBrowser hands a loopback URL to the desktop's default handler. The URL
// is always one agentmeter just bound, never operator input.
func openInBrowser(url string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		return exec.Command("open", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

func (app application) runScan(args []string) int {
	databasePath, err := app.defaultDatabasePath()
	if err != nil {
		writef(app.stderr, "scan: locate configuration directory\n")
		return 1
	}
	flags := flag.NewFlagSet("scan", flag.ContinueOnError)
	flags.SetOutput(app.stderr)
	flags.Usage = func() { writef(app.stderr, "usage: agentmeter scan [--db PATH] [--json]\n") }
	jsonOutput := flags.Bool("json", false, "write machine-readable scan statistics")
	flags.StringVar(&databasePath, "db", databasePath, "SQLite database path")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	if err := ensureDatabaseParent(databasePath); err != nil {
		writef(app.stderr, "scan: create database directory\n")
		return 1
	}
	database, err := app.openStore(context.Background(), databasePath)
	if err != nil {
		writef(app.stderr, "scan: open database\n")
		return 1
	}
	defer func() { _ = database.Close() }()
	result, ok := app.scan("scan", database)
	if !ok {
		return 1
	}
	if *jsonOutput {
		if err := json.NewEncoder(app.stdout).Encode(result); err != nil {
			return 1
		}
		return 0
	}
	if !app.writeScanSummary(result) {
		return 1
	}
	return 0
}

// serveOptions holds what serve and up have in common.
type serveOptions struct {
	databasePath string
	address      string
	live         bool
	budgets      limits.Budgets
	scanFirst    bool
	openBrowser  bool
}

func (app application) runServe(args []string) int {
	options, code := app.parseServeFlags("serve",
		"usage: agentmeter serve [--db PATH] [--addr HOST:PORT] [--live] [--budget-5h N] [--budget-7d N]", args)
	if code != 0 {
		return code
	}
	return app.serveDashboard("serve", options)
}

// runUp is the single step: scan the local session logs, serve the dashboard,
// and open it. It exists so that using agentmeter is one command rather than
// three, which is what a released binary should offer.
func (app application) runUp(args []string) int {
	options, code := app.parseServeFlags("up",
		"usage: agentmeter up [--db PATH] [--addr HOST:PORT] [--live] [--budget-5h N] [--budget-7d N] [--no-open]", args)
	if code != 0 {
		return code
	}
	options.scanFirst = true
	return app.serveDashboard("up", options)
}

func (app application) parseServeFlags(name, usage string, args []string) (serveOptions, int) {
	databasePath, err := app.defaultDatabasePath()
	if err != nil {
		writef(app.stderr, "%s: locate configuration directory\n", name)
		return serveOptions{}, 1
	}
	options := serveOptions{databasePath: databasePath, address: serveAddress}

	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(app.stderr)
	flags.Usage = func() { writef(app.stderr, "%s\n", usage) }
	flags.StringVar(&options.databasePath, "db", options.databasePath, "SQLite database path")
	flags.StringVar(&options.address, "addr", options.address, "listen address")
	flags.BoolVar(&options.live, "live", false,
		"fetch authoritative limits: reads local agent credentials and contacts Anthropic and OpenAI")
	flags.Int64Var(&options.budgets.FiveHour, "budget-5h", 0,
		"token ceiling for the estimated five-hour window (0 compares against your busiest window)")
	flags.Int64Var(&options.budgets.SevenDay, "budget-7d", 0,
		"token ceiling for the estimated weekly window (0 compares against your busiest window)")
	noOpen := false
	if name == "up" {
		flags.BoolVar(&noOpen, "no-open", false, "do not open the dashboard in a browser")
	}
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		flags.Usage()
		return serveOptions{}, 2
	}
	options.openBrowser = name == "up" && !noOpen
	return options, 0
}

func (app application) serveDashboard(name string, options serveOptions) int {
	if err := ensureDatabaseParent(options.databasePath); err != nil {
		writef(app.stderr, "%s: create database directory\n", name)
		return 1
	}
	database, err := app.openStore(context.Background(), options.databasePath)
	if err != nil {
		writef(app.stderr, "%s: open database\n", name)
		return 1
	}
	defer func() { _ = database.Close() }()

	if options.scanFirst && !app.scanInto(name, database) {
		return 1
	}

	configPath, err := app.configPath()
	if err != nil {
		writef(app.stderr, "%s: locate configuration directory\n", name)
		return 1
	}
	settings, err := config.Load(configPath)
	if err != nil {
		writef(app.stderr, "%s: %v\n", name, err)
		return 1
	}

	budgets := options.budgets
	// A flag is a deliberate instruction for this run and outranks the file.
	if budgets.FiveHour == 0 {
		budgets.FiveHour = settings.Limits.FiveHourBudget
	}
	if budgets.SevenDay == 0 {
		budgets.SevenDay = settings.Limits.SevenDayBudget
	}

	service := &limits.Service{Store: database, Budgets: budgets}
	providers, err := app.liveProviders(settings)
	if err != nil {
		writef(app.stderr, "%s: %v\n", name, err)
		return 1
	}
	service.Providers = providers
	if err := service.RestoreLive(context.Background(), options.live); err != nil {
		writef(app.stderr, "%s: read the live-limits preference\n", name)
		return 1
	}
	if service.Live() {
		// Contacting a vendor is the one thing agentmeter does that leaves the
		// machine, so it announces itself rather than happening quietly.
		if !writef(app.stdout, "live limits enabled: reading local agent credentials and contacting %s\n", vendorList(settings)) {
			return 1
		}
	}

	ready := func(bound net.Addr) {
		url := "http://" + bound.String()
		writef(app.stdout, "serving on %s\n", url)
		if !options.openBrowser {
			return
		}
		// A browser that refuses to start is not a reason to stop serving; the
		// address is already on screen.
		if err := app.openURL(url); err != nil {
			writef(app.stderr, "%s: open a browser at %s\n", name, url)
		}
	}
	if err := app.listenAndServe(options.address, app.handler(database, service), ready); err != nil {
		writef(app.stderr, "%s: %v\n", name, err)
		return 1
	}
	return 0
}

func (app application) scanInto(name string, database *store.Store) bool {
	result, ok := app.scan(name, database)
	return ok && app.writeScanSummary(result)
}

func (app application) scan(name string, database *store.Store) (scanner.Result, bool) {
	files, err := app.discoverFiles()
	if err != nil {
		writef(app.stderr, "%s: discover sessions\n", name)
		return scanner.Result{}, false
	}
	result, err := app.scanFiles(context.Background(), database, files)
	if err != nil {
		writef(app.stderr, "%s: process sessions\n", name)
		return scanner.Result{}, false
	}
	return result, true
}

func (app application) writeScanSummary(result scanner.Result) bool {
	return writef(app.stdout, "scanned %d Claude and %d Codex files: %d inserted, %d updated, %d duplicates, %d unparsed lines\n",
		result.Claude.Files, result.Codex.Files, result.Inserted, result.Updated, result.Duplicates,
		result.Claude.UnparsedLines+result.Codex.UnparsedLines)
}

// liveProviders builds the authoritative providers named by configuration.
// Constructing one opens nothing; a provider only reads a credential or
// contacts a vendor once the live switch is on and a report is requested.
func (app application) liveProviders(settings config.Config) ([]limits.Provider, error) {
	home, err := app.userHomeDir()
	if err != nil {
		return nil, err
	}
	built := make([]limits.Provider, 0, len(settings.Limits.Providers))
	for _, candidate := range settings.Limits.Providers {
		if candidate.Disabled {
			continue
		}
		switch candidate.Kind {
		case config.KindAnthropicOAuth:
			built = append(built, provider.NewClaude(
				claudeConfigRoots(home), app.version,
				candidate.BaseURL, candidate.UserAgent, candidate.CredentialPath))
		case config.KindChatGPTUsage:
			built = append(built, provider.NewCodex(
				codexRoot(home), candidate.BaseURL, candidate.UserAgent, candidate.CredentialPath))
		default:
			// Load already rejected unknown kinds, so reaching here means the
			// two lists drifted apart.
			return nil, fmt.Errorf("provider %q has unsupported kind %q", candidate.Source, candidate.Kind)
		}
	}
	return built, nil
}

// vendorList names the hosts the live switch will contact, so the notice
// reflects the configured endpoints rather than a hardcoded sentence.
func vendorList(settings config.Config) string {
	seen := map[string]bool{}
	var hosts []string
	for _, candidate := range settings.Limits.Providers {
		if candidate.Disabled {
			continue
		}
		host := candidate.BaseURL
		if host == "" {
			host = map[string]string{
				config.KindAnthropicOAuth: provider.ClaudeBaseURL,
				config.KindChatGPTUsage:   provider.CodexBaseURL,
			}[candidate.Kind]
		}
		if parsed, err := url.Parse(host); err == nil && parsed.Host != "" {
			host = parsed.Host
		}
		if !seen[host] {
			seen[host] = true
			hosts = append(hosts, host)
		}
	}
	if len(hosts) == 0 {
		return "no configured endpoints"
	}
	return strings.Join(hosts, " and ")
}

func (app application) configPath() (string, error) {
	root, err := app.userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "agentmeter", config.FileName), nil
}

func (app application) defaultDatabasePath() (string, error) {
	root, err := app.userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "agentmeter", "agentmeter.db"), nil
}

func (app application) defaultFiles() ([]discovery.File, error) {
	home, err := app.userHomeDir()
	if err != nil {
		return nil, err
	}
	claudeFiles, err := discovery.Claude(claudeConfigRoots(home))
	if err != nil {
		return nil, err
	}
	codexFiles, err := discovery.Codex(codexRoot(home))
	if err != nil {
		return nil, err
	}
	return append(claudeFiles, codexFiles...), nil
}

// claudeConfigRoots resolves where Claude Code keeps its data. Session
// discovery and credential loading must agree on this, so both read it here.
func claudeConfigRoots(home string) []string {
	if configured := os.Getenv("CLAUDE_CONFIG_DIR"); configured != "" {
		var roots []string
		for _, root := range strings.Split(configured, ",") {
			if root = strings.TrimSpace(root); root != "" {
				roots = append(roots, root)
			}
		}
		return roots
	}
	roots := []string{filepath.Join(home, ".claude")}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		roots = append(roots, filepath.Join(xdg, "claude"))
	}
	return roots
}

// codexRoot resolves where the Codex CLI keeps its data.
func codexRoot(home string) string {
	if configured := os.Getenv("CODEX_HOME"); configured != "" {
		return configured
	}
	return filepath.Join(home, ".codex")
}

func ensureDatabaseParent(path string) error {
	if path == ":memory:" {
		return nil
	}
	return os.MkdirAll(filepath.Dir(path), 0o700)
}

func (app application) parseNoArguments(name string, args []string) bool {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(app.stderr)
	flags.Usage = func() { writef(app.stderr, "usage: agentmeter %s\n", name) }
	if err := flags.Parse(args); err != nil {
		return false
	}
	if flags.NArg() != 0 {
		writef(app.stderr, "%s: unexpected argument %q\n", name, flags.Arg(0))
		flags.Usage()
		return false
	}
	return true
}

func writef(writer io.Writer, format string, args ...any) bool {
	_, err := fmt.Fprintf(writer, format, args...)
	return err == nil
}
