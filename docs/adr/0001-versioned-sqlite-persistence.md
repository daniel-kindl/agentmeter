# ADR-0001: Adopt versioned single-connection SQLite persistence

- **Status:** accepted
- **Date:** 2026-08-05
- **Deciders:** agentmeter maintainers

## Context

Agentmeter needs a local persistence layer before scanners and parsers can save normalized
usage events. Rescanning the same append-only session data must not inflate totals, and a
failed or canceled batch must not leave a partial scan in the database. The SQLite schema
is part of the project's public compatibility surface, so binaries must detect databases
whose schema they cannot safely interpret.

Agentmeter is a local command-line application. Its expected write workload is serialized,
and predictable behavior is more valuable than maximizing concurrent database throughput.
SQLite is already the project's sole runtime dependency through the pure-Go
`modernc.org/sqlite` driver.

## Options considered

| Option | Pros | Cons |
|---|---|---|
| Versioned SQLite with one connection and transactional batches | Embedded, durable, explicit compatibility checks, atomic writes, deterministic local concurrency | Serializes database work; migrations and transaction boundaries require maintenance |
| Versioned SQLite with a larger connection pool | Allows concurrent database calls | Adds lock contention and connection-specific in-memory database behavior without a current workload benefit |
| Flat files with application-managed deduplication | No database schema or SQL | Requires custom indexing, atomic replacement, recovery, and compatibility mechanisms |
| Do nothing and keep usage only in memory | No persistence implementation | Every report must rescan all logs, and incremental/idempotent scanning is unavailable |

## Decision

We will persist normalized usage events in a `PRAGMA user_version`-versioned SQLite schema,
use one database connection, and insert each validated batch in one transaction while
treating only `dedupe_key` conflicts as counted duplicates.

Schema version zero is migrated transactionally to the binary's current schema version.
A binary refuses to open a database with a newer version, because proceeding could corrupt
or misinterpret data. Opening a current-version database makes no schema changes.

Each event batch is validated before writes begin and committed as a unit. Cancellation,
validation failures, constraint violations, and database errors leave the batch unwritten.
An insert uses `ON CONFLICT(dedupe_key) DO NOTHING`, so duplicates already stored or repeated
within a batch are successful no-ops and are reported separately from inserted rows. Other
constraint failures remain errors and roll back the transaction.

## Consequences

**Accepted costs.** All database work shares one connection, so independent callers cannot
execute SQLite operations concurrently. Every schema change needs an ordered migration and
a `user_version` increment. Callers must retain source logs as the ultimate recoverable data
source.

**Risks and failure modes.** A long-running transaction blocks all other store operations.
An incorrect migration can make a previously valid database unusable, while an omitted
version increment can make an incompatible schema appear current. Tests must cover upgrades,
future-version rejection, rollback, and idempotent rescans.

**Revisit when.** Reconsider the single-connection limit if measured store contention makes
a scan or report wait more than one second. Reconsider the migration strategy if a schema
change cannot be completed atomically or needs to preserve data SQLite cannot transform
safely in place.
