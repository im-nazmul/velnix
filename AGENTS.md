# AGENTS.md — Velnix

## Project Identity
Velnix is an open-source, lightweight, native Linux development environment manager. WSL2/Ubuntu first, portable to native Linux. Not a container platform or PHP-only tool.

## Principles
- Lightweight, native and framework-agnostic.
- No mandatory containers or bundled language runtimes.
- Prefer existing package managers and runtime managers via adapters.
- Separate discovery, ownership and management.
- Never assume a detected installation belongs to Velnix.
- No silent system modifications; offer dry-run and explicit approval.
- Daemonless CLI by default; background processes only where needed.
- CLI, TUI and web share application logic.
- Keep privilege boundaries narrow; never run everything as root.
- Document reversible operations and rollback limitations.

## Technology Direction
Go core and CLI; Bubble Tea/Lip Gloss TUI; Vue 3/TypeScript/Vite dashboard compiled to static assets embedded in Go; Nginx first; systemd where available.

## Codex Workflow
1. Read docs/PROJECT.md, docs/ARCHITECTURE.md and docs/DECISIONS.md.
2. Review assumptions and platform dependencies before coding.
3. Propose significant architectural changes before implementation.
4. Make the smallest coherent change; test and document it.
5. Record major decisions in docs/decisions/.
6. Never implement the whole roadmap in one task.

## Current Phase
Phase 0: architecture and feasibility. Do not begin implementation until approved.
