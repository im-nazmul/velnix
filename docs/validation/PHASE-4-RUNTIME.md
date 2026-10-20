# Phase 4 runtime inspection: local validation

Follow-up: persistent selection/exec, detection and proxy evidence is now recorded
in [PHASE-4-EXECUTION](PHASE-4-EXECUTION.md). This report retains its original
inspection-only scope.

Date: 2026-10-20. These are local implementation results; WSL/native Ubuntu
platform acceptance and the outstanding Phase 3 FPM/HTTPS checks remain open.

The installed-runtime integration test successfully checked descriptor-based
version probes for:

| Runtime | Canonical executable | Observed version |
|---|---|---|
| Node | `/home/nazmul/.nvm/versions/node/v25.9.0/bin/node` | 25.9.0 |
| Bun | `/home/nazmul/.bun/bin/bun` | 1.4.3 |
| Python | `/usr/bin/python3.12` | 3.12.3 |

No installation or system configuration was changed. The CLI integration test
also previews and approves Python inspection with no Velnix state creation.

Validation passed: `go test ./...`, `go test -race ./...`, `go vet ./...`,
`go build ./...` and `git diff --check`.

Native C fixtures validate each language's argv, root cwd, EOF stdin and absence
of inherited loader/Node/Python variables. Tests reject version mismatch,
unexpected stderr, nonzero exit and excessive stdout. A sleeping native fixture
is terminated at the timeout; a canceled context cannot start a probe. Fixture
tests skip explicitly when no C compiler is installed. Installed-runtime tests
skip explicitly for missing or unsupported native binaries.

Application tests bind approval to runtime kind, required version and executable
identity, reject replaced binaries and script shims, reject approval plus
dry-run, and fail explicitly for unsupported non-PHP selection/clear/resolve.
CLI tests cover flag routing, invalid combinations, JSON preview and a real
approved inspection. Existing PHP tests continue to cover selection, execution,
executable replacement, descriptor pinning and registry migration/recovery.
