# Phase 3 runtime and HTTPS proposal

Status: approved by “Approved, start implementation” on 2026-10-20, following
“Lets start work with phase 3”. The first PHP CLI slice is implemented; see
[usage](RUNTIMES.md) and [validation](validation/PHASE-3-RUNTIME.md).
FPM and HTTPS implementation remains pending within this approved scope.

## Scope and implementation order

Deliver three bounded slices, each with application services, CLI, tests and
usage/recovery documentation before starting the next. Preserve the existing
static HTTP behavior and registry transactions. Earlier unperformed platform
acceptance checks remain open.

1. Existing PHP CLI discovery, exact resolution, per-project selection and
   explicit `exec`. Separate passive discovery from executable probing. List
   absolute candidates and external provenance without adopting installations;
   require explicit approval to probe an executable. Resolve only verified
   existing selections, fail on missing or mismatched versions, and recheck
   executable identity before use. Do not source profiles, evaluate repository
   configuration, invoke version-manager shims or install anything. Store
   selection changes through the existing reviewed registry transaction model
   with a versioned schema migration. CLI and FPM selections remain independent.
2. Existing PHP-FPM with per-project private Unix sockets and generated owned
   configuration. Add explicit foreground start/status/stop and reviewed
   Nginx routing changes. Support separate static, generic PHP and
   front-controller profiles; PHP execution must be an explicit project choice.
   Validate script existence and containment, deny sensitive paths and symlink
   escapes, and forbid configured upload directories as PHP entrypoints.
   No arbitrary project Nginx/FPM fragments or automatic framework commands.
3. Optional externally installed mkcert, dedicated private Velnix CAROOT,
   reviewed issuance for registered exact `.test` names, and rootless HTTPS on
   configurable loopback port 8443. Keep HTTP independently usable. Provide
   manual exact hosts and trust instructions, public CA fingerprint/export,
   separate trust verification results and removal instructions.

Node/Bun/Python resolution, reverse proxy and provider adapters remain in the
later v0.1 stabilization milestone. Runtime installation, global defaults,
systemd, automatic hosts/trust changes and later roadmap features remain deferred.

## Execution and lifecycle decisions

Accept ADR 0002 only for direct existing PHP executable selection and independent
CLI/FPM resolution. Provider-managed resolution remains proposed. `exec` receives
an explicit argv after `--`, uses the registered project directory, and passes
arguments without a shell. It executes user code only on explicit invocation,
with the selected runtime available through a documented child environment;
the parent environment is unchanged. Forward cancellation/signals and preserve
child exit status, documenting its distinction from Velnix diagnostic exit codes.
Runtime probes use bounded output, deadlines and a controlled environment.

Extend ADRs 0003 and 0006 only to foreground, user-owned FPM and rootless TLS.
Reuse process identity verification, mutation locking, approval digests and
journals rather than introduce a daemon. A separate foreground FPM owner keeps
the pool alive; Nginx routing requires that verified pool and reports unavailable
endpoints. Exiting the owner cleans up its pool. Stop or failure can interrupt
requests; no automatic restart or silent launch replay. Bind executable,
configuration, socket and process identities into reviewed operations. Preserve
external edits and report recovery-required for ambiguous partial effects.

Accept ADR 0004 for manual exact-domain setup and separately verified trust.
Do not run `mkcert -install`: issuance may create a dedicated CA and leaf keys
only through an approved operation; it must not modify any trust store. Review
the installed mkcert adapter contract before invoking it. Private keys remain
in private Linux state and never enter exports or logs. Trusting the public root
is a separate manual user action. Removing files does not remove trust; CA
cleanup requires documented fingerprint-based untrust in each affected store.

## Validation and completion gates

- Test selection migration, exact-version mismatch, stale executable identity,
  untrusted candidates, hook nonexecution, argv/environment handling, signal
  forwarding, child exit status and approval invalidation.
- Exercise real installed PHP CLI/FPM versions independently. Demonstrate two
  projects using different PHP versions concurrently, checking CLI and web SAPI,
  version, ini and extensions. Missing FPM, failed pool startup, denied socket,
  nonexistent scripts and upload execution must fail visibly.
- Exercise real Nginx/FPM with disposable owned state, verify routing and
  cancellation cleanup, preserve occupied listeners and external configuration,
  and test failed validation/reload plus interrupted-operation recovery.
- Exercise installed mkcert with a disposable untrusted CA. Verify exact SANs,
  expiry, permissions, HTTPS handshake and explicit-CA verification. Record Linux
  system trust and Windows browser trust separately from explicit-CA success.
- Run Go tests with race detection, vet and diff checks. Record unavailable
  binaries/platforms as outstanding acceptance evidence, never as passing tests.
  Windows NAT/mirrored networking and native Ubuntu checks require actual access.

Approval authorizes implementation and disposable unprivileged local tests of
these slices. It does not authorize package installation, system service changes,
host DNS/trust mutations or stabilization work. Each shipped mutation retains
its own exact reviewed CLI approval.
