# ADR 0001 — Go application core and private durable state

- Status: Accepted for Phase 1 foundation by user instruction “Lets start implementation”
- Date: 2026-10-20
- Related original decisions: D003, D004, D008

## Context

A shared application layer must work without an API or daemon. State must survive concurrent commands and interrupted writes.

## Proposed decision

Use Go, thin transports and small consumer-defined ports. Use TOML configuration, versioned JSON registry, interprocess locking and a durable operation journal. Defer SQLite, TUI and embedded Vue to their milestones.

## Alternatives and consequences

Alternatives: API-only control requires a daemon; SQLite initially adds schema/build choices without measured query need. JSON requires explicit crash/concurrency discipline. Reconsider SQLite for mail or measured registry contention.

## Validation and approval

Evidence and unresolved experiments: [FEASIBILITY](../FEASIBILITY.md). Required acceptance checks: [TESTING](../TESTING.md). Security conditions: [SECURITY](../SECURITY.md). Scope and phase gate: [IMPLEMENTATION-PLAN](../IMPLEMENTATION-PLAN.md). Foundation authorization is recorded above. Later milestone decisions still require explicit approval; this ADR does not authorize system changes.

## Phase 1 implementation detail

Standard-library flag parsing keeps the initial command tree small. TOML uses pinned go-toml/v2 v2.4.3 (MIT). Initial schema is 1; newer schemas are refused, with migration code deferred until another supported schema exists. One bounded initialization journal records the sole foundation transaction. Linux descriptor-relative/no-follow state access, flock and atomic no-replace publication implement private durable state; pinned golang.org/x/sys v0.30.0 supplies portable syscall wrappers. See [FOUNDATION](../FOUNDATION.md) for behavior and limits.
