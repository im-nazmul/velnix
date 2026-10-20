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

The detailed proposed package boundaries, state strategy and small interfaces are in [IMPLEMENTATION-PLAN](IMPLEMENTATION-PLAN.md); privilege and recovery requirements are in [SECURITY](SECURITY.md). [FEASIBILITY](FEASIBILITY.md) evaluates the open research above. The user authorized the Phase 1 foundation after review; implemented boundaries and limits are in [FOUNDATION](FOUNDATION.md). Later milestones still require approval.
