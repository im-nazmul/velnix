# ADR 0003 — Separate rootless Nginx and per-project FPM first

- Status: Rootless static HTTP implemented; foreground per-project FPM and optional rootless HTTPS accepted for Phase 3, pending implementation
- Date: 2026-10-20
- Related original decisions: D007, D011

## Context

Shared root Nginx interprets configuration with system authority and may already serve unrelated sites.

## Proposed decision

Own a separate unprivileged Nginx prefix with loopback ports 8080/8443, private logs/PID/temp paths, and existing FPM binaries with per-project user-owned configs/sockets. Validate and health-check changes. Do not change shared services or globally switch PHP.

## Alternatives and consequences

Alternatives: system Nginx and privileged ports improve URLs but require a hardened privileged configuration boundary. High-port URLs and explicit workload lifetimes are accepted proposed v0.1 costs. Rootless compatibility must pass the Phase 2/3 experiments; no automatic privileged fallback.

## Validation and approval

The user subsequently approved [Phase 3](../PHASE-3-PROPOSAL.md) with “Approved,
start implementation”, extending acceptance to its scoped FPM/HTTPS design.
This approval does not establish unperformed platform acceptance.

Evidence and unresolved experiments: [FEASIBILITY](../FEASIBILITY.md). Required acceptance checks: [TESTING](../TESTING.md). Security conditions: [SECURITY](../SECURITY.md). Scope and phase gate: [IMPLEMENTATION-PLAN](../IMPLEMENTATION-PLAN.md). Approval must be recorded explicitly before implementation; this ADR does not authorize host changes.

## Phase 2 authorization and implementation

The user approved [the scoped HTTP completion proposal](../PHASE-2-HTTP-PROPOSAL.md)
with “You have my approval” on 2026-10-20. See [HTTP](../HTTP.md) for implemented
behavior and [validation](../validation/PHASE-2-HTTP.md) for actual local evidence.
This accepts only the subset in the status above, not later milestone work or
host package, system service, DNS or trust mutations.
