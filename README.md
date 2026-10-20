# Velnix

**Native Linux development, without the overhead.**

> Status: planning and architecture. Name provisional pending availability checks.

Velnix is an open-source, lightweight local development environment manager for WSL2 and Linux. It aims to unify native services, runtime versions, project linking, `.test` domains, HTTPS and development mail capture without mandatory containers.

## Planned capabilities
- CLI, interactive TUI, optional web dashboard
- Global and per-project runtime versions
- PHP, Node.js, Bun, Python; extensible framework detection
- Laravel, WordPress, Drupal, JS frameworks, Python apps and static sites
- Nginx, local domains and HTTPS
- PostgreSQL, MariaDB/MySQL, Redis integration
- Local SMTP mail capture
- Safe system integration and ownership tracking

## Illustrative commands (not implemented)
```bash
velnix doctor
velnix install php@8.5
velnix link
velnix secure
velnix services
velnix mail
velnix ui
```

## Documentation
- [Project specification](docs/PROJECT.md)
- [Architecture](docs/ARCHITECTURE.md)
- [Feature catalogue](docs/FEATURES.md)
- [Runtime management](docs/RUNTIME-MANAGEMENT.md)
- [Interfaces](docs/CLI-TUI-WEB.md)
- [Roadmap](docs/ROADMAP.md)
- [Decisions](docs/DECISIONS.md)

## License
To be selected before public release.
