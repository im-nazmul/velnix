# Phase 6 — Mailpit compatibility checkpoint

Date: 2026-11-05. Status: checkpoint resolved by the explicitly approved
flow-scoped no-egress amendment. The user approved implementation with “start
implementaion”, then approved the amendment with “You have my approval,
continue”. The bounded adapter is now implemented; see [completion evidence](PHASE-6-COMPLETION.md).
The original strict no-egress experiment remains below as historical evidence,
not a failing gate under the amended scope.

## Current installed provider

The user supplied an updated installed Mailpit v1.33.0. Live reinspection confirms:

- Path: `/usr/local/bin/mailpit` (existing external installation, unchanged by
  Velnix).
- SHA-256: `51d96e115a6c9ed16a29a73ff5d2485767a9db21796c5d41386bce9ef17e073e`.
- Go build metadata module: `github.com/axllent/mailpit v1.33.0+dirty`.
- Build: Go 1.27.3, linux/amd64, CGO disabled.
- Recorded revision: `364916b15e80a8bac1e7b0ab66a4f2b3d7d745c2`,
  `vcs.modified=true`. Results apply to these exact binary bytes; they do not
  certify every build reporting v1.33.0.
- Help advertises both `--allowed-hosts` and `--max-message-size`.

Identity was read using `sha256sum` and `go version -m`; no update-checking
version subcommand was needed. The earlier v1.27.0 flag incompatibility below
is superseded for this executable.

## Reproducible disposable test

[mailpit_security.py](../../scripts/validation/mailpit_security.py) uses only
Python's standard library and an existing Mailpit. The egress experiment also
requires existing `/usr/bin/strace`; it does not install either tool.

```sh
python3 scripts/validation/mailpit_security.py --trace-egress
python3 scripts/validation/mailpit_security.py --normal-umask
```

The first command returns **2**, an intentional strict no-egress gate failure,
with other observations reported separately. Exit 0 without `--trace-egress`
means only the selected subset passed, never the whole Phase 6 gate. No API
credentials or message bodies are printed. A private disposable inbox and random
credentials are used; no existing inbox is queried or modified. Port selection
has an ordinary bind race; a conflict aborts rather than taking over a listener.

Actual local results for the current binary:

| Check | Observed result |
|---|---|
| Multipart capture | Plain text and binary attachment round-trip pass |
| Hostile attachment filename | Provider preserves `../../escape.bin` as metadata; consumer must choose an explicit destination and never trust it as a path |
| Message limit | 11 MiB message rejected with SMTP 552 at configured 10 MiB limit |
| Deep MIME | Service remained responsive after 1,000 nested multipart levels; about 0.3 seconds locally, not a general performance guarantee |
| HTTP authentication | Unauthenticated message read returns 401; authenticated read returns 200 |
| Host validation | Authenticated hostile Host returns 403 |
| Origin validation | Hostile Origin reads and DELETE requests return 403, with and without credentials |
| Private files under umask 077 | Database, WAL and SHM all mode 0600 |
| Files under ordinary umask 022 | Database, WAL and SHM all mode 0644; launch must enforce a private child umask before creation |
| Automatic connection attempts in tested startup/capture/read flow | None observed with connect tracing enabled and a minimal child environment |
| Explicit authenticated link check | Attempts outbound HTTP despite minimal environment and disabled update checks |
| Cleanup | Owned test child exits and both ports can be rebound after cleanup |

For the last network check, every provider `connect` syscall was injected with
`ENETUNREACH` by strace **before** the provider started. The authenticated
`GET /api/v1/message/<id>/link-check` returned a result describing the blocked
request; tracing recorded one connect attempt to the public IP in the synthetic
message. No outbound connection from the provider was allowed. Local incoming
SMTP/API sockets remained usable because they use bind/listen/accept.

