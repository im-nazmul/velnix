# Velnix — Proposed Implementation Plan

Status: Phase 2 authorized by the user with “Lets start work with phase 2”. The first project registry slice is described in [PROJECTS](PROJECTS.md); foundation details are in [FOUNDATION](FOUNDATION.md). Rootless static HTTP and explicit foreground lifecycle are now implemented; see [HTTP](HTTP.md) and [local validation](validation/PHASE-2-HTTP.md). Phase 2 implementation is complete within the approved scope; unperformed platform matrix checks remain open. Later milestones need separate approval. This authorization does not permit provider installation or system configuration changes.

## Architecture and technology

Go core with standard-library process, filesystem, HTTP and JSON facilities; select a small CLI parser during Phase 1 after dependency/license review. TOML for human configuration, versioned JSON for the initial registry, and bounded JSON operation records under XDG state. SQLite is deferred until mail/query volume or measured concurrency justifies it. Registry writes use a common repository abstraction with OS interprocess locks and atomic durable replacement. No public plugin ABI, dependency-injection framework or generic event bus.

Bubble Tea/Lip Gloss remains the TUI direction. Vue 3/TypeScript/Vite static assets embedded through Go remain the web direction; neither ships in v0.1. Bun is the default dashboard package manager and script runner; any Node requirement from dashboard build tools must be documented when those tools are introduced. Neither Bun nor Node is a runtime requirement for installed Velnix. Prefer external mkcert for local HTTPS; provider and certificate dependencies remain optional and externally installed in v0.1.

Proposed package layout (create incrementally, no empty skeleton roadmap):

```text
cmd/velnix/                    composition and process entrypoint
internal/domain/              IDs, resource ownership, version requirements, errors
internal/app/                 doctor, resolve, link, plan/apply, reconcile use cases
internal/ports/               small consumer-defined external contracts
internal/state/               schema, registry, locks, journal, migrations
internal/platform/            Linux/WSL facts, XDG and capability probes
internal/providers/system/    package/runtime discovery
internal/providers/mise/      optional resolution adapter
internal/providers/uv/        optional Python adapter
internal/providers/nginx/     rootless config/render/validate/lifecycle
internal/providers/phpfpm/    independent FPM capability and pool config
internal/providers/systemd/   scoped service adapter
internal/providers/mkcert/    issuance and trust-plan adapter
internal/process/             bounded exec, cancellation, process identity
internal/transport/cli/        output, flags, confirmations, exit codes
internal/transport/tui/        later
internal/transport/http/       later API and embedded assets
web/                          later Vue sources
```

Dependency direction: transports → app → domain and small ports; provider/state/platform adapters implement those ports; composition injects adapters. Domain imports no transports, provider tools or database drivers. Application services own policy, plans, ownership checks and orchestration. Adapters do not prompt users or autonomously install dependencies. Consumers define interfaces close to use cases; move to a shared ports package only when multiple consumers need them.

Illustrative contracts only, not Go implementation or a frozen SDK:

```go
type Discoverer interface {
    Discover(context.Context, DiscoveryRequest) ([]Installation, error)
}
type Resolver interface {
    Resolve(context.Context, ResolveRequest) (ExecutionSpec, error)
}
type InstallPlanner interface {
    PlanInstall(context.Context, InstallRequest) (Plan, error)
}
type RemovePlanner interface {
    PlanRemove(context.Context, ResourceID) (Plan, error)
}
type PlanApplier interface {
    Apply(context.Context, ApprovedPlan) (Receipt, error)
}
type ServiceReader interface {
    Status(context.Context, ServiceRef) (ServiceStatus, error)
}
type ServiceController interface {
    Change(context.Context, ApprovedServiceAction) (Receipt, error)
}
type ConfigValidator interface {
    Validate(context.Context, CandidateConfig) (Validation, error)
}
```

Only implement the interfaces a provider actually supports. A typed capability description includes operation, runtime/SAPI, scope, privilege, platform constraints and unavailable reason; capability reporting is not authorization. Discovery returns provider identity and provenance, not mutation grants. ExecutionSpec contains absolute executable, argv, cwd, environment delta and resolved exact version. Resolve is nonmutating, with automatic downloads disabled and repository hooks excluded. If a provider cannot meet that contract, mark resolution unsupported and use explicitly approved executable paths. Installation and removal are separate optional contracts; FPM endpoint resolution is independent of CLI resolution. Plans contain typed allowlisted effects, not arbitrary command strings; app validates grants before adapter execution.

