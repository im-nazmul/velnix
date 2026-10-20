# Velnix

**Native Linux development, without the overhead.**

> Status: Phase 3 started with existing PHP CLI discovery, exact project selection and exec implemented. FPM/HTTPS and broader platform acceptance remain pending. Name provisional pending availability checks.

Velnix is an open-source, lightweight local development environment manager for WSL2 and Linux. It aims to unify native services, runtime versions, project linking, `.test` domains, HTTPS and development mail capture without mandatory containers.

## Getting started

Requires Go 1.27.3+ on Linux. Use a supported patched Go release for release builds.

```sh
go build -o bin/velnix ./cmd/velnix
bin/velnix doctor
bin/velnix config show
bin/velnix state init --dry-run
```

The foundation includes version, doctor, config show/validate and state status/init/recover. State creation requires an explicitly approved plan digest. Diagnostics execute no runtime/provider tools and create no files. See [foundation usage and recovery](docs/FOUNDATION.md).

Project list/inspect/link/unlink now use the same explicit approval and recovery model. Links store metadata and reserve a `.test` name. Reviewed `http start/reload/stop/recover` commands serve static projects through a separate foreground Nginx instance on loopback port 8080. See [HTTP usage and recovery](docs/HTTP.md). See [project commands](docs/PROJECTS.md).

Existing native PHP CLI binaries can be inspected and selected through reviewed
plans, resolved without execution, then explicitly invoked with
`velnix exec --name NAME -- PHP_ARGUMENTS...`. See [runtime usage](docs/RUNTIMES.md).

## Planned capabilities
- CLI, interactive TUI, optional web dashboard
- Global and per-project runtime versions
- PHP, Node.js, Bun, Python; extensible framework detection
- Laravel, WordPress, Drupal, JS frameworks, Python apps and static sites
- Nginx, local domains and HTTPS
- PostgreSQL, MariaDB/MySQL, Redis integration
- Local SMTP mail capture
- Safe system integration and ownership tracking

## Future commands (not implemented)
```bash
velnix install php@8.5
velnix link
velnix secure
velnix services
velnix mail
velnix ui
```

## Documentation
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
