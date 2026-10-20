# Phase 3 PHP CLI slice — local validation

Date: 2026-10-20. Scope: first slice of the approved
[Phase 3 proposal](../PHASE-3-PROPOSAL.md), documented in [RUNTIMES](../RUNTIMES.md).
This does not establish completion of Phase 3.

## Performed checks

- `go test ./...`: passed.
- `go test -race -count=1 ./...`: passed.
- `go vet ./...`: passed.
- `go build -trimpath -o /tmp/velnix-phase3-runtime ./cmd/velnix`: passed.
- `git diff --check`: passed.

Provider tests exercise passive discovery and alias deduplication without shim
execution, rejection of scripts/relative paths/writable binaries, pinned original
descriptor execution after path replacement, exact-version/SAPI rejection,
bounded output and canceled probes. Actual `/usr/bin/php` resolves to
`/usr/bin/php8.5`, PHP 8.5.11. The real probe succeeds with hostile inherited
PHPRC, PHP_INI_SCAN_DIR and LD_PRELOAD values, proving these are excluded from
the probe environment and ini loading is disabled for that test.

Application/CLI integration tests exercise the approved selection/resolve/exec/
clear flow, no mutation on preview or failed probes, stale executable approval,
literal shell-like argv, stdin, raw output, child exit 23, cancellation/reaping,
selection-preserving relink and independent project metadata. State tests cover
schema 2→3, schema-3 retention on subsequent links, runtime fields refused in old
schemas, exact-version validation, before/after recovery without replay and
external-edit rejection. The full suite also reruns existing static HTTP,
concurrency and state-recovery tests.

A built-binary smoke run used temporary XDG directories and a disposable project:
discovery created no state; reviewed init/link/select succeeded; resolve reported
PHP 8.5.11; exec reported `8.5.11 cli` and returned 23 verbatim; separate SIGINT
and SIGTERM sent to Velnix canceled/reaped its direct PHP child and returned 130;
clear made resolution unavailable while preserving source. The local
`/usr/local/bin/php` resolves to an external Valet shell wrapper; selection
correctly refused it. The successful run explicitly selected the native
`/usr/bin/php8.5` instead. No wrapper, global PHP selection, package, service,
hosts file or trust store was changed.

## Open acceptance

FPM, PHP web routing, HTTPS/mkcert and two concurrent distinct PHP CLI/web
versions have not been implemented or established by this slice. Linux system
trust, Windows browser trust, native Ubuntu and the Windows NAT/mirrored matrix
remain unperformed. No runtime performance budget is claimed. Same-UID hostile
races, dependency/ini changes and deliberately detached child processes are
outside the limited executable-identity/direct-child lifecycle guarantees.
