# ADR 0002 — Capability-driven runtime resolution without implicit installation

- Status: Proposed — pending explicit user approval
- Date: 2026-10-20
- Related original decisions: D006, D011

## Context

General version managers do not guarantee PHP-FPM support, safe discovery or identical installation authority.

## Proposed decision

Separate discover/resolve/install/remove contracts. Read-only existing runtimes in v0.1; optional mise Node/Bun and uv Python resolution only after nonmutating adapter validation. Keep nvm, phpenv, PHPBrew and pyenv external. CLI and FPM resolve independently; exact missing versions fail.

## Alternatives and consequences

Alternatives: one mandatory mise provider narrows user choice and obscures backend trust; custom runtime builds reinvent package managers. Provider compatibility and trusted executable probing remain validation work.

## Validation and approval

Evidence and unresolved experiments: [FEASIBILITY](../FEASIBILITY.md). Required acceptance checks: [TESTING](../TESTING.md). Security conditions: [SECURITY](../SECURITY.md). Scope and phase gate: [IMPLEMENTATION-PLAN](../IMPLEMENTATION-PLAN.md). Approval must be recorded explicitly before implementation; this ADR does not authorize host changes.
