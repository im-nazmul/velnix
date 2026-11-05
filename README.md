# Velnix

**Native Linux development, without the overhead.**

> Status: Phases 3 and 4 are complete within their approved implementation scopes. [Phase 3 evidence](docs/validation/PHASE-3-COMPLETION.md) includes real PHP/FPM, mkcert and scoped Linux/Windows mirrored HTTPS; [Phase 4 evidence](docs/validation/PHASE-4-COMPLETION.md) covers native execution, read-only provider stores, detection and proxying. Remaining [platform/release acceptance](docs/validation/RELEASE-ACCEPTANCE.md) is explicit. The [Phase 5 TUI](docs/TUI.md) includes the completed read-only scope and the completed [Phase 5B management extension](docs/PHASE-5B.md); [completion evidence](docs/validation/PHASE-5-COMPLETION.md) records its terminal and platform limits. The completed [Phase 6 mail CLI](docs/MAIL.md) includes capture, passive inspection, reviewed export/pruning and recovery; [local evidence](docs/validation/PHASE-6-COMPLETION.md) records provider and platform limits. Name provisional pending availability checks.

Velnix is an open-source, lightweight local development environment manager for WSL2 and Linux. It aims to unify native services, runtime versions, project linking, `.test` domains, HTTPS and development mail capture without mandatory containers.

## Getting started

Requires Go 1.27.3+ on Linux. Use a supported patched Go release for release builds.

```sh
go build -o bin/velnix ./cmd/velnix
bin/velnix doctor
bin/velnix tui # optional interactive read-only interface
bin/velnix config show
bin/velnix state init --dry-run
```

The foundation includes version, doctor, config show/validate and state status/init/recover. State creation requires an explicitly approved plan digest. Diagnostics execute no runtime/provider tools and create no files. See [foundation usage and recovery](docs/FOUNDATION.md).

Project list/inspect/link/unlink now use the same explicit approval and recovery model. Links store metadata and reserve a `.test` name. Reviewed `http start/reload/stop/recover` commands serve static projects through a separate foreground Nginx instance on loopback port 8080. See [HTTP usage and recovery](docs/HTTP.md). See [project commands](docs/PROJECTS.md).

Existing native PHP CLI binaries can be inspected and selected through reviewed
plans, resolved without execution, then explicitly invoked with
`velnix exec --name NAME -- PHP_ARGUMENTS...`. See [runtime usage](docs/RUNTIMES.md).

Optional foreground `mail serve` captures local development mail through an existing
Mailpit v1.33.0. Inspect plain text and export attachments through the CLI using
reviewed plans; see [mail usage and recovery](docs/MAIL.md).

## Planned capabilities
- CLI, interactive TUI, optional web dashboard
- Global and per-project runtime versions
- PHP, Node.js, Bun, Python; extensible framework detection
- Laravel, WordPress, Drupal, JS frameworks, Python apps and static sites
- Nginx, local domains and HTTPS
- PostgreSQL, MariaDB/MySQL, Redis integration
- Safe system integration and ownership tracking

## Future commands (not implemented)
```bash
velnix install php@8.5
velnix link
velnix secure
velnix services
velnix ui
```

## Documentation
- [Local mail capture and recovery](docs/MAIL.md)
- [Foundation usage and recovery](docs/FOUNDATION.md)
- [Project registry commands and recovery](docs/PROJECTS.md)
- [Existing PHP CLI selection and exec](docs/RUNTIMES.md)
- [Project specification](docs/PROJECT.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Feature catalogue](docs/FEATURES.md)
- [Runtime management](docs/RUNTIME-MANAGEMENT.md)
- [Interfaces](docs/CLI-TUI-WEB.md)
- [Roadmap](docs/ROADMAP.md)
- [Decisions](docs/DECISIONS.md)
- [Phase 0 feasibility and specification review](docs/FEASIBILITY.md)
- [Security, ownership and recovery](docs/SECURITY.md)
- [Proposed v0.1 implementation plan](docs/IMPLEMENTATION-PLAN.md)
- [Testing and performance acceptance](docs/TESTING.md)

## License
To be selected before public release.
