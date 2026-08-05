# AGENTS.md

Working agreement for AI coding agents in this repository. Claude Code, Codex, and any
other agent read this file. `CLAUDE.md` points here; keep the rules in one place.

## Non-negotiables

1. **Never commit to `main` or `dev`.** Work on a branch, open a PR. See
   `docs/conventions/branching.md`.
2. **Never add a dependency without asking.** This project's dependency budget is one
   runtime module. Propose, justify, wait.
3. **Never put real session log data in the repo.** Fixtures are hand-authored and
   synthetic. See "Privacy" below — this is the most important rule in this file.
4. **Every commit message is a valid Conventional Commit.** See
   `docs/conventions/commits.md`. The `commit-msg` hook enforces it; do not bypass with
   `--no-verify`.
5. **Do not claim a task is done without running `make check`** and reporting the actual
   output.

## Privacy

Claude Code and Codex session logs contain real source code, file contents, and prompts
from whatever the operator was working on. Treat every `.jsonl` file under `~/.claude` or
`~/.codex` as confidential.

- Do not read real session files into your context to "understand the format" unless
  explicitly asked. Ask for a redacted sample instead.
- Do not paste snippets of real session files into commit messages, PR descriptions, issue
  comments, or test fixtures.
- All fixtures live in `testdata/synthetic/` and are written by hand to exercise a specific
  shape. If a fixture contains a real file path, real code, or a real prompt, it is wrong.

## How to work

**Plan before you code.** For anything beyond a one-line fix, state the approach, the files
you will touch, and the commit sequence. Wait for approval.

**Work in small commits.** One logical change per commit. A commit that touches the parser,
the schema, and the UI is three commits.

**Tests are part of the change, not a follow-up.** A PR that adds parsing logic without a
fixture-backed test is incomplete.

**Report failures honestly.** If `make check` fails, say so and show the output. Do not
describe intended behavior as if it were verified behavior. If you could not verify
something, label it explicitly as unverified.

**Ask when the spec is ambiguous.** Guessing at intent and building on the guess is the
most expensive failure mode here. One clarifying question costs a minute; a wrong
assumption costs a session.

## Parser work — specific rules

The two data sources have different semantics. Getting this wrong produces numbers that
look plausible and are wrong, which is worse than a crash.

| | Claude Code | Codex |
|---|---|---|
| Token records | Per-message absolute values | **Cumulative running counters** |
| Aggregation | Sum directly | **Diff consecutive events within a session** |
| Counter reset | N/A | Clamp negative diffs to zero |
| Model attribution | On the record | Separate `turn_context` event — parser state |
| Ordering | File order sufficient | **Strict file order required** |
| Dedupe key | `messageID:requestID` | `sessionID:eventIndex` |

Additional rules:

- **Stream, never load.** Session files reach gigabytes. Use `bufio.Scanner` with a raised
  buffer, one JSON value per line, no whole-file reads.
- **Count unparsed lines; never swallow them.** Surface the count. Silent skipping turns a
  format change into "I used the tool less this month."
- **When the same relative path exists in both `sessions/` and `archived_sessions/`**, the
  active copy wins.
- **Codex session logs before September 2025 contain no token data at all.** Absence is
  expected, not a bug.

## Definition of done

A change is done when all of these are true and you have verified each:

- [ ] `make check` passes (format, vet, lint, test)
- [ ] New behavior has a test; changed behavior has an updated test
- [ ] Cross-compilation to both `windows/amd64` and `linux/amd64` still succeeds
- [ ] Commit messages pass the hook
- [ ] No new dependency was added without approval
- [ ] For parser changes: totals reconcile against `npx ccusage@latest daily --json`
      (and `@ccusage/codex` for Codex), and you have stated the delta

That last item is the correctness oracle for this project. A parser is not done because it
runs — it is done because its numbers match an independently maintained implementation.

## Commands

```bash
make check      # format + vet + lint + test — run before every commit
make test       # go test -race ./...
make build      # host binary into ./bin
make cross      # windows/amd64 and linux/amd64 into ./dist
make hooks      # install .githooks as core.hooksPath (run once after clone)
```
