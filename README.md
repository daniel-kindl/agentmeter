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

## Usage

One step — scan the standard Claude Code and Codex data directories, serve the dashboard,
and open it:

```sh
agentmeter up
```

Release archives ship `agentmeter-up.cmd` and `agentmeter-up.sh` beside the binary, so the
same thing works from a double-click without a terminal. Pass `--no-open` to skip the
browser. The two steps are also available separately:

```sh
agentmeter scan
agentmeter serve
```

The database defaults to the `agentmeter/agentmeter.db` file under the operating system's
user configuration directory. Use `--db PATH` with any command to select another database,
`scan --json` for machine-readable statistics, and `--addr HOST:PORT` to change the loopback
listener.

Costs come from a bundled pricing snapshot. Tokens from an unknown model remain visible and
the dashboard marks the cost total incomplete rather than treating the model as free. See
[CONTRIBUTING.md](CONTRIBUTING.md) before making changes.

## Usage limits

The dashboard shows five-hour and weekly meters for each agent. By default they are derived
entirely from your local history: usage is grouped into five-hour blocks and rolling
seven-day spans, and each window is measured against the **busiest comparable window
agentmeter has recorded**, because no published quota is knowable offline.

These are estimates and are labelled as such. They will not match `/usage` in Claude Code or
`/status` in Codex. If you know your plan's real token ceilings, supply them:

```sh
agentmeter serve --budget-5h 1900000 --budget-7d 20000000
```

### Authoritative limits (opt-in, off by default)

`--live` replaces the estimates with the numbers the agents themselves report:

```sh
agentmeter up --live
```

Read this before using it. Unlike everything else agentmeter does, `--live`:

- reads your local agent credentials — `~/.claude/.credentials.json` (or
  `CLAUDE_CODE_OAUTH_TOKEN`) and `~/.codex/auth.json`;
- sends them to `api.anthropic.com` and `chatgpt.com`, both **undocumented** endpoints that
  can change or disappear without notice;
- means agentmeter is no longer offline.

Anthropic's legal and compliance guidance, effective 2026-02-20, restricts Claude
subscription OAuth tokens to Claude Code and Claude.ai. Reading the token to check your own
quota routes no model traffic through it, but the guidance carves out no exception, so this
is your call to make on your own account.

agentmeter never refreshes an expired token — that would rewrite your credentials file
without being asked — and reports the expiry instead. On macOS, Claude Code keeps credentials
in the keychain rather than on disk, so set `CLAUDE_CODE_OAUTH_TOKEN` there. Responses are
cached for three minutes, and any failure falls back to the last known values, or to the
derived estimate, rather than blanking the panel.

See [ADR-0003](docs/adr/0003-opt-in-authoritative-limit-providers.md) for the reasoning.

## Privacy

Session logs may contain source code, file contents, and prompts. Never add real Claude Code
or Codex session data to this repository. Test fixtures must be hand-authored and live under
`testdata/synthetic/`.

The dashboard embeds its own fonts rather than loading them from a font CDN. A page about
your private usage should not have to announce itself to a third party in order to render,
and the binary is meant to work with no network at all.

## License

Apache License 2.0. See [LICENSE](LICENSE).

The bundled IBM Plex Sans and IBM Plex Mono files under `web/fonts/` are © 2017 IBM Corp.,
licensed under the SIL Open Font License 1.1. See [web/fonts/LICENSE](web/fonts/LICENSE).

