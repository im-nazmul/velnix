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

The approved read-only Phase 5 interface is complete using Go Bubble Tea and
Lip Gloss. Run `velnix tui` for a welcoming home, searchable action picker and
overview/projects/runtimes/owned-services/diagnostics screens. See [usage](TUI.md)
and [completion evidence](validation/PHASE-5-COMPLETION.md). Mail, settings
mutations and system-service lifecycle remain later scope.

## Web

The approved Phase 7 dashboard is complete within its bounded scope. Run
`velnix ui --dry-run`, then approve the exact launch plan. Embedded Vue 3/
TypeScript/Vite assets require no production frontend server. Bun is pinned with
a committed lockfile, frozen CI installs and reproducible asset checks.
Seven pages share typed application services with a small reviewed action
allowlist. Plain-text mail viewing and safe downloads are included; HTML preview
uses the proposal's explicit unavailable fallback. See [DASHBOARD](DASHBOARD.md)
and [completion evidence](validation/PHASE-7-COMPLETION.md).

## Security

Loopback binding by default, origin/CSRF protections, authentication as appropriate, no arbitrary shell execution through API, scoped privileged actions, no private-key exposure.

## Performance

Do not start dashboard unless requested; avoid constant polling, heavyweight frontend packages or duplicated application logic.

## Phase 0 command semantics

The CLI tree is illustrative eventual scope. The proposal adds `exec -- command args` for selected child-process environments; it cannot alter its parent shell. Initial runtime operations are read-only. Mutations use previewed plans and state-bound explicit approval; web cannot elevate. See [IMPLEMENTATION-PLAN](IMPLEMENTATION-PLAN.md) for v0.1 and [SECURITY](SECURITY.md) for authorization/API requirements.
