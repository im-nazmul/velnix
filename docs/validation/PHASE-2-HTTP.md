# Phase 2 HTTP validation

Date: 2026-10-20. The user approved the scoped HTTP completion proposal with
“You have my approval”. The approved Phase 2 implementation is complete; this
report records local evidence, not full v0.1 platform acceptance.

## Environment and commands

- Ubuntu 24.04.5 LTS, linux/amd64, WSL2 kernel
  `6.18.54.1-microsoft-standard-WSL2`.
- Go `go1.27.3 linux/amd64`; Nginx `1.24.0 (Ubuntu)` at `/usr/sbin/nginx`.
- Ordinary UID 1000; no package, system service, hosts or trust modifications.
- `go test -race -count=1 ./...` — passed, including real Nginx tests.
- `go vet ./...` — passed.
- `go build -trimpath -o /tmp/velnix-phase2 ./cmd/velnix` — passed.
- `python3 scripts/smoke-http.py /tmp/velnix-phase2` — passed with disposable
  XDG state/project and a temporary loopback port. All 17 external Nginx files
  hashed unchanged; the owned listener was closed after explicit stop.
- `git diff --check` — passed.

The Go cache/toolchain and loopback tests required execution outside the restricted
sandbox. Initial restricted runs reported inaccessible standard-library/cache
files; those were environment failures, followed by the actual checks above.

## Evidence

Real Nginx serves two exact project hosts, rejects unknown hosts, dotfiles, PHP
source and symlink escapes, and serves ordinary static assets. Generated configs
contain no external includes. Candidate validation rejects invalid directives
without replacing a prior configuration. An occupied external listener remains
untouched and no HTTP lifecycle record is created for that failed start.

Plans do not write configuration or launch processes. Approved startup stays in
the foreground; a duplicate startup approval cannot create another instance.
Registry unlink preserves source files and sets pending routing. Approved reload
removes that route, with the new generation verified over HTTP and old workers
retired before success. Wrong/stale approval fails. External configuration edits
are preserved and block automatic mutation. The independent registry suite still
checks concurrent writers, unchanged/committed transaction recovery and unknown
external edits.

Explicit stop and cancellation terminate owned processes. Identity tests reject
changed start time and absent identity before signaling. Unexpected SIGKILL of
the master exercises recorded-worker cleanup, reports failure/recovery, and
permits a reviewed stop-only recovery without relaunch. A synthetic dead-process
record also reconciles to stopped. The compiled CLI smoke checks JSON planning,
state initialization, link, foreground start, actual HTTP, status, explicit stop,
foreground exit, unlink/source preservation and final closed listener.

Two implementation issues were found and corrected by these checks: a new
worker's successful health response did not establish retirement of old routing,
and inherited worker output pipes could delay observing a killed master. Reload
now waits for recorded old workers; child waits bound inherited-pipe draining.

## Remaining acceptance boundaries

No Windows browser, Windows build/WSL version, NAT/mirrored networking, VPN,
multiple-distribution or native Ubuntu matrix was validated. No non-systemd-host
matrix was run; the implemented foreground adapter does not call systemd.
IPv6 is explicitly unavailable. PHP/FPM, HTTPS, runtime resolution and reverse
proxy support are outside this approved slice.

Tests exercise selected interrupted lifecycle states and an actual killed master;
they do not establish safety under every instruction-boundary SIGKILL, disk-full,
failed fsync/rename or power loss. An interrupted launch before durable process
identity and unknown configuration edits require manual review, as documented in
[HTTP recovery](../HTTP.md). Failed reload health checks preserve before evidence
and require conservative stop recovery rather than claiming successful rollback.
No performance percentile/budget, license clearance or full release acceptance
is claimed. Phase 3 remains unapproved.
