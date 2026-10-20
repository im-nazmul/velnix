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
| D007 | Nginx initially | Accepted for Phase 2 rootless static HTTP |
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
| [0002](decisions/0002-runtime-providers.md) | Small provider capabilities and independent CLI/FPM resolution | Direct existing PHP selection accepted for Phase 3; manager adapters proposed |
| [0003](decisions/0003-rootless-web.md) | Rootless separate Nginx on high ports and per-project FPM | Rootless static HTTP implemented; FPM/HTTPS accepted for Phase 3 |
| [0004](decisions/0004-dns-and-trust.md) | Exact hosts entries and separate trust-store approval | Manual setup and optional isolated mkcert accepted for Phase 3 |
| [0005](decisions/0005-ownership-and-privilege.md) | Provenance, grants, plans and recovery limits | Foundation subset accepted |
| [0006](decisions/0006-optional-processes.md) | Daemonless control and optional processes | Foreground Nginx/FPM accepted; systemd and later workloads proposed |
| [0007](decisions/0007-release-and-performance.md) | Bounded v0.1 and measurable acceptance budgets | Proposed |
| [0008](decisions/0008-project-registry-transactions.md) | Project metadata, schema upgrade and recoverable transactions | Accepted for first Phase 2 slice |
| [0009](decisions/0009-existing-php-cli.md) | Exact existing PHP CLI selections, approved probes and explicit exec | Accepted for first Phase 3 slice |

See [FEASIBILITY](FEASIBILITY.md) for researched sources and unresolved experiments. Phase 1 foundation was subsequently authorized by the user: “Lets start implementation”. ADR 0001 is accepted for the foundation; ADR 0005 has its foundation state/approval subset accepted, while later privilege and resource-management choices remain proposed. Other ADRs retain their proposed status until the relevant milestone.

Phase 2 was subsequently authorized by “Lets start work with phase 2”. ADR 0008 records the first registry slice and extends the durable state/approval subset to project metadata. The user subsequently approved [the HTTP completion proposal](PHASE-2-HTTP-PROPOSAL.md) with “You have my approval”. Phase 3 was authorized by “Lets start work with phase 3” and [the Phase 3 proposal](PHASE-3-PROPOSAL.md) approved with “Approved, start implementation”. ADR 0009 records the first implemented slice. Approval extends ADRs 0002–0004 and 0006 only to the Phase 3 proposal's direct PHP, foreground FPM and rootless optional HTTPS design. Systemd, provider-managed resolution and later workloads remain proposed; host package/service/DNS/trust mutations are not authorized.
