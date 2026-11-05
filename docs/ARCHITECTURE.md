# Velnix — Architecture

## Layers
```text
CLI ─────┐
TUI ─────┼──> Application services ──> Domain modules ──> Provider adapters ──> Native Linux
Web API ─┘
```

## Execution model
Prefer daemonless commands. Run optional background listeners only for mail capture, dashboard/API and app development servers. No mandatory permanent central daemon.

## Modules
- Project registry: paths, domains, metadata, ownership and detection.
- Runtime manager: discovery, version resolution and provider delegation.
- Service manager: native status, lifecycle and logs.
- Web manager: isolated Nginx config, validation and rollback.
- Certificate manager: local CA, issuance, trust and cleanup.
- Mail manager: local-only SMTP capture, persistence and viewer.
- Platform adapters: WSL detection, package managers, service managers, filesystem paths.

## Provider design
Expose capabilities rather than pretending every provider supports identical operations.

```go
type RuntimeProvider interface {
    Name() string
    Detect(ctx context.Context) ([]RuntimeInstallation, error)
    Capabilities(ctx context.Context) (RuntimeCapabilities, error)
}
```

## State
Follow XDG conventions (`~/.config/velnix`, `~/.local/share/velnix`, `~/.local/state/velnix`, `~/.cache/velnix`). Consider JSON/TOML for configuration and SQLite only if justified for registry, mail or operation history.

## Security
Run normal commands unprivileged. Restrict package installs, system service changes, Nginx writes and trust-store edits behind explicit authorization. Dashboard binds loopback and must protect state-changing endpoints.

## Portability
Do not assume APT, systemd, WSL networking or uniform filesystem paths. Discover capabilities and offer clear fallbacks.

## Open research
PHP CLI vs PHP-FPM version selection; external runtime managers; Windows/WSL DNS and HTTPS trust; safe Nginx ownership; daemonless operation; mail storage and security; rollback limitations.

## Phase 0 refinement

The detailed proposed package boundaries, state strategy and small interfaces are in [IMPLEMENTATION-PLAN](IMPLEMENTATION-PLAN.md); privilege and recovery requirements are in [SECURITY](SECURITY.md). [FEASIBILITY](FEASIBILITY.md) evaluates the open research above. The user authorized Phases 1–4; implemented boundaries and limits are in [FOUNDATION](FOUNDATION.md), [PROJECTS](PROJECTS.md), [HTTP](HTTP.md) and [RUNTIMES](RUNTIMES.md). The approved [Phase 3 proposal](PHASE-3-PROPOSAL.md) now has implemented [FPM](FPM.md) and [HTTPS](HTTPS.md) slices with [completion evidence and release boundaries](validation/PHASE-3-COMPLETION.md); the completed Phase 4 native execution, mise/uv store adapters, metadata and proxy scope is documented in [PHASE-4](PHASE-4.md), with [completion evidence](validation/PHASE-4-COMPLETION.md). Later milestones require separate approval.

The approved Phase 5 read-only terminal interface is complete: presentation lives
in `internal/transport/tui`, querying existing services through
`app.TerminalQuery` without a CLI subprocess or daemon. See
[TUI](TUI.md), [ADR 0014](decisions/0014-read-only-terminal-interface.md) and
[completion evidence](validation/PHASE-5-COMPLETION.md). Later milestones still
require separate approval.

The separately authorized Phase 5B extension adds `app.TerminalSession`: typed
plan/apply delegation and session-owned foreground workload cleanup. Views
remain read-only; mutations require exact backend plan confirmation. See
[ADR 0015](decisions/0015-reviewed-terminal-management.md). No CLI subprocess,
new daemon or registry schema was introduced.

The approved Phase 6 adds `app.MailService` over an external Mailpit provider,
private provider-owned storage and explicit foreground lifetimes. Typed API
methods restrict Velnix to passive raw-message inspection and reviewed exact-ID
pruning. A child-only private umask and pinned executable/data descriptors
protect captured mail; no new Velnix HTTP API or registry schema is introduced.
See [MAIL](MAIL.md) and [ADR 0016](decisions/0016-local-mail-capture.md).
