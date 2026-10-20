# ADR 0001 — Go application core and private durable state

- Status: Proposed — pending explicit user approval
- Date: 2026-10-20
- Related original decisions: D003, D004, D008

## Context

A shared application layer must work without an API or daemon. State must survive concurrent commands and interrupted writes.

## Proposed decision

Use Go, thin transports and small consumer-defined ports. Use TOML configuration, versioned JSON registry, interprocess locking and a durable operation journal. Defer SQLite, TUI and embedded Vue to their milestones.

## Alternatives and consequences

Alternatives: API-only control requires a daemon; SQLite initially adds schema/build choices without measured query need. JSON requires explicit crash/concurrency discipline. Reconsider SQLite for mail or measured registry contention.

## Validation and approval

Evidence and unresolved experiments: [FEASIBILITY](../FEASIBILITY.md). Required acceptance checks: [TESTING](../TESTING.md). Security conditions: [SECURITY](../SECURITY.md). Scope and phase gate: [IMPLEMENTATION-PLAN](../IMPLEMENTATION-PLAN.md). Approval must be recorded explicitly before implementation; this ADR does not authorize host changes.