## Proposed v0.1 release boundary

Target Linux amd64, Ubuntu 24.04 LTS on native Linux and WSL2. Record exact Windows build/WSL version in results; validate supported Windows 11 NAT and mirrored configurations. Other distributions, architectures, older Windows, and non-systemd hosts receive capability diagnostics rather than a compatibility promise. Rootless foreground workloads and CLI discovery must work without systemd; optional systemd lifecycle depends on actual availability.

Include:

- CLI doctor/version, structured diagnostics, stable errors/JSON, config validation, project list/inspect/link/unlink, runtime list/inspect/resolve and `exec` as proposed new command.
- Read-only PHP CLI/FPM, Node, Bun and Python discovery; existing absolute installation selection. Optional mise Node/Bun and uv Python resolution only when their nonmutating contract passes. Existing nvm/pyenv/PHP-specific installations remain externally owned.
- Versioned project registry, explicit per-project selection, conflict explanations, private durable state, mutation locks, plans/dry-run and interrupted-operation reconciliation.
- Rootless isolated Nginx on configurable loopback ports, defaults 8080/8443, explicit start/stop, static serving, generic/front-controller PHP routing and reverse proxy to an already running explicitly configured loopback HTTP server. No automatic dev-server scripts.
- Existing FPM binaries with owned per-project configs/sockets; demonstrate two PHP versions simultaneously when supplied by the test environment. Missing CLI or FPM is an explicit unavailable state.
- Exact `.test` domains/aliases with manual Linux/Windows hosts instructions; optional external mkcert issuance, manual trust steps and verification status. HTTP remains usable without HTTPS dependencies.
- Read-only system service status for PostgreSQL, MariaDB/MySQL and Redis when available; explicit unavailable or permission-denied status elsewhere. No database changes.
- User documentation for stop, unlink, trust removal, recovery and supported platform limits.

Defer runtime/provider/package installation, upgrades/uninstall, adoption mutations, shared Nginx, privileged helpers, ports 80/443 integration, automatic Windows writes, wildcard DNS, park scanning, broad framework automation, dependency installs, managed app-server supervision, system database lifecycle/data operations, TUI, web, mail, plugin SDK and non-Ubuntu mutation adapters. These remain product goals, not missing v0.1 implementations. Manual prerequisite setup must remain explicit and documented; do not hide deferred installers behind onboarding.

## Approval-dependent milestones

| Milestone | Deliverable | Exit gate |
|---|---|---|
| Phase 1 foundation | Go/CLI/config/platform facts, doctor, state/journal/locks, CI | No probe mutations; schema/concurrency/crash tests; foundation budgets measured |
| Phase 2 project/HTTP | Registry, rootless Nginx, static/link/unlink, explicit process identity | External config unchanged, conflicts preserved, failure recovery and HTTP tests pass |
| Phase 3 runtime/HTTPS | Existing runtime resolution, exec, independent FPM and optional mkcert | Two PHP CLI/web versions verified; missing versions fail; Linux/Windows trust checks recorded |
| v0.1 stabilization | Limited Node/Bun/Python resolution, explicit reverse proxy, docs and packaging | Full TESTING matrix, performance results, license and security review complete |

The original later roadmap phases remain separate: full multi-language provider mutations, TUI, mail and web follow v0.1. Each milestone needs user approval before proceeding to the next; the current authorization covers Phase 2 only. Do not write prototypes during Phase 0. Failed rootless/provider spikes trigger revised ADR/scope, never automatic privileged fallback.

## Approval decisions

Approve or amend proposed ADRs 0001–0007: Go/package boundaries; provider strategy and read-only v0.1; rootless high-port web/FPM; manual exact-host DNS and separate trust; ownership/journal/privileges; optional process model; release boundary/budgets. Specifically acknowledge the visible high-port URLs and manual Windows setup. Select an open-source license before public distribution; final project name availability remains unresolved. These are proposals, not recorded acceptance.
