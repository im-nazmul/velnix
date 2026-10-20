# ADR 0004 — Exact local domains and separately authorized trust stores

- Status: Implemented for Phase 3 manual hosts/trust instructions and optional isolated mkcert v1.4.4 issuance; real mkcert/platform/trust acceptance open
- Date: 2026-10-20
- Related original decisions: D010

## Context

WSL localhost transport, hostname resolution and browser trust are independent mechanisms. Reserved .test names do not resolve automatically.

## Proposed decision

Use exact Linux/Windows hosts instructions in v0.1; defer wildcard DNS. Optional mkcert with dedicated CAROOT; copy public CA only. Separate Linux, Windows CurrentUser and browser trust verification; no automatic WSL/network/firewall changes.

## Alternatives and consequences

Alternatives: dnsmasq/NRPT can enable wildcards but add resolver/VPN/admin complexity. LocalMachine trust affects all users. Manual setup is a visible v0.1 limitation; actual Windows browser validation remains a release gate.

## Validation and approval

The user approved [Phase 3](../PHASE-3-PROPOSAL.md) with “Approved, start
implementation”. This authorizes no automatic hosts or trust-store mutation;
mkcert must not run `-install`.

Evidence and unresolved experiments: [FEASIBILITY](../FEASIBILITY.md). Required acceptance checks: [TESTING](../TESTING.md). Security conditions: [SECURITY](../SECURITY.md). Scope and phase gate: [IMPLEMENTATION-PLAN](../IMPLEMENTATION-PLAN.md). Approval must be recorded explicitly before implementation; this ADR does not authorize host changes.
