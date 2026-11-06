# Embedded local dashboard

Phase 7 adds `velnix ui`: an optional foreground Go listener serving embedded
Vue assets and a private local API. Installed Velnix needs neither Bun nor a
frontend server. Existing CLI/TUI workflows remain independent. See
[approved scope](PHASE-7-PROPOSAL.md) and [ADR 0017](decisions/0017-embedded-local-dashboard.md).

## Build and open

From the checkout, with Go 1.27.3+ and the Bun version in `.bun-version`:

```sh
sh scripts/build.sh
bin/velnix ui --dry-run
bin/velnix ui --approve PLAN_ID
```

Repeat the same options with the exact reviewed digest. The default address is
`http://127.0.0.1:8090`; choose a different unprivileged port with `--port` in
both preview and apply. Root, non-loopback addresses, incorrect approvals and
occupied ports are refused. No browser opens automatically and no existing
listener is replaced.

The running terminal prints the address and the path to a private one-time
bootstrap file. Read that file deliberately, open the printed address and paste
its 64-character secret into the login form. It expires after five minutes and
is removed after successful login. Do not put it in a URL or shell argument.
Normal output/JSON contains only its file location, never the secret.

A browser session expires after 15 minutes idle or two hours absolute lifetime.
Logout revokes it. Restart the foreground UI and reload the page for another
bootstrap after logout/expiry. Reloading an authenticated page restores the
existing session without storing its credentials in browser storage.

Ctrl-C/SIGTERM revoke sessions, close the listener and remove the owned runtime
file/directory. Closing a tab leaves the explicitly requested foreground process
running until stopped. No daemon, systemd unit, service registration, firewall,
DNS or trust changes are installed. A crash can leave private abandoned runtime
files; a new launch generates fresh credentials and does not adopt or delete
unknown old directories. Inspect abandoned files manually after confirming the
old process is gone.

## Pages and actions

- **Overview:** actual project counts, active owned workload evidence, runtime
  selections, recorded certificate names and platform/configuration findings.
- **Projects:** filter/list/detail and reviewed links/unlinks of explicit
  directories. Unlink removes registry references, never source files. DNS and
  active routes still need their existing CLI setup/reload.
- **Runtimes:** passive native discovery; reviewed exact native selection/clear
  for PHP, Node, Bun and Python. Names/paths are candidates, not verified versions.
  Apply may run the selected executable's existing bounded version probe, fully
  disclosed in review. Discovery and preview never run probes or project code.
- **Owned services:** HTTP/FPM and mail availability. Lifecycle remains with
  foreground CLI/TUI sessions; external/system database services are not invented.
- **Certificates:** recorded phase/names/expiry/public CA fingerprint. No private
  keys, CA export, issuance or trust-store endpoint.
- **Mail:** existing capture inbox, passive plain-text/attachment inspection,
  explicit authenticated downloads and reviewed exact retention pruning.
- **Diagnostics:** bounded platform and configuration findings with no automatic
  repair or configuration editor.

State initialization is also available as a reviewed action before linking a
first project. The dashboard reads do not initialize persistent state.

Every mutation starts with a backend preview. The dialog shows its effects,
target, notes and exact backend identity. Apply uses a session-bound, two-minute,
single-use ticket; only eight pending previews may exist per session. Changed
state requires another review. Duplicate clicks and concurrent session mutation
requests are refused. There is no automatic retry after a timeout or lost
connection. An accepted operation continues independently of a browser disconnect
within a bounded application context; inspect actual state/backend recovery
evidence before proposing another action when its result is uncertain.

Page reads are cancellable and discard stale responses after navigation. Refresh
is explicit. There is no background polling, WebSocket, service worker or remote
font/asset/analytics dependency. Light/dark themes, responsive navigation,
keyboard focus and native modal review are included. Desktop windows at or below
900 CSS pixels high use tighter spacing and recent-project rows; the redundant
overview mail shortcut is hidden there while sidebar mail navigation remains.
Login headings/spacing also adapt to short windows. Tall desktop and mobile
layouts retain their existing density. No user credentials persist
in localStorage/sessionStorage.

## Mail behavior

Start capture separately through the existing reviewed `mail serve` CLI flow.
The UI never adopts an external inbox or launches/restarts Mailpit. A stopped
owned capture returns an explicit unavailable state. The provider API and its
credentials are never exposed or proxied to the browser.

Viewing uses the existing bounded parser and raw-message endpoint, so it does not
mark mail read. HTML preview is explicitly unavailable in this release: sender
HTML never enters the dashboard DOM, iframe, stylesheet or remote asset loader.
The approved proposal permits this safe fallback. Plain text, decoded headers,
limits and truncation follow [MAIL](MAIL.md).

