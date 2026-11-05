# Phase 6 — local mail CLI completion

Status: **COMPLETE within the approved foreground CLI scope**, 2026-11-05.
This is implementation completion, not release-wide or cross-platform acceptance.

## Scope and authorization

The user approved [the proposal](../PHASE-6-PROPOSAL.md) with “start implementaion”.
After the installed provider was updated, the user approved the documented
no-egress amendment with “You have my approval, continue”. [ADR 0016](../decisions/0016-local-mail-capture.md)
records the accepted provider boundary. [MAIL](../MAIL.md) documents commands,
operational approvals, storage, retention and recovery.

Delivered:

- `mail status`, reviewed foreground `serve`, paged `list`, passive plain-text
  `show`, reviewed attachment `export`, exact-candidate `prune` and exited-session
  `recover`, including CLI JSON and ready events.
- Existing external Mailpit selection with static Go module metadata and exact
  executable hashing/device/inode evidence. No provider installation or adoption.
- Private provider-owned persistent SQLite, ownership marker, symlink/hardlink
  rejection, descriptor-pinned paths, independent capture lifetime lock and
  short common mutation transitions. Project registry schema is unchanged.
- Loopback-only SMTP/API, private per-session credentials, a minimal explicit
  child environment, child-only umask 077, process identity checks and parent-death
  cleanup. No permanent supervisor or system service changes.
- Bounded raw/MIME/API reads; depth/part/decoded/text limits; terminal sanitization;
  explicit HTML-only unavailable state; no remote content rendering or read-status
  mutation. Attachment exports require a reviewed new path and cannot overwrite.
- Finite count/age retention, exact IDs and snapshot revalidation for manual
  pruning, operation evidence before effects and reviewed non-replaying recovery.

## Exact local provider

Path `/usr/local/bin/mailpit`, module `github.com/axllent/mailpit v1.33.0+dirty`,
Go 1.27.3 linux/amd64, revision `364916b15e80a8bac1e7b0ab66a4f2b3d7d745c2`,
`vcs.modified=true`. SHA-256:
`51d96e115a6c9ed16a29a73ff5d2485767a9db21796c5d41386bce9ef17e073e`.
These bytes were unchanged by this task. Upstream is MIT-licensed and remains
an external prerequisite; no Mailpit source or binary was bundled. The
[initial checkpoint](PHASE-6-MAIL.md) preserves both version observations and
the authenticated link-check outbound capability experiment.

## Executed validation

All test state and inboxes were disposable. Socket/provider tests ran outside
the restricted sandbox because its socket creation is denied. The initial
sandbox attempts were environmental failures, not passing acceptance results.
A writable `/tmp/velnix-go-build` cache was used for sandbox Go commands.

```sh
VELNIX_TEST_MAILPIT=/usr/local/bin/mailpit go test -race -count=1 ./...
go vet ./...
go build -trimpath -o /tmp/velnix-phase6 ./cmd/velnix
python3 scripts/validate-mail.py /tmp/velnix-phase6
git diff --check
```

Results:

- Full race-enabled suite **PASS**, including existing application, CLI, TUI,
  provider, registry and process tests. The application package completed in
  about 139 seconds locally; no performance budget is inferred from that time.
- Real Mailpit integration **PASS**: capture, passive plain-text/attachment
  inspection, safe export, new-file permissions, concurrent capture rejection,
  stale export/prune rejection, exact provider-ID deletion, stop/restart
  persistence, count retention and age retention.
- Count and age retention were observed to converge at about a minute. The age
  fixture seeded an old reception time using a disposable provider fixture's
  `--use-message-dates`; production Velnix never enables that option. It then
  restarted normal Velnix capture and observed age cleanup independently of
  the count limit.
- A disposable export into a non-writable directory **PASS**: failure preserved
  recovery-required operation evidence, subsequent mutation was blocked, capture
  stopped, reviewed recovery acknowledged the failure and archived its evidence.
- Parser/client/filesystem tests **PASS**: malformed/deep MIME, text truncation,
  HTML exclusion, controls and encoded headers; path aliases/hardlinks, export
  collision and no overwrite; restricted endpoint/ID handling, private auth,
  disabled inherited proxy/provider settings, response limits, cancellation
  and redirect rejection.
- Built CLI acceptance **PASS**: absent-state status creates nothing; SMTP
  capture/list/show preserves unread status; Ctrl-C exits 130 and removes
  credentials; restart retains messages; killing the CLI owner stops its child;
  reviewed recovery clears the stale lifecycle; both ports can be rebound.
- Go vet, build, formatting and diff whitespace checks **PASS**.

The standalone `scripts/validation/mailpit_security.py --trace-egress` experiment
from the checkpoint returns **2** intentionally for the original absolute
no-egress gate: it demonstrates an unused authenticated link-check endpoint can
attempt a connection while strace blocks every outbound connect. Under the
approved amendment this is recorded provider capability, not a failing Velnix
flow. Capture/read-only operations in that experiment did not initiate connects;
Velnix's typed client does not expose network-capable endpoints. strace is not a
runtime dependency of Velnix.

## Limits and release gates

The no-egress boundary is flow policy with private API access, not kernel
isolation. Same-UID hostile code, arbitrary modifications to a provider build,
other Mailpit versions/build layouts, database internals and external installed
libraries are not isolated or certified. Unsupported capabilities fail rather
than silently installing or exposing services.

Retention targets are periodic, not hard storage quotas. Captured data, SQLite
allocation/WAL and exports can exceed an apparent count-based size estimate.
Stopped inspection, browser HTML viewing, mail TUI integration, backup/restore,
database repair, systemd/detached operation and the Vue dashboard are excluded.
No provider/system package, DNS, trust store, existing external inbox or project
environment file was modified.

Native Ubuntu/non-systemd and Windows/WSL network variants, complete resource
exhaustion/filesystem fault-injection matrices and release packaging/security
checks remain open in [RELEASE-ACCEPTANCE](RELEASE-ACCEPTANCE.md). No Phase 7
implementation is authorized by this completion record; it needs a separate
bounded proposal.
