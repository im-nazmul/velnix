# Velnix — Development Roadmap

## Phase 0 — Architecture and Feasibility (current)
Research providers, PHP-FPM version selection, WSL DNS/HTTPS, privilege model, ownership and performance. Deliver feasibility/security/testing/implementation docs and ADRs. No app implementation until approved.

## Phase 1 — Core Foundation
Go module, CLI, platform detection, doctor, configuration, tests and CI.

## Phase 2 — Projects and Web
Project registry, link/park, Nginx detection and config validation, static and PHP HTTP routing.

## Phase 3 — HTTPS and Runtime Selection
Local CA, WSL trust, runtime discovery, version selection and PHP CLI/FPM integration.

## Phase 4 — Multi-language
Node, Bun, Python providers; app server proxying; framework detection.

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

[IMPLEMENTATION-PLAN](IMPLEMENTATION-PLAN.md) maps a restricted v0.1 across the foundation, project/HTTP, runtime/HTTPS and stabilization milestones. Full runtime installation, framework automation, TUI, mail and web remain later work. [TESTING](TESTING.md) defines measurable gates; budgets are not measured results. Each milestone still requires explicit approval. Phase 0 documentation is ready for review, not authorization to start Phase 1.
