# Phase 4 selection, execution, metadata and proxy — local validation

Date: 2026-10-20. This completes the bounded native implementation described in
[PHASE-4](../PHASE-4.md). It does not close platform acceptance or claim support
for optional manager command adapters. The earlier
[inspection report](PHASE-4-RUNTIME.md) records its historical narrower scope.

## Performed checks

```sh
go test ./...
go test -race -count=1 ./...
go vet ./...
go build -trimpath -o /tmp/velnix-phase4 ./cmd/velnix
git diff --check
```

All checks passed. Existing PHP CLI, FPM, static HTTP and fixture-backed HTTPS
regressions remain passing. No packages, services, system Python, shell defaults,
hosts files or trust stores were changed. Tests used disposable registries,
projects, venvs, listeners and certificate generations. No Node/Bun/Python
runtime was installed or adopted.

## Real runtimes and built CLI

| Runtime | Native executable | Observed version |
|---|---|---|
| Node | `/home/nazmul/.nvm/versions/node/v25.9.0/bin/node` | 25.9.0 |
| Bun | `/home/nazmul/.bun/bin/bun` | 1.4.3 |
| Python | `/usr/bin/python3.12` | 3.12.3 |

Real integration tests preview/approve each selection, resolve it passively,
execute it with literal shell-like argv and the registered cwd, check stripped
language variables, preserve the parent environment and return child exit 23.
They verify read-only preview, independent project/ownership metadata, relink
retention and clear behavior. Node/Bun resolve also checks that child PATH's
`node`/`bun` alias identifies the selected binary. Python cancellation kills and
reaps its direct sleeping child.

A built-binary smoke run used temporary XDG roots, a linked project and manifests
containing an untrusted postinstall marker. It approved selection, resolved and
executed all three runtimes, detected FastAPI/Vite/Vue, cleared Node independently
and verified schema 4 retained Bun/Python. The hook marker remained absent.
Node reported `v25.9.0` for both itself and a nested bare `node --version` child.
The first sandbox run hit `spawnSync EPERM`; the same disposable smoke check
passed outside the sandbox. No application fix was inferred from that sandbox
restriction.

The CLI integration test additionally covers JSON plans/diagnostics, explicit
`--kind`, child status/raw output, detect/proxy options, unsupported flag
combinations, capability reporting and resolution failure after clear.

## Python environments and requirements

A real disposable Python 3.12.3 venv was created with `-m venv --without-pip`.
Selection/resolve inspected metadata and queried version without evaluating a
project `sitecustomize.py` hook. Explicit execution using the pinned interpreter
and venv launcher argv[0] reported the venv as `sys.prefix` with a distinct
`sys.base_prefix`. This establishes that canonical interpreter pinning does not
silently discard the environment for the tested CPython installation.

Tests refuse changed pyvenv.cfg, incompatible interpreter aliases, newly appeared
unreviewed `.venv` directories, conflicting exact version files and metadata edits
between preview/approval. Compatible PEP 621 requirements are accepted without
running a build backend. Environment creation, repair, pip and activation remain
outside Velnix's commands. Other Python versions/layouts require compatibility
evidence rather than extrapolation from this test.

## No implicit installation and passive metadata

A real Bun execution imported a deliberately missing package while project
configuration requested forced auto-install and used a disposable local registry.
Velnix's `--no-install` caused child failure and the registry received zero
requests. This proves the default runtime path does not perform that implicit
lookup for installed Bun 1.4.3. Explicit user code/package commands are trusted
execution and are not sandboxed.

Metadata tests cover known dependency-based framework hints, exact files,
supported numeric ranges, multiple-file conflicts, malformed JSON/TOML, shell
expressions, aliases, symlinks, FIFOs and oversized files. Reads are bounded and
nonblocking; no script/build backend runs. Raw-byte changes alter approval
fingerprints. The comparator is a documented subset, not full semver/PEP 440.

`runtime providers --json` reports native exact resolution as available and
mise/uv/nvm/pyenv command integration as unavailable with explicit reasons.
Neither mise nor uv is installed locally. They are never invoked or installed by
detection/resolution. Existing externally managed native binaries can be selected
by absolute path; that does not grant installation management authority.

## Schema and recovery

Schema-3 PHP data migrates to schema 4 without losing PHP or other project data.
Subsequent PHP clear, proxy edits and relink preserve schema 4 and independent
runtime selections. Schema 3 refuses new settings; future schema 5 remains
rejected. The journal recognizes select/clear-runtime and proxy operations.

Before-commit and after-commit recovery fixtures recognize only the recorded
registry digests, never replay executable probes and retain the correct selection
state. External edits require manual review. These tests extend existing registry
concurrency/recovery tests; they do not establish power-loss atomicity or
protection from a hostile process sharing the same UID.

## Real proxy behavior

Installed Nginx forwards to a disposable already-running IPv4 loopback HTTP
server. Tests verify original URI/query, POST method forwarding, Host and
connection-derived forwarding headers, with spoofed forwarding claims overwritten.
An HTTP 101 upgrade tunnel exchanges bytes in both directions, covering the
Nginx WebSocket/HMR forwarding mechanism; no browser framework/HMR acceptance is
inferred. Unknown hosts and sensitive paths remain denied before upstream access.

Stopping the external upstream produces 502. An unavailable upstream refuses a
fresh approved start before lifecycle publication. Direct listener self-proxy and
invalid ports fail. Reviewed port-zero metadata plus HTTP reload restores static
serving. The external app server is never launched, signaled or adopted.

A separate real Nginx HTTPS proxy test uses the existing strict native certificate
fixture, verifies the dedicated CA and receives `X-Forwarded-Proto: https` at the
upstream. This does not establish real mkcert or OS/browser trust compatibility.

## Outstanding acceptance and deferred capabilities

- Optional mise/uv command adapters: unavailable until exact tool versions and
  offline/no-hook/no-download contracts pass dedicated compatibility tests.
- Full Windows NAT/mirrored/VPN/browser and native Ubuntu/non-systemd matrix.
- Real mkcert/trust acceptance from Phase 3 remains open; mkcert is still absent.
- Additional runtime versions, exotic virtualenv layouts, prerelease/general
  constraint dialects and real framework/browser/HMR matrices remain unclaimed.
- Exhaustive fault injection, performance budgets, release packaging, license
  review, runtime installation/global defaults and later roadmap interfaces are
  separate work. No Phase 5 implementation was started.

See [ADR 0012](../decisions/0012-phase4-execution-and-proxy.md) and
[TESTING](../TESTING.md).
