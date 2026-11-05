# Phase 5B reviewed management validation

Status: COMPLETE within the approved scope, 2026-11-05. See
[completion record](PHASE-5B-COMPLETION.md). Scope and usage are in
[PHASE-5B](../PHASE-5B.md); [ADR 0015](../decisions/0015-reviewed-terminal-management.md)
records the explicit authorization and process-lifetime decision.

## Delivered

The existing home/action picker now opens guided forms for private-state init,
link/unlink, existing direct runtime selection/clear, Nginx start/stop/reload
and project FPM start/stop. Nginx may use previously issued HTTPS material.
The existing backend explicitly refuses FPM reload, so no misleading reload
control or automatic restart was added.

Submitting a form produces a bounded review, not an apply. Reviews expose the
request, affected target, effects/limitations and exact backend plan ID. End
(or scrolling to the end) enables the typed APPLY plus Enter confirmation.
Cancellation discards approval and stale messages; preview errors retain input
for correction, while apply errors never automatically retry or reapprove.
Backend checks refuse stale state and changed executable identities as before.

Foreground process ownership remains daemonless: a session tracks only its
explicitly approved starts, returns readiness for the exact reviewed generation,
and cancels/waits for their owners on exit. Observing workloads from another
foreground owner does not adopt them. Explicit approved stop/reload has the
same owned-resource scope as the CLI. Lifecycle and cleanup errors remain
visible in session diagnostics and returned on exit.

## Evidence

- Full `go test -race -count=1 ./...` passed, including the existing suite and
  new real local Nginx/FPM session lifecycle tests. Disposable socket access was
  authorized for those tests.
- New application tests cover init/link/unlink, mutation-free preview, stale
  approval refusal, exact runtime selection/clear, real HTTP serving/reload/stop,
  session exit cleanup, closed-session refusal, and preserving another owner's
  workload. Runtime/PHP-FPM cases require installed binaries and skip explicitly
  if absent; they ran successfully in this local environment.
- Model tests cover end-of-review approval gating, explicit APPLY, cancellation
  and late messages, failure without retry, retained inputs on preview errors,
  bounded pasted text, small forms, action ranking and paths containing spaces.
- `go vet ./...`, `go mod verify`, build and diff checks passed.
- The existing PTY script still passes q/Ctrl-C/SIGTERM and invalid-configuration
  scenarios with terminal restoration and no browsing mutations.
- New `scripts/validate-tui-management.py` passed actual keystroke-driven init,
  canceled link preview, approved link using a path with spaces, approved unlink,
  preservation of external source and terminal restoration. CI invokes it after
  the browsing acceptance script. Hosted CI has not been observed in this run.

Reproduce on Linux with Go and Python 3:

```sh
go build -trimpath -o /tmp/velnix-phase5b ./cmd/velnix
python3 scripts/validate-tui.py /tmp/velnix-phase5b
python3 scripts/validate-tui-management.py /tmp/velnix-phase5b
```

Both scripts isolate state under temporary XDG paths. The second intentionally
creates/changes its disposable registry only after explicit simulated approval.
No host services, packages, DNS or trust are modified.

## Limits

No new package/provider install, manager activation, system database lifecycle,
TLS issuance/trust automation, project scripts, recovery automation, mail or
web dashboard. Runtime selection uses an explicitly approved external probe;
libraries/configuration are not fully pinned. Started workloads stop with their
TUI session, and interruption may require existing CLI recovery. Unlink requires
separate reviewed routing reload/stop. Large reviews are refused instead of
allowing approval of truncated data. Existing blocking-filesystem and platform/
terminal acceptance limits remain in [RELEASE-ACCEPTANCE](RELEASE-ACCEPTANCE.md).
The management PTY workflow exercises registry operations; real process control
is covered by application integration tests rather than every UI field combination.

Closeout adds per-workload owner tracking and waits for cleanup on stop. Repeated
real lifecycle tests now verify immediate restart without a timing delay. Long
apply errors are scrollable and cannot replay approval. The PTY management
script additionally verifies stale TUI approval refusal after a competing
approved CLI registry update, followed by a successful fresh review.
