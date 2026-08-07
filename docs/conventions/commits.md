# Commit conventions

[Conventional Commits 1.0.0](https://www.conventionalcommits.org/en/v1.0.0/). Enforced by
the `commit-msg` hook and re-checked in CI. Release tooling parses these messages to
compute the next version and generate the changelog, so a sloppy message produces a wrong
release, not just an ugly log.

## Format

```
<type>(<scope>)!: <description>

[body]

[footer(s)]
```

- **Header ≤ 72 characters**, imperative mood, no trailing period.
- `!` before the colon marks a breaking change. It may be used with or without a
  `BREAKING CHANGE:` footer, but the footer is preferred because it carries an explanation.
- Body wraps at 100 characters. Explain **why**, not what — the diff already says what.

## Types

| Type | Use for | Version effect |
|---|---|---|
| `feat` | New user-visible capability | minor |
| `fix` | Bug fix | patch |
| `perf` | Performance improvement, no behavior change | patch |
| `refactor` | Restructuring with no behavior change | patch |
| `docs` | Documentation only | none |
| `test` | Tests only | none |
| `build` | Build system, Makefile, compiler flags | none |
| `ci` | CI configuration and workflows | none |
| `chore` | Housekeeping that fits nothing else | none |
| `revert` | Reverts a previous commit | matches reverted |

Anything with `!` or a `BREAKING CHANGE:` footer overrides the table — see
`versioning.md` for how that maps pre-1.0.

## Scopes

Scope is **required** for `feat`, `fix`, `perf`, and `refactor`; optional elsewhere.

Adjust this list per project; keep it short enough to memorize:

`scanner`, `parser`, `store`, `web`, `ui`, `pricing`, `limits`, `cli`, `config`, `hooks`,
`deps`

For parser work, use the sub-scope form: `parser/claude`, `parser/codex`.

## Rules

1. **One logical change per commit.** If the description needs "and", split it.
2. **No `--no-verify`.** If the hook rejects the message, the message is wrong. If the hook
   itself is wrong, fix the hook in its own commit.
3. **Reference issues in the footer**, not the header: `Refs: #12`, `Closes: #12`.
4. **`revert` commits** use the footer `Reverts: <sha>`.
5. **Never rewrite published history on `main` or `dev`.** Rewriting your own unmerged
   feature branch is fine and encouraged.

## Examples

Good:

```
feat(parser/codex): diff cumulative token counters per session

Codex emits token_count events as running session totals rather than per-turn
deltas. Summing them overcounts by orders of magnitude on long sessions, so
consecutive events are diffed and negative diffs are clamped to zero to survive
counter resets after compaction.

Refs: #14
```

```
fix(scanner): reset byte offset when a session file shrinks

A rotated or rewritten file kept its stored offset and every subsequent scan
read from the middle of a JSON line, silently dropping the file's usage.
```

```
refactor(store)!: replace composite dedupe columns with a single primary key

BREAKING CHANGE: existing databases must be deleted and rescanned. There is no
migration path; the data is fully derivable from the source logs.
```

Bad, with the reason:

| Message | Problem |
|---|---|
| `fix: bug` | No scope, no information |
| `feat(parser): add codex parser and fix claude dedupe and update docs` | Three commits wearing a trenchcoat |
| `Updated the scanner.` | Not a Conventional Commit; past tense; trailing period |
| `feat(parser/codex): handle token_count events properly` | "Properly" says nothing — what was wrong? |
