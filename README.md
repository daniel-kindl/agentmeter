# agentmeter

Agentmeter is a local-first usage meter for Claude Code and Codex sessions. This repository
currently contains a streaming Claude Code usage parser and the storage foundation. Codex
parsing, reporting queries, API behavior, and the dashboard are not implemented yet.

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

The CLI surface is reserved for `scan`, `serve`, and `version`; `scan` is not wired to the
Claude parser yet. See [CONTRIBUTING.md](CONTRIBUTING.md) before making changes.

## Privacy

Session logs may contain source code, file contents, and prompts. Never add real Claude Code
or Codex session data to this repository. Test fixtures must be hand-authored and live under
`testdata/synthetic/`.

## License

Apache License 2.0. See [LICENSE](LICENSE).

