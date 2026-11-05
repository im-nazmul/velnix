# Phase 6 — bounded local mail capture

Status: COMPLETE within the approved CLI scope. See [completion evidence](validation/PHASE-6-COMPLETION.md). Approved for implementation by “start implementaion” on 2026-11-05.
The user approved the no-egress amendment on 2026-11-05 with “You have my
approval, continue”. It scopes the guarantee to Velnix's approved flows rather
than unused authenticated Mailpit features. See [ADR 0016](decisions/0016-local-mail-capture.md)
and [provider evidence](validation/PHASE-6-MAIL.md).
Phases 3–5B completion and existing release acceptance remain unchanged.

## Deliverable

An optional local development inbox: explicit foreground SMTP capture, private
persistent messages, CLI message/plain-text inspection, safe attachment export
and finite retention. No outbound delivery or automatic project configuration.
The Vue dashboard and browser HTML viewer remain Phase 7 work.
Implemented commands and limits are documented in [MAIL](MAIL.md).
`mail recover` additionally provides reviewed acknowledgment of exited sessions
and interrupted operations, without replay or database repair.

Proposed commands (final flag spelling follows existing CLI conventions):

- `velnix mail status`: passive availability, owned-instance status, storage
  usage and retention settings; no executable probe or state initialization.
- `velnix mail serve`: review an exact plan, approve it and run the owned
  foreground capture process until cancellation. Default SMTP endpoint
  `127.0.0.1:1025`; refuse non-loopback addresses and occupied ports.
- `velnix mail list` and `velnix mail show <id>`: bounded metadata, envelope,
  plain text and attachment metadata, with existing JSON conventions. Escape
  terminal controls; do not render HTML or dump raw MIME by default.
- `velnix mail export <id> --attachment <id> --output <path>`: reviewed export
  of one selected attachment; explicit destination, no overwrite or symlink
  following, private file permissions and no automatic opening/execution.
- `velnix mail prune`: dry-run exact candidates, then explicit approval with
  stale-plan checks. No blanket inbox deletion command in this slice.

Reads while capture is stopped must give an explicit unavailable result if the
selected provider cannot support offline queries; never silently start it.
Application SMTP settings are documented examples only, not edits to `.env`.

## Provider evaluation before integration

ADR 0006 and FEASIBILITY require evaluating Mailpit first. Prefer an adapter to
an existing user-installed Mailpit executable over writing SMTP/MIME/storage.
Do not download, install, adopt or modify an existing external instance.

The first implementation checkpoint is a disposable compatibility/security
spike against an exact reviewed release and executable identity. Record version,
license, supported flags, API contract, actual test results and limitations.
Review executable probing through existing approval boundaries. Upstream docs
are moving references, not proof that a user's installed release is compatible.

Mailpit documents persistent SQLite, count/age retention, message size limits,
HTTP authentication and Host restrictions. Its documented default HTTP bind is
all interfaces; it must never be inherited. Its remote-CSS blocking flag does
not also block remote images or external links. Therefore exposing its browser
viewer is not part of this proposal.

Sources reviewed on 2026-11-05:

