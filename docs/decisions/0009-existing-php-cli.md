# ADR 0009 — Exact existing PHP CLI selections

- Status: Accepted for the first Phase 3 slice
- Date: 2026-10-20
- Authorization: “Approved, start implementation” for [Phase 3](../PHASE-3-PROPOSAL.md)

## Decision

Passive discovery checks fixed `/usr/bin` and `/usr/local/bin` PHP executable
names, resolves aliases and reports unknown provenance, external authority and
unprobed status. It never consults PATH, profiles or repository configuration.
Other absolute paths can be selected explicitly. Require a regular native ELF
binary, at most 128 MiB, with executable permission, no set-ID or group/world
write bits. Manager scripts/shims are unsupported.

Explicit inspection is a read-only probe plan until its digest is approved.
Project selection is a reviewed registry plan binding the exact version,
canonical executable path, SHA-256 and device/inode. After approval, run a
bounded isolated `-n` probe to verify PHP_VERSION and CLI SAPI before committing.
Probe failure leaves registry and journal unchanged. Resolve rechecks bytes and
identity without executing; an absent or changed selection fails closed.

Use registry schema 3 for an optional per-project PHP CLI selection. Keep schemas
1/2 readable; effective selection changes migrate atomically and retain existing
projects/resources. Never downgrade schema 3 on later link/unlink. Extend the
existing journal with `project-select-php` and `project-clear-php`; use existing
locking, stale approval rejection and exact before/after recovery without replay.
Relinking the same directory preserves its selection. Clear/unlink removes
metadata, never the runtime or project files. Old binaries cannot read schema 3.

`exec --name NAME -- PHP_ARGUMENTS...` invokes that project's selected PHP,
without a shell. It is intentionally PHP-specific in this slice. Connect stdin
and raw stdout/stderr; preserve child exit status without adding diagnostics.
Before launch recheck project directory identities and executable identity.
Execute the checked descriptor through Linux procfs, with argv[0] set to the
canonical path, so path replacement cannot redirect execution after checking.

Run in the registered root, inherit application environment and PHP ini, set
PWD and VELNIX_PHP in the child only. Do not fabricate PATH aliases: scripts
needing this exact interpreter must use VELNIX_PHP or PHP_BINARY, rather than
assuming a nested bare `php` command matches. Runtime resolve documents the
environment delta without exposing inherited secrets. Exec has no JSON envelope.

Forward terminal SIGINT/SIGTERM on context cancellation, allow two seconds for
the direct child to exit, then kill/reap it through Go's process handle. User
commands are foreground children in the terminal group. This is not background
application supervision; descendants deliberately detached by user code are
outside its lifecycle guarantee.

## Limits

Checksum/identity checks do not sandbox same-UID code or pin libraries, PHP ini,
extensions and dependencies. Private state does not defend against a compromised
user account. Probe approval trusts the named executable to run; a native ELF
header is not proof of safety. Exact versions must include the patch component;
partial constraints, repository version-file precedence, providers, global
defaults and generic multi-runtime exec remain unimplemented.

FPM/HTTPS are separately implemented slices under the existing Phase 3 approval.
