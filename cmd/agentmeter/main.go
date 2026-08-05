// Command agentmeter provides the agentmeter command-line interface.
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"

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
	if len(args) == 0 {
		if !writef(app.stderr, "%s\n", usageText) {
			return 1
		}
		return 2
	}

	switch args[0] {
	case "scan":
		if !app.parseNoArguments("scan", args[1:]) {
			return 2
		}
		if !writef(app.stdout, "scan is not implemented\n") {
			return 1
		}
		return 0
	case "serve":
		if !app.parseNoArguments("serve", args[1:]) {
			return 2
		}
		if !writef(app.stdout, "serving on http://%s\n", serveAddress) {
			return 1
		}
		if err := app.listenAndServe(serveAddress, internalweb.Handler()); err != nil {
			writef(app.stderr, "serve: %v\n", err)
			return 1
		}
		return 0
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

func (app application) parseNoArguments(name string, args []string) bool {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(app.stderr)
	flags.Usage = func() {
		writef(app.stderr, "usage: agentmeter %s\n", name)
	}
	if err := flags.Parse(args); err != nil {
		return false
	}
	if flags.NArg() != 0 {
		if !writef(app.stderr, "%s: unexpected argument %q\n", name, flags.Arg(0)) {
			return false
		}
		flags.Usage()
		return false
	}
	return true
}

func writef(writer io.Writer, format string, args ...any) bool {
	_, err := fmt.Fprintf(writer, format, args...)
	return err == nil
}
