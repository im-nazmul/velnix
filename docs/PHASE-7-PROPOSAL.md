# Phase 7 — bounded embedded dashboard

Status: COMPLETE within the approved bounded scope. The user approved this
proposal with “Let continue working on phase 7. you have my aproval” on
2026-11-06. See [usage](DASHBOARD.md), [ADR 0017](decisions/0017-embedded-local-dashboard.md)
and [completion evidence](validation/PHASE-7-COMPLETION.md). The explicitly
permitted HTML-preview fallback is used: plain-text mail and safe downloads
are implemented; HTML preview remains visibly unavailable. Phase 6 completion
and open release gates remain intact.

## Deliverable

An optional `velnix ui` foreground process serving embedded Vue 3/TypeScript/Vite
assets and a private Go API on `127.0.0.1:8090`. Installed Velnix requires neither
Bun nor a frontend development server. CLI/TUI work independently when the
browser and API are closed. No hosted service, public deployment or permanent
supervisor is introduced.

The first dashboard includes useful reads and a small explicit management set:

| Page | Included behavior |
|---|---|
| Overview | Registered project summary, owned workload availability, diagnostics and actionable unavailable states |
| Projects | List/filter/detail; reviewed link/unlink using explicit paths and the existing document-root rules |
| Runtimes | Passive inventory and project selections; reviewed selection of exact existing native executables and clearing a selection |
| Owned services | HTTP/FPM and mail status; explain that foreground lifecycle stays in CLI/TUI for this slice |
| Certificates | Public status/expiry/names; no private keys, issuance, trust-store action or CA download |
| Mail | Paged inbox, plain text, attachment metadata and explicitly requested safe downloads; reviewed retention pruning |
| Diagnostics | Existing doctor and configuration findings with sensitive values redacted; no repair or arbitrary config editor |

No fabricated database/system-service status or empty functional controls.
Unavailable actions explain the current capability and relevant CLI command.
Existing project ownership and backend errors remain authoritative.

## Shared application boundary

Add HTTP presentation in `internal/transport/web`, frontend source in `web/`
and a small embedded-asset package usable by the CLI build. Reuse typed services
in `internal/app`; add transport-neutral query/plan shapes where current TUI
entries are formatted display strings. Do not parse CLI/TUI output, shell out to
Velnix or use the process-owning `TerminalSession` as a web session.

The permitted mutation set is state initialization, project link/unlink, native
runtime selection/clear and mail pruning. Each uses its existing application
plan/apply logic, common lock, current-state checks and journal. Runtime selection
must retain existing approved probe semantics: show executable identity and any
version-probe execution in the plan, never execute on page entry or discovery.
Do not add managed-provider activation, runtime execution or installation.

Review displays the exact target, effects, required authority, relevant hashes,
network/probe behavior and rollback limits before an explicit Apply action.
Bind a pending plan to the authenticated browser session, typed operation and
request body; the session must have requested that exact preview. Apply still
revalidates through the backend and rejects stale plans. Expire and bound pending
plans. Disable duplicate clicks and serialize apply requests within the session;
never retry a mutation automatically after a timeout or reconnect. Present an
uncertain result as requiring inspection, not failure followed by blind replay.

Closing a tab is not a reliable cancellation signal for an accepted mutation.
Bound requests; finish or preserve backend recovery evidence after disconnect.
No registry schema changes or new resource ownership grants are proposed.

## Listener, authentication and browser protections

`velnix ui` defaults to a runtime launch plan; `--dry-run` is read-only and
`--approve PLAN_ID` starts the reviewed listener. Its plan includes loopback
address/port, credential-file location, local state context and process lifetime.
Refuse root, non-loopback configuration and occupied ports. Do not open a browser
or modify system configuration automatically. Ctrl-C/SIGTERM revoke sessions and
close the listener with bounded in-flight shutdown. Browser closure leaves the
explicit foreground process running until the user stops it.

At approved launch, generate a cryptographically random one-time bootstrap
secret in a new private runtime directory (0700, secret file 0600) using existing
safe-path conventions and a validated UID fallback. Print only its location and
the local UI address. The user explicitly reads that file and pastes the secret
into the login form. Never put credentials in URLs, shell arguments, access logs,
JSON diagnostics or browser persistent storage. Rate-limit bootstrap attempts,
expire the secret and consume it once on successful authentication. A fresh
launch creates fresh credentials. No account database or password management.

Exchange the bootstrap secret for an in-memory server session and an opaque
HttpOnly, SameSite=Strict, Path=/ host-only cookie; use a short idle timeout and a
bounded absolute lifetime. On plain loopback HTTP, document that Secure cookies
are unavailable; do not claim transport encryption. TLS and remote access are
separate scope. Logout invalidates the server session; session expiry requires a
new deliberate terminal bootstrap or restart, never a reusable hidden token.
Remove bootstrap files on consumption/shutdown; handle abandoned runtime files
without adopting unknown paths or deleting unrelated content.

Require the exact configured numeric Host including port; reject proxy-host
ambiguity, DNS rebinding and unknown authority before routing. Authenticate all
API data, mail content and downloads. Public static/login assets contain no local
data. Do not expose CORS. Require exact same-origin Origin and a session-bound
CSRF token on every state-changing endpoint, including login/logout and plan
creation/apply. Bootstrap login may obtain a short-lived anonymous CSRF nonce
from the same-origin login page; it grants no read or mutation authority.
Reject inappropriate content types and cross-site Fetch Metadata where present;
missing browser metadata does not bypass Origin/CSRF checks for writes.

