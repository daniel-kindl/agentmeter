# Versioning and releases

[Semantic Versioning 2.0.0](https://semver.org/). Versions are computed from commit
messages by release-please — **never bump a version by hand and never tag manually.**

## The public API

SemVer only means something once "breaking" is defined. For this project, the public
surface is:

1. CLI subcommands, flags, and their output format when `--json` is used
2. The HTTP endpoints served by `serve` and their response schemas
3. The SQLite schema, insofar as an existing database must keep working

Explicitly **not** public: internal package APIs, the human-readable terminal output, the
HTML/CSS of the dashboard, and log line formats. Changing those is `refactor` or `feat`,
never breaking.

## Pre-1.0 rules

While the major version is `0`:

| Change | Bump |
|---|---|
| Breaking (`!` or `BREAKING CHANGE:`) | **minor** — `0.3.1` → `0.4.0` |
| `feat` | minor |
| `fix`, `perf`, `refactor` | patch |
| `docs`, `test`, `ci`, `build`, `chore` | none |

Note the first row: pre-1.0, breaking changes do **not** bump major. This is standard
release-please behavior and it is the main reason to stay at `0.x` while the schema is
still moving.

## Reaching 1.0

Tag `1.0.0` when all of these hold, not when the tool feels finished:

- The SQLite schema has survived a month without a breaking change
- Both parsers reconcile against their `ccusage` counterparts with a delta of zero
- The CLI surface has not changed shape in the last two releases

After 1.0, breaking changes bump major and require an entry in the changelog's
"Breaking Changes" section explaining the migration.

## Release flow

Fully automated. The only manual step is merging a PR.

```
feature branch ──▶ dev ──▶ main
                            │
                            ├─▶ release-please opens a Release PR
                            │   (bumps version, writes CHANGELOG.md)
                            │
                            └─▶ merge Release PR
                                   │
                                   ├─▶ tag vX.Y.Z pushed
                                   └─▶ GoReleaser builds and attaches binaries
```

1. Commits land on `main` via a PR from `dev`.
2. release-please (`release-type: go`) opens or updates a **Release PR** accumulating every
   unreleased commit, with the computed version and generated changelog.
3. Merging that PR creates the tag and the GitHub Release.
4. The tag triggers GoReleaser: `CGO_ENABLED=0` builds for `windows/amd64` and
   `linux/amd64`, plus a checksums file, attached to the release.

Release cadence is "whenever the Release PR is worth merging" — there is no schedule. A
release with one `fix` in it is a perfectly good release.

## Version at runtime

Injected at build time, never hardcoded in source:

```makefile
VERSION ?= dev
LDFLAGS := -X main.version=$(VERSION)
```

Local builds report `dev`. Only GoReleaser sets a real version. If `agentmeter version`
prints `dev` on a machine you thought had a release build, you are running a local build —
that ambiguity is the point.

## Changelog

`CHANGELOG.md` follows [Keep a Changelog](https://keepachangelog.com/) and is
**generated** — do not edit it by hand. Anything you want in the changelog goes in the
commit message. This is the practical reason commit hygiene matters: the commit message is
the release note, written months earlier.
