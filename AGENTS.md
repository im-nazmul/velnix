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

Bun is the default package manager and script runner for Velnix's JavaScript/TypeScript tooling. When dashboard tooling is introduced, commit `bun.lock`, pin the Bun version, and use `bun install --frozen-lockfile` in CI. This does not change runtime selection for user-managed projects. GitHub JavaScript actions use their own embedded Node runtime.

## Codex Workflow
1. Read docs/PROJECT.md, docs/ARCHITECTURE.md and docs/DECISIONS.md.
2. Review assumptions and platform dependencies before coding.
3. Propose significant architectural changes before implementation.
4. Make the smallest coherent change; test and document it.
5. Record major decisions in docs/decisions/.
6. Never implement the whole roadmap in one task.

## Current Phase
Phase 2: projects and HTTP, authorized by the user with “Lets start work with phase 2”.
Project registry transactions and the approved rootless static HTTP slice are implemented; see docs/PROJECTS.md, docs/HTTP.md and docs/validation/PHASE-2-HTTP.md.
The user approved docs/PHASE-2-HTTP-PROPOSAL.md with “You have my approval”. Local implementation validation passes; unperformed platform acceptance remains open.
Continue within the Phase 2 scope in docs/IMPLEMENTATION-PLAN.md. Propose further significant web/process changes before implementation.
Do not proceed to Phase 3 or implement later roadmap features without approval.
