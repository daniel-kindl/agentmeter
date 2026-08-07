# PRODUCT.md

Durable product truth for agentmeter. Visual decisions live in DESIGN.md, not here.

## What it is

A local-first usage meter for Claude Code and Codex. It streams the session logs those tools
already write on this machine into SQLite, and serves a private browser dashboard showing
token consumption, estimated cost, and how much of each agent's five-hour and weekly limits
is left.

## Unique mechanism

Every competitor for attention here is a billing page owned by the vendor whose limit you are
hitting. agentmeter reads the logs already sitting on your disk, so it works with no account,
no network, and no telemetry, and it can put both agents on one surface — which no vendor
will ever do. Its second mechanism is honesty about provenance: every number says whether it
is the agent's own figure or agentmeter's estimate, and never blurs the two.

## Audience and scene

One developer, on their own machines. Open source later, so first-run must be coherent for a
stranger, but the daily user is the author.

The physical scene, which decides more than any adjective: **a second monitor, in a room lit
mainly by other screens, glanced at from about two metres away while both hands are busy on
the primary display.** It is left open for hours. Nobody sits down in front of it to read it.
This is an ambient instrument, not a report — it must be legible in peripheral vision, must
survive being ignored for an hour, and must not demand interaction to stay current.

## The job

Answer, without being touched: how much headroom is left right now, and when does it come
back? Everything else — cost, model mix, history — is secondary context that must not crowd
out that answer.

## Constraints

- **Local-first.** No network by default. Authoritative limit fetching exists but is opt-in,
  reads local agent credentials, and announces itself. See ADR-0003.
- **Privacy.** Session logs contain source code and prompts. Nothing derived from them leaves
  the machine, and no real log data enters the repository.
- **One runtime dependency.** The Go binary is self-contained, fonts included; the dashboard
  ships as embedded static assets with no build step, no framework, and no CDN.
- **Two numbers, two provenances.** Live figures come from the vendors; derived figures are
  measured against the operator's own history. A panel may never present one as the other.

## States that must be designed, not discovered

- First run with no scan yet: no history at all.
- Scanned but idle: no active five-hour block, so no reset time exists.
- Limited history: the current window is its own baseline and reads as 100%.
- Live enabled but credentials expired or rejected.
- A plan with only one window (Codex frequently reports weekly and no five-hour).
- Unpriced models present, so the cost total is knowingly incomplete.

## Brand commitments

None inherited. The current look is an unexamined default and carries no authority.
