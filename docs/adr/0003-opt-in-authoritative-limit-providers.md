# ADR-0003: Derive usage limits locally, fetch authoritative ones on request

- **Status:** accepted
- **Date:** 2026-08-07
- **Deciders:** agentmeter maintainers

## Context

Tokens and cost describe what was spent. The number that decides whether to start another
task is what remains against the five-hour and weekly caps both agents enforce.

Those percentages are not in the session logs. Claude Code's `/usage` reads
`api.anthropic.com/api/oauth/usage` with the OAuth token from `~/.claude/.credentials.json`.
Codex records a `rate_limits` field on `token_count` events, but it is `null` on current
releases, and the CLI polls `chatgpt.com/backend-api/wham/usage` for its own status line.
Neither endpoint is documented, and Anthropic's beta header is versioned.

Anthropic's legal and compliance guidance, effective 2026-02-20, restricts subscription OAuth
tokens to Claude Code and Claude.ai. Reading the token to inspect quota routes no model
traffic, but the guidance carves out no exception for it.

Against that, agentmeter's stated contract is local-first: it streams local logs into SQLite
and serves a private dashboard. Nothing else it does leaves the machine or reads a
credential.

## Decision

Both, with the local path as the default.

Usage events are grouped into five-hour blocks and non-overlapping seven-day spans, and each
window is reported against the busiest comparable window in local history. No quota is
knowable offline, so the comparison is to the operator's own past rather than to a published
number, the weekly window is labelled rolling and omits a reset instant, and windows carry an
estimated origin. Blocks open at the first event floored to the hour and close after five
idle hours, matching the usage oracle. Operators who know their plan's ceilings can supply
them with `--budget-5h` and `--budget-7d`.

Authoritative providers exist behind `serve --live`, which is off by default, prints a notice
when it takes effect, and is documented as reading local agent credentials and contacting
both vendors. Fresh snapshots are cached for three minutes because Anthropic throttles below
that interval. Expiry is reported rather than repaired: a refresh grant would rewrite the
operator's credentials file unasked.

Precedence is live, then the last cached snapshot marked stale, then the derived estimate. A
source never blanks because a credential expired.

## Consequences

The dashboard shows limit meters on any machine with scanned history and no configuration.
Operators who want the numbers `/usage` and `/status` report opt in knowingly, having been
told what that costs, and accept that undocumented endpoints can break without notice.

Estimated percentages will not match the agents' own. That is stated on every panel rather
than papered over. A source whose whole history fits in one block or one week is its own
baseline and reads as a hundred percent; the panel says so.

Schema version 3 adds `limit_snapshots` as a freshness cache and offline fallback. It stores
percentages and reset instants only, never session content. A future vendor change to either
response shape requires a parsing change here, not a schema change, because window kinds are
open strings.
