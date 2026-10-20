# ADR 0011 — Share reviewed foreground transitions for Phase 3 web workloads

- Status: Implemented within the approved Phase 3 design
- Date: 2026-10-20
- Authorization: approved PHASE-3-PROPOSAL, followed by “Lets finishup phase 3 first”

Per-project FPM and rootless Nginx reuse one transition engine, shared mutation
lock, private file operations, process identity checks, bounded diagnostics and
conservative recovery. A named FPM service uses separate files without changing
the project registry schema. Its enabled PHP profile is in pool evidence;
HTTP binds that evidence and socket identity when explicitly started/reloaded.
Stopping a pool does not silently revoke a route; it becomes unavailable until
an explicit routing change. Disable followed by HTTP reload restores static mode.
CLI selection remains independent. Pools use one static worker to make readiness
and crash cleanup observable; ondemand tuning is deferred until measured.

Version probes disable ini. Actual FPM validation/serving uses installed ini and
extensions with a minimal environment, no distro FPM includes, one generated
pool and a private Unix socket. Module/ini/dependency bytes are not pinned;
existing installations and application code remain external and same-UID
projects are not isolated. No global PHP selection or service change occurs.

HTTPS is optional on the same Nginx workload with independent HTTP. The reviewed
mkcert v1.4.4 adapter uses an exclusive private CAROOT per issuance, exact linked
names, no install/uninstall flags and no trust stores. Fresh CA generations avoid
ambiguous partial in-place key rotation; existing generations are preserved.
This trades trust convenience for explicit ownership/recovery. It requires
manual trust rotation and exact-fingerprint untrust before removing old files.
Only public CA material can be exported.

Old HTTP readers reject newly added record fields rather than managing unfamiliar
workloads. Downgrading with active Phase 3 evidence is unsupported; preserve
records and stop/reconcile with the matching version first. Registry schema 3
and Phase 4 discovery/inspection remain unchanged.

See [FPM](../FPM.md), [HTTPS](../HTTPS.md) and
[validation](../validation/PHASE-3-WEB.md). Implementation completion does not
close unperformed mkcert, trust, Windows networking or native Ubuntu acceptance.
