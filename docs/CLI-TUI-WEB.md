# Velnix — CLI, TUI and Web Interfaces

## Principle
Three interfaces, one application layer. CLI must work without dashboard or background API.

## Proposed CLI tree
```text
velnix
├── doctor
├── version
├── install / uninstall / update
├── runtimes (list, inspect, use)
├── projects (list, inspect, remove)
├── link / unlink / park / unpark
├── secure / unsecure / certificates
├── services (list, status, start, stop, restart, logs)
├── mail (start, stop, inbox)
├── ui
└── config
```

CLI requirements: scriptability, useful exit codes, JSON where appropriate, dry-run, noninteractive mode, destructive confirmation and clear errors.

## TUI
Go Bubble Tea + Lip Gloss proposed. Screens: overview, projects, runtimes, services, mail and settings. Keyboard-first, searchable, compact, responsive terminal layout.

## Web
Vue 3/TypeScript/Vite proposed. Build static assets and embed in Go; no production Node server. Screens: overview, projects, runtimes, services, databases, certificates, mail, diagnostics and settings. Modern, responsive, accessible, minimal dependencies, dark/light themes.

## Security
Loopback binding by default, origin/CSRF protections, authentication as appropriate, no arbitrary shell execution through API, scoped privileged actions, no private-key exposure.

## Performance
Do not start dashboard unless requested; avoid constant polling, heavyweight frontend packages or duplicated application logic.
