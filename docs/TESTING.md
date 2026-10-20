# Velnix — Validation and Performance Plan

Status: release acceptance requirements. Phase 0 performed documentation research only; Phase 1 local foundation results are recorded in [validation/PHASE-1.md](validation/PHASE-1.md). The first Phase 2 registry slice is recorded in [validation/PHASE-2-REGISTRY.md](validation/PHASE-2-REGISTRY.md); Local HTTP evidence is recorded in [validation/PHASE-2-HTTP.md](validation/PHASE-2-HTTP.md); unperformed platform and filesystem fault gates remain open. Source research is recorded in FEASIBILITY.md. Documentation-only checks do not establish runtime feasibility.

## Test layers

Unit tests: deterministic version precedence/conflicts, ownership grants, plan digest invalidation, path/domain validation, error mapping, config templates, schema migrations and redaction. Fake providers/runner/clock keep unit tests offline and unprivileged. Fuzz TOML/JSON inputs, hostname/path handling and bounded provider-output parsers. Tests must assert behavior rather than duplicate template implementation.

Adapter contract tests: fixture output for minimum/selected provider versions, unknown fields, malformed output, timeouts, missing commands and unsupported capability reasons. Separate discovery from explicitly authorized executable probes. Assert no auto-install/download/config trust, no shell-profile sourcing, no global default changes and no repository hooks. Use a network-denied disposable environment and an untrusted project with marker hooks; marker absence plus filesystem/process traces verifies nonexecution. Where isolation flags cannot be proven, adapter resolution stays unsupported.

Integration tests: disposable native Linux VMs or dedicated WSL distributions, optional containers for test infrastructure only. No host package/trust mutation in ordinary CI. Privileged test jobs are opt-in, isolated and resettable. Use actual installed Nginx/FPM/mkcert and record versions; fake command success alone cannot establish compatibility. Run Go race detection, vet and vulnerability checks once implementation exists; pin CI toolchains and actions. No sudo password or CA key in CI logs/artifacts.

## Acceptance scenarios

| Area | Required evidence |
|---|---|
| CLI | Scriptable JSON on stdout, diagnostics on stderr, stable success/invalid-input/unavailable/denied/conflict/failure/cancellation codes; no interactive hangs |
| No systemd | Doctor identifies absence; rootless foreground HTTP/FPM works; unsupported lifecycle gives clear remediation |
| PHP | Two existing versions serve different projects concurrently; CLI and HTTP SAPI/version/ini/extensions checked independently; change one leaves other unchanged |
| PHP safety | Missing FPM/extension, denied socket, failed pool startup and wrong-version endpoint fail visibly; `.env`, uploads and nonexistent scripts not executed |
| Node/Bun/Python | Exact selected executable verified; conflicting version files fail; child PATH stable; parent shell/system Python unchanged; compatible/incompatible `.venv` exercised |
| Ownership | External runtime/package/service never adopted or deleted by discovery; unlink preserves source/data; removal refuses references and missing grants |
| Nginx | Separate prefix/loopback, occupied ports preserved, config injection rejected, candidate validation failure preserves old config, reload health failure enters recovery |
| Concurrency/crash | Two CLI writers and future CLI/web writers serialize; SIGKILL at each journal boundary, disk-full, stale lock, failed rename and external file edits recover without clobbering |
| Process lifecycle | Restart/boot ID/PID reuse, child exit and cancellation cannot kill unrelated processes; bounded logs; no implicit lingering |
| DNS | Linux and Windows independently resolve each exact `.test` name; alias removal preserves others; IPv6 has explicit enabled/unavailable status |
| HTTPS | SAN/expiry/private permissions; Linux CLI trust and Windows browser trust separately checked; no CA private key exported; exact-fingerprint untrust tested |
| Network | Windows browser HTTP/HTTPS reaches selected WSL instance; NAT/mirrored, VPN, occupied Windows ports, WSL restart/IP change and multiple distributions exercised |
| Native portability | Same app/domain code on native Ubuntu; no Windows executable requirement; unsupported package/service adapters degrade clearly |
| Later web/mail | Authentication/CSRF/rebinding/Host/origin tests; hostile MIME/HTML, relay rejection, quotas/retention and attachment traversal tests before release |

Full v0.1 matrix: native Ubuntu 24.04 amd64; WSL2 Ubuntu 24.04 with Windows 11 NAT; same with supported mirrored networking; a non-systemd Linux environment for doctor/foreground paths. Record exact OS, kernel, WSL, Windows, provider, CPU, filesystem and browser versions. Windows browser and trust scenarios require real Windows access; Linux-only CI is insufficient. Enterprise trust/VPN restrictions may remain documented unsupported configurations, never fabricated passes. Framework smoke cases: static site, generic PHP and one front-controller app; broad Laravel/WordPress/Drupal support waits for corresponding routing/security fixtures.

## Performance budgets (proposed, unmeasured)

Reference machine: 4 logical CPU cores, 8 GiB RAM available to Linux, SSD, projects/state on native ext4 (WSL virtual disk for WSL runs), 100 registered projects, up to 20 discovered installations. Record actual model and contention. `/mnt/c` results are separate and cannot be compared to ext4 budgets. Release build, no debugger, no network/provider installation; report p50/p95 over at least 50 warm samples plus 10 fresh-process cold-cache samples with cache definition recorded. Do not flush host caches without explicit permission. Separate Velnix CPU/RSS from child providers and workloads; publish both where applicable.

| Operation/resource | Initial target |
|---|---|
| `version` / help | warm p95 ≤50 ms; cold p95 ≤150 ms |
| 100-project list (no deep probes) | warm p95 ≤100 ms; cold p95 ≤250 ms |
| Cached local runtime resolution | warm p95 ≤100 ms; no network |
| Doctor local scan | p95 ≤2 s; total deadline 5 s, per external probe 500 ms, partial results on timeout |
| Dry-run render/plan for one project | p95 ≤200 ms, excluding external validation |
| Nginx config validation + reload + health | p95 ≤2 s on reference machine, timeout 5 s; no package/network work |
| CLI peak RSS for list/resolve | ≤30 MiB Velnix process; external probes reported separately |
| Idle control plane | zero Velnix processes/listeners after command exit unless explicitly requested |
| Requested workloads | idle CPU <1% of one core over 60 s; report Nginx/FPM memory and worker counts separately; no unsupported aggregate promise |
| Distribution | stripped CLI archive ≤25 MiB compressed; no runtimes/frontend in v0.1 |

These are acceptance budgets to validate, not benchmark claims. Record startup/process overhead separately from provider time and HTTP application latency. Rebaseline requires documented evidence and approval, not silently relaxing a failing budget. Later optional API target: idle RSS ≤60 MiB excluding browser/mail/workloads; dashboard initial compressed JS+CSS ≤250 KiB, no polling while hidden. Validate those when their phases begin.

## Release evidence and cleanup

Publish commands/tool versions, matrix results, percentile samples, failed/unsupported cases and recovery evidence in a dated validation report. Compare file hashes and service state before/after, prove external resources unchanged and no unrequested listeners remain. Cleanup only owned temporary files/processes; untrust disposable CA explicitly, destroy test VM when privileged effects cannot be safely reversed. No performance, WSL compatibility or security guarantee until measured. License, dependency inventory, checksums, vulnerability-report contact and documented known limits are release gates.
