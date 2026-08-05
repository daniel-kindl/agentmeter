# ADR-0002: Match usage-oracle replacement semantics

- **Status:** accepted
- **Date:** 2026-08-05
- **Deciders:** agentmeter maintainers

## Context

Claude sidechains can replay a parent response with a different request identifier, and later
records can contain a more complete token total for an existing message. Keeping the first
primary-key match made rescans idempotent but did not match the independently maintained usage
oracle. Some otherwise valid records also omit identifiers or a model.

## Decision

Normalized events retain optional message/request identity, sidechain state, and pricing class.
Missing Claude identity falls back to the session and line index; a missing model becomes
`unknown`. Explicitly blank values remain invalid.

Persistence prefers a non-sidechain event over its replay and otherwise retains the candidate
with the larger combined token total. Exact repeats remain counted duplicates. Schema version 2
adds the metadata and indexes needed for this decision while migrating version 1 databases
transactionally.

## Consequences

Rescans can report inserted, updated, and duplicate records. Historical version 1 rows remain
queryable but lack optional identity metadata, so only their exact dedupe keys participate in
replacement. Future changes to the oracle's identity rules require another explicit schema or
normalization decision.
