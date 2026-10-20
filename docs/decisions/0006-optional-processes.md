# ADR 0006 — Daemonless control with explicit optional workload lifetimes

- Status: Proposed — pending explicit user approval
- Date: 2026-10-20
- Related original decisions: D005, D012

## Context

SMTP, web/API, Nginx and FPM need listeners while active; registry and CLI commands do not.

## Proposed decision

Keep control commands daemonless. Requested processes run foreground or through an available explicit user-systemd adapter. Record process identities and bounded logs; no hidden supervisor or implicit lingering. Evaluate external Mailpit before building mail capture; mail/web remain later milestones.

## Alternatives and consequences

Alternatives: permanent management daemon simplifies scheduling but increases idle/security burden. Fully daemonless mail capture cannot receive messages after exit. WSL service lifetime and shutdown behavior must be documented.

## Validation and approval

Evidence and unresolved experiments: [FEASIBILITY](../FEASIBILITY.md). Required acceptance checks: [TESTING](../TESTING.md). Security conditions: [SECURITY](../SECURITY.md). Scope and phase gate: [IMPLEMENTATION-PLAN](../IMPLEMENTATION-PLAN.md). Approval must be recorded explicitly before implementation; this ADR does not authorize host changes.
