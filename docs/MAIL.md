# Local development mail

Phase 6 adds optional foreground capture through an **existing Mailpit v1.33.0**
with matching Go build metadata. Mailpit remains externally installed and owned;
Velnix never adopts an existing instance or database, installs a provider, sends
external mail, or edits project settings. See [the approved scope](PHASE-6-PROPOSAL.md)
and [ADR 0016](decisions/0016-local-mail-capture.md).

## Capture and inspect

Initialize Velnix state using the existing reviewed `state init` flow first.
All mutations default to a plan; repeat the same options with its exact digest:

```sh
velnix mail status
velnix mail serve --dry-run
velnix mail serve --approve PLAN_ID
```

Capture stays in that terminal until Ctrl-C. In a second terminal:

```sh
velnix mail list --json
velnix mail list --offset 50 --json
velnix mail show MESSAGE_ID --json
velnix mail export MESSAGE_ID --attachment 1 --output /absolute/new/file --dry-run
velnix mail export MESSAGE_ID --attachment 1 --output /absolute/new/file --approve PLAN_ID
velnix mail prune --dry-run
velnix mail prune --approve PLAN_ID
```

`mail show` returns selected message headers, available plain text and attachment
metadata. Attachment IDs are Velnix's stable MIME traversal IDs from that output,
not provider UI IDs. Inspection does not mark messages read. HTML-only mail has
an explicit unavailable plain-text notice; HTML is never rendered. Raw MIME,
provider credentials and HTML content are excluded from these outputs. Mail
content is intentionally available through inspection, but never included in
provider diagnostics. Terminal controls are removed, text is bounded to 256 KiB
and truncation is explicit; unfamiliar character sets may show replacement
characters. Malformed or excessive MIME nesting returns an error.

Export requires an explicit absolute destination with an existing current-user
owned parent directory that is not group/world writable. Parent symlinks,
existing files, symlinks and hard links are refused; no overwrite or automatic
opening occurs. The new file is mode 0600. Filename metadata from a message is
never used to choose its destination. Plan approval is tied to the attachment
bytes and destination-parent identity. Exported bytes remain untrusted content.

Queries require the owned capture session to be running. They return unavailable
when stopped, rather than starting a background service or opening the provider's
database directly. Listing is paged in groups of 50 with a bounded offset;
full prune snapshots are limited to 1,000 messages. If a transient backlog is
larger, allow the approved automatic retention pass to converge before pruning.
No TUI or web dashboard mail integration is included in this phase.

## Local application settings and limits

Configure the application yourself to use host `127.0.0.1`, SMTP port `1025`,
no authentication and no TLS. This setting only works while capture is active.
No app runtime, environment file, framework configuration or system mail service
is changed. Local processes able to reach this endpoint may submit mail.

Capture options are included in the reviewed plan:

```sh
velnix mail serve --mailpit /absolute/mailpit --smtp-port 1025 --api-port 8026 \
  --max-messages 500 --max-days 7 --dry-run
```

Both listeners bind only to IPv4 loopback; distinct ports must be 1024–65535.
The API port is private control traffic for Velnix; no browser is launched or
viewer access advertised. API credentials are random per session, absent from
argv and normal output, and not inherited from the shell. No permissive CORS is
configured. Authenticated API access requires the exact Host and a permitted
Origin. Velnix calls only message listing, raw-message retrieval and reviewed
exact-ID deletion. It never calls send, relay/release, link/HTML checking or
screenshot endpoints. SMTP relay, forwarding, webhooks, reverse DNS and update
checks are not enabled; inherited provider and proxy variables are dropped.

The approved no-outbound guarantee covers Velnix's capture and inspection flows,
not a kernel sandbox. Mailpit still contains unused authenticated features that
can make network requests. Host/Origin checks and private credentials protect
the API; hostile same-UID code is outside the isolation promise. See the
[provider checkpoint and approved amendment](validation/PHASE-6-MAIL.md).

The accepted SMTP payload limit is fixed at 10 MiB. Retention defaults to 500
messages and 7 days; allowed policies are 1–1,000 messages and 1–30 days.
Pruning is periodic while capture is active (observed at about a minute locally),
so count/age targets are **not** hard byte quotas. SQLite allocation, WAL files,
transient excess and exported copies consume additional disk space. No hidden
cleanup runs while capture is stopped. Starting a session explicitly approves
retention, including deletion of already expired messages. Manual prune plans
list exact candidates and invalidate when the mailbox snapshot changes.

## Ownership, shutdown and recovery

Persistent mail lives under `$XDG_DATA_HOME/velnix/mail` (or the standard XDG
fallback), separate from the project registry. A Velnix ownership marker is
required; unknown stores are refused. Directory/file permissions are 0700/0600,
including SQLite WAL/SHM. Use a local Linux filesystem; NFS, SMB/CIFS, 9p/Windows
mounts and FUSE are unsupported. Data is managed through the provider, not a
Velnix-specific SQLite schema. Same-UID projects are not isolated from this data.

A mail lifetime lock prevents a second capture in the same Velnix state. The
common mutation lock is held for transitions, not the whole session. The exact
provider ELF identity and data directory are pinned by open descriptors. A fixed
`/bin/sh` launcher sets only the child's umask to 077 before exec; it never
interprets a project command or user-provided shell fragment. `/bin/sh` and the
provider's installed libraries remain external platform dependencies.

Ephemeral credentials use a private session directory under validated
`XDG_RUNTIME_DIR`, or a validated private UID directory under the temporary
root. Normal shutdown removes credentials and the empty session directory.
Ctrl-C/SIGTERM stop the exact owned child with a bounded graceful wait; a Linux
parent-death signal kills it if the Velnix owner crashes. No supervisor, systemd
unit, lingering, detached process or automatic restart is installed. WSL shutdown
stops capture. Stored mail survives process exit.

After a crash or failed mutation:

```sh
velnix mail status --json
velnix mail recover --dry-run
velnix mail recover --approve PLAN_ID
```

Recovery requires the owned child to have exited and the foreground session to
be stopped. It archives the last operation evidence, removes expired credentials
and acknowledges the stopped lifecycle; it never replays a launch/export/delete,
signals an unknown process, repairs a database or adopts unrecognized files.
Unresolved operations block further mutations. Corrupt/unsafe state and unknown
files require manual inspection. Operation records and one previous recovery
record are bounded to the existing private state-file limit.

An interrupted export may have published the destination before its final
acknowledgment; inspect it rather than rerunning over an existing file. A prune
can partially complete or race automatic retention; inspect the exact IDs and
preserved operation record. Mail deletion has no general undo. For a backup,
stop capture first and copy the owned store using a separately verified backup
procedure. No backup/restore or database-repair guarantee is implemented.
