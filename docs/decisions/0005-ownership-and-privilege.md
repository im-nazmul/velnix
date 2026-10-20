# ADR 0005 — Separate provenance, management grants and recoverable effects

- Status: Foundation subset accepted (private state, plan approval, locks and initialization recovery); later privilege/provider/adoption operations remain proposed
- Date: 2026-10-20
- Related original decisions: D009

## Context

Discovery is not ownership; cross-provider and cross-OS changes cannot be rolled back atomically.

## Proposed decision

Default discovered resources to external/read-only. Track provider provenance, resource identity, explicit grants and before/after evidence. Require plan-bound approval for effects, lock mutations and journal before changes. No privileged helper in v0.1; later elevation uses typed one-shot allowlisted actions.

## Alternatives and consequences

Alternatives: sudo the CLI or infer ownership from location is unsafe. General rollback promises cannot cover package scripts, services, trust or data. Compare-before-restore, reconciliation and manual recovery are required costs.

## Validation and approval

Evidence and unresolved experiments: [FEASIBILITY](../FEASIBILITY.md). Required acceptance checks: [TESTING](../TESTING.md). Security conditions: [SECURITY](../SECURITY.md). Scope and phase gate: [IMPLEMENTATION-PLAN](../IMPLEMENTATION-PLAN.md). Foundation authorization is recorded above. Later milestone decisions still require explicit approval; this ADR does not authorize system changes.
