# ADR 0003 — Separate rootless Nginx and per-project FPM first

- Status: Proposed — pending explicit user approval
- Date: 2026-10-20
- Related original decisions: D007, D011

## Context

Shared root Nginx interprets configuration with system authority and may already serve unrelated sites.

## Proposed decision

Own a separate unprivileged Nginx prefix with loopback ports 8080/8443, private logs/PID/temp paths, and existing FPM binaries with per-project user-owned configs/sockets. Validate and health-check changes. Do not change shared services or globally switch PHP.

## Alternatives and consequences

Alternatives: system Nginx and privileged ports improve URLs but require a hardened privileged configuration boundary. High-port URLs and explicit workload lifetimes are accepted proposed v0.1 costs. Rootless compatibility must pass the Phase 2/3 experiments; no automatic privileged fallback.

## Validation and approval

Evidence and unresolved experiments: [FEASIBILITY](../FEASIBILITY.md). Required acceptance checks: [TESTING](../TESTING.md). Security conditions: [SECURITY](../SECURITY.md). Scope and phase gate: [IMPLEMENTATION-PLAN](../IMPLEMENTATION-PLAN.md). Approval must be recorded explicitly before implementation; this ADR does not authorize host changes.
