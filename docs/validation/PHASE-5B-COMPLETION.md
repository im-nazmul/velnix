# Phase 5B completion

**Status: COMPLETE — approved interactive-management scope, 2026-11-05.**

The user explicitly authorized direct implementation without another proposal
approval, then requested finishing the work. The extension is complete within
[PHASE-5B](../PHASE-5B.md) and [ADR 0015](../decisions/0015-reviewed-terminal-management.md).
Original read-only Phase 5 completion remains recorded separately; no Phase 6
implementation or host-system modification is included.

## Completion checklist

| Item                         | Result                                                                                                                    |
| ---------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| Discoverable management      | Existing Ctrl+P picker, ranked action search and guided keyboard forms                                                    |
| Private-state initialization | Explicit preview/approval; no automatic initialization                                                                    |
| Project management           | Link/unlink, selected-project prefill, source preservation and route-reload warning                                       |
| Runtime selections           | Direct existing PHP/Node/Bun/Python selection/clear; external probe disclosed before approval                             |
| Owned web controls           | Nginx start/stop/reload; optional previously issued HTTPS material                                                        |
| Owned PHP pools              | Per-project FPM start/stop; no unsupported reload or hidden restart                                                       |
| Exact approval               | Complete bounded preview, end-of-review gate, typed APPLY plus Enter; existing backend revalidates the exact ID           |
| Failure handling             | Stale state refuses without retry; preview input is retained for correction; long apply errors are wrapped and scrollable |
| Session process lifetime     | Track only explicitly started foreground owners; cancel/wait on exit; preserve independently owned workloads              |
| Immediate stop/restart       | Stop waits for its session owner cleanup before allowing a new generation                                                 |
| Terminal behavior            | Bounded inputs, literal spaces, minimum management viewport, cancellation, terminal restoration                           |
| Shared logic and automation  | Existing application services, no shell/CLI subprocess, CI browsing and registry-management PTY checks                    |

## Final local evidence

Passed:

```sh
go test -race -count=1 ./...
go test -race ./internal/app -run TestTerminalSession -count=5
go vet ./...
go mod verify
go build -trimpath -o /tmp/velnix-phase5b ./cmd/velnix
python3 scripts/validate-tui.py /tmp/velnix-phase5b
python3 scripts/validate-tui-management.py /tmp/velnix-phase5b
git diff --check
```

Full race and repeated lifecycle tests use authorized disposable local sockets.
The repeated tests exercise real Nginx/FPM and immediate stop/restart without
an artificial wait. Application tests additionally cover exact runtime selection,
clear, stale approval refusal, invalid operations, closed-session refusal and
preservation of another owner's workload.

The management PTY test drives actual initialization, canceled preview, linking
with a space-containing path and unlinking while preserving source files. It
also changes the registry through a separate approved CLI operation while the
TUI review remains open, then confirms the old TUI approval is refused without
linking the requested project. A fresh review succeeds afterward. Terminal
attributes are restored. The original browsing PTY script still passes normal
quit, Ctrl-C, SIGTERM and invalid-configuration scenarios with state preserved.

Model tests cover review gating, input limits, too-small terminal refusal,
retained preview inputs, rejected late results and scrollable errors without
replaying approval. Further details: [management validation](PHASE-5B-MANAGEMENT.md).
CI invokes both PTY scripts; hosted CI results are not claimed from local runs.

## Retained boundaries

No package/provider installs, manager activation, database/system-service
lifecycle, runtime execution of project scripts, automatic recovery, TLS
issuance/trust/DNS changes, mail or dashboard. TUI-started web/FPM workloads stop
with their session. FPM reload remains unsupported, so changes require separately
reviewed stop/start operations. Unlink needs a separate reviewed web reload/stop
to revoke existing routes.

Native Ubuntu/non-systemd and additional terminal/Windows environments remain
unperformed release acceptance. Existing filesystem-call cancellation and
external-runtime pinning limitations remain. See
[RELEASE-ACCEPTANCE](RELEASE-ACCEPTANCE.md). Phase 6 needs its own bounded proposal
and authorization; completion does not start the next milestone.
