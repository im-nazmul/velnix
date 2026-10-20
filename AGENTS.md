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
Phase 4 multi-language work is authorized by “Lets start work with phase 4, no need to ask for my aproval. you can start implementation directly”. Existing Node/Bun/Python discovery, inspection, persistent selection/exec, native provider resolution, bounded framework hints and explicit loopback proxying are implemented; see docs/PHASE-4.md and docs/validation/PHASE-4-EXECUTION.md. The user subsequently requested “Now lets finish phase 4”. Optional mise/uv command adapters remain explicitly unavailable pending compatibility evidence; platform acceptance remains open. Do not implement the whole roadmap.

Phase 3: runtime and HTTPS, previously authorized by “Lets start work with phase 3” and
approval of docs/PHASE-3-PROPOSAL.md with “Approved, start implementation”.
Project registry transactions and the approved rootless static HTTP slice are implemented; see docs/PROJECTS.md, docs/HTTP.md and docs/validation/PHASE-2-HTTP.md.
The user approved docs/PHASE-2-HTTP-PROPOSAL.md with “You have my approval”. Local implementation validation passes; unperformed platform acceptance remains open.
Phase 3 PHP CLI, per-project FPM/web routing and optional HTTPS are implemented; see
docs/RUNTIMES.md, docs/FPM.md, docs/HTTPS.md and docs/validation/PHASE-3-WEB.md.
Real mkcert and platform/trust acceptance remain open within docs/PHASE-3-PROPOSAL.md. Propose significant departures
from that approved design before implementation. Phase 4 authorization does not imply approval for later roadmap phases or host system mutations.