This is an authenticated provider feature, not evidence that an unauthenticated
website can bypass the Host/Origin checks. SMTP capture alone did not fetch the
message URL in this experiment. The mismatch is with the proposal's literal
requirement to disable all external network activity through supported provider
configuration, including unused authenticated features.

The [upstream v1.33.0 link checker](https://github.com/axllent/mailpit/blob/v1.33.0/internal/linkcheck/main.go)
collects message links for HTTP tests. Installed help offers restrictions on
internal targets, but no option disabling link-check endpoints entirely.
The [upstream API contract](https://mailpit.axllent.org/docs/api-v1/) documents
that API authentication follows UI authentication. Velnix must never expose or
launch the bundled viewer in this slice.

A second API detail matters for implementation: `GET /api/v1/message/<id>` marks
a message read. Future passive inspection should use raw/headers plus bounded
parsing, or separately propose explicit read-status semantics; do not describe
that endpoint as a passive read. No choice is implemented here.

## Narrow amendment — approved on 2026-11-05

The user approved replacing provider gate 2 with:

> No automatic outbound delivery, relay, forwarding, webhook, update check or
> remote content fetch during Velnix's approved capture and inspection flows.
> Velnix keeps API credentials private and calls only an explicit allowlist of
> message list/raw/headers/attachment and reviewed exact-message deletion
> endpoints. It never calls link checking, HTML checking, screenshots, release,
> send, or other network-capable provider endpoints. Unused authenticated
> Mailpit endpoints are outside the no-egress guarantee; this is application
> policy, not kernel network isolation.

Retain all existing loopback, authentication, Origin/Host, minimal environment,
retention, size, ownership and runtime approval gates. Require a private child
umask before launch and verify sidecar permissions; do not change the Go parent's
process-wide umask concurrently with other work. A narrowly scoped child launch
mechanism is needed, not a new SMTP server or HTTP proxy. No download or host
Mailpit replacement is needed.

If a kernel-enforced absolute no-egress guarantee is required instead, keep
integration stopped and propose a separate isolation design. Do not silently
add a network namespace, seccomp policy, reverse proxy, provider fork or native
SMTP implementation under this scope.

The initial proposal required stopping at a failed provider gate. Integration
resumed only after explicit approval of this amendment; it is now recorded in
[ADR 0016](../decisions/0016-local-mail-capture.md).

## Implementation acceptance after amendment

Upstream [v1.33.0 LICENSE](https://github.com/axllent/mailpit/blob/v1.33.0/LICENSE)
is MIT; Velnix has not bundled or installed it. No compatibility claim is made
for arbitrary releases or for unreviewed changes to an executable with the same
version string. Runtime plans pin exact selected executable identity.

The completed local tests now cover restart persistence, both count and age
retention, a concurrent-session rejection, owner-crash cleanup, reviewed
recovery, failed-write evidence, safe attachment export, stale export/prune
plans, bounded/cancellable API reads and the restricted client environment and
endpoint surface. See [Phase 6 completion](PHASE-6-COMPLETION.md) for commands and
limits. The original spike alone did not establish these results.

Native Ubuntu/Windows mail networking, additional provider builds/releases,
full filesystem fault injection, performance/resource-exhaustion matrices and
provider-wide kernel egress isolation are not established by these tests.
Existing platform/release gates remain open.

## Historical initial checkpoint

The first installed executable was v1.27.0, Go 1.24.4, SHA-256
`f66ddf2f9c4fe4d96d8cfbd313b53b1727473d67613f6bd18468af2b93b76b22`,
revision `1f1eed8a8b1c8037d4409be51cd019323d9f4ee3`. It rejected
`--allowed-hosts` and `--max-message-size`. `mailpit version` attempted a GitHub
update lookup even with `MP_DISABLE_VERSION_CHECK=true`; the sandbox blocked
that request. Static Go build metadata provided identity instead. The previous
suggestion to download a newer provider is withdrawn because the user already
supplied a newer local installation.
