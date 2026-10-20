# Open platform and release acceptance

Status: open; no unperformed check below is a passing result.
Updated: 2026-10-20 after real Phase 3 mkcert and Windows mirrored HTTPS validation.

Phases 3 and 4 are complete within approved implementation scopes. Local evidence
is recorded in [PHASE-3-COMPLETION](PHASE-3-COMPLETION.md) and
[PHASE-4-COMPLETION](PHASE-4-COMPLETION.md).
This list keeps the pre-existing platform/release gates distinct from delivered
features. It does not expand authorization into host mutations or later phases.

| Gate | Status / evidence needed |
|---|---|
| Native Ubuntu and a separate non-systemd environment | Unperformed; current integration evidence is local WSL2 |
| Windows network/browser matrix | Current mirrored Windows loopback HTTPS with exact name/leaf verification passed; NAT, VPN, other distributions, hosts DNS and browser/HMR UI scenarios remain unperformed |
| Real mkcert v1.4.4 | PASSED locally: official temporary release, real issuance/SAN/expiry/permissions, public export, HTTPS and CA rotation/reload; see Phase 3 completion |
| Linux/Windows trust and untrust | Scoped explicit-CA verification and default rejection before/after passed; Linux child-process root override round trip passed. OS/CurrentUser/browser installation and fingerprint-specific removal remain unperformed; no trust store was modified |
| Additional mise/uv installation variants/releases | Only documented existing-store-v1 fixtures and disposable real runtime copies validated; other layouts refuse rather than guess |
| Additional runtime/venv/framework versions | Only recorded local versions and bounded metadata contracts are claimed |
| Full filesystem fault-injection and performance matrix | Not established by local transaction/race tests; no measured budget claim |
| Release packaging, license, dependency/security review | Separate stabilization/release work; no release readiness implied |

Existing limitations remain: same-UID hostile code is not sandboxed; runtime
libraries/configuration are not fully pinned; a TCP listener does not prove app
identity; metadata changes need reviewed HTTP reload to affect active routes.
No runtime installation, manager command invocation, global defaults, automatic
Windows/DNS/trust changes or later TUI/mail/dashboard work was added to close this
phase. See [TESTING](../TESTING.md) for the complete acceptance matrix.
