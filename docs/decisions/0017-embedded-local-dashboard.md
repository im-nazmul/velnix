# ADR 0017 — Embedded local dashboard with reviewed application actions

- Status: Accepted within the bounded Phase 7 scope
- Date: 2026-11-06
- Authorization: “Let continue working on phase 7. you have my aproval” following
  [the proposal](../PHASE-7-PROPOSAL.md).

## Decision

Serve prebuilt Vue 3/TypeScript/Vite assets and a typed private API from an
optional foreground `velnix ui` command on numeric IPv4 loopback. Keep HTTP
presentation in `internal/transport/web`, typed reads/plans/actions in
`app.Dashboard` and local security/session state in the transport. Do not adapt
formatted terminal output, use `TerminalSession` as a web session or introduce
a CLI subprocess/daemon/hosted service. CLI/TUI remain independent.

Pin Bun 1.4.4, exact frontend dependencies and `web/bun.lock`. Track generated
assets in `internal/dashboardassets/dist` with Go embed for clean-clone builds
and tests. A full build compiles frontend assets before Go. CI rejects missing,
stale or over-budget artifacts through an exact rebuild comparison and frozen
installation. No frontend runtime/server is required by installed Velnix.

Protect login with a private one-time bootstrap file and anonymous CSRF nonce;
exchange it for an opaque host-only HttpOnly SameSite=Strict cookie and in-memory
session with idle/absolute expiry. No credentials in URL/argv/logs/browser
persistent storage. Bind exact Host/Origin, reject forwarded/cross-site requests,
require JSON/CSRF for writes, authenticate data/downloads, bound work and use CSP
without remote assets/unsafe-eval. Plain loopback HTTP is not transport encryption
or a kernel/same-UID sandbox. Consumed/expired sessions require a deliberate restart
for a fresh bootstrap. Unrecognized abandoned runtime entries are preserved.

Allow only initialization, project link/unlink, exact native selection/clear
and mail pruning through existing backend plan/apply services and journals.
Bind each preview to session/request/backend ID with a bounded two-minute ticket.
Consume it before apply and serialize session mutations. Never retry an uncertain
apply; a bounded accepted application operation survives browser disconnect while
shutdown cancellation preserves backend recovery evidence. Workload lifecycle,
privileged integration and generic exec/filesystem/config endpoints remain excluded.

Expose public workload/certificate DTOs rather than raw ownership records that
could include health tokens, credential paths or certificate material. Page reads
remain passive; native selection probes occur only on explicitly approved apply.

Mail uses Phase 6 services and their flow-scoped no-egress boundary. Add a bounded
transport-neutral attachment read, force authenticated downloads to octet-stream
with a safe filename and keep CLI filesystem export unchanged. Never expose
Mailpit credentials, its viewer or generic routes. HTML preview is unavailable
in this slice: the proposal's explicit safe fallback is selected rather than
adding a sanitizer/iframe without complete hostile-content evidence.

## Consequences and alternatives

Build-policy amendment (2026-11-06, explicitly requested): generated dashboard
assets are no longer tracked. Source builders and CI build the frontend before
Go compilation; published-release automation embeds fresh assets and packages
the Linux amd64 binary with license notices. Clean-clone Go checks require the
frontend build first. Two consecutive frontend builds verify reproducibility
and the compressed asset size budget. Vue runtime templates are precompiled; no external fonts, CDN assets,
analytics, hidden polling, WebSockets or service worker are necessary.

One-time login is deliberately simple for a private foreground tool. It provides
no account store, remote access, dashboard TLS or session recovery after process
exit. Browser-owned workloads would need a separate reconnect/lifetime design;
existing CLI/TUI ownership is retained. Hostile HTML rendering, other browsers,
platform matrix, release packaging and comprehensive exhaustion/fault injection
remain separate acceptance work. See [DASHBOARD](../DASHBOARD.md) for the exact
runtime behavior and [local completion](../validation/PHASE-7-COMPLETION.md).
