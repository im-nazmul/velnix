# Phase 4 completion

**Status: COMPLETE — approved implementation scope, 2026-10-20.**

The user requested finishing the remaining Phase 4 work and marking it complete.
The remaining nonmutating provider-resolution item is implemented through
read-only mise/uv installation-store adapters. No Phase 4 implementation item is
left pending within the bounded scope below. This does not assert unperformed
platform/release acceptance; those checks remain open in
[RELEASE-ACCEPTANCE](RELEASE-ACCEPTANCE.md).

## Completion checklist

| Phase 4 item | Result |
|---|---|
| Node/Bun/Python bounded discovery and approved exact-version inspection | Complete; existing inspection tests retained |
| Independent persistent selections, passive resolve and explicit exec | Complete; native/venv execution and CLI regression tests pass |
| Python venv compatibility and literal requirement conflicts | Complete within documented supported layouts/constraint subset |
| Nonmutating providers | Complete: direct native resolution plus `existing-store-v1` mise Node/Bun and uv CPython adapters |
| Explicit loopback app-server proxying, HTTPS and upgrade forwarding | Complete; existing real Nginx integration tests remain passing |
| Passive bounded framework hints | Complete; metadata/hooks/symlink/FIFO/size tests remain passing |
| Plans, state migration/recovery, CLI and usage/decision documentation | Complete; schema 4 retained, provider evidence added to reviewed plans |

## Final validation

```sh
go test ./...
go test -race -count=1 ./...
go vet ./...
go build -trimpath -o /tmp/velnix-phase4-complete ./cmd/velnix
git diff --check
```

All checks passed. The full race-enabled suite includes the earlier PHP CLI/FPM,
static HTTP, proxy, HTTPS fixture, runtime/venv, requirement and registry recovery
regressions. No runtime or manager was installed; no host package, service,
configuration, default, hosts file or trust store was modified.

Provider tests cover exact mise Node/Bun and uv GNU CPython layouts, mise internal
identity-store links, unsupported provider/kind/version combinations, missing and
foreign installations, shims, storage-root escapes (including `/` aliases),
bounded uv directory inspection, stale approvals and actual version mismatch.
Lookup reports `probed: false`; filename version alone never authorizes selection.
Only the approved existing version probe runs before commit. A replaced provider
installation invalidates reviewed plans or later resolution.

The app tests use disposable copies of the real installed native Node 25.9.0,
Bun 1.4.3 and Python 3.12.3 executables in the documented storage layouts.
Lookup, preview, approved selection and passive resolve succeed. Fake manager
commands on PATH and executable-looking manager hooks remain unexecuted; the
adapter imports no command execution mechanism. Provider selection creates no
resource-management grant and keeps the existing schema 4 representation.

The CLI integration test exercises uv lookup, provider evidence in a JSON plan,
approved selection, resolution and rejection of conflicting provider/direct-path
options. A separate built-binary smoke run with temporary XDG state passed:

```text
mise node 25.9.0 locate/approved-select/resolve: passed
mise bun 1.4.3 locate/approved-select/resolve: passed
uv python 3.12.3 locate/approved-select/resolve: passed
provider capabilities: verified; registry schema unchanged at 4
```

These are documented-layout fixtures with real executable copies, not a claim
that mise/uv manager releases or every store layout were installed/tested.
`existing-store-v1` is the explicit supported filesystem contract. Manager
commands, installation/activation, custom external stores, other uv variants and
unsupported layouts do not gain implied support; direct native selection remains
the documented fallback. Because no manager command or output protocol is used,
there is no hidden provider-command version/download/hook contract to accept.

[PHASE-4](../PHASE-4.md), [ADR 0013](../decisions/0013-existing-provider-store-resolution.md),
the [earlier execution report](PHASE-4-EXECUTION.md) and
[release acceptance](RELEASE-ACCEPTANCE.md) provide usage, historical evidence and
remaining release boundaries. Phase 5 has not been started.
