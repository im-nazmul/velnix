# ADR 0007 — Bounded v0.1 with measured acceptance budgets

- Status: Proposed — pending explicit user approval
- Date: 2026-10-20
- Related original decisions: D013

## Context

The feature catalogue and phased roadmap do not define a shippable first release or reproducible performance claims.

## Proposed decision

Adopt the explicit CLI/existing-runtime/rootless-web v0.1 proposal in IMPLEMENTATION-PLAN and measured gates in TESTING. Target Ubuntu 24.04 amd64 on native Linux and WSL2; keep later interfaces/installers/mail/wildcard DNS outside v0.1. Require explicit approval at phase boundaries.

## Alternatives and consequences

Alternatives: all catalogue features in v0.1 delays feedback and expands security surface. Numeric budgets are provisional and unmeasured; publication requires the matrix and evidence, license selection and name review.

## Validation and approval

Evidence and unresolved experiments: [FEASIBILITY](../FEASIBILITY.md). Required acceptance checks: [TESTING](../TESTING.md). Security conditions: [SECURITY](../SECURITY.md). Scope and phase gate: [IMPLEMENTATION-PLAN](../IMPLEMENTATION-PLAN.md). Approval must be recorded explicitly before implementation; this ADR does not authorize host changes.
