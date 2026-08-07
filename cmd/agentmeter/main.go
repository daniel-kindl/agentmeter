// Command agentmeter provides the agentmeter command-line interface.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/daniel-kindl/agentmeter/internal/discovery"
	"github.com/daniel-kindl/agentmeter/internal/limits"
	"github.com/daniel-kindl/agentmeter/internal/limits/provider"
	"github.com/daniel-kindl/agentmeter/internal/scanner"
	"github.com/daniel-kindl/agentmeter/internal/store"
	internalweb "github.com/daniel-kindl/agentmeter/internal/web"
)

const (
	serveAddress = "127.0.0.1:7777"
	usageText    = "usage: agentmeter <scan|serve|version>"
)

var version = "dev"

type application struct {
	stdout         io.Writer
	stderr         io.Writer
	version        string
	listenAndServe func(string, http.Handler) error
	userConfigDir  func() (string, error)
	userHomeDir    func() (string, error)
	openStore      func(context.Context, string) (*store.Store, error)
	discoverFiles  func() ([]discovery.File, error)
	scanFiles      func(context.Context, *store.Store, []discovery.File) (scanner.Result, error)
	handler        func(*store.Store, *limits.Service) http.Handler
}

func main() {
	app := application{
		stdout:         os.Stdout,
		stderr:         os.Stderr,
		version:        version,
		listenAndServe: http.ListenAndServe,
	}
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
	return app
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
	files, err := app.discoverFiles()
	if err != nil {
		writef(app.stderr, "scan: discover sessions\n")
		return 1
	}
	result, err := app.scanFiles(context.Background(), database, files)
	if err != nil {
		writef(app.stderr, "scan: process sessions\n")
		return 1
	}
	if *jsonOutput {
		if err := json.NewEncoder(app.stdout).Encode(result); err != nil {
			return 1
		}
		return 0
	}
	if !writef(app.stdout, "scanned %d Claude and %d Codex files: %d inserted, %d updated, %d duplicates, %d unparsed lines\n",
		result.Claude.Files, result.Codex.Files, result.Inserted, result.Updated, result.Duplicates,
		result.Claude.UnparsedLines+result.Codex.UnparsedLines) {
		return 1
	}
	return 0
}

func (app application) runServe(args []string) int {
	databasePath, err := app.defaultDatabasePath()
	if err != nil {
		writef(app.stderr, "serve: locate configuration directory\n")
		return 1
	}
	address := serveAddress
	flags := flag.NewFlagSet("serve", flag.ContinueOnError)
	flags.SetOutput(app.stderr)
	flags.Usage = func() {
		writef(app.stderr, "usage: agentmeter serve [--db PATH] [--addr HOST:PORT] [--live] [--budget-5h N] [--budget-7d N]\n")
	}
	flags.StringVar(&databasePath, "db", databasePath, "SQLite database path")
	flags.StringVar(&address, "addr", address, "listen address")
	live := flags.Bool("live", false, "fetch authoritative limits: reads local agent credentials and contacts Anthropic and OpenAI")
	budgetFiveHour := flags.Int64("budget-5h", 0, "token ceiling for the estimated five-hour window (0 compares against your busiest window)")
	budgetSevenDay := flags.Int64("budget-7d", 0, "token ceiling for the estimated weekly window (0 compares against your busiest window)")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 {
		flags.Usage()
		return 2
	}
	if err := ensureDatabaseParent(databasePath); err != nil {
		writef(app.stderr, "serve: create database directory\n")
		return 1
	}
	database, err := app.openStore(context.Background(), databasePath)
	if err != nil {
		writef(app.stderr, "serve: open database\n")
		return 1
	}
	defer func() { _ = database.Close() }()

	service := &limits.Service{
		Store:   database,
		Budgets: limits.Budgets{FiveHour: *budgetFiveHour, SevenDay: *budgetSevenDay},
	}
	if *live {
		providers, err := app.liveProviders()
		if err != nil {
			writef(app.stderr, "serve: locate home directory\n")
			return 1
		}
		service.Providers = providers
		// Contacting a vendor is the one thing agentmeter does that leaves the
		// machine, so it announces itself rather than happening quietly.
		if !writef(app.stdout, "live limits enabled: reading local agent credentials and contacting Anthropic and OpenAI\n") {
			return 1
		}
	}

	if !writef(app.stdout, "serving on http://%s\n", address) {
		return 1
	}
	if err := app.listenAndServe(address, app.handler(database, service)); err != nil {
		writef(app.stderr, "serve: %v\n", err)
		return 1
	}
	return 0
}

// liveProviders builds the opt-in authoritative providers. Nothing calls this
// unless the operator passed --live.
func (app application) liveProviders() ([]limits.Provider, error) {
	home, err := app.userHomeDir()
	if err != nil {
		return nil, err
	}
	return []limits.Provider{
		provider.NewClaude(claudeConfigRoots(home), app.version),
		provider.NewCodex(codexRoot(home)),
	}, nil
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
