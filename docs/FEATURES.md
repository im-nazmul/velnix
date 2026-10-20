# Velnix — Feature Catalogue

All items are planned or research items, not implemented.

## Diagnostics
Distribution/WSL detection, systemd and package manager detection, runtime/service discovery, port conflicts, certificate diagnostics. `velnix doctor`.

## Runtime management
PHP, Node.js, Bun and Python initially; discover/install/inspect/update/remove where provider supports; global and per-project versions. Future Ruby and Go.

## Framework detection
- PHP: Laravel, WordPress, Drupal, Symfony, CodeIgniter, generic PHP.
- JS/TS: Vue, React, Next.js, Nuxt, Svelte/SvelteKit, Astro, Vite, Express, generic Node/Bun.
- Python: Django, Flask, FastAPI, generic Python.
- Static HTML/CSS/JS and build outputs.

## Projects
Link/unlink, park/unpark, domain aliases, document roots, runtime config, health, development server proxying.

## Web and HTTPS
Nginx initial provider; static sites, PHP-FPM, reverse proxy, config validation and safe reload. Research `.test` DNS, Windows hosts/DNS, Linux and Windows certificate trust, local CA.

## Databases and cache
PostgreSQL, MariaDB/MySQL, Redis: detection, status, logs and lifecycle initially. Database creation, backup/import and multi-instance management later.

## Mail capture
Local SMTP listener, message list, HTML/plain preview, headers, attachments, search, retention and deletion. No outbound relay.

## Interfaces
CLI: structured output, shell completion, dry-run, noninteractive mode. TUI: overview/projects/runtimes/services/mail. Web: overview, projects, runtimes, services, certificates, mail, diagnostics, settings.

## Future
Other distributions, web servers, plugin SDK, templates, database tooling, import/export environment profiles.

## Release scope

This catalogue describes the eventual product. “Initially” identifies priority language families, not a promise that all provider mutations or interfaces ship in v0.1. The bounded proposed first release and exclusions are in [IMPLEMENTATION-PLAN](IMPLEMENTATION-PLAN.md).
