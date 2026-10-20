# Phase 3 FPM and HTTPS — local validation

Follow-up: [PHASE-3-COMPLETION](PHASE-3-COMPLETION.md) records the final
implementation and real mkcert/Linux/Windows mirrored acceptance. This report
retains its historical scope; then-unperformed checks are superseded only by
explicit evidence in that completion report.

Date: 2026-10-20. The remaining approved implementation slices are present.
This closes implementation work, **not full platform acceptance**. Previous
[PHP CLI evidence](PHASE-3-RUNTIME.md) remains valid for its earlier scope;
its then-pending FPM/HTTPS implementation is superseded by this report.

## Environment and checks

Local WSL2 Linux kernel: `6.18.54.1-microsoft-standard-WSL2`.
Nginx: `1.24.0 (Ubuntu)`. Native PHP CLI/FPM: 8.3.35 and 8.5.11.
Real mkcert is absent; the installed-mkcert test explicitly skips. No package,
system service, hosts file, default runtime or trust store was modified.
All listeners, pools, certificates and registry changes used disposable state.

Validation commands:

```sh
go test ./...
go test -race -count=1 ./...
go vet ./...
go build -trimpath -o /tmp/velnix-phase3 ./cmd/velnix
git diff --check
```

The local implementation suite, race detection, vet and build pass. The built
CLI's help lists the new FPM/TLS/HTTPS options. The new CLI integration test
checks initialized-state plans, profile/upload options, wrong approvals,
non-mkcert rejection, invalid flags and no lifecycle state on preview/failure.

## Real PHP and Nginx results

Two independent foreground pools served different registered projects at once:

| Project | CLI | CLI ini | Web | Web ini |
|---|---|---|---|---|
| one | 8.3.35, `cli` | `/etc/php/8.3/cli/php.ini` | 8.3.35, `fpm-fcgi` | `/etc/php/8.3/fpm/php.ini` |
| two | 8.5.11, `cli` | `/etc/php/8.5/cli/php.ini` | 8.5.11, `fpm-fcgi` | `/etc/php/8.5/fpm/php.ini` |

Both CLI and HTTP checks collect and log `get_loaded_extensions()` independently.
Observed CLI lists contain `pcntl`; web lists contain `cgi-fcgi`. Both versions
load PDO, curl, mbstring, OpenSSL and PostgreSQL/MySQL support. PHP 8.5 additionally
reports modules including `pdo_sqlite`, `redis`, `xdebug`, `imagick` and `bcmath`.
No extension equivalence across versions/SAPIs is inferred. Applications that
require absent extensions must surface their own startup/request failure;
Velnix does not install extensions or provide a dependency validator.

Real requests establish generic PHP index handling, POST method forwarding,
front-controller URI/query preservation, and static behavior. Dotfiles, configured
upload directories, built-in uploads, symlink PHP entrypoints, nonexistent scripts
and PHP path-info requests are denied (403/404). No broad framework compatibility
claim is made from the disposable front-controller script.

Stopping the first pool produces 502 for its still-active Nginx route; the other
version continues serving. Reload refuses a stopped enabled pool. Explicit pool
disable plus HTTP reload restores static-only routing and returns 404 for PHP.
CLI selections and system services remain separate from FPM configuration.

Wrong-version FPM approval fails before publishing lifecycle evidence. Missing
executables, invalid profiles/paths and occupied Unix sockets fail visibly;
occupied sockets are preserved. A killed real FPM master causes its owner to
clean up recorded workers, retain recovery evidence and remove only the recorded
socket. Explicit recovery followed by fresh reviewed startup succeeds. Normal
cancellation stops owned workloads. Existing HTTP tests continue to cover
occupied listeners, external edits, worker cleanup, reload and recovery.

## HTTPS evidence and its limits

A strict **native Go test fixture**, explicitly not mkcert, implements the reviewed
v1.4.4 adapter invocation and generates a disposable CA/leaf. It checks exact argv,
CAROOT/cwd, disabled trust stores and the absence of inherited environment. The
adapter version and argv were reviewed against upstream mkcert v1.4.4 source.
This tests our integration contract, not real mkcert binary compatibility.

Fixture-backed issuance passes exact SAN, chain, expiry, private-key matching,
artifact hash and 0600/0700 permission checks. A public-only PEM export is checked.
Wrong SANs and deliberate child failure retain interrupted issuance; reviewed
recovery preserves artifacts and a fresh issuance uses another generation.
Wrong approvals fail. Externally edited keys prevent subsequent HTTPS plans.

Real installed Nginx consumes those generated files and serves HTTPS. The test
verifies the exact served leaf with the dedicated CA for both registered names,
while HTTP remains usable. The second connection against Linux default roots
fails as expected for the untrusted test CA. Windows/browser trust remains an
explicitly unverified result. No CA is added to any store by the test.

## Outstanding acceptance

- Real installed mkcert v1.4.4 issuance, interruption behavior and HTTPS smoke.
  `TestTLSInstalledMkcert` is ready but skipped here because the binary is absent.
- Manual Linux trust installation/removal and exact-fingerprint untrust checks.
- Actual Windows hosts resolution, browser trust, CurrentUser/browser-store
  untrust, NAT/mirrored networking, VPN/forwarding and multi-distribution matrix.
- Native Ubuntu and a separately exercised non-systemd environment; running
  foreground workloads locally does not establish the whole portability matrix.
- Power-loss/disk-full fault injection at every new journal boundary and measured
  performance budgets. Existing recovery tests do not establish filesystem
  atomicity or protection against a hostile process sharing the same UID.

See [FPM](../FPM.md), [HTTPS](../HTTPS.md), [ADR 0011](../decisions/0011-phase3-web-lifecycle.md)
and [TESTING](../TESTING.md). No Phase 4 execution/proxy/provider expansion was
implemented in this task.