An attachment requires an explicit click on its exact message/part. The response
is authenticated, no-store, nosniff and forced to download as octet-stream with a
safe basename. There is no user-supplied server filesystem path and no inline
HTML/SVG or automatic execution. The existing CLI export retains its separate
reviewed filesystem-write behavior. Pruning still uses exact candidate IDs,
mailbox-snapshot revalidation and irreversible-deletion disclosure.

The Phase 6 no-egress boundary remains policy for Velnix's typed flows, not a
kernel sandbox or a guarantee about unused authenticated Mailpit features.

## Local API protection and limits

Bind only to the reviewed numeric IPv4 loopback authority, including its port.
Reject unknown Host, forwarded-authority headers, hostile Origin and cross-site
Fetch Metadata before routing. Authenticate API data/downloads; require exact
Origin, JSON and a session CSRF token on writes, including plan creation/apply
and logout. Anonymous login challenges carry a separate short-lived CSRF nonce
but grant no data access or mutation authority. Login failures are limited to
five per minute; anonymous pending challenges are capped at 32.

Cookies are opaque, host-only, HttpOnly and SameSite=Strict. Loopback HTTP is not
encrypted and cookies cannot claim the Secure attribute here. Cross-user/same-UID
hostile code isolation, remote access and dashboard TLS are not provided. There
is no CORS, generic command/exec endpoint, filesystem browser, provider proxy,
private-key endpoint or debug server. CSP permits local compiled assets only,
without unsafe-eval or active framing.

Configured limits include 32 KiB write bodies, 16 KiB HTTP headers, eight active
API handlers, 64 accepted HTTP connections, ten-second reads and a 35-second
write timeout. Backend reads and apply contexts have independent bounds. JSON
responses are capped at 2 MiB; attachment reads use the existing bounded MIME
limit. Project views show at most the first 200 registry projects with an explicit
truncation notice; full registry inspection remains available in the CLI. Mail
pagination and prune bounds follow Phase 6. These settings are not a guarantee
against same-UID resource exhaustion.

Runtime files are 0600 in an exclusively created 0700 directory under validated
`XDG_RUNTIME_DIR`, or a private validated UID directory beneath the temporary
root. The launch plan records the parent directory identity and the session-file
template; each approved launch creates its own unpredictable session name. The
project registry schema and resource ownership grants are unchanged.

## Frontend development and release assets

Source lives in `web/`, HTTP presentation in `internal/transport/web`, shared
application data/actions in `internal/app`, and generated embedded files in
`internal/dashboardassets/dist`. Generated assets and binaries are ignored by Git.
A clean clone must build the frontend before Go compilation, tests or analysis,
because Go embeds the generated directory. Use `sh scripts/build.sh` to install
frozen dependencies, build the frontend and compile the binary. Direct Go builds
require a preceding frontend build; missing assets intentionally fail compilation.
Published GitHub releases trigger `.github/workflows/release.yml` to build a
Linux amd64 binary with embedded assets and attach an archive and checksum.
Release users need neither Bun nor frontend dependencies.

```sh
bun install --cwd web --frozen-lockfile
bun run --cwd web typecheck
bun run --cwd web build
python3 scripts/check-dashboard-assets.py
```

Bun 1.4.4 and exact Vue/Vite/TypeScript/test versions are pinned with committed
`web/bun.lock`. Scripts explicitly execute Vite/vue-tsc/Playwright through Bun.
Vite's upstream Node engine range is `^20.19.0 || >=22.12.0` if using its Node
execution path; these project scripts were verified under Bun. Bun/Node remain
build-time tools only. GitHub JavaScript actions have their own embedded runtime.

Vue and Lucide runtime license notices are included at `/assets/licenses.txt`.
The reproducibility check builds twice, compares every generated file and enforces
the 250 KiB compressed JS+CSS budget. CI performs frozen installation, asset
comparison/typecheck/build, Go checks and isolated desktop/mobile browser checks.
It does not publish the UI. Existing Mailpit is optional for additional local
browser mail acceptance, never installed implicitly by the test suite:

```sh
VELNIX_UI_BINARY=/absolute/velnix VELNIX_TEST_MAILPIT=/usr/local/bin/mailpit \
  bun run --cwd web test:browser
python3 scripts/validate-dashboard.py /absolute/velnix
```

Install the pinned Playwright browser explicitly when needed with
`bun --bun node_modules/@playwright/test/cli.js install chromium` from `web/`.
This is test tooling, not a runtime prerequisite. Browser tests use isolated
state, temporary projects and inboxes, and stop only their own CLI children.
The mail test is explicitly skipped when `VELNIX_TEST_MAILPIT` is absent.

See [completion evidence](validation/PHASE-7-COMPLETION.md) and
[open platform/release checks](validation/RELEASE-ACCEPTANCE.md) for actual local
results and limits. Phase 8 is not automatically authorized.
