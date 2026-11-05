# Open platform and release acceptance

Status: open; no unperformed check below is a passing result.
Updated: 2026-11-05 after Phase 6 local mail completion; prior platform gates retained.

Phases 3, 4, 5 and 6 are complete within approved implementation scopes. Local evidence
is recorded in [PHASE-3-COMPLETION](PHASE-3-COMPLETION.md) and
[PHASE-4-COMPLETION](PHASE-4-COMPLETION.md) and
[PHASE-5-COMPLETION](PHASE-5-COMPLETION.md), with the separately authorized
[Phase 5B management extension](PHASE-5B-COMPLETION.md).
This list keeps the pre-existing platform/release gates distinct from delivered
features. Phase 6 local mail evidence is in [PHASE-6-COMPLETION](PHASE-6-COMPLETION.md). It does not expand authorization into host mutations or later phases.

| Gate | Status / evidence needed |
|---|---|
| Additional terminal emulator/Windows-terminal TUI validation | Unperformed; Phase 5 has local Linux/WSL PTY evidence and user acceptance of the home skeleton only |
| Native Ubuntu and a separate non-systemd environment | Unperformed; current integration evidence is local WSL2 |
| Windows network/browser matrix | Current mirrored Windows loopback HTTPS with exact name/leaf verification passed; NAT, VPN, other distributions, hosts DNS and browser/HMR UI scenarios remain unperformed |
| Real mkcert v1.4.4 | PASSED locally: official temporary release, real issuance/SAN/expiry/permissions, public export, HTTPS and CA rotation/reload; see Phase 3 completion |
| Linux/Windows trust and untrust | Scoped explicit-CA verification and default rejection before/after passed; Linux child-process root override round trip passed. OS/CurrentUser/browser installation and fingerprint-specific removal remain unperformed; no trust store was modified |
| Additional mise/uv installation variants/releases | Only documented existing-store-v1 fixtures and disposable real runtime copies validated; other layouts refuse rather than guess |
| Additional runtime/venv/framework versions | Only recorded local versions and bounded metadata contracts are claimed |
| Mail provider/platform breadth | Only the recorded local Mailpit v1.33.0 binary and disposable fixtures are validated; native Ubuntu/Windows connectivity, other provider versions, resource exhaustion and full disk-failure/recovery matrices remain open |
| Full filesystem fault-injection and performance matrix | Not established by local transaction/race tests; no measured budget claim |
| Release packaging, license, dependency/security review | Separate stabilization/release work; no release readiness implied |

Existing limitations remain: same-UID hostile code is not sandboxed; runtime
libraries/configuration are not fully pinned; a TCP listener does not prove app
identity; metadata changes need reviewed HTTP reload to affect active routes.
No runtime installation, manager command invocation, global defaults, automatic
Windows/DNS/trust changes or dashboard were added. Phase 5 adds the approved
TUI and separately reviewed Phase 5B management extension. Phase 6 adds the
separately approved foreground mail CLI using an external provider; its
no-egress guarantee is application policy for approved flows, not kernel
isolation or a guarantee about unused authenticated provider features. See [TESTING](../TESTING.md) for the complete acceptance matrix.
