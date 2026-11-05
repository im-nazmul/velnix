# Phase 5 — bounded terminal interface

Status: COMPLETE within the approved read-only scope. See
[completion evidence](validation/PHASE-5-COMPLETION.md). Approved for implementation
by “Lets begun implementation”. This
approval covers the bounded read-only milestone below. Existing Phase 3/4
completion and open release gates remain unchanged.

## Deliverable

Add `velnix tui`, an optional Bubble Tea/Lip Gloss terminal interface with
overview, projects, runtimes, services and diagnostics screens. The first
milestone is read-only: users can inspect the existing environment without
granting a terminal session blanket mutation authority.

- Overview: registered project count, configuration/platform diagnostics and
  existing owned HTTP/FPM/TLS status, with unavailable and partial states.
- Projects: list, filter and inspect registered paths, domains, web profiles,
  runtime selections and bounded framework metadata.
- Runtimes: passive discovery and provider capabilities; show selection and
  provenance separately from verified version evidence. Never probe binaries,
  invoke managers or execute project code on screen entry or refresh.
- Services: existing Velnix-owned web/FPM status. Show database/system-service
  inspection as unsupported in this slice; do not invent a new systemd adapter
  merely to populate a screen.
- Diagnostics: existing doctor/configuration findings, actionable unavailable
  reasons and documented CLI next steps. No automatic repair.

## Architecture and interaction

Put terminal presentation in `internal/transport/tui`. Reuse typed application
services in `internal/app`, extracting small read-only query methods where the
current CLI-oriented result shapes require it. Do not shell out to the Velnix
CLI, parse rendered CLI text, duplicate registry logic or introduce a daemon.
Record the accepted boundary in a new ADR when implementation is approved.

Use asynchronous, cancellable reads with bounded work; ignore stale results
after navigation or refresh. Refresh is explicit, without continuous background
polling. Errors remain visible per panel so one unavailable capability does not
hide other results. Reading must not initialize state or repair corrupt state.

Provide keyboard navigation, filter, detail/back, refresh, help and quit;
support arrows and familiar letter shortcuts, readable focus indicators,
scrolling and small-terminal layouts. Treat paths, metadata and tool output as
untrusted display text: remove terminal control sequences and bound output.
Do not display private keys or full environments. No mouse is required.

Require interactive input/output terminals and return a clear error otherwise;
keep existing CLI/JSON workflows intact. Restore terminal state on normal exit,
cancellation and failures. No persistent process remains after exit.

## Implementation order and acceptance

1. Review and pin compatible Bubble Tea/Lip Gloss versions; introduce the command,
   model, navigation and injected query boundary. Document launch and keys.
2. Connect overview/projects, then runtimes/services/diagnostics to existing
   application reads. Keep empty, missing, denied and corrupt-state distinctions.
3. Test model transitions, cancellation, stale responses, display sanitization,
   partial failures and small sizes. Verify disposable state is unchanged by
   navigation/refresh, including absent-state startup.
4. Run Go tests, race checks, vet and build. Exercise the real interactive binary
   in a PTY for navigation, resize, refresh, Ctrl-C/quit and terminal restoration.
   Record actual platform/terminal evidence and unperformed checks separately.

Completion requires all five screens, documented keys/limitations, meaningful
model/query tests and recorded interactive acceptance. It does not establish
native Ubuntu, Windows terminal or release-wide acceptance without those runs.

## Explicit exclusions and approval boundary

Defer link/unlink, runtime selection/probing/execution, web start/stop/reload,
certificate issuance, recovery, package/runtime installation, system service
lifecycle, mail and web dashboard. These retain existing CLI approval flows.
No new registry schema, host mutation, privilege escalation or background
supervisor is needed. A later interactive mutation slice needs a separate
reviewed scope with exact-plan confirmation and stale-plan handling.

Approval of this proposal authorizes the read-only TUI implementation, pinned Go
dependencies and disposable local validation. Host package, DNS, service and
trust changes remain outside authorization. Existing release gates in
[RELEASE-ACCEPTANCE](validation/RELEASE-ACCEPTANCE.md) stay open.
