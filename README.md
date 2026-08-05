# agentmeter

Agentmeter is a local-first usage meter for Claude Code and Codex sessions. It streams local
session logs into SQLite and serves a private browser dashboard with token and estimated-cost
history.

## Requirements

- Go 1.26
- GNU Make
- `golangci-lint` v2

## Development

```sh
make hooks
make check
make build
make cross
```

Scan the standard Claude Code and Codex data directories, then start the dashboard:

```sh
agentmeter scan
agentmeter serve
```

The database defaults to the `agentmeter/agentmeter.db` file under the operating system's
user configuration directory. Use `--db PATH` with both commands to select another database,
`scan --json` for machine-readable statistics, and `serve --addr HOST:PORT` to change the
loopback listener.

Costs come from a bundled pricing snapshot. Tokens from an unknown model remain visible and
the dashboard marks the cost total incomplete rather than treating the model as free. See
[CONTRIBUTING.md](CONTRIBUTING.md) before making changes.

## Privacy

Session logs may contain source code, file contents, and prompts. Never add real Claude Code
or Codex session data to this repository. Test fixtures must be hand-authored and live under
`testdata/synthetic/`.

## License

Apache License 2.0. See [LICENSE](LICENSE).