- [Runtime options](https://mailpit.axllent.org/docs/configuration/runtime-options/)
- [SMTP configuration](https://mailpit.axllent.org/docs/configuration/smtp/)
- [HTTP/API configuration](https://mailpit.axllent.org/docs/configuration/http/)

The spike must demonstrate all of the following before integration proceeds:

1. Explicit loopback SMTP and private authenticated loopback API bindings;
   exact Host checks, no permissive CORS, and rejection of unauthenticated,
   hostile-Origin and browser cross-origin mutation requests. Internal API
   credentials never appear in URLs, command arguments, logs or normal output.
2. No automatic outbound delivery, relay, forwarding, webhook, update check or
   remote-content fetch in Velnix's approved capture and inspection flows.
   Keep API credentials private and call only an explicit allowlist of message
   list/raw/headers/attachment and reviewed exact-message deletion endpoints.
   Never call link/HTML checking, screenshots, release, send or other
   network-capable endpoints. Unused authenticated provider features are outside
   this guarantee; this is application policy, not kernel network isolation.
   Build a minimal child environment; do not inherit provider/proxy settings.
   Test suppression rather than relying only on documented defaults.
3. Bounded message acceptance, parsing, concurrency, read deadlines and API
   output; malformed/deep MIME and hostile attachment names fail safely.
4. Private durable storage, count/age retention, restart persistence, safe
   locking and cancellation. No access to another Mailpit database or process.
5. CLI access through a bounded typed provider API without launching a browser,
   executing message content or introducing Velnix HTTP endpoints.

If Mailpit cannot meet these requirements through supported configuration,
stop at the evidence report and propose a revised scope. Do not silently add a
reverse proxy, fork Mailpit or implement a native SMTP/MIME server. Such a
change needs its own reviewed architectural decision.

## Application and state boundaries

Place lifecycle/query orchestration in `internal/app`, provider-specific code in
`internal/providers`, and CLI presentation in `internal/transport/cli`. Reuse
existing plan approval, cancellation and Linux process identity mechanisms;
never shell out to the Velnix CLI. Keep interfaces small and usable by later
TUI/web transports, without adding those integrations now.

Store the provider-owned database in a dedicated private XDG data subdirectory;
keep operation/process evidence under XDG state and secrets in private runtime
storage. Directories are 0700, files 0600, including database sidecars. Reject
symlinks, wrong ownership and unsafe parent paths using existing filesystem
patterns. Do not change the project registry schema or directly manipulate a
provider's database schema. Document local Linux filesystem requirements;
network shares and Windows-mounted database paths are unsupported initially.

Only one owned capture session per configured Velnix state may run. Hold a dedicated mail lifetime
lock within the configured Velnix state, not the global mutation lock for the entire SMTP session; use the common
mutation lock for reviewed state transitions. Verify executable identity and
boot/start-time process identity; terminate only the child owned by that
session. No detached supervisor, systemd unit, lingering or root privileges.
Ctrl-C and termination stop listeners; stored messages survive. WSL shutdown
stops capture and no automatic restart is promised.

Capture approval explicitly authorizes incoming mail writes and the configured
automatic retention for that session. Proposed defaults: 10 MiB maximum accepted
message, 500 messages and 7 days maximum age. Provider pruning may be periodic:
these are retention targets, not a hard disk quota. Document transient excess,
SQLite overhead/WAL and free-space failures; never claim a hard byte cap without
evidence. Retention runs only while capture is active. No hidden cleanup timer.
Deletion is irreversible without a separately verified backup. A startup plan
must show that existing expired messages may be deleted. Changing policy needs
a fresh plan; explicit prune approval identifies the exact current candidates.

SMTP is unauthenticated within the explicitly approved local-user boundary;
other local processes can submit messages. Reject non-loopback configuration
and do not promise same-UID isolation. User-provided sender/recipient data,
subjects, filenames and bodies are untrusted. Logs contain operational metadata
only; message bodies, credentials and attachment bytes stay out of diagnostics.
Plain-text fallback for HTML-only mail is explicit; no unsanitized HTML preview.

## Implementation checkpoints and acceptance

1. Complete the provider spike and record its evidence. Accept an ADR only once
   the proposed adapter is demonstrated compatible; otherwise return the
   blocking evidence and revised options for review.
2. Add reviewed foreground lifecycle and private persistent storage, then typed
   message reads, safe attachment export and reviewed pruning. Document commands,
   defaults, local SMTP setup, backup/deletion limits and unavailable states.
3. Add meaningful integration tests with disposable state: SMTP capture, restart,
   multipart text/HTML and binary attachments; malformed/deep/oversized input;
   terminal escapes, attachment traversal and export collisions; retention and
   stale prune plans; auth/Host/Origin attacks; outbound-network suppression;
   unsafe paths, concurrent sessions, port conflicts and disk/write failures.
4. Run Go tests, race checks, vet and build. Exercise real provider capture and
   shutdown, verify no listeners remain, and record the exact platform and
   provider version. Keep native Ubuntu/WSL/Windows and release-wide checks open
   until actually exercised. Documentation-only planning is not completion.

## Approval boundary

Approval authorizes only this bounded implementation, any reviewed pinned Go
client dependency, and disposable local tests. It does not authorize host
package installation, external network exposure, DNS/trust/service changes,
external mail delivery, automatic project edits, a web dashboard or later
roadmap phases. Each real capture/export/prune operation retains its runtime
plan and approval requirements.