Use bounded request bodies, query pagination, timeouts, connection counts,
response sizes and per-session work. Return stable domain errors without stack
traces or secrets. No generic filesystem reader, directory browser, arbitrary
provider proxy, command execution, private-key endpoint, debug server or shell
endpoint. A compromised same-UID account remains outside the isolation promise.

## Mail viewing and downloads

Use `MailService` and the restricted Mailpit client, never the provider browser
viewer or direct database access. Mail capture must already be running through
its approved foreground CLI session; stopped mail remains explicitly unavailable.
Do not disclose provider credentials, forward browser headers or expose a generic
Mailpit route. Preserve the approved flow-scoped no-egress boundary in ADR 0016.

Plain-text inspection remains passive, bounded and sanitized. Browser download
uses an exact message/attachment ID and explicit user click, not a user-supplied
server filesystem path. Add a transport-neutral bounded attachment-read method;
keep existing CLI export's reviewed filesystem-write behavior unchanged. Bind the
download to the authenticated session; force an attachment response with safe
filename, octet-stream, nosniff, no-store and no inline rendering. Never navigate
to the sender-supplied filename or URL. Prune keeps exact-plan confirmation,
irreversible-deletion disclosure and stale mailbox-snapshot rejection.

Include a separately enabled HTML preview only after hostile-content tests prove
that a maintained, pinned sanitizer plus an empty-sandbox iframe and restrictive
CSP prevent scripts, forms, active navigation, popups and all remote/subresource
loads. No allow-scripts or allow-same-origin; strip URL-bearing attributes and
CSS imports/URLs rather than trusting iframe isolation alone. Do not interpret
email HTML in the dashboard's Vue DOM. Show a plain-text/unavailable fallback if
the parser/sanitizer rejects a message. If safe HTML preview cannot be demonstrated,
ship the rest with HTML preview explicitly unavailable and record that limitation;
do not silently relax the security policy or block unrelated dashboard work.

## UI and tooling

Use a compact sidebar and clear content hierarchy with responsive small-screen
navigation, keyboard access, visible focus, readable tables/cards, light/dark
system preference and reduced-motion support. Include empty, loading, partial,
expired-session and error states. Use real reactive Vue components and local
assets. No remote fonts, CDN resources, analytics or continuous hidden polling.
Refresh is explicit in this slice, with cancellation and stale-response handling.
No WebSocket or event-stream infrastructure is necessary.

Use Bun 1.4.4, verified locally, as the initial pinned package manager/script
runner. Resolve and review compatible Vue/TypeScript/Vite and any sanitizer/test
dependencies during implementation; commit exact dependency inputs and `bun.lock`.
Commit the Bun version pin and use `bun install --frozen-lockfile` in CI. Do not
install/update the user's system runtimes. Document any build-tool Node
requirement rather than silently changing package managers.

Frontend build output must be embedded reproducibly in release builds. Define a
single build command that builds assets before Go compilation; CI also rejects
missing/stale frontend artifacts. Keep `go test ./...` usable from a clean clone
with the chosen documented asset strategy (for example tracked generated assets
with a reproducibility check). Do not use an empty placeholder embed as a
successful production dashboard. Record the final strategy in an accepted ADR
after approval. Use CSP without unsafe-eval or remote dependencies.

## Implementation checkpoints and acceptance

1. Introduce pinned tooling and the embedded shell, reviewed UI launch lifecycle,
   authentication and request guards. Record ADR 0017 for the approved boundary.
   Test the security boundary before exposing local data or mutation endpoints.
2. Connect shared typed reads and all seven pages. Verify no initialization,
   executable probe, network content fetch or host change on page entry/refresh.
3. Add the explicit plan/apply action allowlist, mail reads/downloads and reviewed
   pruning; test stale plans, session isolation, expiry, duplicate requests,
   disconnects, ambiguous results and backend recovery evidence.
4. Test hostile Host/Origin, CSRF, unauthenticated data/downloads, token leakage,
   content types, oversized/slow requests, path traversal, redirects and expired
   bootstrap/session credentials. Hostile MIME/HTML/attachment cases must prove
   no sender-controlled remote requests or active content in the browser.
5. Run frozen dependency install, type checks, frontend build, Go tests/race/vet,
   embedding reproducibility and compiled-binary smoke checks. Inspect the actual
   browser on desktop and mobile widths for navigation, login, refresh, plan
   review, stale-plan errors, mail and keyboard focus. Static checks alone do not
   establish browser acceptance. Record tooling, platforms, measured asset sizes
   and open release checks separately.

Target the existing frontend compressed-asset budget in TESTING.md; measure it
rather than claim it passes in advance. Bound launch/idle overhead and document
actual measurements without silently redefining existing budgets.

## Exclusions and approval boundary

Defer web-owned HTTP/FPM/mail start/stop/reload, certificate issuance/trust changes,
provider probes outside an explicitly reviewed native selection, package/runtime
installation, arbitrary runtime execution, database/system-service lifecycle,
configuration editing, mail backup/repair and host integration. Session-owned
workload management from a browser needs its own lifetime/reconnect design and
separate scope; existing foreground CLI/TUI remains available.

Approval authorizes this dashboard implementation, reviewed pinned frontend
packages, dependency downloads and disposable local/browser validation. Each real
state mutation still needs runtime plan review and exact approval. It does not
authorize public hosting, existing inbox deletion during tests, system package
installation, DNS/trust/firewall/service changes or Phase 8 stabilization work.
