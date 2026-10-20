# Phase 3 completion

**Status: COMPLETE within the approved implementation scope. Real local acceptance
passed on 2026-10-20.** Phase 4 is also complete within scope. The next milestone
is the bounded Phase 5 TUI scope; this record does not authorize its implementation
or claim the open release/platform checks have passed.

## Closed Phase 3 work

| Item | Result |
|---|---|
| Exact existing PHP CLI discovery, approval, selection, resolve and exec | Complete; regression suite passes |
| Independent per-project PHP-FPM, generic PHP/front-controller/static routing | Complete; concurrent real PHP 8.3.35 and 8.5.11 CLI/web checks pass |
| Rootless Nginx lifecycle, private sockets, denied paths, recovery and cleanup | Complete within documented identity/recovery limits; regression suite passes |
| Real mkcert v1.4.4 issuance, SAN/expiry, permissions and HTTPS | Passed using the official native release in `/tmp`, not the test fixture |
| Real CA rotation with active listener, reviewed reload and retained prior evidence | Passed in `TestTLSInstalledMkcert` |
| Public-only CA export and exact served-certificate verification | Passed; no private key exported |
| Linux explicit CA and process-scoped trust/untrust round trip | Passed; normal default roots reject before and after |
| Windows-to-WSL mirrored HTTPS, exact TLS name/leaf and HTTP 200 | Passed using Windows SslStream and a connection-scoped explicit CA |
| Windows default trust before/after the explicit-CA connection | Correctly rejects the disposable CA both times; no root imported |

The PHP version/ini/extension evidence and failure/recovery tests remain recorded
in [PHASE-3-WEB](PHASE-3-WEB.md) and [PHASE-3-RUNTIME](PHASE-3-RUNTIME.md).

## Final local checks

```sh
scripts/validate-phase3.sh /tmp/velnix-phase3-acceptance/mkcert
```

Passed: full `go test -race -count=1 ./...`, `go vet ./...`, `go build ./...` and
`git diff --check`. The runner requires existing ordinary-user Go, Nginx,
PHP CLI/FPM 8.3 and 8.5, and an explicitly supplied mkcert v1.4.4. It installs
nothing and fails missing prerequisites rather than silently treating them as
acceptance. `VELNIX_TEST_MKCERT=/absolute/binary` also makes the real-mkcert test
mandatory in an ordinary test run; missing/relative explicit binaries fail.

The real-mkcert test now exercises both registered names, parses the actual leaf
certificate, checks exact SANs/expiry and 0600 artifacts, validates public export,
rotates to a fresh CA, rejects the old active endpoint as the new certificate,
then approves Nginx reload and verifies the new served leaf. Prior CA evidence is
retained. Normal test cleanup stops only owned workloads and removes disposable
state; no system service or trust setting is touched.

Official binary used:

- Source: [mkcert v1.4.4 Linux amd64 release](https://github.com/FiloSottile/mkcert/releases/download/v1.4.4/mkcert-v1.4.4-linux-amd64)
- Version output: `v1.4.4`
- Observed SHA-256: `6d31c65b03972c6dc4a14ab429f2928300518b26503f58723e532d1b0a3bbb52`

This records the downloaded artifact identity; no upstream signature-verification
claim is made. It remains a temporary test tool, not an installed/bundled runtime.

## Linux and Windows transport/trust evidence

A built CLI with temporary XDG roots linked `demo.test`, issued a disposable
untrusted CA through reviewed `tls issue`, started reviewed rootless HTTP/HTTPS,
exported only the public root and checked the live endpoint. A fresh Linux CLI
process succeeded using explicit CA verification while its default roots rejected
the CA. A child-only `SSL_CERT_FILE` plus empty `SSL_CERT_DIR` made the process's
default verification succeed; removing those overrides restored rejection. This
proves process-scoped trust configuration and reversal, **not OS trust-store
installation/removal**.

Current platform observations:

- WSL networking: `mirrored` (read with `wslinfo --networking-mode`).
- Windows: `Microsoft Windows NT 10.0.26340.0`.
- Windows PowerShell: `5.1.26100.9750`.

The [read-only Windows check](../../scripts/validate-phase3-windows.ps1) connects
to loopback, sends the exact `.test` name as TLS SNI, builds the supplied public
CA chain, verifies hostname and expected leaf DER SHA-256, then requests HTTP.
Windows returned `HTTP/1.1 200 OK`. Default Windows verification rejected the
CA both before and after. No global certificate-validation override, hosts edit,
certificate-store import, execution-policy change or networking change was made.
Only public CA DER and a public leaf fingerprint crossed into Windows memory.

This directly establishes the current mirrored-mode Windows transport and scoped
TLS contract. It does not establish browser UI trust, hosts-file DNS, NAT mode,
VPN behavior or another WSL/native Linux installation. Those remain explicit in
[RELEASE-ACCEPTANCE](RELEASE-ACCEPTANCE.md). A native Ubuntu/non-systemd host can
run the same existing-prerequisite runner; no unperformed run is claimed here.

## Repeating the Windows check

With a disposable reviewed endpoint still running, use the script's `Port`,
`Name`, public DER `RootCertificateBase64` and `LeafSHA256` parameters. It performs
no imports and expects the disposable CA to be untrusted by default. It must be
run in a Windows PowerShell context; using an inline script block also works
without changing execution policy. For a manually trusted CA/browser test, use
the separate [HTTPS instructions](../HTTPS.md) and explicit user approval.

No browser/system trust mutation was performed merely to mark completion. The
remaining release acceptance is not a missing PHP/FPM/HTTPS implementation and
does not start Phase 5 automatically. Define that milestone's bounded scope first.
