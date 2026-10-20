# ADR 0006 — Daemonless control with explicit optional workload lifetimes

- Status: Foreground Nginx implemented; foreground per-project FPM accepted for Phase 3; systemd and later workloads remain proposed
- Date: 2026-10-20
- Related original decisions: D005, D012

## Context

SMTP, web/API, Nginx and FPM need listeners while active; registry and CLI commands do not.

## Proposed decision

Keep control commands daemonless. Requested processes run foreground or through an available explicit user-systemd adapter. Record process identities and bounded logs; no hidden supervisor or implicit lingering. Evaluate external Mailpit before building mail capture; mail/web remain later milestones.

## Alternatives and consequences

Alternatives: permanent management daemon simplifies scheduling but increases idle/security burden. Fully daemonless mail capture cannot receive messages after exit. WSL service lifetime and shutdown behavior must be documented.

## Validation and approval

The user subsequently approved [Phase 3](../PHASE-3-PROPOSAL.md) with “Approved,
start implementation”, accepting its explicit foreground FPM lifecycle.

Evidence and unresolved experiments: [FEASIBILITY](../FEASIBILITY.md). Required acceptance checks: [TESTING](../TESTING.md). Security conditions: [SECURITY](../SECURITY.md). Scope and phase gate: [IMPLEMENTATION-PLAN](../IMPLEMENTATION-PLAN.md). Approval must be recorded explicitly before implementation; this ADR does not authorize host changes.

## Phase 2 authorization and implementation

The user approved [the scoped HTTP completion proposal](../PHASE-2-HTTP-PROPOSAL.md)
with “You have my approval” on 2026-10-20. See [HTTP](../HTTP.md) for implemented
behavior and [validation](../validation/PHASE-2-HTTP.md) for actual local evidence.
This accepts only the subset in the status above, not later milestone work or
host package, system service, DNS or trust mutations.
