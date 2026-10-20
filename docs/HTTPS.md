# Optional rootless HTTPS

HTTP remains independently usable. HTTPS requires an externally installed native
**mkcert v1.4.4**, whose adapter contract is explicitly checked after approval.
Other versions fail visibly until reviewed. Velnix does not install mkcert,
change hosts files, call `mkcert -install`, or modify any trust store.

Link the exact project names first, then preview issuance:

```sh
velnix tls issue --mkcert /absolute/mkcert --dry-run --json
velnix tls issue --mkcert /absolute/mkcert --approve ISSUE_PLAN_ID --json
velnix tls status --json
```

The approved plan pins the native executable and registered names. It checks
`-version`, journals intent, creates a new exclusive 0700 `tls-GENERATION` under
private XDG state, and runs `-cert-file cert.pem -key-file key.pem NAME.test ...`
there. `CAROOT` is that generation, `TRUST_STORES=none`, cwd is the generation and
other inherited environment is absent. Commands have a 20-second overall limit
and bounded retained diagnostics. Issuance never uses wildcards, IP SANs,
external domains or arbitrary user-supplied certificate names.

Every issuance creates an independent **untrusted CA** and leaf. Rotation keeps
previous generations intact, including keys, so an active HTTPS listener is not
silently broken. This simple first adapter intentionally does not reuse a CA.
After rotation, manual trust must be established for the new fingerprint and
removed separately for the old one. Newly linked/unlinked names require fresh
issuance before HTTPS start/reload.

Before publishing ready evidence, Velnix checks the exact SAN set, validity,
server-auth chain, CA self-signature, matching leaf/CA private keys and private
0600 files. It records artifact hashes, CA SHA-256 fingerprint, expiry and the
public CA certificate. No private key bytes enter plans, CLI output or exports.
A subsequent HTTPS plan rejects modified artifacts rather than overwriting them.
An explicitly approved external binary is trusted code, not sandboxed code.

```sh
velnix http start --https --https-port 8443 --dry-run --json
velnix http start --https --https-port 8443 --approve START_PLAN_ID --json
velnix tls verify --name demo --port 8443 --json
```

Nginx keeps HTTP on loopback port 8080 and adds TLS 1.2/1.3 on loopback 8443.
Both ports are configurable unprivileged ports and must differ. Existing HTTPS
instances retain HTTPS on reload; stop and start without `--https` to disable it.
Candidate configuration is validated before publication. Start checks both ports;
occupied listeners are preserved. Readiness checks both HTTP generation and the
exact served certificate with the dedicated CA. Reload/stop/recovery retain the
[HTTP lifecycle](HTTP.md), including failed-reload evidence and interruption limits.

## Names, trust and public export

Manually configure each exact name, for example `127.0.0.1 demo.test`, in the
relevant Linux/Windows hosts file. Linux and Windows DNS are independent. URLs
retain explicit ports. Without hosts changes, local curl can use:

```sh
velnix tls export-ca > /tmp/velnix-public-root.pem
curl --cacert /tmp/velnix-public-root.pem --resolve demo.test:8443:127.0.0.1 https://demo.test:8443/
```

`tls export-ca` emits only PEM for the ready public CA (`--json` emits a string).
Never export/copy `rootCA-key.pem` or `key.pem`. Compare the fingerprint reported
by `tls status` with the DER SHA-256 fingerprint shown by your certificate tool.

`tls verify` connects directly to Linux loopback with the exact registered name
as SNI. It separately reports:

- `explicit_ca`: chain/hostname verification and exact current leaf identity.
- `linux_system_trust`: a second connection using Go's Linux default roots;
  failure includes `linux_error`. This does not establish browser/NSS trust.
- `windows_browser_trust`: always explicitly unverified by this Linux command.

Trust is a separate manual decision. On Ubuntu, review the exported certificate,
install only the public root under `/usr/local/share/ca-certificates/` with a
unique filename, then explicitly run `update-ca-certificates` with the required
privileges. To reverse, verify that file's fingerprint, remove that exact file
and run `update-ca-certificates` again. Velnix runs none of these commands.

On Windows, import only the reviewed public certificate into the **Current User**
Trusted Root Certification Authorities store using `certmgr.msc`. Verify the
certificate details and fingerprint. Test the actual Windows browser URL and
certificate chain separately; Firefox/managed browsers may use different stores.
To untrust, identify the exact imported certificate and remove it from each
store/browser where you installed it. Do not remove roots by display name alone.
An enterprise policy can prohibit trust; that is an unavailable acceptance result.

WSL NAT/mirrored networking, VPNs, browser stores and Windows forwarding require
actual platform validation. A successful Linux explicit-CA handshake establishes
none of them. No automatic Windows networking/firewall/resolver changes occur.

## Interrupted issuance and cleanup

```sh
velnix tls recover --dry-run --json
velnix tls recover --approve RECOVERY_PLAN_ID --json
```

A failed/interrupted issuance leaves `phase: issuing`, `tls-before.json` and the
partial generation. Recovery marks it `abandoned` and preserves everything; it
never reruns mkcert or removes keys. A fresh reviewed issuance uses another
exclusive generation. An existing directory is never adopted or overwritten.

Before manually deleting a generation, stop/reload any listener that references
it, retain the public certificate/fingerprint, and explicitly untrust that CA in
all affected Linux, Windows and browser stores. Deleting key/certificate files
alone does **not** remove trust. Old CA trust is not detected automatically; no
recursive cleanup command is offered. Ambiguous/external evidence requires
manual review. Registry recovery, HTTP recovery and TLS recovery are independent.

[Completion validation](validation/PHASE-3-COMPLETION.md) records passing real
mkcert issuance/rotation and Linux/Windows mirrored scoped HTTPS checks. Browser
and OS trust-store acceptance remains explicitly open. Repeat local real-tool
checks with `scripts/validate-phase3.sh /absolute/mkcert` or set
`VELNIX_TEST_MKCERT=/absolute/mkcert` for Go tests. Adapter design
was checked against [mkcert v1.4.4 source](https://github.com/FiloSottile/mkcert/blob/v1.4.4/main.go)
and its [CAROOT documentation](https://github.com/FiloSottile/mkcert#changing-the-location-of-the-ca-files).
