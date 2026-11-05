# ADR 0016 — Optional foreground mail through an existing Mailpit provider

- Status: Accepted for the bounded Phase 6 implementation
- Date: 2026-11-05
- Related: ADRs 0005, 0006; D005, D009, D012

## Context and authorization

The user approved [Phase 6](../PHASE-6-PROPOSAL.md) with “start implementaion”,
then approved its documented no-egress amendment with “You have my approval,
continue”. Installed Mailpit v1.33.0 passed the evaluated capture, limit and
Host/Origin/authentication checks. Its unused authenticated link checker can
initiate outbound HTTP, so absolute provider-wide network isolation is not a
supported-configuration guarantee. The [checkpoint](../validation/PHASE-6-MAIL.md)
records that evidence and the exact locally tested executable identity.

## Decision

Use an existing external Mailpit v1.33.0 ELF as a provider; verify static Go
module identity and pin exact executable bytes/device/inode in reviewed launch
plans. Do not install, bundle, adopt, fork or embed Mailpit and do not implement
a new SMTP server or provider database schema. Upstream is MIT-licensed; this
choice adds no Go dependencies to Velnix.

Keep orchestration in `internal/app.MailService`, private filesystem operations
in `internal/state`, provider/API/MIME code in `internal/providers/mailpit` and
CLI presentation in `internal/transport/cli`. No CLI subprocess, Velnix HTTP API,
background supervisor, TUI mail feature or Vue dashboard is introduced.

Use private provider-owned persistent SQLite in a Velnix-created data directory,
with independent mail lifetime locking and short common mutation-lock sections.
Use loopback SMTP and authenticated loopback API, minimal explicit environment,
finite retention and a 10 MiB incoming message limit. The fixed system shell
launcher sets child-only umask 077 and execs the pinned provider descriptor;
there is no user-supplied shell text. Pin process identity for signaling and use
Linux parent-death cleanup. System shell/runtime libraries remain external.

The amended guarantee is no automatic outbound delivery, forwarding, relay,
webhook, update check or remote-content fetch through Velnix's approved flows.
Keep credentials private and expose only typed list/raw/exact-delete methods.
Unused authenticated Mailpit endpoints are outside this guarantee; there is no
kernel egress isolation. Velnix does not launch or expose the bundled viewer.

Read raw messages rather than the provider summary endpoint that marks mail as
read. Parse with standard-library MIME tools, bounded input/depth/parts/decoded
bytes, text limits and terminal sanitization. Do not render HTML. Export only
reviewed attachment bytes to an explicit new path using descriptor-relative
checks and no-replace atomic publication. Prune only exact reviewed IDs after
revalidating the bounded mailbox snapshot. Deletion is irreversible without a
separately verified backup.

Journal effects before execution. Preserve incomplete operations, block new
mutations and require reviewed recovery after stopping capture. Recovery
acknowledges observed state, preserves the previous operation and never replays
an effect or repairs/adopts unknown storage. Registry schema remains unchanged.

## Alternatives and consequences

A native SMTP/MIME/storage service duplicates mature provider work. A reverse
proxy or network sandbox adds a new security/lifecycle boundary; both remain
outside approved scope. Direct provider database reads couple Velnix to an
internal schema, so stopped-session inspection is explicitly unavailable.

This adds a foreground listener only when requested and an external Mailpit
prerequisite only for mail commands. Retention is periodic, not a hard storage
quota. Same-UID isolation, external provider modifications, additional Mailpit
versions/build layouts, browser HTML viewing, backup/repair and cross-platform
acceptance require separate work. See [MAIL](../MAIL.md) for runtime approvals,
recovery and user-visible limits.
