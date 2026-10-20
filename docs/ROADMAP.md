# Velnix — Development Roadmap

## Phase 0 — Architecture and Feasibility (review delivered)
Research providers, PHP-FPM version selection, WSL DNS/HTTPS, privilege model, ownership and performance. Deliver feasibility/security/testing/implementation docs and ADRs. No app implementation until approved.

## Phase 1 — Core Foundation (implemented; platform validation limits documented)
Go module, CLI, platform detection, doctor, configuration, tests and CI.

## Phase 2 — Projects and Web (authorized; project registry slice implemented)
Project registry, link/unlink, rootless Nginx, static HTTP and explicit process identity. See [PROJECTS](PROJECTS.md) for the implemented commands. HTTP remains pending. The bounded release plan defers park scanning and places PHP integration in Phase 3.

## Phase 3 — HTTPS and Runtime Selection
Local CA, WSL trust, runtime discovery, version selection and PHP CLI/FPM integration.

## Phase 4 — Multi-language (authorized; discovery slice implemented)
Node, Bun, Python providers; app server proxying; framework detection. The first read-only candidate discovery slice is implemented; see [PHASE-4](PHASE-4.md). Proxying, framework detection and runtime selection remain pending.

## Phase 5 — TUI
Overview, projects, runtimes, services and diagnostics.

## Phase 6 — Mail Capture
Local SMTP, persistence, message/attachment preview and retention.

## Phase 7 — Web Dashboard
Embedded Vue UI, loopback API, management pages and mail viewer.

## Phase 8 — Stabilization
Security/performance review, WSL/native Ubuntu testing, installers, docs, contributor guidance and release automation.

## Future
More distributions, web servers, plugins, database tools and runtimes.

Each phase needs acceptance criteria, tests, docs and known limitations. Do not proceed automatically.

## Proposed first release and approval gates

[IMPLEMENTATION-PLAN](IMPLEMENTATION-PLAN.md) maps a restricted v0.1 across the foundation, project/HTTP, runtime/HTTPS and stabilization milestones. Full runtime installation, framework automation, TUI, mail and web remain later work. [TESTING](TESTING.md) defines measurable gates; budgets are not measured results. Each milestone still requires explicit approval. Phase 0 documentation alone did not authorize implementation; subsequent user authorization starts Phase 1 only.

Phase 1 authorization: user instruction “Lets start implementation”. Implemented surface and limitations are documented in [FOUNDATION](FOUNDATION.md). Phase 2 authorization: “Lets start work with phase 2”. The first registry slice is documented in [PROJECTS](PROJECTS.md). Phase 3 and later approval gates remain in effect.

Phase 4 authorization: “Lets start work with phase 4, no need to ask for my aproval. you can start implementation directly”. This does not mark unfinished Phase 3 work or platform acceptance complete.
