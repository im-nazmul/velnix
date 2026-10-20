# Rootless static HTTP

Phase 2 adds an explicitly requested foreground Nginx workload. An existing
Nginx executable and Linux pidfd support are required; no packages are installed.
Run as your ordinary user. No systemd, root privileges or permanent Velnix daemon
are needed. Default executable: `/usr/sbin/nginx`; default listener:
`127.0.0.1:8080`. Alternative ports must be 1024–65535. IPv6 is unavailable in
the static slice. Phase 3 adds [per-project FPM](FPM.md) and [optional HTTPS](HTTPS.md);
reverse proxying remains later work.

## Start and inspect

Initialize state and link projects using [PROJECTS](PROJECTS.md), preferably with
a dedicated public document root. Then review the complete HTTP plan:

```sh
velnix http start --dry-run --json
velnix http start --approve START_PLAN_ID --json
```

The approved start command stays in the foreground until stopped or canceled.
It produces its final JSON result on exit, rather than a streaming startup event.
Use another terminal to inspect startup and lifecycle state:

```sh
velnix http status --json
curl --resolve demo.test:8080:127.0.0.1 http://demo.test:8080/
```

For another executable or port, supply the same options during preview and apply:
`http start --nginx /absolute/nginx --port 8081 ...`. Executing that selected
binary, including `nginx -t`, is part of approval; a custom executable must be
trusted by the user. Preview does not execute it or create a listener. Plans hash
its contents, registry, generated configuration, prior HTTP evidence and project
directory identities. Changed preconditions invalidate approval.

To use a browser, manually add `127.0.0.1 demo.test` to the relevant hosts file.
Linux and Windows have separate hosts files. Windows access to WSL loopback
forwarding depends on that machine's WSL networking configuration and is not
validated by the Linux tests. Velnix never edits either hosts file. URLs retain
the explicit port. Curl's `--resolve` needs no hosts-file changes.

## Reload, stop and unlink

```sh
velnix http reload --dry-run --json
velnix http reload --approve RELOAD_PLAN_ID --json
velnix http stop --dry-run --json
velnix http stop --approve STOP_PLAN_ID --json
```

Link/unlink changes registry metadata; it does not silently reload a workload.
`pending_routing` reports a difference from the active registry snapshot. Reload
explicitly applies that snapshot. Unlink alone does **not** revoke an active
route: stop HTTP first when immediate revocation is needed. Source files and
project data are never removed. Reload waits for previous workers to retire;
old requests can be terminated after the two-second worker shutdown deadline.
Stop uses immediate Nginx termination and can interrupt requests.

Ctrl-C or SIGTERM in the foreground terminal stops the owned master and workers.
Cancellation returns exit code 130. An explicit stop from another terminal lets
the foreground start command exit successfully. Boot ID, PID, process start time
and executable path are checked through a pinned pidfd before signals. No process
is selected by name or PID alone. Workers are recorded too, allowing cleanup if
a master is killed. Linux parent-death signaling asks Nginx to terminate if its
foreground owner dies; after a hard crash, inspect recovery rather than assuming
cleanup succeeded. Same-user compromise is outside the isolation guarantee.

## Content and configuration

Only registered exact `NAME.test` hosts are served; unknown hosts return 404.
A separate internal `velnix-health.invalid` host returns a configuration generation
marker for readiness checks. This marker is not an authentication credential.
Static index files use `index.html`; there is no directory listing or PHP execution
unless an explicitly enabled, running [FPM profile](FPM.md) is approved in routing.
Dotfiles, common dependency/configuration directories, executable source suffixes,
keys, logs and common metadata files are denied. Symlink serving is disabled,
including paths that traverse symlinks. Use a dedicated public root: the deny
rules cannot recognize every application's secret filename. Applications running
under the same UID are not isolated from one another.

The complete generated configuration uses no external includes, project snippets
or dynamically loaded modules. Existing `/etc/nginx` configuration and system
services are untouched. The private XDG state directory is the independent Nginx
prefix; configuration/evidence use 0600 and the directory requires 0700. Nginx
creates its own PID and temporary paths there. Access logging is off; child
output is continuously drained with at most 8 KiB retained in memory. No disk
log grows indefinitely. Ordinary stdout stays reserved for CLI results.

## Recovery and limitations

```sh
velnix http status --json
velnix http recover --dry-run --json
velnix http recover --approve RECOVERY_PLAN_ID --json
```

HTTP evidence is separate from the registry journal (`http.json` versus
`journal.json`), but all transitions share the registry mutation lock. Status
reports `recovery_required` for interrupted transitions or an unexpectedly exited
running master. Recovery stops verified owned processes and marks known evidence
stopped; it never replays a launch. Preview a fresh start afterward. Use
`state recover` independently if a project/registry operation was interrupted.

Candidate validation failure preserves `http.conf` and the active workload.
Before publishing a new configuration, Velnix saves `http-before.conf` and
`http-before.json`. A failed reload/health check retains recovery evidence and
these prior bytes; it does not claim that the prior generation is active or
silently roll back. Recovery conservatively stops HTTP. Prior requests and
connections cannot be restored.

If current configuration differs from the recorded digest, preserve the files
and review the edit manually; automatic recovery refuses to overwrite it. A
crash after launch intent but before durable process identity is inherently
ambiguous: Velnix refuses automatic recovery and never guesses from the PID file.
Inspect `http.json`, the before files, `http.conf`, the prefix and OS process
identities; do not delete evidence or kill a process merely because its PID
matches a stale file. After establishing no owned workload remains, manually
reconcile the HTTP record/configuration to a known stopped state. This narrow
launch window and filesystem/power-loss faults are not an atomic transaction.

Private candidate/before files and Nginx temp directories may remain after stop.
They are owned evidence/cache, not listeners; no recursive cleanup command is
provided. Local validation and untested platform boundaries are recorded in
[PHASE-2-HTTP](validation/PHASE-2-HTTP.md).
