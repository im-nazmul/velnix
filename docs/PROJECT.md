# Velnix — Project Specification

## Vision
A lightweight, native Linux development environment manager for WSL2 first, native Linux later. Centralize control without replacing system package managers.

## Users
WSL2 Ubuntu developers working with PHP/Laravel/WordPress/Drupal, JavaScript/TypeScript, Python and static sites; future native Linux users.

## Functional requirements
### Runtimes
Discover, inspect, install, update, uninstall where supported; global defaults and per-project versions; detect conflicts and ownership.
### Projects
Link/unlink projects; park/unpark directories; detect frameworks; configure domains, document roots, runtime requirements and app servers.
### Web
Nginx initially; static hosting, FastCGI and reverse proxy; validate configs before reload; rollback owned config changes.
### HTTPS and DNS
`.test` domains, aliases, local certificates, WSL/Windows trust and DNS integration with explicit consent.
### Services
Inspect status, lifecycle and logs of supported system services; support PostgreSQL, MariaDB/MySQL and Redis.
### Mail
Local SMTP receiver with no external relay, HTML/plain text viewer, attachments and retention.
### Interfaces
Scriptable CLI, keyboard-friendly TUI and optional Vue web dashboard sharing one application layer.

## Nonfunctional requirements
Low idle overhead, fast CLI startup, minimal persistent processes, secure local defaults, reliable errors, tests, documented ownership and reversible changes where feasible.

## Non-goals
No container orchestration, production hosting panel, IDE, cloud deployment platform, or replacement for APT/systemd.

## Release discipline
Incremental milestones with tests, acceptance criteria and documented limitations. Do not promise support for every framework on day one.
