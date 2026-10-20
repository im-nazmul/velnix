# Velnix — Architectural Decisions

Detailed ADRs belong in `docs/decisions/`.

| ID | Decision | Status |
|---|---|---|
| D001 | Native Linux, no mandatory containers | Accepted |
| D002 | WSL2/Ubuntu first; portable core | Accepted |
| D003 | Go core | Accepted for Phase 1 |
| D004 | CLI first, optional TUI/web | Accepted |
| D005 | Daemonless by default | Accepted |
| D006 | Delegate to package/runtime providers | Accepted in principle |
| D007 | Nginx initially | Proposed |
| D008 | Embedded Vue/TypeScript dashboard | Proposed |
| D009 | Explicit resource ownership | Accepted |
| D010 | Local HTTPS with Windows/WSL trust | Accepted requirement |
| D011 | Per-project runtime versions | Accepted requirement |
| D012 | Local-only SMTP mail capture | Accepted requirement |
| D013 | Performance budget after prototype measurement | Research |
| D014 | Velnix name and `velnix` CLI | Provisional; verify availability |

## Phase 0 detailed proposals (2026-10-20)

The statuses above describe original direction and requirements, not implementation approval. The table below records scoped acceptance for the foundation; other choices remain proposed. accepted requirements such as HTTPS and mail do not imply inclusion in v0.1.

| ADR | Proposal | Status |
|---|---|---|
| [0001](decisions/0001-core-and-state.md) | Go core, transport boundaries and durable state | Accepted for Phase 1 |
| [0002](decisions/0002-runtime-providers.md) | Small provider capabilities and independent CLI/FPM resolution | Proposed |
| [0003](decisions/0003-rootless-web.md) | Rootless separate Nginx on high ports and per-project FPM | Proposed |
| [0004](decisions/0004-dns-and-trust.md) | Exact hosts entries and separate trust-store approval | Proposed |
| [0005](decisions/0005-ownership-and-privilege.md) | Provenance, grants, plans and recovery limits | Foundation subset accepted |
| [0006](decisions/0006-optional-processes.md) | Daemonless control and optional processes | Proposed |
| [0007](decisions/0007-release-and-performance.md) | Bounded v0.1 and measurable acceptance budgets | Proposed |

See [FEASIBILITY](FEASIBILITY.md) for researched sources and unresolved experiments. Phase 1 foundation was subsequently authorized by the user: “Lets start implementation”. ADR 0001 is accepted for the foundation; ADR 0005 has its foundation state/approval subset accepted, while later privilege and resource-management choices remain proposed. Other ADRs retain their proposed status until the relevant milestone.
